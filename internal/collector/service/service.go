package collector_service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	aggregatorpb "github.com/Akkato47/go-grpc-example/gen/aggregator"
	metricspb "github.com/Akkato47/go-grpc-example/gen/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxStoredMetrics = 10_000

type Metric struct {
	Name      string
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
	Source    string
}

type Service struct {
	mu               sync.RWMutex
	metrics          []Metric
	aggClient        aggregatorpb.AggregatorServiceClient
	receivedTotal    prometheus.Counter
	batchesProcessed prometheus.Counter
	logger           *slog.Logger
}

func New(
	aggClient aggregatorpb.AggregatorServiceClient,
	reg prometheus.Registerer,
	logger *slog.Logger,
) *Service {
	receivedTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "metrics_received_total",
		Help: "Total number of individual metrics received by the collector.",
	})
	batchesProcessed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "collector_batches_processed_total",
		Help: "Total number of metric batches processed by the collector.",
	})
	reg.MustRegister(receivedTotal, batchesProcessed)

	return &Service{
		metrics:          make([]Metric, 0, 1024),
		aggClient:        aggClient,
		receivedTotal:    receivedTotal,
		batchesProcessed: batchesProcessed,
		logger:           logger,
	}
}

func (s *Service) ProcessBatch(ctx context.Context, batch *metricspb.MetricBatch) (int64, error) {
	incoming := make([]Metric, 0, len(batch.Metrics))
	for _, m := range batch.Metrics {
		ts := time.Now()
		if m.Timestamp != nil {
			ts = m.Timestamp.AsTime()
		}
		labels := make(map[string]string, len(m.Labels))
		for k, v := range m.Labels {
			labels[k] = v
		}
		incoming = append(incoming, Metric{
			Name:      m.Name,
			Value:     m.Value,
			Labels:    labels,
			Timestamp: ts,
			Source:    batch.Source,
		})
	}

	s.mu.Lock()
	s.metrics = append(s.metrics, incoming...)
	if len(s.metrics) > maxStoredMetrics {
		s.metrics = s.metrics[len(s.metrics)-maxStoredMetrics:]
	}
	s.mu.Unlock()

	s.receivedTotal.Add(float64(len(incoming)))
	s.batchesProcessed.Inc()

	go s.pushToAggregator(context.Background(), incoming)

	return int64(len(incoming)), nil
}

func (s *Service) pushToAggregator(ctx context.Context, metrics []Metric) {
	stream, err := s.aggClient.PushMetrics(ctx)
	if err != nil {
		s.logger.Error("failed to open push stream to aggregator", slog.String("error", err.Error()))
		return
	}

	for _, m := range metrics {
		labels := make(map[string]string, len(m.Labels)+1)
		for k, v := range m.Labels {
			labels[k] = v
		}
		labels["source"] = m.Source

		if err := stream.Send(&aggregatorpb.Metric{
			Name:      m.Name,
			Value:     m.Value,
			Labels:    labels,
			Timestamp: timestamppb.New(m.Timestamp),
		}); err != nil {
			s.logger.Error("failed to send metric to aggregator", slog.String("error", err.Error()))
			return
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		s.logger.Error("aggregator push stream close error", slog.String("error", err.Error()))
	}
}
