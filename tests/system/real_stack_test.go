package system_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type sessionPayload struct {
	CSRFToken string `json:"csrf_token"`
}

type transferPayload struct {
	TransferID             string            `json:"transfer_id"`
	Status                 string            `json:"status"`
	FinancialStatus        string            `json:"financial_status"`
	MinimumBalanceVersions map[string]string `json:"minimum_balance_versions"`
	Balances               map[string]struct {
		PostedMinor string `json:"posted_minor"`
		Version     string `json:"version"`
	} `json:"balances"`
	Replayed bool `json:"-"`
}

type balancePayload struct {
	AvailableMinor string `json:"available_minor"`
}

func TestRealBFFAPIAndPostgreSQLRetryPath(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LEDGERSYNC_SYSTEM_WEB_URL"), "/")
	if baseURL == "" {
		t.Skip("LEDGERSYNC_SYSTEM_WEB_URL is required for the real-stack smoke test")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var session sessionPayload
	getJSON(t, client, baseURL+"/api/session", &session)
	if session.CSRFToken == "" {
		t.Fatal("real BFF did not establish the server-gated demo session")
	}
	var accounts struct {
		Accounts []struct {
			AccountID string `json:"account_id"`
		} `json:"accounts"`
		NextCursor string `json:"next_cursor"`
	}
	getJSON(t, client, baseURL+"/api/me/accounts?limit=100&status=active", &accounts)
	if len(accounts.Accounts) < 2 {
		t.Fatalf("real API returned %d authorized accounts, want at least 2", len(accounts.Accounts))
	}
	var accountDetail struct {
		AccountID string `json:"account_id"`
	}
	getJSON(t, client, baseURL+"/api/accounts/"+accounts.Accounts[0].AccountID, &accountDetail)
	if accountDetail.AccountID != accounts.Accounts[0].AccountID {
		t.Fatalf("object-specific account detail mismatch: %+v", accountDetail)
	}

	var before balancePayload
	getJSON(t, client, baseURL+"/api/accounts/10000000-0000-4000-8000-000000000001/balance", &before)
	body := []byte(`{"sourceAccountId":"10000000-0000-4000-8000-000000000001","destinationAccountId":"10000000-0000-4000-8000-000000000004","amount":{"currency":"USD","minorUnits":"1"}}`)
	key := os.Getenv("LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY")
	if key == "" {
		key = "system-stack-idempotency-00000001"
	}
	first := postTransfer(t, client, baseURL, session.CSRFToken, key, body)
	if first.Replayed {
		t.Fatalf("first request unexpectedly replayed idempotency key %q; use a new key for each intentional movement", key)
	}
	var afterFirst balancePayload
	getJSON(t, client, baseURL+"/api/accounts/10000000-0000-4000-8000-000000000001/balance?require_version=1", &afterFirst)
	second := postTransfer(t, client, baseURL, session.CSRFToken, key, body)
	if !second.Replayed {
		t.Fatalf("same-key retry was not identified as a replay for transfer %s", first.TransferID)
	}
	var afterRetry balancePayload
	getJSON(t, client, baseURL+"/api/accounts/10000000-0000-4000-8000-000000000001/balance", &afterRetry)
	if first.TransferID == "" || first.TransferID != second.TransferID || first.Status != "posted" || second.Status != "posted" {
		t.Fatalf("retry did not return one stable posted transfer: first=%+v second=%+v", first, second)
	}
	var detail transferPayload
	getJSON(t, client, baseURL+"/api/transfers/"+first.TransferID, &detail)
	if detail.TransferID != first.TransferID || detail.FinancialStatus != "posted" {
		t.Fatalf("immutable transfer detail is inconsistent: %+v", detail)
	}
	beforeMinor, err := strconv.ParseInt(before.AvailableMinor, 10, 64)
	if err != nil {
		t.Fatalf("parse initial exact balance %q: %v", before.AvailableMinor, err)
	}
	afterFirstMinor, err := strconv.ParseInt(afterFirst.AvailableMinor, 10, 64)
	if err != nil {
		t.Fatalf("parse updated exact balance %q: %v", afterFirst.AvailableMinor, err)
	}
	if afterFirstMinor != beforeMinor-1 || afterRetry.AvailableMinor != afterFirst.AvailableMinor {
		t.Fatalf("immediate balance or replay safety failed: before=%s after_first=%s after_retry=%s", before.AvailableMinor, afterFirst.AvailableMinor, afterRetry.AvailableMinor)
	}
	sourceID := "10000000-0000-4000-8000-000000000001"
	committed, ok := first.Balances[sourceID]
	if !ok || committed.PostedMinor != afterFirst.AvailableMinor || committed.Version == "" || first.MinimumBalanceVersions[sourceID] != committed.Version {
		t.Fatalf("authoritative transfer result and immediate balance disagree: committed=%+v minimum_version=%q visible=%s", committed, first.MinimumBalanceVersions[sourceID], afterFirst.AvailableMinor)
	}
}

func TestRealBFFReconciliationEvidence(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LEDGERSYNC_SYSTEM_WEB_URL"), "/")
	if baseURL == "" {
		t.Skip("LEDGERSYNC_SYSTEM_WEB_URL is required for the real-stack smoke test")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var session sessionPayload
	getJSON(t, client, baseURL+"/api/session", &session)
	var evidence struct {
		Runs []struct {
			Status        string `json:"status"`
			MismatchCount string `json:"mismatch_count"`
			RunID         string `json:"run_id"`
		} `json:"runs"`
	}
	getJSON(t, client, baseURL+"/api/reconciliation/runs?limit=1", &evidence)
	if len(evidence.Runs) != 1 || evidence.Runs[0].Status != "matched" || evidence.Runs[0].MismatchCount != "0" || evidence.Runs[0].RunID == "" {
		t.Fatalf("latest reconciliation is not authoritative matched evidence: %+v", evidence.Runs)
	}
}

func getJSON(t *testing.T, client *http.Client, url string, target any) {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	decodeResponse(t, response, target)
}

func postTransfer(t *testing.T, client *http.Client, baseURL, csrfToken, key string, body []byte) transferPayload {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/transfers", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", baseURL)
	request.Header.Set("X-CSRF-Token", csrfToken)
	request.Header.Set("Idempotency-Key", key)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	replayed := response.Header.Get("Idempotent-Replay") == "true"
	t.Logf("transfer response status=%d replay=%t session_updated=%t", response.StatusCode, replayed, len(response.Cookies()) > 0)
	var payload transferPayload
	decodeResponse(t, response, &payload)
	payload.Replayed = replayed
	return payload
}

func decodeResponse(t *testing.T, response *http.Response, target any) {
	t.Helper()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		t.Fatalf("%s returned %d: %s", response.Request.URL, response.StatusCode, body)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		t.Fatal(err)
	}
}
