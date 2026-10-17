package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

// These owned HTTP adapters deliberately accept one fixed synthetic intent.
// They test transport/commit boundaries, not production authentication or LB policy.
func guidedHTTPFixture(t *testing.T, dropCommittedResponse bool) (*sql.DB, *httptest.Server, *httptest.Server, []byte) {
	t.Helper()
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	command := transferCommand(t, "guided-http-original-key", "1.00")
	body, err := json.Marshal(map[string]string{"key": command.IdempotencyKey, "debit": command.DebitAccountID.String(), "credit": command.CreditAccountID.String(), "minor": "100", "currency": "USD"})
	if err != nil {
		t.Fatal(err)
	}
	server := func(drop bool) *httptest.Server {
		login := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
		repository, err := db.NewTransferRepository(login.db, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		service, err := transfers.NewService(repository, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		owned := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			input, err := io.ReadAll(io.LimitReader(r.Body, 4097))
			if err != nil || r.Method != http.MethodPost || r.URL.Path != "/transfer" || !bytes.Equal(input, body) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			result, err := service.Submit(r.Context(), command)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if drop {
				// Submit has committed. Abort before writing any success header/body.
				panic(http.ErrAbortHandler)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result)
		}))
		t.Cleanup(owned.Close)
		return owned
	}
	return admin, server(dropCommittedResponse), server(false), body
}

func guidedPOST(ctx context.Context, url string, body []byte) (transfers.Submission, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/transfer", bytes.NewReader(body))
	if err != nil {
		return transfers.Submission{}, err
	}
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return transfers.Submission{}, err
	}
	defer func() { _ = response.Body.Close() }()
	var result transfers.Submission
	err = json.NewDecoder(io.LimitReader(response.Body, 16384)).Decode(&result)
	return result, err
}

func TestPortfolioCommittedResponseLoss(t *testing.T) {
	admin, first, survivor, body := guidedHTTPFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := guidedPOST(ctx, first.URL, body); err == nil {
		t.Fatal("the owned response connection did not fail")
	}
	if countRows(t, admin, `SELECT count(*) FROM transfers WHERE status='posted'`) != 1 {
		t.Fatal("response fault was not after commit")
	}
	result, err := guidedPOST(ctx, survivor.URL, body)
	if err != nil || !result.Replayed || result.Result.Status != "posted" || countRows(t, admin, `SELECT count(*) FROM ledger_postings`) != 2 {
		t.Fatalf("original intent did not resolve once: replay=%t err=%v", result.Replayed, err)
	}
	t.Log("FAULT_POINT committed_then_connection_aborted_before_response; original_key_recovered=true")
}

func TestPortfolioAPIFailover(t *testing.T) {
	admin, first, survivor, body := guidedHTTPFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	original, err := guidedPOST(ctx, first.URL, body)
	if err != nil || original.Replayed || original.Result.Status != "posted" {
		t.Fatalf("first API was not healthy: %v", err)
	}
	first.Close()
	if _, err := guidedPOST(ctx, first.URL, body); err == nil {
		t.Fatal("stopped API still served the request")
	}
	recovered, err := guidedPOST(ctx, survivor.URL, body)
	if err != nil || !recovered.Replayed || recovered.Result.TransferID != original.Result.TransferID || countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 1 {
		t.Fatalf("surviving API did not recover the original result: %v", err)
	}
	t.Log("FAULT_POINT proven_healthy_owned_api_stopped; surviving_api_original_key_recovered=true")
}
