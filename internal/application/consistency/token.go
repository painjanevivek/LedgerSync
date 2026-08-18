// Package consistency issues compact, signed read requirements. A requirement
// authorizes a caller to demand at least a specific committed balance version;
// it is not a bearer authorization token and contains no monetary amount.
package consistency

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidRequirement = errors.New("invalid consistency requirement")
	ErrExpiredRequirement = errors.New("expired consistency requirement")
)

type Requirement struct {
	TenantID       string    `json:"tenant_id"`
	AccountID      string    `json:"account_id"`
	MinimumVersion int64     `json:"minimum_version"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	KeyID          string    `json:"kid"`
}

type Key struct {
	ID     string
	Secret []byte
}

// Issuer supports key rotation by retaining verification keys until all tokens
// issued with them have expired. Current is the only key used for issuance.
type Issuer struct {
	current Key
	verify  map[string][]byte
	clock   func() time.Time
	ttl     time.Duration
}

func NewIssuer(current Key, previous []Key, clock func() time.Time, ttl time.Duration) (*Issuer, error) {
	if strings.TrimSpace(current.ID) == "" || len(current.Secret) < 32 {
		return nil, errors.New("a consistency signing key with at least 32 bytes is required")
	}
	if clock == nil {
		clock = time.Now
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	verify := map[string][]byte{current.ID: append([]byte(nil), current.Secret...)}
	for _, key := range previous {
		if strings.TrimSpace(key.ID) == "" || len(key.Secret) < 32 {
			return nil, errors.New("previous consistency signing key is invalid")
		}
		if _, exists := verify[key.ID]; exists {
			return nil, errors.New("consistency signing key IDs must be unique")
		}
		verify[key.ID] = append([]byte(nil), key.Secret...)
	}
	return &Issuer{current: Key{ID: current.ID, Secret: append([]byte(nil), current.Secret...)}, verify: verify, clock: clock, ttl: ttl}, nil
}

func (i *Issuer) Issue(tenantID, accountID string, minimumVersion int64) (string, error) {
	if i == nil || tenantID == "" || accountID == "" || minimumVersion < 0 {
		return "", ErrInvalidRequirement
	}
	now := i.clock().UTC()
	requirement := Requirement{TenantID: tenantID, AccountID: accountID, MinimumVersion: minimumVersion, IssuedAt: now, ExpiresAt: now.Add(i.ttl), KeyID: i.current.ID}
	payload, err := json.Marshal(requirement)
	if err != nil {
		return "", fmt.Errorf("encode consistency requirement: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + sign(i.current.Secret, encoded), nil
}

func (i *Issuer) Verify(raw string) (Requirement, error) {
	if i == nil {
		return Requirement{}, ErrInvalidRequirement
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Requirement{}, ErrInvalidRequirement
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Requirement{}, ErrInvalidRequirement
	}
	var requirement Requirement
	if err := json.Unmarshal(payload, &requirement); err != nil {
		return Requirement{}, ErrInvalidRequirement
	}
	secret, ok := i.verify[requirement.KeyID]
	if !ok || !hmac.Equal([]byte(parts[1]), []byte(sign(secret, parts[0]))) {
		return Requirement{}, ErrInvalidRequirement
	}
	if requirement.TenantID == "" || requirement.AccountID == "" || requirement.MinimumVersion < 0 || requirement.IssuedAt.IsZero() || requirement.ExpiresAt.IsZero() || !requirement.ExpiresAt.After(requirement.IssuedAt) {
		return Requirement{}, ErrInvalidRequirement
	}
	if !i.clock().UTC().Before(requirement.ExpiresAt) {
		return Requirement{}, ErrExpiredRequirement
	}
	return requirement, nil
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
