// Command assertion-contract is a non-networked cross-runtime contract probe.
// It reads one synthetic assertion from stdin and verifies it with production
// identity code. It is intended for CI and local release evidence only.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

func main() {
	secret := os.Getenv("LEDGERSYNC_BFF_ASSERTION_SECRET")
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 16*1024))
	if err != nil {
		fail(err)
	}
	provider := identity.DevelopmentProvider{SubjectID: "bff-contract-probe", TenantID: "tenant-a", Scopes: []string{identity.BFFActorScope}}
	authenticator, err := identity.NewRequestAuthenticator(provider, secret)
	if err != nil {
		fail(err)
	}
	principal, err := authenticator.Authenticate(context.Background(), "development-local-only", strings.TrimSpace(string(raw)))
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"subject_id": principal.SubjectID, "tenant_id": principal.TenantID, "accounts_read": principal.HasScope("accounts:read")}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
