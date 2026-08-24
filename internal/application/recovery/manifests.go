package recovery

import (
	"context"
	"errors"
)

var ErrManifestEvidenceUnavailable = errors.New("recovery manifest evidence unavailable")

type BackupManifestEvidence struct {
	BackupID         string `json:"backup_id"`
	FinalizedAtUTC   string `json:"finalized_at_utc"`
	SizeBytes        int64  `json:"size_bytes"`
	SchemaVersion    string `json:"schema_version"`
	DigestStatus     string `json:"digest_status"`
	ValidationStatus string `json:"validation_status"`
	SourceCommit     string `json:"source_commit"`
}

type RestoreManifestEvidence struct {
	BackupID               string  `json:"backup_id"`
	CompletedAtUTC         string  `json:"completed_at_utc"`
	Status                 string  `json:"status"`
	ReconciliationStatus   string  `json:"reconciliation_status"`
	MismatchCount          int64   `json:"mismatch_count"`
	NormalProjectUnchanged bool    `json:"normal_project_unchanged"`
	LocalRTOSeconds        float64 `json:"local_rto_seconds"`
}

type ManifestRetention struct {
	ValidBackupCount    int `json:"valid_backup_count"`
	IgnoredEntryCount   int `json:"ignored_entry_count"`
	ConfiguredKeepCount int `json:"configured_keep_count"`
}

type ManifestSnapshot struct {
	FormatVersion  string                   `json:"format_version"`
	GeneratedAtUTC string                   `json:"generated_at_utc"`
	LatestBackup   *BackupManifestEvidence  `json:"latest_backup"`
	LatestRestore  *RestoreManifestEvidence `json:"latest_restore"`
	Retention      ManifestRetention        `json:"retention"`
}

type ManifestIndex interface {
	Snapshot(context.Context) (ManifestSnapshot, error)
}

type ManifestService struct{ index ManifestIndex }

func NewManifestService(index ManifestIndex) (*ManifestService, error) {
	if index == nil {
		return nil, errors.New("recovery manifest index is required")
	}
	return &ManifestService{index: index}, nil
}

func (s *ManifestService) Snapshot(ctx context.Context) (ManifestSnapshot, error) {
	if s == nil || s.index == nil || ctx == nil {
		return ManifestSnapshot{}, ErrManifestEvidenceUnavailable
	}
	return s.index.Snapshot(ctx)
}
