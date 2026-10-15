package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

const recoveryReference = "50000000-0000-4000-8000-000000000001"

func TestRecoveryRoutesCoexistWithCorrections(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/transfers/{transferID}/corrections", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("POST /api/transfers/recovery/{requestReference}/retry", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /api/transfers/{transferID}", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /api/transfers/recovery", func(http.ResponseWriter, *http.Request) {})
}

type recoveryFixture struct {
	prepared      int
	submitted     []transfers.Command
	tenant, actor string
}

func (r *recoveryFixture) PrepareRecovery(_ context.Context, c transfers.Command) error {
	r.prepared++
	return nil
}
func (r *recoveryFixture) GetRecovery(_ context.Context, tenant, actor, ref string) (transfers.RecoveryIntent, error) {
	r.tenant = tenant
	r.actor = actor
	return transfers.RecoveryIntent{RequestReference: ref, IdempotencyKey: "original-recovery-key-001", SourceAccountID: consistencySource, DestinationAccountID: consistencyDestination, Currency: "INR", AmountMinor: "123", State: "unresolved"}, nil
}
func (r *recoveryFixture) ListRecovery(ctx context.Context, tenant, actor, cursor string) (transfers.RecoveryPage, error) {
	item, _ := r.GetRecovery(ctx, tenant, actor, recoveryReference)
	return transfers.RecoveryPage{Requests: []transfers.RecoveryIntent{item}}, nil
}
func (r *recoveryFixture) Submit(ctx context.Context, c transfers.Command, f [sha256.Size]byte) (transfers.Submission, error) {
	r.submitted = append(r.submitted, c)
	return consistencyTransferRepository{}.Submit(ctx, c, f)
}
func recoveryHandler(t *testing.T, scopes ...string) (*TransferHandler, *recoveryFixture) {
	t.Helper()
	repo := &recoveryFixture{}
	service, err := transfers.NewService(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewTransferHandler(service, identity.DevelopmentProvider{SubjectID: "demo-operator", TenantID: consistencyTenant, Scopes: scopes}).WithRecoveryRepository(repo), repo
}
func recoveryRequest(body string) *http.Request {
	r := httptest.NewRequest("POST", "/api/transfers/recovery/"+recoveryReference, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer development-local-only")
	r.SetPathValue("requestReference", recoveryReference)
	return r
}

func TestRecoveryUsesOnlyServerIdentity(t *testing.T) {
	h, repo := recoveryHandler(t, "transfers:write")
	r := recoveryRequest("")
	r.Header.Set("Idempotency-Key", "attacker-replacement-key")
	r.Header.Set("X-LedgerSync-Request-Reference", "50000000-0000-4000-8000-000000000099")
	w := httptest.NewRecorder()
	h.Recover(w, r)
	if w.Code != 201 || len(repo.submitted) != 1 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	c := repo.submitted[0]
	if c.IdempotencyKey != "original-recovery-key-001" || c.RequestReference != recoveryReference || c.Amount.Minor() != 123 || c.DebitAccountID.String() != consistencySource || repo.actor != "demo-operator" || repo.tenant != consistencyTenant {
		t.Fatalf("changed recovery command: %#v", c)
	}
}
func TestRecoveryRejectsReplacementBodyAndMissingScope(t *testing.T) {
	for _, body := range []string{`{"amount":"9.00"}`, " "} {
		h, repo := recoveryHandler(t, "transfers:write")
		w := httptest.NewRecorder()
		h.Recover(w, recoveryRequest(body))
		if w.Code != 400 || len(repo.submitted) != 0 {
			t.Fatalf("body=%q status=%d", body, w.Code)
		}
	}
	h, repo := recoveryHandler(t, "transfers:read")
	w := httptest.NewRecorder()
	h.Recover(w, recoveryRequest(""))
	if w.Code != 403 || len(repo.submitted) != 0 {
		t.Fatalf("scope denial=%d", w.Code)
	}
}
func TestRecoveryListNeverSerializesKey(t *testing.T) {
	h, repo := recoveryHandler(t, "transfers:read")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/transfers/recovery", nil)
	r.Header.Set("Authorization", "Bearer development-local-only")
	h.ListRecovery(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), "original-recovery-key") || strings.Contains(w.Body.String(), "idempotency") {
		t.Fatalf("unsafe list=%s", w.Body.String())
	}
	if repo.actor != "demo-operator" {
		t.Fatal("missing actor scope")
	}
}
func TestRecoveryPrepareDoesNotPost(t *testing.T) {
	h, repo := recoveryHandler(t, "transfers:write")
	input := createTransferRequest{SourceAccountID: consistencySource, DestinationAccountID: consistencyDestination, Currency: "INR", Amount: "1.23"}
	b, _ := json.Marshal(input)
	r := httptest.NewRequest("POST", "/api/transfers/recovery", strings.NewReader(string(b)))
	r.Header.Set("Authorization", "Bearer development-local-only")
	r.Header.Set("Idempotency-Key", "original-recovery-key-001")
	r.Header.Set("X-LedgerSync-Request-Reference", recoveryReference)
	w := httptest.NewRecorder()
	h.PrepareRecovery(w, r)
	if w.Code != 201 || repo.prepared != 1 || len(repo.submitted) != 0 {
		t.Fatalf("prepare status=%d body=%s", w.Code, w.Body.String())
	}
}
