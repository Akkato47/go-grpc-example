package interceptors

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MetricsInterceptor struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	activeStreams    *prometheus.GaugeVec
}

func NewMetricsInterceptor(reg prometheus.Registerer, service string) *MetricsInterceptor {
	constLabels := prometheus.Labels{"service": service}

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "grpc_requests_total",
			Help:        "Total number of gRPC requests by method and status.",
			ConstLabels: constLabels,
		},
		[]string{"method", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "grpc_request_duration_seconds",
			Help:        "Duration of gRPC requests in seconds.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: constLabels,
		},
		[]string{"method"},
	)
	activeStreams := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "grpc_active_streams",
			Help:        "Number of currently active gRPC streaming RPCs.",
			ConstLabels: constLabels,
		},
		[]string{"method"},
	)

	reg.MustRegister(requestsTotal, requestDuration, activeStreams)

	return &MetricsInterceptor{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
		activeStreams:    activeStreams,
	}
}

func (m *MetricsInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		m.requestsTotal.WithLabelValues(info.FullMethod, code.String()).Inc()
		m.requestDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

func (m *MetricsInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		m.activeStreams.WithLabelValues(info.FullMethod).Inc()
		defer m.activeStreams.WithLabelValues(info.FullMethod).Dec()

		start := time.Now()
		err := handler(srv, ss)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		m.requestsTotal.WithLabelValues(info.FullMethod, code.String()).Inc()
		m.requestDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return err
	}
}
