package integration_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	delivery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
	verification "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookverification"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	deliveryhttp "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/webhookdelivery"
	verificationhttp "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/webhookverification"
)

func writeExamplePrivateJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write(encoded)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal("private example configuration write failed")
	}
}

func runExampleStep(t *testing.T, ctx context.Context, root, config, action string) {
	t.Helper()
	command := exec.CommandContext(ctx, "node", filepath.Join(liveMigrationRoot(t), "examples/typescript/src/workflow-check.ts"), config, action)
	output, err := command.CombinedOutput()
	if writeErr := os.WriteFile(filepath.Join(root, "example-"+action+".txt"), output, 0600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil {
		t.Fatalf("example step %s failed; inspect the retained private step log", action)
	}
}

func startExampleReceiver(t *testing.T, ctx context.Context, root, config, ready, label string) (string, func()) {
	t.Helper()
	log, err := os.OpenFile(filepath.Join(root, "receiver-"+label+".txt"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "node", filepath.Join(liveMigrationRoot(t), "examples/typescript/src/receiver-main.ts"), config)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		_ = log.Close()
		t.Fatal("local Node receiver did not start")
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			select {
			case <-done:
			default:
				_ = command.Process.Kill()
				<-done
			}
			_ = log.Close()
		})
	}
	t.Cleanup(stop)
	for i := 0; i < 100; i++ {
		encoded, err := os.ReadFile(ready)
		if err == nil {
			var record struct {
				URL string `json:"url"`
				PID int    `json:"pid"`
			}
			if json.Unmarshal(encoded, &record) != nil || record.PID != command.Process.Pid {
				t.Fatal("receiver readiness does not belong to the owned process")
			}
			address, err := url.Parse(record.URL)
			if err != nil || address.Scheme != "http" || address.Hostname() != "127.0.0.1" || address.Port() == "" || address.Path != "/hooks" {
				t.Fatal("receiver escaped its local allocation")
			}
			return record.URL, stop
		}
		select {
		case <-ctx.Done():
			t.Fatal("receiver readiness timed out")
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatal("receiver readiness was not recorded")
	return "", stop
}

func TestPortfolioDeveloperIntegration(t *testing.T) {
	if os.Getenv("LEDGERSYNC_RUN_DEVELOPER") != "true" {
		t.Skip("requires guarded independent developer profile")
	}
	root := portfolioControlDirectory(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	started := time.Now()
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	server, tenant, principals, _ := exampleAPIFixture(t, admin)
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	worker.db.SetMaxOpenConns(2)
	key := []byte(exampleSecret(t))
	config := map[string]any{"statePath": filepath.Join(root, "example-intents.sqlite"), "baseUrl": server.URL + "/api", "principals": principals}
	ready, failure := filepath.Join(root, "example-receiver-ready.json"), filepath.Join(root, "example-receiver-failure.enabled")
	receiverConfig := map[string]any{"statePath": filepath.Join(root, "example-receiver.sqlite"), "keyId": "example-key", "keyBase64": base64.StdEncoding.EncodeToString(key), "readyPath": ready, "failurePath": failure}
	config["receiver"] = receiverConfig
	configPath := filepath.Join(root, "example-private-config.json")
	writeExamplePrivateJSON(t, configPath, config)
	receiverURL, stop := startExampleReceiver(t, ctx, root, configPath, ready, "first")
	runExampleStep(t, ctx, root, configPath, "setup")
	target, _ := url.Parse(receiverURL)
	// This test-only transport rewrites the fixed synthetic HTTPS destination
	// to its PID-bound Node loopback receiver. Production network policy is unchanged.
	client := &http.Client{Transport: capacityReceiverTransport{base: http.DefaultTransport, target: target}, Timeout: 3 * time.Second}
	resolver, _ := deliveryhttp.NewStaticKeyResolver(map[string][]byte{"synthetic-example-key": key})
	verifyDispatcher, _ := verificationhttp.NewDispatcher(client, resolver, time.Now)
	verifyRepo, _ := db.NewWebhookVerificationJobRepository(worker.db, time.Now)
	verifyWorker, _ := verification.NewWorker(verifyRepo, verifyDispatcher, time.Now, verification.Config{WorkerID: "example-verification", BatchSize: 5})
	if count, err := verifyWorker.RunOnce(ctx); err != nil || count != 1 {
		t.Fatal("actual signed endpoint verification failed", count, err)
	}
	runExampleStep(t, ctx, root, configPath, "transfer-once")
	runExampleStep(t, ctx, root, configPath, "retry")
	if countRows(t, admin, `SELECT count(*) FROM journal_transactions WHERE tenant_id=$1`, tenant) != 2 || countRows(t, admin, `SELECT count(*) FROM ledger_postings WHERE tenant_id=$1`, tenant) != 4 {
		t.Fatal("HTTP retries changed financial effect counts")
	}
	if countRows(t, admin, `SELECT COALESCE(sum(ledger_minor),0) FROM account_balance_projections p JOIN accounts a ON a.id=p.account_id WHERE a.tenant_id=$1`, tenant) != 0 {
		t.Fatal("synthetic funding and transfer do not conserve value")
	}
	queue, _ := db.NewOutboxRepository(worker.db, time.Now)
	events, err := queue.Claim(ctx, "example-publisher", 50, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := queue.MarkPublished(ctx, "example-publisher", event.ID, time.Now(), event.ClaimGeneration); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(failure, []byte("host-only synthetic receiver failure"), 0600); err != nil {
		t.Fatal(err)
	}
	dispatcher, _ := deliveryhttp.NewDispatcher(client, resolver, time.Now)
	jobs, _ := db.NewWebhookDeliveryJobRepository(worker.db, time.Now)
	deliveryWorker, _ := delivery.NewWorker(jobs, dispatcher, time.Now, delivery.Config{WorkerID: "example-delivery", BatchSize: 10})
	if count, err := deliveryWorker.RunOnce(ctx); err != nil || count != 1 {
		t.Fatal("receiver failure attempt did not record one job", count, err)
	}
	if countRows(t, admin, `SELECT count(*) FROM delivery_attempts WHERE tenant_id=$1 AND status='retrying' AND response_class='http_503'`, tenant) != 1 {
		t.Fatal("receiver failure did not retain retry evidence")
	}
	if err := os.Remove(failure); err != nil {
		t.Fatal(err)
	}
	delivered := false
	for i := 0; i < 60; i++ {
		if _, err := deliveryWorker.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if countRows(t, admin, `SELECT count(*) FROM webhook_delivery_jobs WHERE tenant_id=$1 AND status='delivered'`, tenant) == 1 {
			delivered = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !delivered {
		t.Fatal("actual webhook delivery did not recover")
	}
	stop()
	ready2 := filepath.Join(root, "example-receiver-restarted.json")
	receiverConfig["readyPath"] = ready2
	config2 := filepath.Join(root, "example-private-config-restarted.json")
	writeExamplePrivateJSON(t, config2, config)
	restarted, _ := startExampleReceiver(t, ctx, root, config2, ready2, "restarted")
	duplicate := delivery.Delivery{ID: uuid.NewString(), EventType: "transfer.posted", EndpointURL: "https://receiver.example.test/hooks", SigningKeyReference: "synthetic-example-key", SigningKeyID: "example-key"}
	if err := admin.QueryRowContext(ctx, `SELECT event_id::text,payload FROM webhook_delivery_jobs WHERE tenant_id=$1`, tenant).Scan(&duplicate.EventID, &duplicate.Payload); err != nil {
		t.Fatal(err)
	}
	target2, _ := url.Parse(restarted)
	duplicateClient := &http.Client{Transport: capacityReceiverTransport{base: http.DefaultTransport, target: target2}, Timeout: 3 * time.Second}
	duplicateDispatcher, _ := deliveryhttp.NewDispatcher(duplicateClient, resolver, time.Now)
	if outcome, err := duplicateDispatcher.Dispatch(ctx, duplicate); err != nil || outcome.ResponseClass != "http_204" {
		t.Fatal("actual duplicate after receiver restart failed", err)
	}
	runExampleStep(t, ctx, root, config2, "receiver-check")
	t.Logf("DEVELOPER_INTEGRATION actual_http=passed own_principal_auth=passed restricted_api=passed funding=independent_synthetic_review exact_retry=original journals=2 postings=4 conservation=balanced reconciliation=matched verification=real_signed failure=http_503 recovery=delivered receiver_restart=deduplicated elapsed_seconds=%.3f", time.Since(started).Seconds())
}
