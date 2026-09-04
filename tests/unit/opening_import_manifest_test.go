package unit_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/openingimports"
)

func TestOpeningImportManifestHashIsCanonicalAndOrderIndependent(t *testing.T) {
	manifest := openingimports.Manifest{
		BatchID: "00000000-0000-4000-8000-000000008801", TenantID: "00000000-0000-4000-8000-000000008802", Currency: "USD",
		Rows: []openingimports.Row{
			{AccountID: "00000000-0000-4000-8000-000000008804", OpeningMinor: "200"},
			{AccountID: "00000000-0000-4000-8000-000000008803", OpeningMinor: "100"},
		},
	}
	prepared, err := manifest.Validate(context.Background(), "USD")
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte("00000000-0000-4000-8000-000000008802\nUSD\n00000000-0000-4000-8000-000000008803,100\n00000000-0000-4000-8000-000000008804,200\n"))
	if prepared.ContentHash != expected || prepared.TotalMinor != 300 || prepared.Rows[0].AccountID != "00000000-0000-4000-8000-000000008803" {
		t.Fatalf("prepared manifest hash=%x total=%d rows=%+v", prepared.ContentHash, prepared.TotalMinor, prepared.Rows)
	}
	manifest.Rows[0], manifest.Rows[1] = manifest.Rows[1], manifest.Rows[0]
	reordered, err := manifest.Validate(context.Background(), "USD")
	if err != nil || reordered.ContentHash != prepared.ContentHash {
		t.Fatalf("row order changed canonical hash: hash=%x error=%v", reordered.ContentHash, err)
	}
}

func TestOpeningImportManifestRejectsUnsafeOrAmbiguousRows(t *testing.T) {
	base := openingimports.Manifest{
		BatchID: "00000000-0000-4000-8000-000000008811", TenantID: "00000000-0000-4000-8000-000000008812", Currency: "USD",
		Rows: []openingimports.Row{{AccountID: "00000000-0000-4000-8000-000000008813", OpeningMinor: "100"}},
	}
	cases := []openingimports.Manifest{
		{BatchID: base.BatchID, TenantID: base.TenantID, Currency: "EUR", Rows: base.Rows},
		{BatchID: base.BatchID, TenantID: base.TenantID, Currency: base.Currency, Rows: []openingimports.Row{{AccountID: base.Rows[0].AccountID, OpeningMinor: "0"}}},
		{BatchID: base.BatchID, TenantID: base.TenantID, Currency: base.Currency, Rows: []openingimports.Row{{AccountID: base.Rows[0].AccountID, OpeningMinor: "01"}}},
		{BatchID: base.BatchID, TenantID: base.TenantID, Currency: base.Currency, Rows: []openingimports.Row{{AccountID: base.Rows[0].AccountID, OpeningMinor: "100"}, {AccountID: base.Rows[0].AccountID, OpeningMinor: "200"}}},
	}
	for index, manifest := range cases {
		if _, err := manifest.Validate(context.Background(), "USD"); !errors.Is(err, openingimports.ErrInvalid) {
			t.Errorf("unsafe case %d error=%v", index, err)
		}
	}
	encoded, _ := json.Marshal(base)
	if _, err := openingimports.DecodeManifest(strings.NewReader(string(encoded) + `{}`)); !errors.Is(err, openingimports.ErrInvalid) {
		t.Fatalf("trailing manifest object error=%v", err)
	}
	if _, err := openingimports.DecodeManifest(strings.NewReader(`{"batch_id":"x","unknown":true}`)); !errors.Is(err, openingimports.ErrInvalid) {
		t.Fatalf("unknown manifest field error=%v", err)
	}
}
