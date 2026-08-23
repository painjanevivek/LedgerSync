// Package provisioning defines the narrow internal design-partner onboarding
// contract. It is deliberately not a public self-service API.
package provisioning

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

var supportedAccountCategories = []string{
	"customer_funds", "expenses", "operating", "payables", "payroll", "reserve",
}

type Subject struct {
	ID    string   `json:"id"`
	Roles []string `json:"roles"`
}

type Credential struct {
	Reference string   `json:"reference"`
	Audience  string   `json:"audience"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at"`
}

type Account struct {
	ID                string   `json:"id"`
	DisplayName       string   `json:"display_name"`
	Category          string   `json:"category"`
	ExternalReference string   `json:"external_reference,omitempty"`
	OpeningMinor      string   `json:"opening_minor"`
	ReadSubjects      []string `json:"read_subjects"`
	DebitSubjects     []string `json:"debit_subjects"`
	CreditSubjects    []string `json:"credit_subjects"`
}

type Config struct {
	TenantID              string       `json:"tenant_id"`
	ExternalReference     string       `json:"external_reference"`
	Currency              string       `json:"currency"`
	MinimumTransferMinor  string       `json:"minimum_transfer_minor"`
	MaximumTransferMinor  string       `json:"maximum_transfer_minor"`
	ActorRolling24hMinor  string       `json:"actor_rolling_24h_minor"`
	SourceRolling24hMinor string       `json:"source_rolling_24h_minor"`
	TenantRolling24hMinor string       `json:"tenant_rolling_24h_minor"`
	Subjects              []Subject    `json:"subjects"`
	Credentials           []Credential `json:"credentials"`
	Accounts              []Account    `json:"accounts"`
}

func (c Config) Validate(pilotCurrency string) ([sha256.Size]byte, error) {
	var empty [sha256.Size]byte
	if strings.TrimSpace(c.TenantID) == "" || strings.TrimSpace(c.ExternalReference) == "" ||
		c.Currency != pilotCurrency || len(c.Accounts) < 1 || len(c.Accounts) > 10_000 {
		return empty, errors.New("tenant, external reference, selected pilot currency, and 1..10000 accounts are required")
	}

	limits := []string{c.MinimumTransferMinor, c.MaximumTransferMinor, c.ActorRolling24hMinor, c.SourceRolling24hMinor, c.TenantRolling24hMinor}
	parsed := make([]int64, len(limits))
	for i, value := range limits {
		number, err := strconv.ParseInt(value, 10, 64)
		if err != nil || number <= 0 || strconv.FormatInt(number, 10) != value {
			return empty, errors.New("policy limits must be positive canonical integer strings")
		}
		parsed[i] = number
	}
	if parsed[1] < parsed[0] || parsed[2] < parsed[1] || parsed[3] < parsed[1] || parsed[4] < parsed[2] || parsed[4] < parsed[3] {
		return empty, errors.New("policy limit hierarchy is invalid")
	}

	subjects := map[string]bool{}
	for _, subject := range c.Subjects {
		if strings.TrimSpace(subject.ID) == "" {
			return empty, errors.New("subject ID is required")
		}
		if subjects[subject.ID] {
			return empty, fmt.Errorf("duplicate subject %q", subject.ID)
		}
		subjects[subject.ID] = true
		for _, role := range subject.Roles {
			if !slices.Contains([]string{"operator", "finance", "support", "viewer"}, role) {
				return empty, fmt.Errorf("unsupported subject role %q", role)
			}
		}
	}

	credentialReferences := map[string]bool{}
	for _, credential := range c.Credentials {
		if credential.Reference == "" || credential.Audience == "" || credential.ExpiresAt == "" || len(credential.Scopes) == 0 {
			return empty, errors.New("credential reference, audience, expiry, and scopes are required")
		}
		if credentialReferences[credential.Reference] {
			return empty, fmt.Errorf("duplicate credential reference %q", credential.Reference)
		}
		credentialReferences[credential.Reference] = true
		expiresAt, err := time.Parse(time.RFC3339, credential.ExpiresAt)
		if err != nil || !expiresAt.After(time.Now().UTC()) {
			return empty, fmt.Errorf("credential %q must have a future RFC3339 expiry", credential.Reference)
		}
		for _, scope := range credential.Scopes {
			if !slices.Contains([]string{"accounts:read", "transactions:read", "reconciliation:read", "transfers:read", "transfers:write"}, scope) {
				return empty, fmt.Errorf("credential %q uses unsupported scope %q", credential.Reference, scope)
			}
		}
	}

	accounts := map[string]bool{}
	for _, account := range c.Accounts {
		if account.ID == "" || account.DisplayName == "" || !slices.Contains(supportedAccountCategories, account.Category) {
			return empty, errors.New("account ID, name, and supported category are required")
		}
		if accounts[account.ID] {
			return empty, errors.New("duplicate account ID")
		}
		accounts[account.ID] = true
		opening, err := strconv.ParseInt(account.OpeningMinor, 10, 64)
		if err != nil || opening < 0 || strconv.FormatInt(opening, 10) != account.OpeningMinor {
			return empty, errors.New("opening balances must be non-negative canonical integer strings")
		}
		permissions := append(append(append([]string{}, account.ReadSubjects...), account.DebitSubjects...), account.CreditSubjects...)
		for _, id := range permissions {
			if !subjects[id] {
				return empty, fmt.Errorf("account permission references unknown subject %q", id)
			}
		}
	}

	encoded, err := json.Marshal(c)
	if err != nil {
		return empty, err
	}
	return sha256.Sum256(encoded), nil
}
