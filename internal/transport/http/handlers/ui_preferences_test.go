package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type uiPreferenceTestProvider struct{ authorized bool }

func (provider uiPreferenceTestProvider) Authenticate(context.Context, string) (identity.Principal, error) {
	if !provider.authorized {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return identity.Principal{
		TenantID:  "00000000-0000-4000-8000-000000000001",
		SubjectID: "operator-1",
		Roles:     map[string]struct{}{"tenant:operator": {}},
	}, nil
}

type uiPreferenceTestStore struct {
	preference db.UIPreference
	mode       string
	version    int64
	err        error
}

func (store *uiPreferenceTestStore) Get(context.Context, string, string) (db.UIPreference, error) {
	return store.preference, store.err
}

func (store *uiPreferenceTestStore) Update(_ context.Context, _, _ string, mode string, version int64) (db.UIPreference, error) {
	store.mode, store.version = mode, version
	return store.preference, store.err
}

func newUIPreferenceHTTPRequest(method, body string) *http.Request {
	request := httptest.NewRequest(method, "/api/internal/bff/ui-preferences", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer workload-token")
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestUIPreferenceHandlerDefaultsAndUpdatesExactScopedMode(t *testing.T) {
	store := &uiPreferenceTestStore{preference: db.UIPreference{ExperienceMode: "simple", Version: 0}}
	handler := NewUIPreferenceHandler(store, uiPreferenceTestProvider{authorized: true}, nil)

	read := httptest.NewRecorder()
	handler.Get(read, newUIPreferenceHTTPRequest(http.MethodGet, ""))
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"experience_mode":"simple"`) {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}

	store.preference = db.UIPreference{ExperienceMode: "expert", Version: 2}
	update := httptest.NewRecorder()
	handler.Patch(update, newUIPreferenceHTTPRequest(http.MethodPatch, `{"experience_mode":"expert","expected_version":"1"}`))
	if update.Code != http.StatusOK || store.mode != "expert" || store.version != 1 {
		t.Fatalf("update status=%d mode=%q version=%d body=%s", update.Code, store.mode, store.version, update.Body.String())
	}
}

func TestUIPreferenceHandlerFailsClosed(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		provider uiPreferenceTestProvider
		body     string
		storeErr error
		status   int
	}{
		{name: "unauthenticated", body: `{"experience_mode":"simple","expected_version":"0"}`, status: http.StatusUnauthorized},
		{name: "unknown mode", provider: uiPreferenceTestProvider{authorized: true}, body: `{"experience_mode":"admin","expected_version":"0"}`, status: http.StatusBadRequest},
		{name: "unknown field", provider: uiPreferenceTestProvider{authorized: true}, body: `{"experience_mode":"simple","expected_version":"0","tenant_id":"other"}`, status: http.StatusBadRequest},
		{name: "storage unavailable", provider: uiPreferenceTestProvider{authorized: true}, body: `{"experience_mode":"simple","expected_version":"0"}`, storeErr: errors.New("database secret detail"), status: http.StatusInternalServerError},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := &uiPreferenceTestStore{err: scenario.storeErr}
			response := httptest.NewRecorder()
			NewUIPreferenceHandler(store, scenario.provider, nil).Patch(response, newUIPreferenceHTTPRequest(http.MethodPatch, scenario.body))
			if response.Code != scenario.status || strings.Contains(response.Body.String(), "database secret detail") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
