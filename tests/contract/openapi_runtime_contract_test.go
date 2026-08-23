package contract_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenAPIContainsEveryRegisteredMVPRouteAndLosslessBoundaries(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate contract test")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "contracts", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	contract := string(content)
	for _, route := range []string{"/me/accounts:", "/accounts/{accountId}:", "/accounts/{accountId}/balance:", "/accounts/{accountId}/transactions:", "/transfers:", "/transfers/{transferId}:", "/reconciliation/runs:", "/reconciliation/runs/{runId}:"} {
		if !strings.Contains(contract, route) {
			t.Errorf("OpenAPI missing runtime route %s", route)
		}
	}
	for _, marker := range []string{"ExactMinor: { type: string", "ExactVersion: { type: string", "'429':", "Retry-After:", "identifier: Apache-2.0", "unknown outcome"} {
		if !strings.Contains(contract, marker) {
			t.Errorf("OpenAPI missing contract marker %q", marker)
		}
	}
}
