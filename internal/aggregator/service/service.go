package aggregator_service

import (
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const maxStoredMetrics = 50_000

type Metric struct {
	Name      string
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

type subscriber struct {
	names map[string]struct{}
	ch    chan Metric
}

type Service struct {
	mu          sync.RWMutex
	metrics     []Metric
	subMu       sync.RWMutex
	subscribers []*subscriber

	aggregationsTotal prometheus.Counter
	activeWatchers    prometheus.Gauge
	logger            *slog.Logger
}

func New(reg prometheus.Registerer, logger *slog.Logger) *Service {
	aggregationsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aggregations_computed_total",
		Help: "Total number of GetSummary aggregations computed.",
	})
	activeWatchers := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aggregator_active_watchers",
		Help: "Current number of active WatchMetrics subscribers.",
	})
	reg.MustRegister(aggregationsTotal, activeWatchers)

	return &Service{
		metrics:           make([]Metric, 0, 1024),
		subscribers:       make([]*subscriber, 0),
		aggregationsTotal: aggregationsTotal,
		activeWatchers:    activeWatchers,
		logger:            logger,
	}
}

func (s *Service) Push(metrics []Metric) {
	s.mu.Lock()
	s.metrics = append(s.metrics, metrics...)
	if len(s.metrics) > maxStoredMetrics {
		s.metrics = s.metrics[len(s.metrics)-maxStoredMetrics:]
	}
	s.mu.Unlock()

	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, sub := range s.subscribers {
		for _, m := range metrics {
			if len(sub.names) == 0 {
				select {
				case sub.ch <- m:
				default:
				}
				continue
			}
			if _, ok := sub.names[m.Name]; ok {
				select {
				case sub.ch <- m:
				default:
				}
			}
		}
	}
}

func (s *Service) GetSummary(name string, from, to time.Time) (min, max, avg, p95, p99 float64, count int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	values := make([]float64, 0)
	for _, m := range s.metrics {
		if m.Name == name && !m.Timestamp.Before(from) && !m.Timestamp.After(to) {
			values = append(values, m.Value)
		}
	}

	if len(values) == 0 {
		return 0, 0, 0, 0, 0, 0
	}

	sort.Float64s(values)

	var sum float64
	for _, v := range values {
		sum += v
	}

	s.aggregationsTotal.Inc()

	return values[0],
		values[len(values)-1],
		sum / float64(len(values)),
		percentile(values, 95),
		percentile(values, 99),
		int64(len(values))
}

func (s *Service) Subscribe(names []string) (*subscriber, func()) {
	sub := &subscriber{
		names: make(map[string]struct{}, len(names)),
		ch:    make(chan Metric, 256),
	}
	for _, n := range names {
		sub.names[n] = struct{}{}
	}

	s.subMu.Lock()
	s.subscribers = append(s.subscribers, sub)
	s.subMu.Unlock()
	s.activeWatchers.Inc()

	cancel := func() {
		s.subMu.Lock()
		defer s.subMu.Unlock()
		for i, existing := range s.subscribers {
			if existing == sub {
				s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
				break
			}
		}
		s.activeWatchers.Dec()
	}
	return sub, cancel
}

func (sub *subscriber) Chan() <-chan Metric {
	return sub.ch
}

func percentile(sorted []float64, p float64) float64 {
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
