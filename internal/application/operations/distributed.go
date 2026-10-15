package operations

import "time"

// DistributedObservation contains bounded technical evidence, not ledger authority.
type DistributedObservation struct {
	State             string     `json:"state"`
	SampledAt         time.Time  `json:"sampled_at"`
	Count             string     `json:"count,omitempty"`
	OwnerID           string     `json:"owner_id,omitempty"`
	Generation        string     `json:"generation,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	LatestPublishedAt *time.Time `json:"latest_published_at,omitempty"`
}
type DistributedEvidence struct {
	APIReplicas        DistributedObservation `json:"api_replicas"`
	WorkerPublications DistributedObservation `json:"worker_publications"`
	Scheduler          DistributedObservation `json:"scheduler"`
}
