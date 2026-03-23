package aggregator_server

import (
	"context"
	"io"
	"log/slog"
	"time"

	aggregatorpb "github.com/Akkato47/go-grpc-example/gen/aggregator"
	aggregator_service "github.com/Akkato47/go-grpc-example/internal/aggregator/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	aggregatorpb.UnimplementedAggregatorServiceServer
	svc    *aggregator_service.Service
	logger *slog.Logger
}

func New(svc *aggregator_service.Service, logger *slog.Logger) *Server {
	return &Server{svc: svc, logger: logger}
}

func (s *Server) GetSummary(ctx context.Context, req *aggregatorpb.SummaryRequest) (*aggregatorpb.Summary, error) {
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()
	if req.From != nil {
		from = req.From.AsTime()
	}
	if req.To != nil {
		to = req.To.AsTime()
	}

	min, max, avg, p95, p99, count := s.svc.GetSummary(req.MetricName, from, to)

	s.logger.Info("summary computed",
		slog.String("metric", req.MetricName),
		slog.Int64("count", count),
		slog.Float64("avg", avg),
	)

	return &aggregatorpb.Summary{
		MetricName: req.MetricName,
		Min:        min,
		Max:        max,
		Avg:        avg,
		P95:        p95,
		P99:        p99,
		Count:      count,
		ComputedAt: timestamppb.Now(),
	}, nil
}

func (s *Server) WatchMetrics(req *aggregatorpb.WatchRequest, stream aggregatorpb.AggregatorService_WatchMetricsServer) error {
	sub, cancel := s.svc.Subscribe(req.MetricNames)
	defer cancel()

	s.logger.Info("watch started", slog.Any("metric_names", req.MetricNames))

	for {
		select {
		case <-stream.Context().Done():
			s.logger.Info("watch cancelled by client")
			return nil
		case m := <-sub.Chan():
			if err := stream.Send(&aggregatorpb.MetricUpdate{
				MetricName: m.Name,
				Value:      m.Value,
				Timestamp:  timestamppb.New(m.Timestamp),
				Labels:     m.Labels,
			}); err != nil {
				s.logger.Error("watch send error", slog.String("error", err.Error()))
				return err
			}
		}
	}
}

func (s *Server) PushMetrics(stream aggregatorpb.AggregatorService_PushMetricsServer) error {
	batch := make([]aggregator_service.Metric, 0, 64)

	for {
		m, err := stream.Recv()
		if err == io.EOF {
			s.svc.Push(batch)
			return stream.SendAndClose(&aggregatorpb.PushResponse{
				Accepted: int64(len(batch)),
			})
		}
		if err != nil {
			return err
		}

		ts := time.Now()
		if m.Timestamp != nil {
			ts = m.Timestamp.AsTime()
		}
		labels := make(map[string]string, len(m.Labels))
		for k, v := range m.Labels {
			labels[k] = v
		}

		batch = append(batch, aggregator_service.Metric{
			Name:      m.Name,
			Value:     m.Value,
			Labels:    labels,
			Timestamp: ts,
		})
	}
}
