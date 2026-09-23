package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type failingConsistencyIssuer struct{}

func (failingConsistencyIssuer) Issue(string, string, int64) (string, error) {
	return "", errors.New("injected token issuer failure")
}

const (
	consistencyTenant      = "00000000-0000-4000-8000-000000000001"
	consistencySource      = "10000000-0000-4000-8000-000000000001"
	consistencyDestination = "10000000-0000-4000-8000-000000000002"
)

type consistencyTransferRepository struct{}

func (consistencyTransferRepository) Submit(context.Context, transfers.Command, [sha256.Size]byte) (transfers.Submission, error) {
	return transfers.Submission{Result: transfers.Result{
		TransferID:  "20000000-0000-4000-8000-000000000001",
		Status:      "posted",
		Currency:    "INR",
		AmountMinor: 123,
		OccurredAt:  "2026-08-24T14:00:00Z",
		MinimumBalanceVersions: map[string]int64{
			consistencySource: 42,
		},
		Balances: map[string]transfers.Balance{
			consistencySource: {AccountID: consistencySource, Currency: "INR", PostedMinor: 999_877, Version: 42, AsOf: "2026-08-24T14:00:00Z"},
		},
	}}, nil
}

type consistencyDestinationReader struct{ authorized bool }

func (r consistencyDestinationReader) ReadCurrent(_ context.Context, tenantID, actorID, accountID string) (accounts.Balance, error) {
	if !r.authorized || tenantID != consistencyTenant || actorID != "demo-operator" || accountID != consistencyDestination {
		return accounts.Balance{}, accounts.ErrCurrentBalanceUnavailable
	}
	return accounts.Balance{TenantID: tenantID, AccountID: accountID, Currency: "INR", Version: 77}, nil
}

func TestPostedTransferPrivatelyCarriesOwnedDestinationConsistency(t *testing.T) {
	issuer, err := consistency.NewIssuer(consistency.Key{ID: "test", Secret: []byte("01234567890123456789012345678901")}, nil, nil, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := transfers.NewService(consistencyTransferRepository{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewTransferHandler(service, identity.DevelopmentProvider{
		SubjectID: "demo-operator", TenantID: consistencyTenant, Scopes: []string{"transfers:write"},
	}, issuer).WithConsistencyBalanceReader(consistencyDestinationReader{authorized: true})

	request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(`{
"source_account_id":"10000000-0000-4000-8000-000000000001",
"destination_account_id":"10000000-0000-4000-8000-000000000002",
"amount":"1.23","currency":"INR"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", strings.Join([]string{"owned", "destination", "consistency", "001"}, "-"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var requirements map[string]string
	if err := json.Unmarshal([]byte(recorder.Header().Get("X-LedgerSync-Consistency-Requirements")), &requirements); err != nil {
		t.Fatalf("decode private consistency requirements: %v", err)
	}
	if len(requirements) != 2 {
		t.Fatalf("requirements=%v, want source and owned destination", requirements)
	}
	for accountID, wantVersion := range map[string]int64{consistencySource: 42, consistencyDestination: 77} {
		requirement, err := issuer.Verify(requirements[accountID])
		if err != nil {
			t.Fatalf("verify requirement for %s: %v", accountID, err)
		}
		if requirement.AccountID != accountID || requirement.MinimumVersion != wantVersion {
			t.Fatalf("requirement=%+v, want account=%s version=%d", requirement, accountID, wantVersion)
		}
	}
	if strings.Contains(recorder.Body.String(), consistencyDestination) || strings.Contains(recorder.Body.String(), `"77"`) {
		t.Fatalf("public result disclosed destination evidence: %s", recorder.Body.String())
	}
}

func TestPostedTransferOmitsUnreadableDestinationConsistency(t *testing.T) {
	issuer, err := consistency.NewIssuer(consistency.Key{ID: "test", Secret: []byte("01234567890123456789012345678901")}, nil, nil, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := transfers.NewService(consistencyTransferRepository{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewTransferHandler(service, identity.DevelopmentProvider{
		SubjectID: "demo-operator", TenantID: consistencyTenant, Scopes: []string{"transfers:write"},
	}, issuer).WithConsistencyBalanceReader(consistencyDestinationReader{})
	request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(`{
"source_account_id":"10000000-0000-4000-8000-000000000001",
"destination_account_id":"10000000-0000-4000-8000-000000000002",
"amount":"1.23","currency":"INR"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", strings.Join([]string{"credit", "only", "destination", "consistency", "001"}, "-"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	var requirements map[string]string
	if err := json.Unmarshal([]byte(recorder.Header().Get("X-LedgerSync-Consistency-Requirements")), &requirements); err != nil {
		t.Fatal(err)
	}
	if len(requirements) != 1 || requirements[consistencySource] == "" || requirements[consistencyDestination] != "" {
		t.Fatalf("unreadable destination requirement leaked: %v", requirements)
	}
}

func TestPostedTransferRetainsCommittedSuccessWhenConsistencyIssuanceFails(t *testing.T) {
	service, err := transfers.NewService(consistencyTransferRepository{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewTransferHandler(service, identity.DevelopmentProvider{
		SubjectID: "demo-operator", TenantID: consistencyTenant, Scopes: []string{"transfers:write"},
	}).WithConsistencyIssuer(failingConsistencyIssuer{})
	request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(`{
"source_account_id":"10000000-0000-4000-8000-000000000001",
"destination_account_id":"10000000-0000-4000-8000-000000000002",
"amount":"1.23","currency":"INR"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", "post-commit-token-failure-001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		TransferID     string   `json:"transfer_id"`
		MetadataStatus string   `json:"metadata_status"`
		Warnings       []string `json:"warnings"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TransferID == "" || body.MetadataStatus != "unavailable" || len(body.Warnings) == 0 {
		t.Fatalf("body=%+v", body)
	}
	if recorder.Header().Get("X-LedgerSync-Consistency-Requirements") != "" {
		t.Fatal("failed optional metadata was exposed as a consistency guarantee")
	}
}

func TestPostedTransferRetainsCommittedSuccessWhenConsistencyHeaderEncodingFails(t *testing.T) {
	issuer, err := consistency.NewIssuer(consistency.Key{ID: "test", Secret: []byte("01234567890123456789012345678901")}, nil, nil, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := transfers.NewService(consistencyTransferRepository{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewTransferHandler(service, identity.DevelopmentProvider{
		SubjectID: "demo-operator", TenantID: consistencyTenant, Scopes: []string{"transfers:write"},
	}, issuer).WithConsistencyMetadataEncoder(func(map[string]string) (string, error) {
		return "", errors.New("injected header encoding failure")
	})
	request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(`{
"source_account_id":"10000000-0000-4000-8000-000000000001",
"destination_account_id":"10000000-0000-4000-8000-000000000002",
"amount":"1.23","currency":"INR"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", "post-commit-header-failure-001")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated || recorder.Header().Get("X-LedgerSync-Metadata-Status") != "unavailable" {
		t.Fatalf("status=%d metadata=%q body=%s", recorder.Code, recorder.Header().Get("X-LedgerSync-Metadata-Status"), recorder.Body.String())
	}
	if recorder.Header().Get("X-LedgerSync-Consistency-Requirements") != "" {
		t.Fatal("failed header encoding was exposed as a consistency guarantee")
	}
}
