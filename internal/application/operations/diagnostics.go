package operations

import (
	"context"
	"errors"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const dependencyTimeout = 750 * time.Millisecond

var safeBuildFact = regexp.MustCompile(`^[A-Za-z0-9._+()-]{1,128}$`)

type BuildFacts struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Environment string `json:"environment"`
}

type DatabaseFacts struct {
	SchemaVersion        string
	PendingOutboxCount   int64
	DeadOutboxCount      int64
	LatestPublishedAt    time.Time
	OldestPendingAt      time.Time
	ReconciliationID     string
	ReconciliationStatus string
	ReconciledAt         time.Time
}

type DiagnosticRepository interface {
	Facts(context.Context, string) (DatabaseFacts, error)
}

type DependencyProbe interface {
	Ping(context.Context) error
}

type DependencyStatus struct {
	State string `json:"state"`
}

type PostgreSQLStatus struct {
	State         string `json:"state"`
	SchemaVersion string `json:"schema_version,omitempty"`
}

type ReconciliationStatus struct {
	State       string     `json:"state"`
	Status      string     `json:"status,omitempty"`
	RunID       string     `json:"run_id,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type OutboxStatus struct {
	State             string     `json:"state"`
	PendingCount      string     `json:"pending_count,omitempty"`
	DeadCount         string     `json:"dead_count,omitempty"`
	WorkerProgress    string     `json:"worker_progress"`
	LatestPublishedAt *time.Time `json:"latest_published_at,omitempty"`
	OldestPendingAt   *time.Time `json:"oldest_pending_at,omitempty"`
}

type RedisStatus struct {
	State string `json:"state"`
	Label string `json:"label"`
}

type DiagnosticSnapshot struct {
	OverallState       string     `json:"overall_state"`
	GeneratedAt        time.Time  `json:"generated_at"`
	Application        BuildFacts `json:"application"`
	FinancialAuthority struct {
		PostgreSQL           PostgreSQLStatus     `json:"postgres"`
		LatestReconciliation ReconciliationStatus `json:"latest_reconciliation"`
	} `json:"financial_authority"`
	DeliveryCache struct {
		Outbox OutboxStatus `json:"outbox"`
		Redis  RedisStatus  `json:"redis"`
	} `json:"delivery_cache"`
}

type DiagnosticService struct {
	repository DiagnosticRepository
	cache      DependencyProbe
	build      BuildFacts
	clock      func() time.Time
	timeout    time.Duration
}

func NewDiagnosticService(repository DiagnosticRepository, cache DependencyProbe, build BuildFacts, clock func() time.Time) (*DiagnosticService, error) {
	if repository == nil || cache == nil {
		return nil, errors.New("diagnostic repository and cache probe are required")
	}
	if clock == nil {
		clock = time.Now
	}
	build.Version = boundedBuildValue(build.Version, "development")
	build.Commit = boundedBuildValue(build.Commit, "unknown")
	if build.Environment == "development" {
		build.Environment = "local_demo"
	} else {
		build.Environment = "configured"
	}
	return &DiagnosticService{repository: repository, cache: cache, build: build, clock: clock, timeout: dependencyTimeout}, nil
}

func CurrentBuildFacts(environment string) BuildFacts {
	facts := BuildFacts{Version: "development", Commit: "unknown", Environment: environment}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			facts.Version = info.Main.Version
		}
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				facts.Commit = setting.Value
			}
		}
	}
	return facts
}

func (s *DiagnosticService) Snapshot(ctx context.Context, tenantID string) DiagnosticSnapshot {
	now := s.clock().UTC()
	result := DiagnosticSnapshot{OverallState: "ready", GeneratedAt: now, Application: s.build}
	result.FinancialAuthority.PostgreSQL.State = "unavailable"
	result.FinancialAuthority.LatestReconciliation.State = "unavailable"
	result.DeliveryCache.Outbox = OutboxStatus{State: "unavailable", WorkerProgress: "unknown"}
	result.DeliveryCache.Redis = RedisStatus{State: "unavailable", Label: "disposable_cache"}

	type databaseResult struct {
		facts DatabaseFacts
		err   error
	}
	databaseChannel := make(chan databaseResult, 1)
	cacheChannel := make(chan error, 1)
	databaseContext, cancelDatabase := context.WithTimeout(ctx, s.timeout)
	defer cancelDatabase()
	cacheContext, cancelCache := context.WithTimeout(ctx, s.timeout)
	defer cancelCache()
	go func() {
		facts, err := s.repository.Facts(databaseContext, tenantID)
		databaseChannel <- databaseResult{facts, err}
	}()
	go func() { cacheChannel <- s.cache.Ping(cacheContext) }()

	database := databaseResult{err: context.DeadlineExceeded}
	cacheErr := error(context.DeadlineExceeded)
	databasePending, cachePending := true, true
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	for databasePending || cachePending {
		if databasePending {
			select {
			case database = <-databaseChannel:
				databasePending = false
			default:
			}
		}
		if cachePending {
			select {
			case cacheErr = <-cacheChannel:
				cachePending = false
			default:
			}
		}
		if !databasePending && !cachePending {
			break
		}
		select {
		case database = <-databaseChannel:
			databasePending = false
		case cacheErr = <-cacheChannel:
			cachePending = false
		case <-timer.C:
			if databasePending {
				database.err = context.DeadlineExceeded
			}
			if cachePending {
				cacheErr = context.DeadlineExceeded
			}
			databasePending, cachePending = false, false
		case <-ctx.Done():
			if databasePending {
				database.err = ctx.Err()
			}
			if cachePending {
				cacheErr = ctx.Err()
			}
			databasePending, cachePending = false, false
		}
	}
	if database.err == nil {
		applyDatabaseFacts(&result, database.facts, now)
	} else {
		result.OverallState = "unavailable"
	}
	if cacheErr == nil {
		result.DeliveryCache.Redis.State = "reachable"
	} else if result.OverallState == "ready" {
		result.OverallState = "degraded"
	}
	return result
}

func applyDatabaseFacts(result *DiagnosticSnapshot, facts DatabaseFacts, now time.Time) {
	result.FinancialAuthority.PostgreSQL = PostgreSQLStatus{State: "reachable", SchemaVersion: boundedBuildValue(facts.SchemaVersion, "unknown")}
	result.DeliveryCache.Outbox = OutboxStatus{State: "reachable", PendingCount: decimal(facts.PendingOutboxCount), DeadCount: decimal(facts.DeadOutboxCount), WorkerProgress: workerProgress(facts, now), LatestPublishedAt: optionalTime(facts.LatestPublishedAt), OldestPendingAt: optionalTime(facts.OldestPendingAt)}
	if facts.ReconciliationID == "" {
		result.FinancialAuthority.LatestReconciliation = ReconciliationStatus{State: "none"}
		result.OverallState = "degraded"
	} else {
		result.FinancialAuthority.LatestReconciliation = ReconciliationStatus{State: "available", Status: facts.ReconciliationStatus, RunID: facts.ReconciliationID, CompletedAt: optionalTime(facts.ReconciledAt)}
	}
	if facts.DeadOutboxCount > 0 || facts.ReconciliationStatus == "mismatch" || facts.ReconciliationStatus == "failed" || workerProgress(facts, now) == "stalled" {
		result.OverallState = "degraded"
	}
}

func workerProgress(facts DatabaseFacts, now time.Time) string {
	if facts.PendingOutboxCount == 0 {
		return "idle"
	}
	if !facts.LatestPublishedAt.IsZero() && now.Sub(facts.LatestPublishedAt) <= 2*time.Minute {
		return "recent"
	}
	if !facts.OldestPendingAt.IsZero() && now.Sub(facts.OldestPendingAt) > 2*time.Minute {
		return "stalled"
	}
	return "unknown"
}

func boundedBuildValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || !safeBuildFact.MatchString(value) {
		return fallback
	}
	return value
}

func decimal(value int64) string { return strconv.FormatInt(value, 10) }

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}
