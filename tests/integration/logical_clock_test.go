package integration_test

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"math"
	"os"
	"sync"
	"testing"
	"time"
)

func TestLamportStorageSendAndReceiveRules(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	nodeID := "ledgersync/api/test-" + uuid.NewString()
	var first, received, second int64
	for _, step := range []struct {
		incoming int64
		target   *int64
	}{{0, &first}, {20, &received}, {0, &second}} {
		if err := database.QueryRowContext(context.Background(), `SELECT advance_logical_clock($1,$2)`, nodeID, step.incoming).Scan(step.target); err != nil {
			t.Fatal(err)
		}
	}
	if first != 1 || received != 21 || second != 22 {
		t.Fatalf("send=%d receive=%d next=%d", first, received, second)
	}
}

func TestLamportConcurrencyRestartRollbackAndOverflow(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	nodeID := "ledgersync/worker/test-" + uuid.NewString()
	clock, _ := db.NewLogicalClockRepository(database, nodeID)
	var wg sync.WaitGroup
	values := make(chan int64, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := clock.Advance(ctx, 0)
			if err != nil {
				t.Error(err)
				return
			}
			values <- value
		}()
	}
	wg.Wait()
	close(values)
	unique := map[int64]bool{}
	for value := range values {
		if unique[value] {
			t.Error("concurrent counter repeated")
		}
		unique[value] = true
	}
	if len(unique) != 40 {
		t.Fatal("counter increments were lost")
	}
	reopened, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: os.Getenv("LEDGERSYNC_TEST_DATABASE_URL")})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, _ := db.NewLogicalClockRepository(reopened, nodeID)
	next, err := restored.Advance(ctx, 0)
	if err != nil || next != 41 {
		t.Fatalf("restart did not restore counter: %d %v", next, err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var rolledBack int64
	if err := tx.QueryRow(`SELECT advance_logical_clock($1,0)`, nodeID).Scan(&rolledBack); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	next, err = clock.Advance(ctx, 0)
	if err != nil || next != 42 {
		t.Fatalf("rollback consumed committed counter: %d %v", next, err)
	}
	if _, err := clock.Advance(ctx, math.MaxInt64); err == nil {
		t.Fatal("counter overflow did not fail closed")
	}
	next, err = clock.Advance(ctx, 0)
	if err != nil || next != 43 {
		t.Fatal("overflow corrupted persistent counter")
	}
	if _, err := clock.Advance(ctx, -1); err == nil {
		t.Fatal("negative remote counter was accepted")
	}
}

func TestLamportEnvelopeRetryAndLegacyReceipt(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	if _, err := service.Submit(ctx, transferCommand(t, "lamport-envelope-test", "1.00")); err != nil {
		t.Fatal(err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT set_config('application_name',$1,true)`, "ledgersync/api/test-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	eventID := uuid.NewString()
	if _, err := tx.Exec(`INSERT INTO outbox_events(id,tenant_id,transfer_id,account_id,aggregate_type,aggregate_id,aggregate_version,event_type,payload,occurred_at,available_at)
 SELECT $1,tenant_id,transfer_id,account_id,aggregate_type,aggregate_id,aggregate_version,'study.lamport.v1',payload,occurred_at,clock_timestamp() FROM outbox_events ORDER BY created_at,id LIMIT 1`, eventID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE outbox_events SET available_at=clock_timestamp()+interval '1 hour' WHERE id<>$1`, eventID); err != nil {
		t.Fatal(err)
	}
	requireWorkloadRoles(t, database)
	worker := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_worker")
	worker.db.SetMaxOpenConns(1)
	repository, _ := db.NewOutboxRepository(worker.db, time.Now)
	first, err := repository.Claim(ctx, "clock-owner", 1, time.Second)
	if err != nil || len(first) != 1 {
		t.Fatalf("first claim %v", err)
	}
	if first[0].Causal == nil || first[0].Causal.Timestamp != 1 {
		t.Fatal("transactional origin stamp is unavailable")
	}
	clock, _ := db.NewLogicalClockRepository(worker.db, "ledgersync/worker/test-publisher")
	sent, err := clock.PreparePublication(ctx, "clock-owner", first[0])
	if err != nil || sent.Delivery == nil || sent.Delivery.Timestamp <= first[0].Causal.Timestamp {
		t.Fatalf("publication order %v", err)
	}
	if err := repository.Reschedule(ctx, "clock-owner", eventID, time.Now(), "publish_failed", sent.ClaimGeneration); err != nil {
		t.Fatal(err)
	}
	second, err := repository.Claim(ctx, "clock-owner", 1, time.Second)
	if err != nil || len(second) != 1 {
		t.Fatal(err)
	}
	retried, err := clock.PreparePublication(ctx, "clock-owner", second[0])
	if err != nil {
		t.Fatal(err)
	}
	if retried.ID != sent.ID || *retried.Causal != *sent.Causal || retried.Delivery.Timestamp <= sent.Delivery.Timestamp {
		t.Fatal("retry changed immutable origin or failed to record a new send")
	}
	receiver, _ := db.NewLogicalClockRepository(worker.db, "ledgersync/worker/test-receiver")
	if err := receiver.Receive(ctx, projection.Message{ID: "100-0", Event: retried}); err != nil {
		t.Fatal(err)
	}
	if err := receiver.Receive(ctx, projection.Message{ID: "101-0", Event: sent}); err != nil {
		t.Fatal(err)
	}
	legacy := outbox.Event{ID: eventID, TenantID: sent.TenantID}
	if err := receiver.Receive(ctx, projection.Message{ID: "102-0", Event: legacy}); err != nil {
		t.Fatal(err)
	}
	var receive int64
	var sender sql.NullInt64
	if err := database.QueryRow(`SELECT receive_counter,sender_counter FROM logical_message_receipts ORDER BY receive_counter DESC LIMIT 1`).Scan(&receive, &sender); err != nil || sender.Valid || receive <= retried.Delivery.Timestamp {
		t.Fatal("legacy or delayed receive evidence is invalid")
	}
	if _, err := database.Exec(`UPDATE outbox_events SET logical_timestamp=99 WHERE id=$1`, eventID); err == nil {
		t.Fatal("event origin metadata was mutable")
	}
}
