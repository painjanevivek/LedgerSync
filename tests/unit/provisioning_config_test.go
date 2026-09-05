package unit_test

import (
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/provisioning"
)

func TestProvisioningConfigRequiresExactMoneyKnownSubjectsAndExternalCredentials(t *testing.T) {
	configuration := validProvisioningConfig()
	first, err := configuration.Validate("USD")
	if err != nil {
		t.Fatal(err)
	}
	second, err := configuration.Validate("USD")
	if err != nil || first != second {
		t.Fatalf("fingerprint must be stable: err=%v", err)
	}

	configuration.Accounts[0].OpeningMinor = "1.00"
	if _, err = configuration.Validate("USD"); err == nil {
		t.Fatal("decimal opening balance must be rejected")
	}
	configuration = validProvisioningConfig()
	configuration.Accounts[0].OpeningMinor = "1"
	if _, err = configuration.Validate("USD"); err == nil {
		t.Fatal("non-zero opening value must require the approved import workflow")
	}
	configuration = validProvisioningConfig()
	configuration.Accounts[0].DebitSubjects = []string{"unknown"}
	if _, err = configuration.Validate("USD"); err == nil {
		t.Fatal("unknown permission subject must be rejected")
	}
	configuration = validProvisioningConfig()
	configuration.Credentials[0].Scopes = []string{"admin:*"}
	if _, err = configuration.Validate("USD"); err == nil {
		t.Fatal("unsupported workload scope must be rejected")
	}
}

func TestProvisioningConfigDecoderRejectsIgnoredOrTrailingInstructions(t *testing.T) {
	for _, raw := range []string{
		`{"tenant_id":"tenant","unknown_limit":"100"}`,
		`{} {}`,
	} {
		if _, err := provisioning.DecodeConfig(strings.NewReader(raw)); err == nil {
			t.Fatalf("unsafe provisioning JSON accepted: %s", raw)
		}
	}
}

func TestProvisioningConfigSupportsTheAccountHistoryScopeUsedByTheOperator(t *testing.T) {
	configuration := validProvisioningConfig()
	configuration.Credentials[0].Scopes = []string{"accounts:read", "transactions:read", "transfers:read"}
	if _, err := configuration.Validate("USD"); err != nil {
		t.Fatalf("operator history scope must be provisionable: %v", err)
	}
}

func validProvisioningConfig() provisioning.Config {
	return provisioning.Config{
		TenantID: "00000000-0000-0000-0000-000000000101", ExternalReference: "partner-101", Currency: "USD",
		MinimumTransferMinor: "1", MaximumTransferMinor: "100000", ActorRolling24hMinor: "500000", SourceRolling24hMinor: "500000", TenantRolling24hMinor: "1000000",
		Subjects:    []provisioning.Subject{{ID: "partner-operator", Roles: []string{"operator"}}},
		Credentials: []provisioning.Credential{{Reference: "idp://pilot/client-101", Audience: "ledgersync-api", Scopes: []string{"accounts:read", "transfers:write"}, ExpiresAt: "2035-01-01T00:00:00Z"}},
		Accounts:    []provisioning.Account{{ID: "00000000-0000-0000-0000-000000000111", DisplayName: "Operating", Category: "operating", OpeningMinor: "0", ReadSubjects: []string{"partner-operator"}, DebitSubjects: []string{"partner-operator"}, CreditSubjects: []string{"partner-operator"}}},
	}
}
