package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	aggregatorpb "github.com/Akkato47/go-grpc-example/gen/aggregator"
	metricspb "github.com/Akkato47/go-grpc-example/gen/metrics"
	"github.com/Akkato47/go-grpc-example/internal/core/config"
	"github.com/Akkato47/go-grpc-example/internal/generator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.New()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	interval, err := time.ParseDuration(cfg.GeneratorInterval)
	if err != nil || interval == 0 {
		interval = 2 * time.Second
	}

	collConn, err := grpc.NewClient(
		cfg.CollectorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to collector", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer collConn.Close()

	aggConn, err := grpc.NewClient(
		cfg.AggregatorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to aggregator", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer aggConn.Close()

	collClient := metricspb.NewMetricsServiceClient(collConn)
	aggClient := aggregatorpb.NewAggregatorServiceClient(aggConn)

	go streamToCollector(ctx, collClient, interval, cfg.GeneratorBatchSize, logger)
	go watchAggregator(ctx, aggClient, logger)
	go pollSummaries(ctx, aggClient, logger)

	<-ctx.Done()
	logger.Info("generator shutting down")
}

func streamToCollector(
	ctx context.Context,
	client metricspb.MetricsServiceClient,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
) {
	for {
		if err := runSendStream(ctx, client, interval, batchSize, logger); err != nil {
			logger.Error("collector stream ended with error", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func runSendStream(
	ctx context.Context,
	client metricspb.MetricsServiceClient,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
) error {
	stream, err := client.SendMetrics(ctx)
	if err != nil {
		return err
	}

	ackDone := make(chan struct{})
	go func() {
		defer close(ackDone)
		for {
			ack, err := stream.Recv()
			if err == io.EOF || err != nil {
				return
			}
			logger.Info("ack from collector",
				slog.Int64("total_received", ack.Received),
				slog.Int64("total_processed", ack.Processed),
			)
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = stream.CloseSend()
			<-ackDone
			return nil
		case <-ticker.C:
			batch := generator.GenerateBatch("generator-1", batchSize)
			if err := stream.Send(batch); err != nil {
				<-ackDone
				return err
			}
			logger.Info("sent batch",
				slog.Int("metrics", len(batch.Metrics)),
				slog.String("source", batch.Source),
			)
		}
	}
}

func watchAggregator(ctx context.Context, client aggregatorpb.AggregatorServiceClient, logger *slog.Logger) {
	for {
		if err := runWatch(ctx, client, logger); err != nil {
			logger.Error("watch stream ended with error", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func runWatch(ctx context.Context, client aggregatorpb.AggregatorServiceClient, logger *slog.Logger) error {
	stream, err := client.WatchMetrics(ctx, &aggregatorpb.WatchRequest{
		MetricNames: []string{
			"cpu_usage_percent",
			"memory_usage_bytes",
			"http_requests_total",
			"http_request_duration_ms",
		},
		IntervalMs: 1000,
	})
	if err != nil {
		return err
	}

	for {
		update, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		logger.Info("metric update",
			slog.String("name", update.MetricName),
			slog.Float64("value", update.Value),
			slog.Any("labels", update.Labels),
		)
	}
}

func pollSummaries(ctx context.Context, client aggregatorpb.AggregatorServiceClient, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	names := []string{
		"cpu_usage_percent",
		"memory_usage_bytes",
		"http_requests_total",
		"http_request_duration_ms",
		"error_rate",
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, name := range names {
				summary, err := client.GetSummary(ctx, &aggregatorpb.SummaryRequest{
					MetricName: name,
				})
				if err != nil {
					logger.Error("get summary failed",
						slog.String("metric", name),
						slog.String("error", err.Error()),
					)
					continue
				}
				logger.Info("summary",
					slog.String("metric", summary.MetricName),
					slog.Int64("count", summary.Count),
					slog.Float64("min", summary.Min),
					slog.Float64("max", summary.Max),
					slog.Float64("avg", summary.Avg),
					slog.Float64("p95", summary.P95),
					slog.Float64("p99", summary.P99),
				)
			}
		}
	}
}
