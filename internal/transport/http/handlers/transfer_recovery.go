package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

func (h *TransferHandler) WithRecoveryRepository(repository transfers.RecoveryRepository) *TransferHandler {
	h.recovery = repository
	return h
}

func (h *TransferHandler) recoveryPrincipal(w http.ResponseWriter, r *http.Request, scope string) (identity.Principal, bool) {
	if h.recovery == nil {
		httptransport.WriteError(w, r, errors.New("recovery is unavailable"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(r)
	if err != nil {
		writeAuthenticationError(w, r, err)
		return principal, false
	}
	if identity.RequireScope(principal, scope) != nil {
		writeScopeDenial(w, r, h.audit, principal, scope)
		return principal, false
	}
	if !enforceRateLimit(w, r, h.rateLimiter, principal, "transfers:recovery", h.rateLimit, r.Method == http.MethodPost) {
		return principal, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return principal, true
}

func (h *TransferHandler) PrepareRecovery(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.recoveryPrincipal(w, r, "transfers:write")
	if !ok {
		return
	}
	input, err := decodeTransferRequest(w, r)
	if err != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	amount, err := money.Parse(input.Currency, input.Amount)
	tenant, tenantOK := parseIdentifier(r, identifier.KindTenant, principal.TenantID)
	source, sourceOK := parseIdentifier(r, identifier.KindAccount, input.SourceAccountID)
	destination, destinationOK := parseIdentifier(r, identifier.KindAccount, input.DestinationAccountID)
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	reference := strings.ToLower(strings.TrimSpace(r.Header.Get("X-LedgerSync-Request-Reference")))
	if err != nil || !amount.IsPositive() || !tenantOK || !sourceOK || !destinationOK || source == destination || transfers.ValidateKey(key) != nil || !transfers.CanonicalRequestReference(reference) {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	err = h.recovery.PrepareRecovery(r.Context(), transfers.Command{TenantID: tenant, ActorSubjectID: principal.SubjectID,
		DebitAccountID: source, CreditAccountID: destination, Amount: amount, IdempotencyKey: key, RequestReference: reference})
	if err != nil {
		httptransport.WriteError(w, r, publicTransferError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"request_reference": reference, "state": "saved", "message": "Intent saved. No funds reserved or moved."})
}

func (h *TransferHandler) ListRecovery(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.recoveryPrincipal(w, r, "transfers:read")
	if !ok {
		return
	}
	for key, values := range r.URL.Query() {
		if key != "cursor" || len(values) != 1 || values[0] == "" || len(values[0]) > 2048 {
			httptransport.WriteError(w, r, httptransport.ErrBadRequest)
			return
		}
	}
	page, err := h.recovery.ListRecovery(r.Context(), principal.TenantID, principal.SubjectID, r.URL.Query().Get("cursor"))
	if err != nil {
		httptransport.WriteError(w, r, publicTransferError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}

func (h *TransferHandler) Recover(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.recoveryPrincipal(w, r, "transfers:write")
	if !ok {
		return
	}
	reference := r.PathValue("requestReference")
	if !transfers.CanonicalRequestReference(reference) || len(r.URL.Query()) != 0 {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	// Recovery accepts no replacement body, accounts, amount, or retry identity.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
	if err != nil || len(body) != 0 {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	item, err := h.recovery.GetRecovery(r.Context(), principal.TenantID, principal.SubjectID, reference)
	if errors.Is(err, db.ErrInvestigationNotFound) {
		httptransport.WriteError(w, r, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		httptransport.WriteError(w, r, err)
		return
	}
	amount, err := strconv.ParseInt(item.AmountMinor, 10, 64)
	if err != nil {
		httptransport.WriteError(w, r, err)
		return
	}
	exact, err := money.New(item.Currency, amount)
	if err != nil {
		httptransport.WriteError(w, r, err)
		return
	}
	encoded, _ := json.Marshal(createTransferRequest{SourceAccountID: item.SourceAccountID, DestinationAccountID: item.DestinationAccountID, Currency: item.Currency, Amount: strings.TrimPrefix(exact.String(), item.Currency+" ")})
	recovered := r.Clone(r.Context())
	recovered.Header = r.Header.Clone()
	recovered.Body = io.NopCloser(bytes.NewReader(encoded))
	recovered.Header.Set("Idempotency-Key", item.IdempotencyKey)
	recovered.Header.Set("X-LedgerSync-Request-Reference", item.RequestReference)
	// Already authenticated: do not consume the single-use actor assertion twice.
	h.submitAuthenticated(w, recovered, principal)
}
