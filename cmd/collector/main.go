package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	aggregatorpb "github.com/Akkato47/go-grpc-example/gen/aggregator"
	metricspb "github.com/Akkato47/go-grpc-example/gen/metrics"
	collector_server "github.com/Akkato47/go-grpc-example/internal/collector/server"
	collector_service "github.com/Akkato47/go-grpc-example/internal/collector/service"
	"github.com/Akkato47/go-grpc-example/internal/core/config"
	"github.com/Akkato47/go-grpc-example/pkg/interceptors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.New()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	aggConn, err := grpc.NewClient(
		cfg.AggregatorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to aggregator", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer aggConn.Close()

	aggClient := aggregatorpb.NewAggregatorServiceClient(aggConn)

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	svc := collector_service.New(aggClient, reg, logger)
	srv := collector_server.New(svc, logger)

	mi := interceptors.NewMetricsInterceptor(reg, "collector")

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryLogging(logger),
			mi.Unary(),
		),
		grpc.ChainStreamInterceptor(
			interceptors.StreamLogging(logger),
			mi.Stream(),
		),
	)
	metricspb.RegisterMetricsServiceServer(grpcServer, srv)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		addr := fmt.Sprintf(":%s", cfg.CollectorMetricsPort)
		logger.Info("collector metrics server listening", slog.String("addr", addr))
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Error("metrics server error", slog.String("error", err.Error()))
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.CollectorGRPCPort))
	if err != nil {
		logger.Error("failed to listen", slog.String("error", err.Error()))
		os.Exit(1)
	}

	go func() {
		logger.Info("collector gRPC server listening", slog.String("port", cfg.CollectorGRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("collector gRPC serve error", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	logger.Info("collector shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		logger.Info("collector stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("collector shutdown timed out, forcing stop")
		grpcServer.Stop()
	}
}
