package investigation

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	EvidenceBundleSchemaVersion = "1"
	MaxEvidenceBundleBytes      = 512 * 1024
	EvidenceBundleLifetime      = 15 * time.Minute
)

var ErrEvidenceBundleUnavailable = errors.New("evidence bundle unavailable")
var evidenceBundleRequestReference = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var evidenceBundleToken = regexp.MustCompile(`^[a-z][a-z0-9_:-]{0,63}$`)

type EvidenceBundleRequest struct {
	Workspace     Workspace
	CorrelationID string
	GeneratedAt   time.Time
}

type EvidenceBundle struct {
	Content       []byte
	SHA256        string
	Filename      string
	GeneratedAt   time.Time
	ExpiresAt     time.Time
	FileCount     int
	ReferenceRows int
	EvidenceRows  int
}

type evidenceBundleManifest struct {
	SchemaVersion string                       `json:"schema_version"`
	BundleType    string                       `json:"bundle_type"`
	GeneratedAt   string                       `json:"generated_at_utc"`
	ExpiresAt     string                       `json:"expires_at_utc"`
	Authority     string                       `json:"authority_notice"`
	Retention     string                       `json:"server_retention"`
	Workspace     evidenceBundleWorkspace      `json:"workspace_reference"`
	Request       evidenceBundleRequestRef     `json:"request_reference"`
	Files         []evidenceBundleManifestFile `json:"files"`
	Redactions    []string                     `json:"excluded_content"`
}

type evidenceBundleWorkspace struct {
	InvestigationID string `json:"investigation_id"`
	Version         string `json:"workspace_version"`
	Status          string `json:"workspace_status"`
	Taxonomy        string `json:"taxonomy"`
}

type evidenceBundleRequestRef struct {
	CorrelationID string `json:"correlation_id"`
}

type evidenceBundleManifestFile struct {
	Name          string `json:"name"`
	MediaType     string `json:"media_type"`
	SchemaVersion string `json:"schema_version"`
	Rows          int    `json:"rows"`
	Bytes         int    `json:"bytes"`
	SHA256        string `json:"sha256"`
}

type bundleFile struct {
	name, mediaType string
	rows            int
	content         []byte
}

func GenerateEvidenceBundle(request EvidenceBundleRequest) (EvidenceBundle, error) {
	workspace := request.Workspace
	generatedAt := request.GeneratedAt.UTC()
	if generatedAt.IsZero() || !canonicalSavedViewUUID.MatchString(strings.ToLower(workspace.ID)) || workspace.Version == "" || !evidenceBundleRequestReference.MatchString(request.CorrelationID) || !evidenceBundleToken.MatchString(workspace.Status) || !WorkspaceTaxonomy(workspace.Taxonomy) {
		return EvidenceBundle{}, ErrEvidenceBundleUnavailable
	}
	historical, err := historicalReferencesCSV(workspace)
	if err != nil {
		return EvidenceBundle{}, err
	}
	current, err := currentEvidenceCSV(workspace)
	if err != nil {
		return EvidenceBundle{}, err
	}
	requestRefs, err := requestReferencesCSV(workspace, request.CorrelationID, generatedAt)
	if err != nil {
		return EvidenceBundle{}, err
	}
	files := []bundleFile{
		{name: "historical-references.csv", mediaType: "text/csv", rows: len(workspace.HistoricalContext.References), content: historical},
		{name: "current-evidence.csv", mediaType: "text/csv", rows: currentEvidenceRows(workspace), content: current},
		{name: "request-references.csv", mediaType: "text/csv", rows: 1, content: requestRefs},
	}
	manifest := evidenceBundleManifest{
		SchemaVersion: EvidenceBundleSchemaVersion,
		BundleType:    "ledgersync.investigation-evidence",
		GeneratedAt:   generatedAt.Format(time.RFC3339Nano),
		ExpiresAt:     generatedAt.Add(EvidenceBundleLifetime).Format(time.RFC3339Nano),
		Authority:     "Historical evidence snapshot only. Reopen the authorized workspace to verify current financial state.",
		Retention:     "The generated archive is not retained by LedgerSync after this HTTP response completes.",
		Workspace:     evidenceBundleWorkspace{InvestigationID: workspace.ID, Version: workspace.Version, Status: workspace.Status, Taxonomy: workspace.Taxonomy},
		Request:       evidenceBundleRequestRef{CorrelationID: request.CorrelationID},
		Redactions:    []string{"amounts and balances", "payloads and request bodies", "secrets and credentials", "free-form titles and notes", "operator and tenant labels"},
	}
	for _, file := range files {
		digest := sha256.Sum256(file.content)
		manifest.Files = append(manifest.Files, evidenceBundleManifestFile{Name: file.name, MediaType: file.mediaType, SchemaVersion: EvidenceBundleSchemaVersion, Rows: file.rows, Bytes: len(file.content), SHA256: hex.EncodeToString(digest[:])})
	}
	manifestContent, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return EvidenceBundle{}, fmt.Errorf("%w: manifest", ErrEvidenceBundleUnavailable)
	}
	manifestContent = append(manifestContent, '\n')
	files = append([]bundleFile{{name: "manifest.json", mediaType: "application/json", rows: 1, content: manifestContent}}, files...)

	var destination bytes.Buffer
	archive := zip.NewWriter(&destination)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.SetModTime(generatedAt)
		header.SetMode(0o600)
		entry, createErr := archive.CreateHeader(header)
		if createErr != nil {
			return EvidenceBundle{}, fmt.Errorf("%w: create archive entry", ErrEvidenceBundleUnavailable)
		}
		if _, writeErr := entry.Write(file.content); writeErr != nil {
			return EvidenceBundle{}, fmt.Errorf("%w: write archive entry", ErrEvidenceBundleUnavailable)
		}
	}
	if err = archive.Close(); err != nil || destination.Len() > MaxEvidenceBundleBytes {
		return EvidenceBundle{}, fmt.Errorf("%w: archive bound", ErrEvidenceBundleUnavailable)
	}
	digest := sha256.Sum256(destination.Bytes())
	return EvidenceBundle{
		Content: append([]byte(nil), destination.Bytes()...), SHA256: hex.EncodeToString(digest[:]),
		Filename:    "ledgersync-investigation-" + strings.ToLower(workspace.ID) + "-" + generatedAt.Format("20060102T150405Z") + "-v" + EvidenceBundleSchemaVersion + ".zip",
		GeneratedAt: generatedAt, ExpiresAt: generatedAt.Add(EvidenceBundleLifetime), FileCount: len(files),
		ReferenceRows: len(workspace.HistoricalContext.References), EvidenceRows: currentEvidenceRows(workspace),
	}, nil
}

func historicalReferencesCSV(workspace Workspace) ([]byte, error) {
	rows := [][]string{{"schema_version", "relationship_type", "source_record_type", "source_record_id", "record_type", "record_id", "captured_at_utc"}}
	for _, reference := range workspace.HistoricalContext.References {
		if !ValidWorkspaceRelationship(reference.RelationshipType) || !WorkspaceRecordType(reference.RecordType) || !canonicalSavedViewUUID.MatchString(strings.ToLower(reference.RecordID)) || reference.CapturedAt.IsZero() || (reference.SourceRecordType != "" && !WorkspaceRecordType(reference.SourceRecordType)) || (reference.SourceRecordID != "" && !canonicalSavedViewUUID.MatchString(strings.ToLower(reference.SourceRecordID))) {
			return nil, fmt.Errorf("%w: historical reference", ErrEvidenceBundleUnavailable)
		}
		rows = append(rows, []string{EvidenceBundleSchemaVersion, reference.RelationshipType, reference.SourceRecordType, reference.SourceRecordID, reference.RecordType, reference.RecordID, reference.CapturedAt.UTC().Format(time.RFC3339Nano)})
	}
	return encodeBundleCSV(rows)
}

func currentEvidenceCSV(workspace Workspace) ([]byte, error) {
	rows := [][]string{{"schema_version", "evidence_kind", "relationship_type", "record_type", "record_id", "status", "occurred_at_utc", "source", "freshness"}}
	if root := workspace.CurrentEvidence.Root; root != nil {
		if !WorkspaceRecordType(root.RecordType) || !canonicalSavedViewUUID.MatchString(strings.ToLower(root.RecordID)) || root.OccurredAt.IsZero() || !evidenceBundleToken.MatchString(root.Status) || !evidenceBundleToken.MatchString(root.Source) || !evidenceBundleToken.MatchString(root.Freshness) {
			return nil, fmt.Errorf("%w: current root", ErrEvidenceBundleUnavailable)
		}
		rows = append(rows, []string{EvidenceBundleSchemaVersion, "root", "workspace_root", root.RecordType, root.RecordID, root.Status, root.OccurredAt.UTC().Format(time.RFC3339Nano), root.Source, root.Freshness})
	}
	for _, relation := range workspace.CurrentEvidence.Relationships {
		if !ValidWorkspaceRelationship(relation.RelationshipType) || !WorkspaceRecordType(relation.TargetType) || !canonicalSavedViewUUID.MatchString(strings.ToLower(relation.TargetID)) || relation.OccurredAt.IsZero() || !evidenceBundleToken.MatchString(relation.Status) || !evidenceBundleToken.MatchString(relation.Source) || !evidenceBundleToken.MatchString(relation.Freshness) {
			return nil, fmt.Errorf("%w: current relationship", ErrEvidenceBundleUnavailable)
		}
		rows = append(rows, []string{EvidenceBundleSchemaVersion, "relationship", relation.RelationshipType, relation.TargetType, relation.TargetID, relation.Status, relation.OccurredAt.UTC().Format(time.RFC3339Nano), relation.Source, relation.Freshness})
	}
	return encodeBundleCSV(rows)
}

func requestReferencesCSV(workspace Workspace, correlationID string, generatedAt time.Time) ([]byte, error) {
	return encodeBundleCSV([][]string{{"schema_version", "correlation_id", "investigation_id", "workspace_version", "generated_at_utc", "expires_at_utc"}, {EvidenceBundleSchemaVersion, correlationID, workspace.ID, workspace.Version, generatedAt.Format(time.RFC3339Nano), generatedAt.Add(EvidenceBundleLifetime).Format(time.RFC3339Nano)}})
}

func encodeBundleCSV(rows [][]string) ([]byte, error) {
	var destination bytes.Buffer
	writer := csv.NewWriter(&destination)
	writer.UseCRLF = true
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("%w: csv", ErrEvidenceBundleUnavailable)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("%w: csv", ErrEvidenceBundleUnavailable)
	}
	return destination.Bytes(), nil
}

func currentEvidenceRows(workspace Workspace) int {
	rows := len(workspace.CurrentEvidence.Relationships)
	if workspace.CurrentEvidence.Root != nil {
		rows++
	}
	return rows
}
