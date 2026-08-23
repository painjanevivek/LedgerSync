package system_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"
	"time"
)

type sessionPayload struct {
	CSRFToken string `json:"csrf_token"`
}

type transferPayload struct {
	TransferID      string `json:"transfer_id"`
	Status          string `json:"status"`
	FinancialStatus string `json:"financial_status"`
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
		Accounts []json.RawMessage `json:"accounts"`
	}
	getJSON(t, client, baseURL+"/api/me/accounts", &accounts)
	if len(accounts.Accounts) < 2 {
		t.Fatalf("real API returned %d authorized accounts, want at least 2", len(accounts.Accounts))
	}

	body := []byte(`{"sourceAccountId":"10000000-0000-4000-8000-000000000001","destinationAccountId":"10000000-0000-4000-8000-000000000004","amount":{"currency":"USD","minorUnits":"1"}}`)
	key := "system-stack-idempotency-00000001"
	first := postTransfer(t, client, baseURL, session.CSRFToken, key, body)
	second := postTransfer(t, client, baseURL, session.CSRFToken, key, body)
	if first.TransferID == "" || first.TransferID != second.TransferID || first.Status != "posted" || second.Status != "posted" {
		t.Fatalf("retry did not return one stable posted transfer: first=%+v second=%+v", first, second)
	}
	var detail transferPayload
	getJSON(t, client, baseURL+"/api/transfers/"+first.TransferID, &detail)
	if detail.TransferID != first.TransferID || detail.FinancialStatus != "posted" {
		t.Fatalf("immutable transfer detail is inconsistent: %+v", detail)
	}
}

func getJSON(t *testing.T, client *http.Client, url string, target any) {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
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
	defer response.Body.Close()
	var payload transferPayload
	decodeResponse(t, response, &payload)
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
