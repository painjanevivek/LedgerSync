package unit_test

import (
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/pilotgate"
)

func TestPilotGateRequiresManagedControlsAndProviderRestoreEvidence(t *testing.T) {
	evidence := validPilotEvidence()
	if err := evidence.Validate(true); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}

	evidence.Network.DatabasePrivate = false
	evidence.Restore.MismatchCount = 1
	err := evidence.Validate(true)
	if err == nil || !strings.Contains(err.Error(), "only web may be public") || !strings.Contains(err.Error(), "zero mismatches") {
		t.Fatalf("unsafe network/restore evidence was not explained: %v", err)
	}
}

func TestPilotGateAllowsDeploymentPreflightToPrecedeTheRestoreDrill(t *testing.T) {
	evidence := validPilotEvidence()
	evidence.Restore = pilotgate.Restore{}
	if err := evidence.Validate(false); err != nil {
		t.Fatalf("deployment preflight should not claim or require a completed restore: %v", err)
	}
	if err := evidence.Validate(true); err == nil {
		t.Fatal("release preflight accepted missing provider restore evidence")
	}
}

func validPilotEvidence() pilotgate.Evidence {
	return pilotgate.Evidence{
		Revision: "git:0123456789abcdef", Environment: "production",
		OIDC:          pilotgate.OIDC{IssuerURL: "https://id.example.test", RedirectURL: "https://ledger.example.test/api/auth/callback", TenantClaim: "tenant_id", RoleClaim: "roles", SessionExpiryMinutes: 30, RotationEvidenceRef: "ticket:oidc-1", ExpiredSessionTestRef: "test:oidc-expired-1", CrossTenantDenialRef: "test:tenant-denial-1"},
		Secrets:       pilotgate.Secrets{Manager: "managed-secret-store", WorkloadIdentity: true, RotationEvidenceRef: "ticket:secret-1", BreakGlassEvidenceRef: "ticket:break-glass-1"},
		Network:       pilotgate.Network{PublicServices: []string{"web"}, APIPrivate: true, WorkerPrivate: true, DatabasePrivate: true, RedisPrivate: true, EvidenceRef: "scan:network-1"},
		Backup:        pilotgate.Backup{Encrypted: true, ContinuousArchiving: true, RetentionDays: 35, MaxBackupAgeMinutes: 10, SeparateTrustBoundary: true, EvidenceRef: "provider:backup-1"},
		Restore:       pilotgate.Restore{Isolated: true, Reconciled: true, MismatchCount: 0, RedisRebuilt: true, RPOSeconds: 60, RTOSeconds: 300, EvidenceRef: "provider:restore-1"},
		Observability: pilotgate.Observability{AlertRoutesTested: true, RedactionVerified: true, EvidenceRef: "test:alerts-1"},
		Security:      pilotgate.Security{OpenCritical: 0, OpenHigh: 0, EvidenceRef: "scan:security-1"},
	}
}
