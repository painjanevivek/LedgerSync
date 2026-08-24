package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

func TestRegisterAccountCommandRoutesAddsOnlyPostAndPatchSeams(t *testing.T) {
	router := http.NewServeMux()
	err := registerAccountCommandRoutes(router, accountCommandRouteConfig{
		Database: &sql.DB{},
		Identity: identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant", Scopes: []string{"accounts:write"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/accounts"},
		{method: http.MethodPatch, path: "/api/accounts/account-1"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d body=%s", testCase.method, testCase.path, response.Code, response.Body.String())
		}
	}
}

func TestRegisterAccountCommandRoutesValidatesCompositionInputs(t *testing.T) {
	if err := registerAccountCommandRoutes(nil, accountCommandRouteConfig{}); err == nil {
		t.Fatal("nil router was accepted")
	}
	if err := registerAccountCommandRoutes(http.NewServeMux(), accountCommandRouteConfig{}); err == nil {
		t.Fatal("nil database was accepted")
	}
}
