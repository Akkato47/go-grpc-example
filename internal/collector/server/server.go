package collector_server

import (
	"io"
	"log/slog"
	"time"

	metricspb "github.com/Akkato47/go-grpc-example/gen/metrics"
	collector_service "github.com/Akkato47/go-grpc-example/internal/collector/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	metricspb.UnimplementedMetricsServiceServer
	svc    *collector_service.Service
	logger *slog.Logger
}

func New(svc *collector_service.Service, logger *slog.Logger) *Server {
	return &Server{svc: svc, logger: logger}
}

func (s *Server) SendMetrics(stream metricspb.MetricsService_SendMetricsServer) error {
	var totalReceived, totalProcessed int64

	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			s.logger.Error("recv error on SendMetrics stream", slog.String("error", err.Error()))
			return err
		}

		processed, err := s.svc.ProcessBatch(stream.Context(), batch)
		if err != nil {
			s.logger.Error("process batch error", slog.String("error", err.Error()))
			return err
		}

		totalReceived += int64(len(batch.Metrics))
		totalProcessed += processed

		if err := stream.Send(&metricspb.SendAck{
			Received:  totalReceived,
			Processed: totalProcessed,
			Status:    "ok",
			AckedAt:   timestamppb.New(time.Now()),
		}); err != nil {
			s.logger.Error("send ack error", slog.String("error", err.Error()))
			return err
		}

		s.logger.Info("batch processed",
			slog.String("source", batch.Source),
			slog.Int("metrics", len(batch.Metrics)),
			slog.Int64("total_received", totalReceived),
		)
	}
}
