package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type sessionTestProvider struct{ allowed bool }

func (p sessionTestProvider) Authenticate(context.Context, string) (identity.Principal, error) {
	if !p.allowed {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return identity.Principal{TenantID: "00000000-0000-4000-8000-000000000001", Scopes: map[string]struct{}{identity.BFFActorScope: {}}}, nil
}

type sessionTestStore struct {
	created db.SessionRecord
	rotated string
	revoked string
	updated map[string]string
	record  db.SessionRecord
	err     error
}

func (s *sessionTestStore) Create(_ context.Context, record db.SessionRecord, rotate string) (string, error) {
	s.created, s.rotated = record, rotate
	return strings.Repeat("a", 43), s.err
}
func (s *sessionTestStore) Resolve(context.Context, string) (db.SessionRecord, error) {
	return s.record, s.err
}
func (s *sessionTestStore) Revoke(_ context.Context, token string) error {
	s.revoked = token
	return s.err
}
func (s *sessionTestStore) UpdateConsistency(_ context.Context, _ string, values map[string]string) error {
	s.updated = values
	return s.err
}

func sessionRequestForTest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer workload-token")
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestSessionHandlerCreatesBoundedRotatingOpaqueSession(t *testing.T) {
	store := &sessionTestStore{}
	handler := NewSessionHandler(store, sessionTestProvider{allowed: true})
	expires := time.Now().Add(20 * time.Minute).UnixMilli()
	body := `{"rotate_token":"` + strings.Repeat("b", 43) + `","subject_id":"operator-1","tenant_id":"00000000-0000-4000-8000-000000000001","csrf_token":"csrf-token-long-enough","expires_at":` + strconv.FormatInt(expires, 10) + `}`
	response := httptest.NewRecorder()
	handler.Create(response, sessionRequestForTest(http.MethodPost, "/api/internal/bff/sessions", body))
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if store.created.SubjectID != "operator-1" || store.created.TenantID != "00000000-0000-4000-8000-000000000001" || store.rotated != strings.Repeat("b", 43) {
		t.Fatalf("unexpected persisted session: %+v rotate=%q", store.created, store.rotated)
	}
}

func TestSessionHandlerFailsClosedForCallerAndPayload(t *testing.T) {
	valid := `{"subject_id":"operator-1","tenant_id":"00000000-0000-4000-8000-000000000001","csrf_token":"csrf-token-long-enough","expires_at":` + strconv.FormatInt(time.Now().Add(20*time.Minute).UnixMilli(), 10) + `}`
	for _, scenario := range []struct {
		name     string
		provider sessionTestProvider
		body     string
		status   int
	}{
		{name: "unauthenticated workload", provider: sessionTestProvider{}, body: valid, status: http.StatusUnauthorized},
		{name: "unknown field", provider: sessionTestProvider{allowed: true}, body: strings.TrimSuffix(valid, "}") + `,"admin":true}`, status: http.StatusBadRequest},
		{name: "invalid tenant", provider: sessionTestProvider{allowed: true}, body: strings.Replace(valid, "00000000-0000-4000-8000-000000000001", "tenant-a", 1), status: http.StatusBadRequest},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			NewSessionHandler(&sessionTestStore{}, scenario.provider).Create(response, sessionRequestForTest(http.MethodPost, "/api/internal/bff/sessions", scenario.body))
			if response.Code != scenario.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestSessionHandlerRevocationAndConsistencyAreOpaqueAndBounded(t *testing.T) {
	token := strings.Repeat("a", 43)
	store := &sessionTestStore{}
	handler := NewSessionHandler(store, sessionTestProvider{allowed: true})
	revoke := httptest.NewRecorder()
	handler.Revoke(revoke, sessionRequestForTest(http.MethodPost, "/api/internal/bff/sessions/revoke", `{"token":"`+token+`"}`))
	if revoke.Code != http.StatusNoContent || store.revoked != token {
		t.Fatalf("revoke status=%d token=%q", revoke.Code, store.revoked)
	}
	update := httptest.NewRecorder()
	handler.UpdateConsistency(update, sessionRequestForTest(http.MethodPatch, "/api/internal/bff/sessions/consistency", `{"token":"`+token+`","consistency_requirements":{"account-1":"requirement-1"}}`))
	if update.Code != http.StatusNoContent || store.updated["account-1"] != "requirement-1" {
		t.Fatalf("update status=%d values=%v", update.Code, store.updated)
	}
	store.err = errors.New("database unavailable")
	failure := httptest.NewRecorder()
	handler.Resolve(failure, sessionRequestForTest(http.MethodPost, "/api/internal/bff/sessions/resolve", `{"token":"`+token+`"}`))
	if failure.Code != http.StatusInternalServerError || strings.Contains(failure.Body.String(), "database unavailable") {
		t.Fatalf("status=%d body=%s", failure.Code, failure.Body.String())
	}
}
