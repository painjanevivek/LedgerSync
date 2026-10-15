package integration_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	appdelivery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	deliveryhttp "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/webhookdelivery"
)

// The public-looking synthetic destination is rewritten only by this injected
// test transport to its owned TLS receiver. Production destination checks and
// the secure production transport remain unchanged.
type capacityReceiverTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (c capacityReceiverTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	destination := *c.target
	destination.Path = request.URL.Path
	copy.URL = &destination
	copy.Host = destination.Host
	return c.base.RoundTrip(copy)
}

func TestSlowWebhookReceiverKeepsPostingAndLeaseRenewalIndependent(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	key := bytes.Repeat([]byte{0x11}, 32)
	entered := make(chan struct{})
	release := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	var signatureVerified atomic.Bool
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(io.LimitReader(r.Body, 256*1024+1))
		if err != nil || len(payload) > 256*1024 {
			w.WriteHeader(400)
			return
		}
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(r.Header.Get("X-LedgerSync-Timestamp") + "." + r.Header.Get("X-LedgerSync-Delivery-ID") + "."))
		_, _ = mac.Write(payload)
		_, _ = mac.Write([]byte("." + r.Header.Get("X-LedgerSync-Key-ID")))
		if r.Header.Get("X-LedgerSync-Signature") != "v1="+hex.EncodeToString(mac.Sum(nil)) {
			w.WriteHeader(401)
			return
		}
		signatureVerified.Store(true)
		enterOnce.Do(func() { close(entered) })
		select {
		case <-release:
			w.WriteHeader(204)
		case <-r.Context().Done():
		}
	}))
	defer receiver.Close()
	defer releaseOnce.Do(func() { close(release) })
	target, _ := url.Parse(receiver.URL)
	client := &http.Client{Transport: capacityReceiverTransport{receiver.Client().Transport, target}, Timeout: 5 * time.Second}
	resolver, _ := deliveryhttp.NewStaticKeyResolver(map[string][]byte{"synthetic-capacity-key": key})
	dispatcher, err := deliveryhttp.NewDispatcher(client, resolver, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	const endpoint = "00000000-0000-4000-8000-000000009301"
	if _, err := admin.ExecContext(ctx, `INSERT INTO developer_webhook_endpoints(id,tenant_id,display_name,endpoint_url,subscribed_events,signing_key_reference,signing_key_id,status,version,verified_at,created_at,updated_at) VALUES($1,$2,'Synthetic slow receiver','https://slow.example.test/hooks',ARRAY['transfer.posted'],'synthetic-capacity-key','synthetic','active',1,now(),now(),now())`, endpoint, testTenantID); err != nil {
		t.Fatal(err)
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	api.db.SetMaxOpenConns(4)
	repository, _ := db.NewTransferRepository(api.db, time.Now)
	service, _ := transfers.NewService(repository, time.Now)
	if _, err := service.Submit(ctx, transferCommand(t, "slow-receiver-origin-key", "1.00")); err != nil {
		t.Fatal(err)
	}
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	worker.db.SetMaxOpenConns(1)
	queue, _ := db.NewOutboxRepository(worker.db, time.Now)
	events, err := queue.Claim(ctx, "slow-profile-publisher", 10, time.Minute)
	if err != nil || len(events) != 3 {
		t.Fatal("three initial committed events required", err)
	}
	for _, event := range events {
		if err := queue.MarkPublished(ctx, "slow-profile-publisher", event.ID, time.Now(), event.ClaimGeneration); err != nil {
			t.Fatal(err)
		}
	}
	jobs, _ := db.NewWebhookDeliveryJobRepository(worker.db, time.Now)
	delivery, err := appdelivery.NewWorker(jobs, dispatcher, time.Now, appdelivery.Config{WorkerID: "slow-profile-delivery", BatchSize: 1, ClaimLease: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		count, err := delivery.RunOnce(ctx)
		if count != 1 && err == nil {
			err = io.ErrUnexpectedEOF
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("actual receiver was not entered")
	}
	// A blocked HTTP receiver must not retain the worker's sole DB connection.
	probeCtx, probeCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	var one int
	err = worker.db.QueryRowContext(probeCtx, `SELECT 1`).Scan(&one)
	probeCancel()
	if err != nil || one != 1 {
		t.Fatal("slow HTTP retained the worker database connection", err)
	}
	time.Sleep(1200 * time.Millisecond)
	if countRows(t, admin, `SELECT count(*) FROM webhook_delivery_jobs WHERE claim_owner='slow-profile-delivery' AND claimed_until>clock_timestamp() AND started_at<clock_timestamp()-interval '1 second'`) != 1 {
		t.Fatal("slow receiver lease was not renewed")
	}
	latencies := make([]float64, 12)
	failures := make(chan error, 12)
	tasks := make(chan int, 12)
	var group sync.WaitGroup
	commands := make([]transfers.Command, 12)
	for i := range commands {
		commands[i] = transferCommand(t, "slow-profile-posting-key-"+string(rune('a'+i)), "0.01")
	}
	started := time.Now()
	for range 4 {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range tasks {
				begin := time.Now()
				result, err := service.Submit(ctx, commands[index])
				latencies[index] = float64(time.Since(begin)) / float64(time.Millisecond)
				if err != nil {
					failures <- err
				} else if result.Result.Status != "posted" {
					failures <- io.ErrUnexpectedEOF
				}
			}
		}()
	}
	for i := range 12 {
		tasks <- i
	}
	close(tasks)
	group.Wait()
	close(failures)
	for err := range failures {
		t.Fatal("posting failed while receiver was slow", err)
	}
	elapsed := time.Since(started)
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("slow delivery did not complete")
	}
	if !signatureVerified.Load() || countRows(t, admin, `SELECT count(*) FROM delivery_attempts WHERE endpoint_reference=$1 AND status='delivered'`, endpoint) != 1 {
		t.Fatal("signed slow delivery lacks successful evidence")
	}
	var source, destination int64
	if err := admin.QueryRowContext(ctx, `SELECT (SELECT ledger_minor FROM account_balance_projections WHERE account_id=$1),(SELECT ledger_minor FROM account_balance_projections WHERE account_id=$2)`, testSourceID, testDestinationID).Scan(&source, &destination); err != nil || source != 9888 || destination != 2112 {
		t.Fatal("slow profile changed financial conservation", err)
	}
	if capacityPercentile(latencies, .95) > 500 {
		t.Fatal("slow profile posting p95 exceeded selected 500 ms budget")
	}
	recon := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_reconciliation")
	reconciler, _ := db.NewReconciliationRepository(recon.db)
	matched, err := reconciler.Reconcile(ctx, testTenantID, time.Now())
	if err != nil || matched.MismatchCount != 0 {
		t.Fatal("slow receiver reconciliation mismatch", err)
	}
	report, _ := json.Marshal(map[string]any{"profile": "slow-real-TLS-receiver", "commands": 12, "concurrency": 4, "receiver_minimum_hold_ms": 1200, "worker_database_pool": 1, "worker_connection_free": true, "lease_renewed": true, "signature_verified": true, "posted_p50_ms": capacityPercentile(latencies, .5), "posted_p95_ms": capacityPercentile(latencies, .95), "posted_p99_ms": capacityPercentile(latencies, .99), "burst_elapsed_seconds": elapsed.Seconds(), "financial_conservation": true, "limits": "One controlled receiver and finite posting burst. Test-only transport redirect; no production destination-policy exception or receiver-capacity claim."})
	t.Logf("SLOW_RECEIVER_REPORT %s", report)
}
