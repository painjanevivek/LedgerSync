package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

// TestPortfolioCapacity is opt-in. The host coordinator creates and validates
// its disposable targets before this test can construct any financial fixture.
func TestPortfolioCapacity(t *testing.T) {
	if os.Getenv("LEDGERSYNC_RUN_CAPACITY") != "true" {
		t.Skip("capacity requires the guarded local host coordinator")
	}
	seconds, err := strconv.Atoi(os.Getenv("LEDGERSYNC_CAPACITY_STEADY_SECONDS"))
	if err != nil || (seconds != 10 && seconds != 300) {
		t.Fatal("unsupported bounded steady duration")
	}
	batch, err := strconv.Atoi(os.Getenv("LEDGERSYNC_CAPACITY_OUTBOX_BATCH_SIZE"))
	if err != nil || (batch != 10 && batch != 25 && batch != 50) {
		t.Fatal("unsupported bounded worker batch")
	}
	maximumRate, err := strconv.Atoi(os.Getenv("LEDGERSYNC_CAPACITY_RATE"))
	if err != nil || (maximumRate != 10 && maximumRate != 20 && maximumRate != 30) {
		t.Fatal("unsupported bounded capacity rate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 530*time.Second)
	defer cancel()
	fixture := newCapacityFixture(t, ctx)
	work := newCapacityWorkload(fixture)
	stages := []capacityStage{}
	qualified := false
	constraint := ""
	faultsVerified := false
	add := func(name, mode string, rate, duration int, recovery bool) bool {
		stage := work.stage(ctx, name, mode, rate, duration, recovery)
		stages = append(stages, stage)
		t.Logf("CAPACITY_STAGE %s offered=%d submitted=%d p95_ms=%.2f oldest_seconds=%.2f accepted=%t constraint=%s", name, stage.Offered, stage.Submitted, stage.P95, stage.SampledPeakOldest, stage.Accepted, stage.StopConstraint)
		if stage.TechnicalFailures > 0 {
			t.Fatal("capacity command failed; no qualification claimed")
		}
		if !stage.Accepted {
			constraint = stage.StopConstraint
		}
		return stage.Accepted
	}
	add("eight-worker-burst", "closed-clients", 0, 0, false)
	fixture.drain(t, ctx)
	rampSeconds := 15
	if seconds == 10 {
		rampSeconds = 5
	}
	accepted := add("warmup", "open-loop", 5, 5, false)
	for _, rate := range []int{5, 10, 20, 30} {
		if rate > maximumRate {
			break
		}
		if !accepted {
			break
		}
		accepted = add(fmt.Sprintf("ramp-%d", rate), "open-loop", rate, rampSeconds, false)
	}
	if accepted {
		accepted = add("steady", "open-loop", maximumRate, seconds, false)
	}
	if accepted && seconds == 300 {
		faultCtx, faultCancel := context.WithCancel(ctx)
		faultDone := make(chan struct{})
		go func() {
			defer close(faultDone)
			for _, fault := range []struct {
				after time.Duration
				name  string
			}{{10 * time.Second, "pause-redis.requested"}, {20 * time.Second, "restart-worker.requested"}} {
				timer := time.NewTimer(fault.after)
				select {
				case <-timer.C:
					encoded, _ := json.Marshal(map[string]bool{"fixed_fault": true})
					if os.WriteFile(filepath.Join(fixture.control, fault.name), encoded, 0600) != nil {
						return
					}
				case <-faultCtx.Done():
					timer.Stop()
					return
				}
			}
		}()
		accepted = add("dependency-and-worker-recovery", "open-loop", maximumRate/2, 60, true)
		faultCancel()
		<-faultDone
		_, pauseErr := os.Stat(filepath.Join(fixture.control, "redis-pause-result.json"))
		_, restartErr := os.Stat(filepath.Join(fixture.control, "worker-restart-result.json"))
		faultsVerified = pauseErr == nil && restartErr == nil
		if !faultsVerified {
			t.Fatal("requested real recovery faults did not complete")
		}
		qualified = accepted
	}
	drain := fixture.drain(t, ctx)
	projectionDrain := work.waitForProjection(t, ctx)
	work.verify(t, ctx)
	repository, err := db.NewReconciliationRepository(fixture.recon)
	if err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []string{testTenantID, capacityTenant2} {
		result, err := repository.Reconcile(ctx, tenant, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if result.MismatchCount != 0 {
			t.Fatalf("capacity reconciliation mismatch: %+v", result)
		}
	}
	report := map[string]any{"schema": "ledgersync-local-capacity/v1", "batch_size": batch, "target_commands_per_second": maximumRate, "projection_drain_seconds": projectionDrain.Seconds(), "steady_seconds": seconds, "qualification_requested": seconds == 300, "qualified": qualified, "calibration_only": seconds == 10, "financial_invariants": true, "recovery_faults_verified": faultsVerified, "stop_constraint": constraint, "drain_seconds": drain.Seconds(), "stages": stages, "journals": work.ledger.Journals, "unique_intents": len(work.receipts), "limits": []string{"Local financial application service and actual native continuous worker; excludes HTTP, BFF and candidate API images.", "Synthetic USD, six accounts, two tenants, eight bounded driver workers, API pool four and worker pool two.", "Database and Redis each have one CPU; memory limits are 768 MiB and 256 MiB.", "Queue and lock statistics are sampled once per second. Driver CPU excludes the native worker; container resource samples are separate.", "A short calibration never qualifies the five-minute steady state. No maximum capacity or production SLA claim."}}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CAPACITY_REPORT %s", encoded)
}

func (c *capacityFixture) drain(t *testing.T, ctx context.Context) time.Duration {
	t.Helper()
	started := time.Now()
	deadline := time.NewTimer(60 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		sample, err := c.sample(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if sample.Dead != 0 {
			t.Fatal("dead delivery work after capacity stage")
		}
		if sample.Pending == 0 {
			return time.Since(started)
		}
		select {
		case <-deadline.C:
			t.Fatal("capacity delivery drain exceeded 60 seconds")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

func (w *capacityWorkload) waitForProjection(t *testing.T, ctx context.Context) time.Duration {
	t.Helper()
	started := time.Now()
	deadline := time.NewTimer(60 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	projection, err := cache.NewBalanceCache(w.fixture.redis, "", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for {
		complete := true
		for id, expected := range w.ledger.Accounts {
			tenant := testTenantID
			for _, pair := range w.fixture.pairs {
				if pair.Source == id || pair.Destination == id {
					tenant = pair.Tenant
				}
			}
			actual, err := projection.Get(ctx, tenant, id)
			if err != nil || uint64(actual.Version) != expected.Version {
				complete = false
				break
			}
		}
		if complete {
			return time.Since(started)
		}
		select {
		case <-deadline.C:
			t.Fatal("Redis projection did not converge within 60 seconds after publication")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

func (w *capacityWorkload) verify(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := w.ledger.Check(); err != nil {
		t.Fatal(err)
	}
	projection, err := cache.NewBalanceCache(w.fixture.redis, "", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for id, expected := range w.ledger.Accounts {
		var ledger, available, version string
		if err := w.fixture.admin.QueryRowContext(ctx, `SELECT ledger_minor::text,available_minor::text,balance_version::text FROM account_balance_projections WHERE account_id=$1`, id).Scan(&ledger, &available, &version); err != nil {
			t.Fatal(err)
		}
		if ledger != expected.Minor || available != expected.Minor || version != strconv.FormatUint(expected.Version, 10) {
			t.Fatal("capacity database differs from independent oracle")
		}
		tenant := testTenantID
		for _, pair := range w.fixture.pairs {
			if pair.Source == id || pair.Destination == id {
				tenant = pair.Tenant
			}
		}
		cached, err := projection.Get(ctx, tenant, id)
		if err != nil {
			t.Fatal(err)
		}
		if strconv.FormatInt(cached.LedgerMinor, 10) != expected.Minor || strconv.FormatInt(cached.AvailableMinor, 10) != expected.Minor || uint64(cached.Version) != expected.Version {
			t.Fatal("capacity Redis projection differs from authoritative ledger")
		}
	}
	for _, receipt := range w.receipts {
		if receipt.Originals != 1 {
			t.Fatal("capacity intent did not have exactly one original durable result")
		}
	}
	var journals, postings, unbalanced int
	if err := w.fixture.admin.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM journal_transactions),(SELECT count(*) FROM ledger_postings),(SELECT count(*) FROM (SELECT journal_transaction_id FROM ledger_postings GROUP BY journal_transaction_id HAVING count(*)<>2 OR count(DISTINCT currency)<>1 OR sum(CASE WHEN direction='debit' THEN -amount_minor::numeric ELSE amount_minor::numeric END)<>0) bad)`).Scan(&journals, &postings, &unbalanced); err != nil {
		t.Fatal(err)
	}
	if journals != w.ledger.Journals || postings != 2*journals || unbalanced != 0 {
		t.Fatal("capacity journal conservation failed")
	}
}
