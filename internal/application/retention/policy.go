// Package retention defines bounded cleanup policy for disposable operational
// data. Immutable financial and evidence records are intentionally absent.
package retention

import "time"

type Policy struct {
	TenantID             string
	BatchSize            int
	PublishedOutboxAfter time.Duration
	RateWindowAfter      time.Duration
}

type Result struct {
	RunID, Mode, CorrelationID                         string
	PublishedOutbox, RetainedIdempotency, ExpiredRates int64
	StartedAt, CompletedAt                             time.Time
}
