package integration_test

import (
	"bufio"
	"context"
	"fmt"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/redis/go-redis/v9"
	"os"
	"os/exec"
	"testing"
	"time"
)

type crashPublisher struct {
	streams *events.RedisStreams
	stage   string
}

func (p crashPublisher) Publish(ctx context.Context, event outbox.Event) error {
	if p.stage == "after_publish" {
		if err := p.streams.Publish(ctx, event); err != nil {
			return err
		}
	}
	fmt.Println("STUDY_CLAIM_READY")
	<-ctx.Done()
	return ctx.Err()
}

// This helper runs only as a child of the isolated process-crash test.
func TestOutboxCrashHelper(t *testing.T) {
	stage := os.Getenv("LEDGERSYNC_TEST_CRASH_STAGE")
	if stage == "" {
		t.Skip("process crash helper")
	}
	harness := RequireHarness(t)
	database, err := db.OpenPool(context.Background(), db.PoolConfig{DriverName: "pgx", DSN: harness.DatabaseURL})
	if err != nil {
		t.Fatal("helper database unavailable")
	}
	defer func() { _ = database.Close() }()
	repository, _ := db.NewOutboxRepository(database, time.Now)
	client := redis.NewClient(&redis.Options{Addr: harness.RedisAddr})
	defer func() { _ = client.Close() }()
	stream, _ := events.NewRedisStreams(client, os.Getenv("LEDGERSYNC_TEST_CRASH_STREAM"))
	worker, _ := outbox.NewWorker(repository, crashPublisher{stream, stage}, nil, time.Now, outbox.Config{WorkerID: "crashed-process", BatchSize: 1, ClaimLease: 500 * time.Millisecond})
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOutboxProcessCrashRecovery(t *testing.T) {
	for _, stage := range []string{"before_publish", "after_publish"} {
		t.Run(stage, func(t *testing.T) {
			service, database := requireTransferService(t, 10_000)
			harness := RequireHarness(t)
			if _, err := service.Submit(context.Background(), transferCommand(t, "crash-worker-"+stage, "1.00")); err != nil {
				t.Fatal(err)
			}
			var id string
			if err := database.QueryRow(`SELECT id::text FROM outbox_events ORDER BY created_at,id LIMIT 1`).Scan(&id); err != nil {
				t.Fatal(err)
			}
			if _, err := database.Exec(`UPDATE outbox_events SET available_at=CASE WHEN id=$1 THEN clock_timestamp() ELSE clock_timestamp()+interval '1 hour' END`, id); err != nil {
				t.Fatal(err)
			}
			streamName := "study:crash:" + id
			child := exec.Command(os.Args[0], "-test.run=^TestOutboxCrashHelper$", "-test.timeout=20s")
			child.Env = append(os.Environ(), "LEDGERSYNC_TEST_CRASH_STAGE="+stage, "LEDGERSYNC_TEST_CRASH_STREAM="+streamName)
			output, err := child.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if child.Process != nil {
					_ = child.Process.Kill()
				}
			})
			ready := make(chan bool, 1)
			go func() {
				scanner := bufio.NewScanner(output)
				for scanner.Scan() {
					if scanner.Text() == "STUDY_CLAIM_READY" {
						ready <- true
						return
					}
				}
				ready <- false
			}()
			select {
			case ok := <-ready:
				if !ok {
					_ = child.Wait()
					t.Fatal("worker did not reach its crash boundary")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("worker crash boundary timed out")
			}
			recoveryStarted := time.Now()
			if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			_ = child.Wait()
			time.Sleep(600 * time.Millisecond)
			repository, _ := db.NewOutboxRepository(database, time.Now)
			client := redis.NewClient(&redis.Options{Addr: harness.RedisAddr})
			defer func() { _ = client.Close() }()
			stream, _ := events.NewRedisStreams(client, streamName)
			worker, _ := outbox.NewWorker(repository, stream, nil, time.Now, outbox.Config{WorkerID: "replacement-process", BatchSize: 1})
			if count, err := worker.RunOnce(context.Background()); err != nil || count != 1 {
				t.Fatalf("recover claim: count=%d error=%v", count, err)
			}
			recoveryMilliseconds := time.Since(recoveryStarted).Milliseconds()
			length, err := client.XLen(context.Background(), streamName).Result()
			if err != nil {
				t.Fatal(err)
			}
			want := int64(1)
			if stage == "after_publish" {
				want = 2
			}
			if length != want {
				t.Fatalf("deliveries=%d want=%d", length, want)
			}
			messages, err := client.XRange(context.Background(), streamName, "-", "+").Result()
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range messages {
				if message.Values["event_id"] != id {
					t.Fatal("retry changed the event identity")
				}
			}
			var published bool
			var generation int64
			if err := database.QueryRow(`SELECT published_at IS NOT NULL,claim_generation FROM outbox_events WHERE id=$1`, id).Scan(&published, &generation); err != nil || !published || generation != 2 {
				t.Fatalf("durable recovery: %v published=%v generation=%d", err, published, generation)
			}
			reconciliationRepository, err := db.NewReconciliationRepository(database)
			if err != nil {
				t.Fatal(err)
			}
			reconciler, err := reconciliation.NewService(reconciliationRepository, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			result, err := reconciler.Run(context.Background(), testTenantID)
			if err != nil || result.Status != reconciliation.StatusMatched || result.MismatchCount != 0 {
				t.Fatalf("crash reconciliation: result=%+v error=%v", result, err)
			}
			t.Logf("STUDY_WORKER_RECOVERY stage=%s recovery_ms=%d deliveries=%d reconciliation=matched", stage, recoveryMilliseconds, length)
			if err := stream.EnsureConsumerGroup(context.Background(), "recovery"); err != nil {
				t.Fatal(err)
			}
			pending, err := stream.ReadGroup(context.Background(), "recovery", "abandoned-consumer", 1, time.Millisecond)
			if err != nil || len(pending) != 1 {
				t.Fatalf("pending %v", err)
			}
			time.Sleep(30 * time.Millisecond)
			recovered, err := stream.RecoverPending(context.Background(), "recovery", "new-consumer", 20*time.Millisecond, 1)
			if err != nil || len(recovered) != 1 || recovered[0].ID != pending[0].ID {
				t.Fatalf("pending recovery %v", err)
			}
			if err := stream.Ack(context.Background(), "recovery", recovered[0].ID); err != nil {
				t.Fatal(err)
			}
			if err := client.Del(context.Background(), streamName).Err(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
