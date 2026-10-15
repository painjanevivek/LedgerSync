package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
)

type DistributedEvidenceRepository struct {
	database *sql.DB
	study    bool
	client   *http.Client
}

func NewDistributedEvidenceRepository(database *sql.DB, study bool) *DistributedEvidenceRepository {
	return &DistributedEvidenceRepository{database: database, study: study, client: &http.Client{Timeout: 750 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (r *DistributedEvidenceRepository) Read(ctx context.Context, tenantID, actorID string) operations.DistributedEvidence {
	unavailable := func() operations.DistributedObservation {
		return operations.DistributedObservation{State: "unavailable", SampledAt: time.Now().UTC()}
	}
	result := operations.DistributedEvidence{APIReplicas: unavailable(), WorkerPublications: unavailable(), Scheduler: unavailable()}
	// Independent observations have independent timeouts and sample times.
	type sample struct {
		kind  string
		value operations.DistributedObservation
	}
	samples := make(chan sample, 3)
	for _, kind := range []string{"api", "workers", "scheduler"} {
		go func(kind string) {
			scoped, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			defer cancel()
			value := unavailable()
			var tx *tenantTransaction
			if kind != "api" && r.database != nil {
				var err error
				tx, err = beginTenantRead(scoped, r.database, TenantScope{TenantID: tenantID, ActorID: actorID})
				if err != nil {
					samples <- sample{kind, value}
					return
				}
				defer tx.Rollback()
			}
			switch kind {
			case "api":
				if r.study {
					request, err := http.NewRequestWithContext(scoped, http.MethodGet, "http://study-proxy:8080/internal/study/replicas", nil)
					if err == nil {
						response, err := r.client.Do(request)
						if err == nil {
							defer response.Body.Close()
							var payload struct {
								CapturedAt time.Time `json:"captured_at"`
								Count      *int      `json:"ready_api_count"`
							}
							body, readErr := io.ReadAll(io.LimitReader(response.Body, 4097))
							if readErr == nil && len(body) <= 4096 && response.StatusCode == 200 && json.Unmarshal(body, &payload) == nil && payload.Count != nil && *payload.Count >= 0 && *payload.Count <= 16 && !payload.CapturedAt.IsZero() && time.Since(payload.CapturedAt) >= -time.Second && time.Since(payload.CapturedAt) < 5*time.Second {
								value.State = "available"
								value.Count = strconv.Itoa(*payload.Count)
								value.SampledAt = payload.CapturedAt
							}
						}
					}
				}
			case "workers":
				if r.database != nil {
					var count string
					var latest sql.NullTime
					err := tx.QueryRowContext(scoped, `SELECT count(DISTINCT worker_id)::text,max(completed_at) FROM (SELECT a.worker_id,a.completed_at FROM outbox_publication_attempts a JOIN outbox_events e ON e.id=a.event_id WHERE e.tenant_id=$1 AND a.status='published' ORDER BY a.completed_at DESC LIMIT 1000) observed`, tenantID).Scan(&count, &latest)
					value.SampledAt = time.Now().UTC()
					if err == nil {
						value.State = "historical"
						value.Count = count
						value.LatestPublishedAt = optionalDatabaseTime(latest)
					}
				}
			case "scheduler":
				if r.database != nil {
					var owner, generation, state string
					var expiry, sampled time.Time
					err := tx.QueryRowContext(scoped, `SELECT owner_id,generation::text,expires_at,clock_timestamp(),CASE WHEN expires_at>clock_timestamp() THEN 'active' ELSE 'expired' END FROM reconciliation_scheduler_leases WHERE tenant_id=$1`, tenantID).Scan(&owner, &generation, &expiry, &sampled, &state)
					value.SampledAt = time.Now().UTC()
					if err == sql.ErrNoRows {
						value.State = "not_observed"
					} else if err == nil {
						value.State = state
						value.OwnerID = causalNode(owner)
						value.Generation = generation
						value.ExpiresAt = &expiry
						value.SampledAt = sampled
					}
				}
			}
			samples <- sample{kind, value}
		}(kind)
	}
	for range 3 {
		var item sample
		select {
		case item = <-samples:
		case <-ctx.Done():
			return result
		}
		switch item.kind {
		case "api":
			result.APIReplicas = item.value
		case "workers":
			result.WorkerPublications = item.value
		case "scheduler":
			result.Scheduler = item.value
		}
	}
	return result
}
