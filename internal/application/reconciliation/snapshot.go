package reconciliation

import "time"

// Delivery counts share the ledger transaction snapshot. They do not establish
// a global snapshot of Redis, workers, or messages in transit.
type OutboxSnapshot struct {
	Isolation      string    `json:"isolation"`
	SampledAt      time.Time `json:"sampled_at"`
	PendingCount   string    `json:"pending_count"`
	PublishedCount string    `json:"published_count"`
	DeadCount      string    `json:"dead_count"`
}
