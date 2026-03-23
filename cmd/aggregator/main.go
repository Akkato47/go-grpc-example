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
	aggregator_server "github.com/Akkato47/go-grpc-example/internal/aggregator/server"
	aggregator_service "github.com/Akkato47/go-grpc-example/internal/aggregator/service"
	"github.com/Akkato47/go-grpc-example/internal/core/config"
	"github.com/Akkato47/go-grpc-example/pkg/interceptors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.New()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	svc := aggregator_service.New(reg, logger)
	srv := aggregator_server.New(svc, logger)

	mi := interceptors.NewMetricsInterceptor(reg, "aggregator")

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
	aggregatorpb.RegisterAggregatorServiceServer(grpcServer, srv)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		addr := fmt.Sprintf(":%s", cfg.AggregatorMetricsPort)
		logger.Info("aggregator metrics server listening", slog.String("addr", addr))
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Error("metrics server error", slog.String("error", err.Error()))
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.AggregatorGRPCPort))
	if err != nil {
		logger.Error("failed to listen", slog.String("error", err.Error()))
		os.Exit(1)
	}

	go func() {
		logger.Info("aggregator gRPC server listening", slog.String("port", cfg.AggregatorGRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("aggregator gRPC serve error", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	logger.Info("aggregator shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		logger.Info("aggregator stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("aggregator shutdown timed out, forcing stop")
		grpcServer.Stop()
	}
}
