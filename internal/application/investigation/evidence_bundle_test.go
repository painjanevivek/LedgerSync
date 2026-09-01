package investigation

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestGenerateEvidenceBundleIsBoundedHashedAndRedacted(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 30, 0, 0, time.UTC)
	workspace := Workspace{
		WorkspaceSummary:  WorkspaceSummary{ID: "11111111-1111-4111-8111-111111111111", Title: "Do not export this title", Taxonomy: "transfer_delivery", Status: "open", Version: "3"},
		HistoricalContext: WorkspaceHistoricalContext{References: []WorkspaceReference{{RelationshipType: "workspace_root", RecordType: "transfer", RecordID: "22222222-2222-4222-8222-222222222222", CapturedAt: now}}},
		CurrentEvidence:   WorkspaceCurrentEvidence{Root: &SearchResult{RecordType: "transfer", RecordID: "22222222-2222-4222-8222-222222222222", SafeLabel: "secret label", Status: "posted", OccurredAt: now, Source: "postgresql", Freshness: "current"}, Relationships: []Relationship{{RelationshipType: "debit_account", TargetType: "account", TargetID: "33333333-3333-4333-8333-333333333333", SafeLabel: "Account with 999.99", Status: "active", OccurredAt: now, Source: "postgresql", Freshness: "current"}}},
	}
	bundle, err := GenerateEvidenceBundle(EvidenceBundleRequest{Workspace: workspace, CorrelationID: "request-safe-1", GeneratedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Content) > MaxEvidenceBundleBytes || bundle.FileCount != 4 || bundle.ReferenceRows != 1 || bundle.EvidenceRows != 2 {
		t.Fatalf("unexpected bundle bounds: %#v", bundle)
	}
	digest := sha256.Sum256(bundle.Content)
	if bundle.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("archive digest mismatch")
	}

	reader, err := zip.NewReader(bytes.NewReader(bundle.Content), int64(len(bundle.Content)))
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string][]byte{}
	for _, file := range reader.File {
		opened, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(opened)
		_ = opened.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		contents[file.Name] = content
	}
	for _, name := range []string{"manifest.json", "historical-references.csv", "current-evidence.csv", "request-references.csv"} {
		if _, ok := contents[name]; !ok {
			t.Fatalf("missing %s", name)
		}
	}
	all := string(bytes.Join([][]byte{contents["manifest.json"], contents["historical-references.csv"], contents["current-evidence.csv"], contents["request-references.csv"]}, nil))
	for _, forbidden := range []string{"Do not export this title", "secret label", "999.99"} {
		if strings.Contains(all, forbidden) {
			t.Fatalf("bundle leaked %q", forbidden)
		}
	}
	var manifest evidenceBundleManifest
	if err = json.Unmarshal(contents["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "1" || len(manifest.Files) != 3 || manifest.ExpiresAt != now.Add(EvidenceBundleLifetime).Format(time.RFC3339Nano) {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	for _, file := range manifest.Files {
		digest = sha256.Sum256(contents[file.Name])
		if file.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("manifest hash mismatch for %s", file.Name)
		}
	}
}

func TestGenerateEvidenceBundleRejectsMalformedEvidence(t *testing.T) {
	_, err := GenerateEvidenceBundle(EvidenceBundleRequest{Workspace: Workspace{WorkspaceSummary: WorkspaceSummary{ID: "11111111-1111-4111-8111-111111111111", Version: "1"}, HistoricalContext: WorkspaceHistoricalContext{References: []WorkspaceReference{{RelationshipType: "BAD", RecordType: "transfer", RecordID: "id"}}}}, CorrelationID: "request", GeneratedAt: time.Now()})
	if err == nil {
		t.Fatal("expected malformed evidence rejection")
	}
}
