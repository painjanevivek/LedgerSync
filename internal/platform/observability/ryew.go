package observability

import (
	"sync/atomic"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
)

// RYEWMetrics is deliberately dependency-free. It exposes the signal names
// needed for the Phase 6 OpenTelemetry exporter without coupling financial
// reads or the worker to a metrics vendor.
type RYEWMetrics struct{ published, retries, dead, cacheHits, primaryFallbacks, unsatisfied atomic.Uint64 }
type RYEWMetricSnapshot struct{ Published, Retries, Dead, CacheHits, PrimaryFallbacks, Unsatisfied uint64 }

func (m *RYEWMetrics) ObservePublished(outbox.Event) {
	if m != nil {
		m.published.Add(1)
	}
}
func (m *RYEWMetrics) ObserveRetry(outbox.Event, error) {
	if m != nil {
		m.retries.Add(1)
	}
}
func (m *RYEWMetrics) ObserveDead(outbox.Event, error) {
	if m != nil {
		m.dead.Add(1)
	}
}
func (m *RYEWMetrics) ObserveCacheHit() {
	if m != nil {
		m.cacheHits.Add(1)
	}
}
func (m *RYEWMetrics) ObservePrimaryFallback() {
	if m != nil {
		m.primaryFallbacks.Add(1)
	}
}
func (m *RYEWMetrics) ObserveUnsatisfiedRequirement() {
	if m != nil {
		m.unsatisfied.Add(1)
	}
}
func (m *RYEWMetrics) Snapshot() RYEWMetricSnapshot {
	if m == nil {
		return RYEWMetricSnapshot{}
	}
	return RYEWMetricSnapshot{m.published.Load(), m.retries.Load(), m.dead.Load(), m.cacheHits.Load(), m.primaryFallbacks.Load(), m.unsatisfied.Load()}
}
