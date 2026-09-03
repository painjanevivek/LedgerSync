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
}

func TestRetiredApplicationSliceDoesNotExist(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range []string{"backend", "dashboard", "simulation", "setup", "docker-compose.legacy-demo.yml", filepath.Join("tests", "consistency_test.go"), filepath.Join("tests", "dashboard_test.py")} {
		_, err := os.Stat(filepath.Join(root, relative))
		if err == nil || !os.IsNotExist(err) {
			t.Errorf("retired application path must remain absent: %s (error=%v)", relative, err)
		}
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
		for _, forbidden := range []string{"backend/", "dashboard/", "simulation/", "setup/", "docker-compose.legacy-demo.yml"} {
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
