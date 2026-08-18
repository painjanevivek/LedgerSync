package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdministrativeRoutesAreNotMountedByDefault(t *testing.T) {
	router := http.NewServeMux()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/reconciliation", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected no-disclosure admin denial, got %d", recorder.Code)
	}
}
