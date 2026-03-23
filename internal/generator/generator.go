package generator

import (
	"math/rand"
	"time"

	metricspb "github.com/Akkato47/go-grpc-example/gen/metrics"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type metricDef struct {
	name       string
	metricType metricspb.MetricType
	minVal     float64
	maxVal     float64
	labels     map[string]string
}

var definitions = []metricDef{
	{"cpu_usage_percent", metricspb.MetricType_GAUGE, 0, 100, map[string]string{"host": "node-1"}},
	{"cpu_usage_percent", metricspb.MetricType_GAUGE, 0, 100, map[string]string{"host": "node-2"}},
	{"memory_usage_bytes", metricspb.MetricType_GAUGE, 1e8, 8e9, map[string]string{"host": "node-1"}},
	{"memory_usage_bytes", metricspb.MetricType_GAUGE, 1e8, 8e9, map[string]string{"host": "node-2"}},
	{"http_requests_total", metricspb.MetricType_COUNTER, 0, 100, map[string]string{"method": "GET", "status": "200"}},
	{"http_requests_total", metricspb.MetricType_COUNTER, 0, 10, map[string]string{"method": "POST", "status": "201"}},
	{"http_requests_total", metricspb.MetricType_COUNTER, 0, 5, map[string]string{"method": "GET", "status": "500"}},
	{"http_request_duration_ms", metricspb.MetricType_HISTOGRAM, 1, 500, map[string]string{"method": "GET"}},
	{"http_request_duration_ms", metricspb.MetricType_HISTOGRAM, 5, 1000, map[string]string{"method": "POST"}},
	{"disk_io_bytes", metricspb.MetricType_COUNTER, 0, 1e7, map[string]string{"device": "sda", "op": "read"}},
	{"disk_io_bytes", metricspb.MetricType_COUNTER, 0, 5e6, map[string]string{"device": "sda", "op": "write"}},
	{"active_connections", metricspb.MetricType_GAUGE, 0, 1000, map[string]string{"service": "api"}},
	{"error_rate", metricspb.MetricType_GAUGE, 0, 0.05, map[string]string{"service": "api"}},
	{"gc_pause_ms", metricspb.MetricType_HISTOGRAM, 0, 50, map[string]string{}},
	{"goroutines_total", metricspb.MetricType_GAUGE, 10, 500, map[string]string{}},
}

func GenerateBatch(source string, batchSize int) *metricspb.MetricBatch {
	now := time.Now()
	metrics := make([]*metricspb.Metric, 0, batchSize)

	for i := 0; i < batchSize; i++ {
		def := definitions[rand.Intn(len(definitions))]
		value := def.minVal + rand.Float64()*(def.maxVal-def.minVal)

		labels := make(map[string]string, len(def.labels))
		for k, v := range def.labels {
			labels[k] = v
		}

		metrics = append(metrics, &metricspb.Metric{
			Name:      def.name,
			Value:     value,
			Labels:    labels,
			Timestamp: timestamppb.New(now),
			Type:      def.metricType,
		})
	}

	return &metricspb.MetricBatch{
		Source:  source,
		Metrics: metrics,
		SentAt:  timestamppb.New(now),
	}
}
