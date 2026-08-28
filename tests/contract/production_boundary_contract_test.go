package contract_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultComposeDelegatesToSupportedTopology(t *testing.T) {
	root := repositoryRoot(t)
	defaultCompose := readContractFile(t, filepath.Join(root, "docker-compose.yml"))
	if !strings.Contains(defaultCompose, "deploy/compose/docker-compose.yml") {
		t.Fatal("default Compose entry point does not delegate to the supported topology")
	}
	legacyCompose := readContractFile(t, filepath.Join(root, "docker-compose.legacy-demo.yml"))
	if !strings.Contains(legacyCompose, "LEGACY DEMONSTRATION REFERENCE ONLY") || !strings.Contains(legacyCompose, "ledgersync-legacy-demo-do-not-use") {
		t.Fatal("legacy Compose topology is not unmistakably isolated")
	}
}

func TestProductionDeploymentFilesDoNotReferenceLegacyApplications(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{
		filepath.Join(root, "deploy", "compose", "docker-compose.yml"),
		filepath.Join(root, "deploy", "docker", "api.Dockerfile"),
		filepath.Join(root, "deploy", "docker", "outbox-worker.Dockerfile"),
		filepath.Join(root, "deploy", "docker", "web.Dockerfile"),
	}
	for _, path := range paths {
		content := readContractFile(t, path)
		for _, forbidden := range []string{"backend/", "dashboard/", "simulation/", "docker-compose.legacy-demo.yml"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("production deployment file %s references legacy path %q", filepath.Base(path), forbidden)
			}
		}
	}
}

func TestSupportedComposeHasNoLegacyDiagnosticProfile(t *testing.T) {
	compose := readContractFile(t, filepath.Join(repositoryRoot(t), "deploy", "compose", "docker-compose.yml"))
	for _, forbidden := range []string{"legacy-simulation", "profiles: [diagnostic]", "sleep infinity"} {
		if strings.Contains(compose, forbidden) {
			t.Errorf("supported Compose retains legacy diagnostic marker %q", forbidden)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate production-boundary contract test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readContractFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
