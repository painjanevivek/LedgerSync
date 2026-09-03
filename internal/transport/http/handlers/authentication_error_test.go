package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

func TestAuthenticationErrorPreservesCredentialAndDependencySemantics(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		publicCode string
	}{
		{name: "invalid credentials", err: identity.ErrUnauthenticated, status: http.StatusUnauthorized, publicCode: "unauthorized"},
		{name: "replay store unavailable", err: identity.ErrAuthenticationUnavailable, status: http.StatusServiceUnavailable, publicCode: "temporary_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeAuthenticationError(response, httptest.NewRequest(http.MethodGet, "/api/me/accounts", nil), test.err)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.publicCode+`"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), test.err.Error()) && !errors.Is(test.err, identity.ErrUnauthenticated) {
				t.Fatal("dependency detail escaped the sanitized authentication response")
			}
		})
	}
}
