package contract_test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	postgresEvidenceImage  = "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
	redisEvidenceImage     = "redis:7.4-alpine@sha256:e7723ff73d963f5cc6d9c4643ea3d989527a402a319239054e9472a7fb9219a2"
	toxiproxyEvidenceImage = "ghcr.io/shopify/toxiproxy:2.12.0@sha256:9378ed52a28bc50edc1350f936f518f31fa95f0d15917d6eb40b8e376d1a214e"
)

func TestEvidenceContainerImagesAreDigestPinned(t *testing.T) {
	root := repositoryRoot(t)
	testCases := []struct {
		path   string
		images []string
	}{
		{filepath.Join(root, "deploy", "compose", "docker-compose.restore.yml"), []string{postgresEvidenceImage, redisEvidenceImage}},
		{filepath.Join(root, "deploy", "compose", "docker-compose.fault.yml"), []string{postgresEvidenceImage, redisEvidenceImage, toxiproxyEvidenceImage}},
		{filepath.Join(root, ".github", "workflows", "quality.yml"), []string{postgresEvidenceImage, redisEvidenceImage}},
		{filepath.Join(root, ".github", "workflows", "release-evidence.yml"), []string{postgresEvidenceImage, redisEvidenceImage}},
	}

	for _, testCase := range testCases {
		content := readContractFile(t, testCase.path)
		for _, image := range testCase.images {
			if strings.Count(content, image) != 1 {
				t.Errorf("%s must reference %q exactly once", testCase.path, image)
			}
		}
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "postgres:16-alpine") && !strings.Contains(line, postgresEvidenceImage) {
				t.Errorf("%s contains a tag-only or unexpected PostgreSQL evidence image: %s", testCase.path, strings.TrimSpace(line))
			}
			if strings.Contains(line, "redis:7.4-alpine") && !strings.Contains(line, redisEvidenceImage) {
				t.Errorf("%s contains a tag-only or unexpected Redis evidence image: %s", testCase.path, strings.TrimSpace(line))
			}
			if strings.Contains(line, "ghcr.io/shopify/toxiproxy:2.12.0") && !strings.Contains(line, toxiproxyEvidenceImage) {
				t.Errorf("%s contains a tag-only or unexpected Toxiproxy evidence image: %s", testCase.path, strings.TrimSpace(line))
			}
		}
	}
}

func TestQualityRestoreRetainsTheExplicitBackupRoot(t *testing.T) {
	workflow := readContractFile(t, filepath.Join(repositoryRoot(t), ".github", "workflows", "quality.yml"))
	backupCommand := "./scripts/backup-local.ps1 -BackupRoot data/ci-backups -RetentionCount 1"
	restoreCommand := "./scripts/local-restore-drill.ps1 -BackupDirectory $backup.FullName -BackupRoot data/ci-backups"
	if strings.Count(workflow, backupCommand) != 1 || strings.Count(workflow, restoreCommand) != 1 {
		t.Fatal("quality restore must validate the selected bundle against the same explicit dedicated backup root")
	}
}

func TestOpenAPIValidatorIsLockedAndInstalledOffline(t *testing.T) {
	root := repositoryRoot(t)
	workflow := readContractFile(t, filepath.Join(root, ".github", "workflows", "contract.yml"))
	if strings.Contains(workflow, "npx") {
		t.Fatal("contract workflow must not install or execute an unlocked validator through npx")
	}
	for _, required := range []string{
		"run: npm ci",
		"run: ./node_modules/.bin/redocly lint ../contracts/openapi.yaml",
		"working-directory: web",
		"web/package.json",
		"web/package-lock.json",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("contract workflow is missing %q", required)
		}
	}

	var packageManifest struct {
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(readContractFile(t, filepath.Join(root, "web", "package.json"))), &packageManifest); err != nil {
		t.Fatal(err)
	}
	if got := packageManifest.DevDependencies["@redocly/cli"]; got != "1.34.0" {
		t.Fatalf("package.json @redocly/cli version = %q, want exact version 1.34.0", got)
	}

	var packageLock struct {
		Packages map[string]struct {
			Version         string            `json:"version"`
			Resolved        string            `json:"resolved"`
			Integrity       string            `json:"integrity"`
			DevDependencies map[string]string `json:"devDependencies"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readContractFile(t, filepath.Join(root, "web", "package-lock.json"))), &packageLock); err != nil {
		t.Fatal(err)
	}
	if got := packageLock.Packages[""].DevDependencies["@redocly/cli"]; got != "1.34.0" {
		t.Fatalf("package-lock root @redocly/cli version = %q, want exact version 1.34.0", got)
	}
	lockedCLI := packageLock.Packages["node_modules/@redocly/cli"]
	if lockedCLI.Version != "1.34.0" || lockedCLI.Resolved == "" || lockedCLI.Integrity == "" {
		t.Fatalf("package-lock @redocly/cli entry is incomplete: version=%q resolved=%t integrity=%t", lockedCLI.Version, lockedCLI.Resolved != "", lockedCLI.Integrity != "")
	}
}

func TestSecretHistoryExceptionsAreExactReviewedFingerprints(t *testing.T) {
	root := repositoryRoot(t)
	content := readContractFile(t, filepath.Join(root, ".gitleaksignore"))
	fingerprintPattern := regexp.MustCompile(`^[0-9a-f]{40}:(?:internal|tests|docs)/[^:]+:generic-api-key:[1-9][0-9]*$`)
	fingerprints := make([]string, 0, 12)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !fingerprintPattern.MatchString(line) {
			t.Fatalf("secret-history exception is broader than one exact finding fingerprint: %q", line)
		}
		fingerprints = append(fingerprints, line)
	}
	if len(fingerprints) != 12 {
		t.Fatalf("reviewed secret-history exception count = %d, want exactly 12", len(fingerprints))
	}
	if strings.Contains(content, "allowlist") || strings.Contains(content, "regex") || strings.Contains(content, "path =") {
		t.Fatal("secret-history exceptions must not introduce rule-, regex-, or path-wide allowlisting")
	}
}
