// Package pilotgate validates bounded, secret-free evidence for a managed pilot.
package pilotgate

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

type Evidence struct {
	Revision      string        `json:"revision"`
	Environment   string        `json:"environment"`
	OIDC          OIDC          `json:"oidc"`
	Secrets       Secrets       `json:"secrets"`
	Network       Network       `json:"network"`
	Backup        Backup        `json:"backup"`
	Restore       Restore       `json:"restore"`
	Observability Observability `json:"observability"`
	Security      Security      `json:"security"`
}

type OIDC struct {
	IssuerURL             string `json:"issuer_url"`
	RedirectURL           string `json:"redirect_url"`
	TenantClaim           string `json:"tenant_claim"`
	RoleClaim             string `json:"role_claim"`
	SessionExpiryMinutes  int    `json:"session_expiry_minutes"`
	RotationEvidenceRef   string `json:"rotation_evidence_ref"`
	ExpiredSessionTestRef string `json:"expired_session_test_ref"`
	CrossTenantDenialRef  string `json:"cross_tenant_denial_ref"`
}

type Secrets struct {
	Manager               string `json:"manager"`
	WorkloadIdentity      bool   `json:"workload_identity"`
	RotationEvidenceRef   string `json:"rotation_evidence_ref"`
	BreakGlassEvidenceRef string `json:"break_glass_evidence_ref"`
}

type Network struct {
	PublicServices  []string `json:"public_services"`
	APIPrivate      bool     `json:"api_private"`
	WorkerPrivate   bool     `json:"worker_private"`
	DatabasePrivate bool     `json:"database_private"`
	RedisPrivate    bool     `json:"redis_private"`
	EvidenceRef     string   `json:"evidence_ref"`
}

type Backup struct {
	Encrypted             bool   `json:"encrypted"`
	ContinuousArchiving   bool   `json:"continuous_archiving"`
	RetentionDays         int    `json:"retention_days"`
	MaxBackupAgeMinutes   int    `json:"max_backup_age_minutes"`
	SeparateTrustBoundary bool   `json:"separate_trust_boundary"`
	EvidenceRef           string `json:"evidence_ref"`
}

type Restore struct {
	Isolated      bool   `json:"isolated"`
	Reconciled    bool   `json:"reconciled"`
	MismatchCount int    `json:"mismatch_count"`
	RedisRebuilt  bool   `json:"redis_rebuilt"`
	RPOSeconds    int    `json:"rpo_seconds"`
	RTOSeconds    int    `json:"rto_seconds"`
	EvidenceRef   string `json:"evidence_ref"`
}

type Observability struct {
	AlertRoutesTested bool   `json:"alert_routes_tested"`
	RedactionVerified bool   `json:"redaction_verified"`
	EvidenceRef       string `json:"evidence_ref"`
}

type Security struct {
	OpenCritical int    `json:"open_critical"`
	OpenHigh     int    `json:"open_high"`
	EvidenceRef  string `json:"evidence_ref"`
}

func (e Evidence) Validate(requireRestore bool) error {
	var problems []error
	if !reference(e.Revision) {
		problems = append(problems, errors.New("revision must be a non-placeholder immutable deployment reference"))
	}
	if e.Environment != "production" {
		problems = append(problems, errors.New("environment must be production"))
	}
	if !secureURL(e.OIDC.IssuerURL) || !secureURL(e.OIDC.RedirectURL) {
		problems = append(problems, errors.New("OIDC issuer and redirect URLs must use HTTPS"))
	}
	if e.OIDC.TenantClaim != "tenant_id" || e.OIDC.RoleClaim != "roles" {
		problems = append(problems, errors.New("OIDC tenant and role claims must match the application contract"))
	}
	if e.OIDC.SessionExpiryMinutes < 1 || e.OIDC.SessionExpiryMinutes > 60 {
		problems = append(problems, errors.New("OIDC session expiry must be between 1 and 60 minutes"))
	}
	references := []struct{ name, value string }{
		{"OIDC rotation", e.OIDC.RotationEvidenceRef},
		{"expired-session test", e.OIDC.ExpiredSessionTestRef},
		{"cross-tenant denial", e.OIDC.CrossTenantDenialRef},
		{"secret rotation", e.Secrets.RotationEvidenceRef},
		{"break-glass", e.Secrets.BreakGlassEvidenceRef},
		{"network", e.Network.EvidenceRef},
		{"backup", e.Backup.EvidenceRef},
		{"observability", e.Observability.EvidenceRef},
		{"security", e.Security.EvidenceRef},
	}
	for _, item := range references {
		if !reference(item.value) {
			problems = append(problems, fmt.Errorf("%s evidence reference is required", item.name))
		}
	}
	if !reference(e.Secrets.Manager) || !e.Secrets.WorkloadIdentity {
		problems = append(problems, errors.New("managed secrets and workload identity evidence are required"))
	}
	if len(e.Network.PublicServices) != 1 || !slices.Contains(e.Network.PublicServices, "web") ||
		!e.Network.APIPrivate || !e.Network.WorkerPrivate || !e.Network.DatabasePrivate || !e.Network.RedisPrivate {
		problems = append(problems, errors.New("only web may be public; API, worker, database, and Redis must be private"))
	}
	if !e.Backup.Encrypted || !e.Backup.ContinuousArchiving || e.Backup.RetentionDays < 35 ||
		e.Backup.MaxBackupAgeMinutes < 1 || e.Backup.MaxBackupAgeMinutes > 15 || !e.Backup.SeparateTrustBoundary {
		problems = append(problems, errors.New("backup evidence must prove encryption, continuous archiving, 35-day retention, backup age <=15 minutes, and a separate trust boundary"))
	}
	if !e.Observability.AlertRoutesTested || !e.Observability.RedactionVerified {
		problems = append(problems, errors.New("alert routing and telemetry redaction must be tested"))
	}
	if e.Security.OpenCritical != 0 || e.Security.OpenHigh != 0 {
		problems = append(problems, errors.New("critical and high security findings must be zero"))
	}
	if requireRestore {
		if !e.Restore.Isolated || !e.Restore.Reconciled || e.Restore.MismatchCount != 0 || !e.Restore.RedisRebuilt ||
			e.Restore.RPOSeconds < 0 || e.Restore.RTOSeconds <= 0 || !reference(e.Restore.EvidenceRef) {
			problems = append(problems, errors.New("provider restore must be isolated, reconciled with zero mismatches, cache-rebuilt, timed, and evidenced"))
		}
	}
	return errors.Join(problems...)
}

func secureURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func reference(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized != "" && !strings.Contains(normalized, "pending") && !strings.Contains(normalized, "example") && !strings.Contains(normalized, "todo")
}
