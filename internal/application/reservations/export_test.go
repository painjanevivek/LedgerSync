package reservations

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReservationSnapshotExportIsExactBoundedAndFormulaSafe(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	repo := &recordingRepository{snapshot: Snapshot{AsOf: now, Holds: []Hold{{ID: "33333333-3333-4333-8333-333333333333", AmountMinor: "9223372036854775807", State: "active", CreatedBy: "=synthetic_formula", CreatedAt: now, ExpiresAt: now.Add(-time.Second)}}}}
	service, _ := NewService(repo)
	raw, err := service.Export(context.Background(), validCreateFixture().Scope)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil || len(rows) != 2 || rows[1][4] != "9223372036854775807" || rows[1][5] != "9223372036854775807" || rows[1][9] != "'=synthetic_formula" {
		t.Fatalf("exact active obligation or formula guard differs: %v %v", rows, err)
	}
	if rows[1][1] != now.Format(time.RFC3339Nano) {
		t.Fatal("snapshot identity missing")
	}
	if _, err := service.Export(context.Background(), Scope{}); !errors.Is(err, ErrInvalid) {
		t.Fatal("unscoped export accepted")
	}
	repo.snapshot.Holds = make([]Hold, MaxExportHolds+1)
	if _, err := service.Export(context.Background(), validCreateFixture().Scope); !errors.Is(err, ErrLimit) {
		t.Fatal("truncated export accepted")
	}
}
