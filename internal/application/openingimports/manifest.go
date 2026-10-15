// Package openingimports defines the offline, reviewed contract for importing
// non-zero opening value. It is deliberately not exposed as a public API.
package openingimports

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
)

var (
	ErrInvalid   = errors.New("invalid opening import")
	ErrForbidden = errors.New("opening import is forbidden")
	ErrNotFound  = errors.New("opening import was not found")
	ErrConflict  = errors.New("opening import conflicts with immutable state")
)

type Row struct {
	AccountID    string `json:"account_id"`
	OpeningMinor string `json:"opening_minor"`
}

type Manifest struct {
	BatchID  string `json:"batch_id"`
	TenantID string `json:"tenant_id"`
	Currency string `json:"currency"`
	Rows     []Row  `json:"rows"`
}

type PreparedRow struct {
	AccountID    string
	OpeningMinor int64
}

type PreparedManifest struct {
	BatchID     string
	TenantID    string
	Currency    string
	Rows        []PreparedRow
	ContentHash [sha256.Size]byte
	TotalMinor  int64
}

type Result struct {
	Replayed bool
}

func DecodeManifest(reader io.Reader) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: decode manifest", ErrInvalid)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Manifest{}, fmt.Errorf("%w: manifest must contain one JSON object", ErrInvalid)
	}
	return manifest, nil
}

func (manifest Manifest) Validate(ctx context.Context, pilotCurrency string) (PreparedManifest, error) {
	batchID, err := identifier.Canonicalize(ctx, identifier.KindOpeningImport, manifest.BatchID)
	if err != nil {
		return PreparedManifest{}, fmt.Errorf("%w: batch identifier", ErrInvalid)
	}
	tenantID, err := identifier.Canonicalize(ctx, identifier.KindTenant, manifest.TenantID)
	if err != nil {
		return PreparedManifest{}, fmt.Errorf("%w: tenant identifier", ErrInvalid)
	}
	if manifest.Currency == "" || manifest.Currency != pilotCurrency || manifest.Currency != strings.ToUpper(manifest.Currency) || len(manifest.Rows) < 1 || len(manifest.Rows) > 10_000 {
		return PreparedManifest{}, fmt.Errorf("%w: selected currency and 1..10000 rows are required", ErrInvalid)
	}
	prepared := PreparedManifest{BatchID: batchID, TenantID: tenantID, Currency: manifest.Currency, Rows: make([]PreparedRow, 0, len(manifest.Rows))}
	seen := make(map[string]struct{}, len(manifest.Rows))
	for _, row := range manifest.Rows {
		accountID, parseErr := identifier.Canonicalize(ctx, identifier.KindAccount, row.AccountID)
		value, valueErr := strconv.ParseInt(row.OpeningMinor, 10, 64)
		if parseErr != nil || valueErr != nil || value <= 0 || strconv.FormatInt(value, 10) != row.OpeningMinor {
			return PreparedManifest{}, fmt.Errorf("%w: rows require canonical account identifiers and positive integer minor units", ErrInvalid)
		}
		if _, duplicate := seen[accountID]; duplicate {
			return PreparedManifest{}, fmt.Errorf("%w: duplicate account row", ErrInvalid)
		}
		seen[accountID] = struct{}{}
		if prepared.TotalMinor > math.MaxInt64-value {
			return PreparedManifest{}, fmt.Errorf("%w: total exceeds supported range", ErrInvalid)
		}
		prepared.TotalMinor += value
		prepared.Rows = append(prepared.Rows, PreparedRow{AccountID: accountID, OpeningMinor: value})
	}
	sort.Slice(prepared.Rows, func(i, j int) bool { return prepared.Rows[i].AccountID < prepared.Rows[j].AccountID })
	var canonical bytes.Buffer
	_, _ = fmt.Fprintf(&canonical, "%s\n%s\n", prepared.TenantID, prepared.Currency)
	for _, row := range prepared.Rows {
		_, _ = fmt.Fprintf(&canonical, "%s,%d\n", row.AccountID, row.OpeningMinor)
	}
	prepared.ContentHash = sha256.Sum256(canonical.Bytes())
	return prepared, nil
}

func (manifest PreparedManifest) AccountIDs() []string {
	result := make([]string, len(manifest.Rows))
	for index, row := range manifest.Rows {
		result[index] = row.AccountID
	}
	return result
}

func (manifest PreparedManifest) OpeningMinors() []int64 {
	result := make([]int64, len(manifest.Rows))
	for index, row := range manifest.Rows {
		result[index] = row.OpeningMinor
	}
	return result
}
