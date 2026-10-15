package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeQueryTemplatesPrepareAgainstControlledSchema(t *testing.T) {
	_, database := requireTransferService(t, 10000)
	source, err := os.ReadFile(filepath.Join(liveMigrationRoot(t), "internal", "platform", "db", "queries.sql"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, statement := range strings.Split(string(source), ";") {
		if !strings.Contains(statement, "-- name:") {
			continue
		}
		prepared, err := database.PrepareContext(context.Background(), statement)
		if err != nil {
			t.Fatalf("controlled query template %d: %v", count+1, err)
		}
		if err := prepared.Close(); err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 7 {
		t.Fatalf("query template count=%d want 7", count)
	}
}
