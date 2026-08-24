package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
)

const (
	recoveryEvidenceFileName = "recovery-evidence-index.json"
	recoveryEvidenceFormat   = "ledgersync-recovery-evidence-index/v1"
	maxRecoveryEvidenceBytes = 64 * 1024
)

var (
	backupIDPattern = regexp.MustCompile(`^backup-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{7,40}$`)
	commitPattern   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	schemaPattern   = regexp.MustCompile(`^[0-9]{6}_[a-z0-9._-]{1,120}$`)
)

type ManifestIndex struct{ root string }

func NewManifestIndex(root string) (*ManifestIndex, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("recovery evidence root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, errors.New("resolve recovery evidence root")
	}
	return &ManifestIndex{root: filepath.Clean(absolute)}, nil
}

func (i *ManifestIndex) Snapshot(ctx context.Context) (apprecovery.ManifestSnapshot, error) {
	if i == nil || ctx == nil {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	if err := ctx.Err(); err != nil {
		return apprecovery.ManifestSnapshot{}, err
	}
	rootInfo, err := os.Lstat(i.root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	canonicalRoot, err := filepath.EvalSymlinks(i.root)
	if err != nil {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	path := filepath.Join(canonicalRoot, recoveryEvidenceFileName)
	if !exactContainedIndex(canonicalRoot, path) {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	defer func() { _ = file.Close() }()
	limited := io.LimitReader(&contextReader{ctx: ctx, reader: file}, maxRecoveryEvidenceBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		if ctx.Err() != nil {
			return apprecovery.ManifestSnapshot{}, ctx.Err()
		}
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	if len(content) > maxRecoveryEvidenceBytes {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	var snapshot apprecovery.ManifestSnapshot
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&snapshot); err != nil {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || !validSnapshot(snapshot) {
		return apprecovery.ManifestSnapshot{}, apprecovery.ErrManifestEvidenceUnavailable
	}
	return snapshot, nil
}

func validSnapshot(snapshot apprecovery.ManifestSnapshot) bool {
	if snapshot.FormatVersion != recoveryEvidenceFormat || !strictUTC(snapshot.GeneratedAtUTC) || snapshot.Retention.ValidBackupCount < 0 || snapshot.Retention.ValidBackupCount > 200 || snapshot.Retention.IgnoredEntryCount < 0 || snapshot.Retention.IgnoredEntryCount > 200 || snapshot.Retention.ConfiguredKeepCount < 1 || snapshot.Retention.ConfiguredKeepCount > 100 {
		return false
	}
	if snapshot.LatestBackup != nil {
		backup := snapshot.LatestBackup
		if !backupIDPattern.MatchString(backup.BackupID) || !strictUTC(backup.FinalizedAtUTC) || backup.SizeBytes < 1 || !schemaPattern.MatchString(backup.SchemaVersion) || backup.DigestStatus != "verified" || backup.ValidationStatus != "passed" || !commitPattern.MatchString(backup.SourceCommit) {
			return false
		}
	}
	if snapshot.LatestRestore != nil {
		restore := snapshot.LatestRestore
		if !backupIDPattern.MatchString(restore.BackupID) || !strictUTC(restore.CompletedAtUTC) || restore.Status != "passed" || (restore.ReconciliationStatus != "matched" && restore.ReconciliationStatus != "completed" && restore.ReconciliationStatus != "passed") || restore.MismatchCount != 0 || !restore.NormalProjectUnchanged || restore.LocalRTOSeconds < 0 || restore.LocalRTOSeconds > 86_400 {
			return false
		}
	}
	return true
}

func exactContainedIndex(root, path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	canonicalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, canonicalPath)
	return err == nil && relative == recoveryEvidenceFileName && filepath.Base(canonicalPath) == recoveryEvidenceFileName
}

func strictUTC(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && strings.HasSuffix(value, "Z") && parsed.Location() == time.UTC
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
