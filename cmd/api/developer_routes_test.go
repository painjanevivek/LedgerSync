package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

func TestRegisterDeveloperRoutesExposesOnlyBoundedGETAssets(t *testing.T) {
	router := http.NewServeMux()
	err := registerDeveloperRoutes(router, developerRouteConfig{Identity: identity.DevelopmentProvider{
		SubjectID: "developer", TenantID: "tenant-a", Scopes: []string{"developer:read"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/developer/metadata", "/api/openapi.yaml"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer development-local-only")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/api/developer/distributed", "/api/developer/distributed/evidence"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("default optional endpoint %s status=%d", path, response.Code)
		}
	}
	post := httptest.NewRequest(http.MethodPost, "/api/developer/metadata", nil)
	postResponse := httptest.NewRecorder()
	router.ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected developer write surface status=%d", postResponse.Code)
	}
}
