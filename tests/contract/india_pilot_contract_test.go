package contract_test

import (
	"os"
	"strings"
	"testing"
)

func TestSupportedPilotArtifactsUseApprovedINRBoundary(t *testing.T) {
	paths := []string{
		"../../.env.example",
		"../../deploy/compose/docker-compose.yml",
		"../../deploy/compose/demo-seed.sql",
		"../../contracts/openapi.yaml",
		"../../docs/api-guide.md",
		"../../docs/product/pilot-decision-register.md",
		"../system/real_stack_test.go",
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "INR") {
			t.Fatalf("%s does not expose the approved INR pilot boundary", path)
		}
	}
}
