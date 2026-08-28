package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	contractassets "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/contracts"
)

func TestContractHeadersUseTrustedEnvironment(t *testing.T) {
	for _, testCase := range []struct {
		name, environment, wantMode string
	}{
		{name: "development", environment: "development", wantMode: "sandbox"},
		{name: "test", environment: "test", wantMode: "sandbox"},
		{name: "production", environment: "production", wantMode: "production"},
		{name: "unknown fails closed", environment: "staging", wantMode: "production"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler := Contract(testCase.environment, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			request.Header.Set("X-LedgerSync-Mode", "sandbox")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if got := response.Header().Get("X-LedgerSync-Mode"); got != testCase.wantMode {
				t.Fatalf("mode=%q want=%q", got, testCase.wantMode)
			}
			if got := response.Header().Get("X-LedgerSync-API-Version"); got != contractassets.Version {
				t.Fatalf("version=%q want=%q", got, contractassets.Version)
			}
		})
	}
}
