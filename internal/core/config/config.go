package config

import "github.com/Akkato47/go-grpc-example/internal/core/common/env"

type Config struct {
	CollectorGRPCPort  string
	CollectorMetricsPort string
	AggregatorGRPCPort string
	AggregatorMetricsPort string
	CollectorAddr      string
	AggregatorAddr     string
	GeneratorInterval  string
	GeneratorBatchSize int
}

func New() *Config {
	return &Config{
		CollectorGRPCPort:     env.GetString("COLLECTOR_GRPC_PORT", "50051"),
		CollectorMetricsPort:  env.GetString("COLLECTOR_METRICS_PORT", "9091"),
		AggregatorGRPCPort:    env.GetString("AGGREGATOR_GRPC_PORT", "50052"),
		AggregatorMetricsPort: env.GetString("AGGREGATOR_METRICS_PORT", "9092"),
		CollectorAddr:         env.GetString("COLLECTOR_ADDR", "localhost:50051"),
		AggregatorAddr:        env.GetString("AGGREGATOR_ADDR", "localhost:50052"),
		GeneratorInterval:     env.GetString("GENERATOR_INTERVAL", "2s"),
		GeneratorBatchSize:    10,
	}
}
