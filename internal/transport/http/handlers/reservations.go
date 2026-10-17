package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	hold "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reservations"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type ReservationHandler struct {
	service                       *hold.Service
	identity                      identity.Provider
	authenticator                 *identity.RequestAuthenticator
	limiter                       RateLimiter
	readRate, writeRate, capacity int
	audit                         AuditRecorder
}

func NewReservationHandler(s *hold.Service, p identity.Provider) *ReservationHandler {
	return &ReservationHandler{service: s, identity: p}
}
func (h *ReservationHandler) WithRequestAuthenticator(a *identity.RequestAuthenticator) *ReservationHandler {
	h.authenticator = a
	return h
}
func (h *ReservationHandler) WithRateLimiter(l RateLimiter, read, write, capacity int) *ReservationHandler {
	h.limiter = l
	h.readRate = read
	h.writeRate = write
	h.capacity = capacity
	return h
}
func (h *ReservationHandler) WithAuditRecorder(a AuditRecorder) *ReservationHandler {
	h.audit = a
	return h
}
func (h *ReservationHandler) authorize(w http.ResponseWriter, r *http.Request, write bool) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		writeReservationError(w, r, hold.ErrUnavailable)
		return identity.Principal{}, false
	}
	method := http.MethodGet
	if write {
		method = http.MethodPost
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		httptransport.WriteError(w, r, &httptransport.PublicError{Status: 405, Code: "method_not_allowed", Message: "Use the documented method."})
		return identity.Principal{}, false
	}
	var p identity.Principal
	var err error
	assertion := r.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		p, err = h.authenticator.Authenticate(r.Context(), bearerToken(r.Header.Get("Authorization")), assertion)
	} else if assertion != "" {
		err = identity.ErrUnauthenticated
	} else {
		p, err = h.identity.Authenticate(r.Context(), bearerToken(r.Header.Get("Authorization")))
	}
	if err != nil {
		writeAuthenticationError(w, r, err)
		return p, false
	}
	scope, rate := "transfers:read", h.readRate
	if write {
		scope, rate = "transfers:write", h.writeRate
	}
	if identity.RequireScope(p, scope) != nil {
		writeScopeDenial(w, r, h.audit, p, scope)
		return p, false
	}
	if write && !enforceTenantCapacity(w, r, h.limiter, p, "reservations", h.capacity) {
		return p, false
	}
	if !enforceRateLimit(w, r, h.limiter, p, "reservations", rate, true) {
		return p, false
	}
	return p, true
}
func reservationScope(p identity.Principal) hold.Scope {
	return hold.Scope{TenantID: p.TenantID, ActorSubjectID: p.SubjectID}
}
func reservationCommandMeta(p identity.Principal, r *http.Request) hold.Meta {
	return hold.Meta{Scope: reservationScope(p), IdempotencyKey: r.Header.Get("Idempotency-Key"), CorrelationID: middleware.CorrelationID(r.Context())}
}
func decodeReservation(w http.ResponseWriter, r *http.Request, out any) error {
	if r.URL.RawQuery != "" {
		return hold.ErrInvalid
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &httptransport.PublicError{Status: 415, Code: "unsupported_media_type", Message: "Use application/json."}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return hold.ErrInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return hold.ErrInvalid
	}
	return nil
}
func writeReservationError(w http.ResponseWriter, r *http.Request, err error) {
	var public *httptransport.PublicError
	if errors.As(err, &public) {
		httptransport.WriteError(w, r, public)
		return
	}
	status, code, message := 503, "reservation_unavailable", "Reservation evidence is unavailable. Keep the original command."
	switch {
	case errors.Is(err, hold.ErrInvalid):
		status, code, message = 400, "invalid_request", "Use exact positive minor units, two distinct accounts and a valid expiry."
	case errors.Is(err, hold.ErrForbidden):
		status, code, message = 403, "forbidden", "Reservation access was denied."
	case errors.Is(err, hold.ErrNotFound):
		status, code, message = 404, "not_found", "Reservation evidence was not found."
	case errors.Is(err, hold.ErrConflict):
		status, code, message = 409, "reservation_conflict", "The original intent or current state conflicts. Read current evidence."
	case errors.Is(err, hold.ErrLimit):
		status, code, message = 422, "reservation_limit", "The active reservation limit was reached."
	}
	httptransport.WriteError(w, r, &httptransport.PublicError{Status: status, Code: code, Message: message})
}
func writeReservationJSON(w http.ResponseWriter, r *http.Request, value any, err error) {
	if err != nil {
		writeReservationError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}
func writeReservationReceipt(w http.ResponseWriter, r *http.Request, value hold.Receipt, err error) {
	if err == nil && value.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeReservationJSON(w, r, value, err)
}
func (h *ReservationHandler) Create(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	var body struct {
		DebitAccountID  string `json:"debit_account_id"`
		CreditAccountID string `json:"credit_account_id"`
		AmountMinor     string `json:"amount_minor"`
		Currency        string `json:"currency"`
		TTLSeconds      int    `json:"ttl_seconds"`
	}
	if err := decodeReservation(w, r, &body); err != nil {
		writeReservationError(w, r, err)
		return
	}
	value, err := h.service.Create(r.Context(), hold.Create{Meta: reservationCommandMeta(p, r), DebitAccountID: body.DebitAccountID, CreditAccountID: body.CreditAccountID, AmountMinor: body.AmountMinor, Currency: body.Currency, TTLSeconds: body.TTLSeconds})
	writeReservationReceipt(w, r, value, err)
}
func (h *ReservationHandler) Decide(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	var body struct {
		Decision string `json:"decision"`
	}
	if err := decodeReservation(w, r, &body); err != nil {
		writeReservationError(w, r, err)
		return
	}
	value, err := h.service.Decide(r.Context(), hold.Decide{Meta: reservationCommandMeta(p, r), HoldID: r.PathValue("holdID"), Decision: body.Decision})
	writeReservationReceipt(w, r, value, err)
}
func (h *ReservationHandler) Read(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeReservationError(w, r, hold.ErrInvalid)
		return
	}
	value, err := h.service.Read(r.Context(), reservationScope(p), r.PathValue("holdID"))
	writeReservationJSON(w, r, value, err)
}
func (h *ReservationHandler) List(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeReservationError(w, r, hold.ErrInvalid)
		return
	}
	for key, values := range query {
		if key != "cursor" || len(values) != 1 || values[0] == "" || len(values[0]) > 256 {
			writeReservationError(w, r, hold.ErrInvalid)
			return
		}
	}
	value, err := h.service.List(r.Context(), reservationScope(p), query.Get("cursor"))
	writeReservationJSON(w, r, value, err)
}
