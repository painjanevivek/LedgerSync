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

type fundingEventPayload struct {
	FundingEventID       string `json:"funding_event_id"`
	Status               string `json:"status"`
	DestinationAccountID string `json:"destination_account_id"`
	AmountMinor          string `json:"amount_minor"`
	Currency             string `json:"currency"`
	ExternalReference    string `json:"external_reference"`
	JournalTransactionID string `json:"journal_transaction_id"`
	BalanceVersion       string `json:"balance_version"`
}

type fundingSubmissionPayload struct {
	Event    fundingEventPayload `json:"event"`
	Replayed bool                `json:"replayed"`
}

func TestRealBFFControlledFundingLifecycle(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LEDGERSYNC_SYSTEM_WEB_URL"), "/")
	if baseURL == "" {
		t.Skip("LEDGERSYNC_SYSTEM_WEB_URL is required for the real-stack funding test")
	}
	client, session := newAuthenticatedClient(t, baseURL)

	const destinationID = "10000000-0000-4000-8000-000000000006"
	const key = "system-funding-idempotency-000001"
	const amountMinor = "250"
	var before balancePayload
	getJSON(t, client, baseURL+"/api/accounts/"+destinationID+"/balance", &before)
	requestBody := []byte(`{"destinationAccountId":"10000000-0000-4000-8000-000000000006","amountMinor":"250","currency":"INR","externalReference":"system-funding-evidence-000001","evidenceReference":"customer-evidence://system/funding/000001"}`)
	var created fundingSubmissionPayload
	postJSON(t, client, baseURL+"/api/funding-requests", baseURL, session.CSRFToken, key, requestBody, &created)
	if created.Replayed || created.Event.FundingEventID == "" || created.Event.Status != "requested" || created.Event.DestinationAccountID != destinationID || created.Event.AmountMinor != amountMinor || created.Event.Currency != "INR" {
		t.Fatalf("funding request evidence=%+v", created)
	}
	var replay fundingSubmissionPayload
	postJSON(t, client, baseURL+"/api/funding-requests", baseURL, session.CSRFToken, key, requestBody, &replay)
	if !replay.Replayed || replay.Event.FundingEventID != created.Event.FundingEventID {
		t.Fatalf("same-key funding retry did not resolve to one event: created=%+v replay=%+v", created, replay)
	}

	eventURL := baseURL + "/api/funding-events/" + created.Event.FundingEventID
	var approved fundingEventPayload
	postJSON(t, client, eventURL+"/approve", baseURL, session.CSRFToken, "", []byte(`{"reason":"verified local customer evidence"}`), &approved)
	if approved.Status != "approved" {
		t.Fatalf("demo funding approval=%+v", approved)
	}
	var posted fundingSubmissionPayload
	const postKey = "system-funding-post-000001"
	postJSON(t, client, eventURL+"/post", baseURL, session.CSRFToken, postKey, nil, &posted)
	if posted.Event.Status != "posted" || posted.Event.JournalTransactionID == "" || posted.Event.BalanceVersion == "" {
		t.Fatalf("posted funding journal=%+v", posted)
	}
	var postReplay fundingSubmissionPayload
	postJSON(t, client, eventURL+"/post", baseURL, session.CSRFToken, postKey, nil, &postReplay)
	if !postReplay.Replayed || postReplay.Event.JournalTransactionID != posted.Event.JournalTransactionID {
		t.Fatalf("funding post replay=%+v", postReplay)
	}

	var after balancePayload
	getJSON(t, client, baseURL+"/api/accounts/"+destinationID+"/balance?require_version="+posted.Event.BalanceVersion, &after)
	beforeMinor, parseErr := strconv.ParseInt(before.AvailableMinor, 10, 64)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	afterMinor, parseErr := strconv.ParseInt(after.AvailableMinor, 10, 64)
	if parseErr != nil || afterMinor != beforeMinor+250 {
		t.Fatalf("funding balance did not advance exactly once: before=%s after=%s error=%v", before.AvailableMinor, after.AvailableMinor, parseErr)
	}
	var reconciliation struct {
		Status            string `json:"status"`
		ExpectedMinor     string `json:"expected_minor"`
		PostedDebitMinor  string `json:"posted_debit_minor"`
		PostedCreditMinor string `json:"posted_credit_minor"`
	}
	getJSON(t, client, eventURL+"/reconciliation", &reconciliation)
	if reconciliation.Status != "matched" || reconciliation.ExpectedMinor != amountMinor || reconciliation.PostedDebitMinor != amountMinor || reconciliation.PostedCreditMinor != amountMinor {
		t.Fatalf("funding reconciliation=%+v", reconciliation)
	}
	var orientation struct {
		Steps []struct {
			ID         string `json:"id"`
			State      string `json:"state"`
			EvidenceID string `json:"evidence_id"`
		} `json:"steps"`
	}
	getJSON(t, client, baseURL+"/api/local/orientation", &orientation)
	if len(orientation.Steps) != 12 || orientation.Steps[4].ID != "fund_account" || orientation.Steps[4].State != "completed" || orientation.Steps[4].EvidenceID != created.Event.FundingEventID {
		t.Fatalf("funding onboarding evidence=%+v", orientation.Steps)
	}
}

func TestRealBFFAPIAndPostgreSQLRetryPath(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LEDGERSYNC_SYSTEM_WEB_URL"), "/")
	if baseURL == "" {
		t.Skip("LEDGERSYNC_SYSTEM_WEB_URL is required for the real-stack smoke test")
	}
	client, session := newAuthenticatedClient(t, baseURL)
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
	body := []byte(`{"sourceAccountId":"10000000-0000-4000-8000-000000000001","destinationAccountId":"10000000-0000-4000-8000-000000000004","amount":{"currency":"INR","minorUnits":"100"}}`)
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
	if afterFirstMinor != beforeMinor-100 || afterRetry.AvailableMinor != afterFirst.AvailableMinor {
		t.Fatalf("immediate balance or replay safety failed: before=%s after_first=%s after_retry=%s", before.AvailableMinor, afterFirst.AvailableMinor, afterRetry.AvailableMinor)
	}
	sourceID := "10000000-0000-4000-8000-000000000001"
	committed, ok := first.Balances[sourceID]
	if !ok || committed.PostedMinor != afterFirst.AvailableMinor || committed.Version == "" || first.MinimumBalanceVersions[sourceID] != committed.Version {
		t.Fatalf("authoritative transfer result and immediate balance disagree: committed=%+v minimum_version=%q visible=%s", committed, first.MinimumBalanceVersions[sourceID], afterFirst.AvailableMinor)
	}
}

func TestRealBFFSameKeyReplayAfterAPIProcessRestart(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LEDGERSYNC_SYSTEM_WEB_URL"), "/")
	key := os.Getenv("LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY")
	if baseURL == "" || key == "" || os.Getenv("LEDGERSYNC_SYSTEM_EXPECT_RESTART_REPLAY") != "true" {
		t.Skip("explicit post-restart system-test configuration is required for restart replay evidence")
	}
	client, session := newAuthenticatedClient(t, baseURL)
	var before balancePayload
	getJSON(t, client, baseURL+"/api/accounts/10000000-0000-4000-8000-000000000001/balance", &before)

	body := []byte(`{"sourceAccountId":"10000000-0000-4000-8000-000000000001","destinationAccountId":"10000000-0000-4000-8000-000000000004","amount":{"currency":"INR","minorUnits":"100"}}`)
	firstReplay := postTransfer(t, client, baseURL, session.CSRFToken, key, body)
	secondReplay := postTransfer(t, client, baseURL, session.CSRFToken, key, body)
	var after balancePayload
	getJSON(t, client, baseURL+"/api/accounts/10000000-0000-4000-8000-000000000001/balance", &after)

	if !firstReplay.Replayed || !secondReplay.Replayed || firstReplay.Status != "posted" || firstReplay.TransferID == "" || firstReplay.TransferID != secondReplay.TransferID {
		t.Fatalf("restart retry did not resolve to one stable replay: first=%+v second=%+v", firstReplay, secondReplay)
	}
	if before.AvailableMinor != after.AvailableMinor {
		t.Fatalf("restart replays changed the balance: before=%s after=%s", before.AvailableMinor, after.AvailableMinor)
	}
}

func TestRealBFFReconciliationEvidence(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LEDGERSYNC_SYSTEM_WEB_URL"), "/")
	if baseURL == "" {
		t.Skip("LEDGERSYNC_SYSTEM_WEB_URL is required for the real-stack smoke test")
	}
	client, _ := newAuthenticatedClient(t, baseURL)
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

func newAuthenticatedClient(t *testing.T, baseURL string) (*http.Client, sessionPayload) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	response, err := client.Get(baseURL + "/api/auth/sign-in?return_to=/")
	if err != nil {
		t.Fatalf("establish local test session: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		t.Fatalf("local test sign-in returned %d: %s", response.StatusCode, body)
	}
	var session sessionPayload
	getJSON(t, client, baseURL+"/api/session", &session)
	if session.CSRFToken == "" {
		t.Fatal("real BFF did not establish the server-gated local session")
	}
	return client, session
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

func postJSON(t *testing.T, client *http.Client, url, origin, csrfToken, idempotencyKey string, body []byte, target any) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	request.Header.Set("X-CSRF-Token", csrfToken)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := client.Do(request)
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
