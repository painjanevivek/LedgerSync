package observability

import (
	"context"
	"sync/atomic"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
)

// TransferMetrics is an in-process, dependency-free adapter for the financial
// command's core counters. A future Prometheus/OpenTelemetry exporter can read
// Snapshot without changing the application service.
type TransferMetrics struct {
	posted           atomic.Uint64
	rejected         atomic.Uint64
	idempotentReplay atomic.Uint64
	failures         atomic.Uint64
	telemetry        *Telemetry
}

type TransferMetricSnapshot struct {
	Posted           uint64
	Rejected         uint64
	IdempotentReplay uint64
	Failures         uint64
}

func NewTransferMetrics(telemetry *Telemetry) *TransferMetrics {
	return &TransferMetrics{telemetry: telemetry}
}

func (m *TransferMetrics) ObserveSubmission(result transfers.Result, replayed bool) {
	if m == nil {
		return
	}
	if replayed {
		m.idempotentReplay.Add(1)
		m.observe(result.Status, true)
		return
	}
	switch result.Status {
	case "posted":
		m.posted.Add(1)
	case "rejected":
		m.rejected.Add(1)
	}
	m.observe(result.Status, false)
}

func (m *TransferMetrics) observe(status string, replayed bool) {
	if m.telemetry != nil {
		m.telemetry.ObserveTransfer(context.Background(), status, replayed)
	}
}

func (m *TransferMetrics) ObserveFailure(error) {
	if m != nil {
		m.failures.Add(1)
	}
}

func (m *TransferMetrics) Snapshot() TransferMetricSnapshot {
	if m == nil {
		return TransferMetricSnapshot{}
	}
	return TransferMetricSnapshot{
		Posted:           m.posted.Load(),
		Rejected:         m.rejected.Load(),
		IdempotentReplay: m.idempotentReplay.Load(),
		Failures:         m.failures.Load(),
	}
}
