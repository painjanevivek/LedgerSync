package integration_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// portfolioControlDirectory repeats the host runner's target binding before
// any opt-in fixture can reset a database. Docker identity checks stay in the
// host runner; the test proves its URLs match that exact recorded allocation.
func portfolioControlDirectory(t *testing.T) string {
	t.Helper()
	root := os.Getenv("LEDGERSYNC_PORTFOLIO_CONTROL_DIRECTORY")
	if root == "" {
		root = os.Getenv("LEDGERSYNC_CAPACITY_CONTROL_DIRECTORY")
	}
	absolute, err := filepath.Abs(root)
	if err != nil || root == "" {
		t.Fatal("guarded portfolio control directory required")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil || resolved != absolute || !regexp.MustCompile(`^demo-\d{14}-[a-f0-9]{8}$`).MatchString(filepath.Base(absolute)) {
		t.Fatal("invalid exact portfolio run identity")
	}
	repository := os.Getenv("LEDGERSYNC_TEST_REPOSITORY_ROOT")
	expectedParent := filepath.Join(repository, "data", "portfolio", "demo-runs")
	if repository == "" || !filepath.IsAbs(repository) || filepath.Dir(absolute) != expectedParent {
		t.Fatal("portfolio run is outside the recorded repository root")
	}
	filename := os.Getenv("LEDGERSYNC_PORTFOLIO_TARGETS_FILE")
	if filename == "" {
		filename = "isolated-targets.json"
	}
	if filename != "isolated-targets.json" && (filename != "restored-targets.json" || os.Getenv("LEDGERSYNC_RUN_RECOVERY") != "true") {
		t.Fatal("unsupported fixed target metadata")
	}
	path := filepath.Join(absolute, filename)
	metadata, err := os.Lstat(path)
	if err != nil || !metadata.Mode().IsRegular() || metadata.Size() > 32768 {
		t.Fatal("bounded regular target metadata required")
	}
	var targets []struct {
		Service, Project, ID, Image string
		Port                        int
	}
	encoded, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(encoded, &targets) != nil || len(targets) != 2 {
		t.Fatal("exact database/cache target pair required")
	}
	database, err := url.Parse(os.Getenv("LEDGERSYNC_TEST_DATABASE_URL"))
	if err != nil || database.Hostname() != "127.0.0.1" || database.Port() == "5432" || database.Path != "/ledgersync" || (database.Scheme != "postgres" && database.Scheme != "postgresql") {
		t.Fatal("portfolio test requires its separate loopback database")
	}
	project := "ledgersync-acceptance-" + strings.TrimPrefix(filepath.Base(absolute), "demo-")
	matchedPG, matchedRedis := false, false
	for _, target := range targets {
		if target.Project != project || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(target.ID) || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(target.Image) || target.Port < 1 || target.Port > 65535 {
			t.Fatal("foreign or invalid portfolio target")
		}
		switch target.Service {
		case "postgres":
			if matchedPG {
				t.Fatal("duplicate database target")
			}
			matchedPG = database.Port() == strconv.Itoa(target.Port)
		case "redis":
			if matchedRedis {
				t.Fatal("duplicate Redis target")
			}
			matchedRedis = os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR") == "127.0.0.1:"+strconv.Itoa(target.Port)
		default:
			t.Fatal("unexpected portfolio dependency")
		}
	}
	if !matchedPG || !matchedRedis {
		t.Fatal("test URLs do not match the exact guarded target pair")
	}
	return absolute
}
