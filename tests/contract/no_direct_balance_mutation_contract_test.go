package contract_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNoDirectBalanceMutationEndpointOrOperatorControlExists(t *testing.T) {
	root := repositoryRoot(t)
	assertPrivateBalanceRouteIsReadOnly(t, filepath.Join(root, "web", "src", "app", "api", "accounts", "[accountId]", "balance", "route.ts"))
	assertOpenAPIBalancePathIsReadOnly(t, filepath.Join(root, "contracts", "openapi.yaml"))
	assertGoRouterHasNoBalanceMutation(t, filepath.Join(root, "cmd", "api"))
	assertNoBalanceEditorControl(t, filepath.Join(root, "web", "src"))
}

func assertPrivateBalanceRouteIsReadOnly(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutation := regexp.MustCompile(`(?m)\b(?:export\s+(?:async\s+)?function\s+|as\s+)(POST|PATCH|PUT|DELETE)\b`)
	if match := mutation.Find(content); match != nil {
		t.Fatalf("private balance BFF route exports forbidden mutation method %q", match)
	}
	if !regexp.MustCompile(`(?m)\bGET\b`).Match(content) {
		t.Fatal("private balance BFF route lost its read-only GET contract")
	}
}

func assertOpenAPIBalancePathIsReadOnly(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contract := strings.ReplaceAll(string(content), "\r\n", "\n")
	start := strings.Index(contract, "  /accounts/{accountId}/balance:\n")
	if start < 0 {
		t.Fatal("OpenAPI balance path is missing")
	}
	remainder := contract[start+1:]
	end := strings.Index(remainder, "\n  /")
	if end >= 0 {
		remainder = remainder[:end]
	}
	if regexp.MustCompile(`(?m)^    (post|patch|put|delete):`).MatchString(remainder) {
		t.Fatalf("OpenAPI exposes a forbidden direct balance mutation:\n%s", remainder)
	}
	if !regexp.MustCompile(`(?m)^    get:`).MatchString(remainder) {
		t.Fatal("OpenAPI balance path lost GET")
	}
}

func assertGoRouterHasNoBalanceMutation(t *testing.T, root string) {
	t.Helper()
	mutation := regexp.MustCompile(`(?:POST|PATCH|PUT|DELETE) /api/(?:accounts/\{accountID\}/balance|balances(?:/|"))`)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if match := mutation.Find(content); match != nil {
			t.Errorf("Go router %s exposes forbidden direct balance mutation %q", path, match)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertNoBalanceEditorControl(t *testing.T, root string) {
	t.Helper()
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(edit|set|adjust|override|change)\s+(?:account\s+)?balance\b|\bbalance\s+(editor|input)\b`),
		regexp.MustCompile(`(?is)<label[^>]*>[^<]{0,80}\bbalance\b[^<]{0,80}<(?:input|textarea|select)\b`),
		regexp.MustCompile(`(?is)<(?:input|textarea|select)[^>]*(?:aria-label|id|name)=["'][^"']*\bbalance\b[^"']*["']`),
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".tsx" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, pattern := range forbidden {
			if match := pattern.Find(content); match != nil {
				t.Errorf("operator UI %s contains forbidden direct-balance control %q", path, match)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
