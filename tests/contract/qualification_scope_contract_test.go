package contract_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const supportedGoPackageCommand = "./cmd/... ./contracts/... ./internal/... ./tests/..."

func TestQualificationUsesOnlySupportedGoPackageRoots(t *testing.T) {
	root := repositoryRoot(t)
	workflow := readContractFile(t, filepath.Join(root, ".github", "workflows", "security.yml"))
	script := readContractFile(t, filepath.Join(root, "scripts", "run-security-supply-chain-qualification.ps1"))
	broadRootPattern := regexp.MustCompile(`(?:^|[\s'",(])\./\.\.\.(?:$|[\s'",)])`)

	for path, content := range map[string]string{
		".github/workflows/security.yml":                      workflow,
		"scripts/run-security-supply-chain-qualification.ps1": script,
	} {
		if broadRootPattern.MatchString(content) {
			t.Errorf("%s contains broad root Go package discovery", path)
		}
	}
	if !strings.Contains(workflow, supportedGoPackageCommand) {
		t.Fatalf("security workflow does not scan the explicit supported Go package roots: %s", supportedGoPackageCommand)
	}
	for _, packageRoot := range strings.Fields(supportedGoPackageCommand) {
		if !strings.Contains(script, "'"+packageRoot+"'") {
			t.Errorf("security qualification script is missing supported package root %q", packageRoot)
		}
	}
}

func TestRealStackQualificationSelectsDemoFixturesWithoutChangingLocalDefault(t *testing.T) {
	root := repositoryRoot(t)
	workflow := readContractFile(t, filepath.Join(root, ".github", "workflows", "quality.yml"))
	compose := readContractFile(t, filepath.Join(root, "deploy", "compose", "docker-compose.yml"))

	if !strings.Contains(workflow, "LEDGERSYNC_INITIALIZATION_MODE: demo") {
		t.Fatal("the disposable real-stack qualification must explicitly select its deterministic demo fixture")
	}
	if !strings.Contains(compose, "LEDGERSYNC_INITIALIZATION_MODE:-empty") {
		t.Fatal("the local runtime must remain empty by default")
	}
}
