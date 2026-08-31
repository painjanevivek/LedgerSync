package contract_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
)

type nonReturningDiagnosticRepository struct{ release <-chan struct{} }

func (r nonReturningDiagnosticRepository) Facts(context.Context, string) (operations.DatabaseFacts, error) {
	<-r.release
	return operations.DatabaseFacts{}, context.Canceled
}

type nonReturningDependencyProbe struct{ release <-chan struct{} }

func (p nonReturningDependencyProbe) Ping(context.Context) error {
	<-p.release
	return context.Canceled
}

type successfulDependencyProbe struct{}

func (successfulDependencyProbe) Ping(context.Context) error { return nil }

func TestPhaseFiveDiagnosticsRemainBoundedWhenProbesIgnoreCancellation(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	service, err := operations.NewDiagnosticService(
		nonReturningDiagnosticRepository{release: release},
		nonReturningDependencyProbe{release: release},
		operations.BuildFacts{Version: "test", Commit: "test", Environment: "development"},
		func() time.Time { return time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan operations.DiagnosticSnapshot, 1)
	go func() { result <- service.Snapshot(context.Background(), "tenant-a") }()

	select {
	case snapshot := <-result:
		if snapshot.OverallState != "unavailable" || snapshot.FinancialAuthority.PostgreSQL.State != "unavailable" || snapshot.DeliveryCache.Redis.State != "unavailable" {
			t.Fatalf("non-returning probes produced an untruthful snapshot: %+v", snapshot)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("diagnostic snapshot exceeded its bounded dependency window")
	}
}

func TestPhaseFiveDiagnosticsPreserveCompletedProbeEvidenceAtDeadline(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	service, err := operations.NewDiagnosticService(
		nonReturningDiagnosticRepository{release: release},
		successfulDependencyProbe{},
		operations.BuildFacts{Version: "test", Commit: "test", Environment: "development"},
		time.Now,
	)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := service.Snapshot(context.Background(), "tenant-a")
	if snapshot.DeliveryCache.Redis.State != "reachable" {
		t.Fatalf("completed cache probe was discarded while another dependency timed out: %+v", snapshot)
	}
}

func TestPhaseFiveReadSurfaceExcludesRawPayloadAndWriteRoutes(t *testing.T) {
	root := repositoryRoot(t)
	repositoryPath := filepath.Join(root, "internal", "platform", "db", "operations_repository.go")
	repository, err := os.ReadFile(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	lowerRepository := strings.ToLower(string(repository))
	for _, forbidden := range []string{
		"e.payload",
		"claim_owner",
		"database_url",
		"redis_address",
		"docker.sock",
		"docker_engine",
	} {
		if strings.Contains(lowerRepository, forbidden) {
			t.Errorf("operations read repository contains forbidden sensitive field or infrastructure reference %q", forbidden)
		}
	}
	modelPath := filepath.Join(root, "internal", "application", "operations", "webhook_endpoints.go")
	model, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"json:\"endpoint_url", "json:\"signing_key", "json:\"payload", "json:\"raw_error"} {
		if strings.Contains(strings.ToLower(string(model)), forbidden) {
			t.Errorf("webhook evidence model exposes forbidden response field %q", forbidden)
		}
	}

	paths := []string{
		filepath.Join(root, "cmd", "api", "operations_routes.go"),
		filepath.Join(root, "web", "src", "app", "api", "events", "route.ts"),
		filepath.Join(root, "web", "src", "app", "api", "events", "[eventId]", "route.ts"),
		filepath.Join(root, "web", "src", "app", "api", "local", "diagnostics", "route.ts"),
	}
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, forbidden := range []string{"POST /api/", "PUT /api/", "PATCH /api/", "DELETE /api/", "function POST(", "function PUT(", "function PATCH(", "function DELETE("} {
			if strings.Contains(string(content), forbidden) {
				t.Errorf("Phase 5 read route %s exposes forbidden write method %q", path, forbidden)
			}
		}
	}
}
