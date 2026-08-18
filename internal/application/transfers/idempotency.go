package transfers

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var (
	ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")
	ErrIdempotencyConflict   = errors.New("idempotency key belongs to a different request")
	ErrRequestInProgress     = errors.New("matching request is still in progress")
)

const transferOperation = "transfers.create.v1"

type IdempotencyRequest struct {
	TenantID        string
	ActorSubjectID  string
	Operation       string
	Key             string
	DebitAccountID  string
	CreditAccountID string
	Amount          money.Money
}

// ValidateKey rejects trivially guessable or unbounded keys before they reach
// storage. UUIDv4 and 32-byte random base64url keys both satisfy this contract.
func ValidateKey(key string) error {
	key = strings.TrimSpace(key)
	if len(key) < 16 || len(key) > 255 {
		return ErrInvalidIdempotencyKey
	}
	for _, character := range key {
		if character < 0x21 || character > 0x7e {
			return ErrInvalidIdempotencyKey
		}
	}
	return nil
}

// Fingerprint is a fixed-length hash of the canonical financial request. The
// idempotency key itself is not included, and exact minor units avoid decimal
// spelling differences such as 1.2 versus 1.20 changing the meaning.
func Fingerprint(request IdempotencyRequest) ([sha256.Size]byte, error) {
	if err := ValidateKey(request.Key); err != nil {
		return [sha256.Size]byte{}, err
	}
	if request.Operation != transferOperation || strings.TrimSpace(request.TenantID) == "" || strings.TrimSpace(request.ActorSubjectID) == "" ||
		strings.TrimSpace(request.DebitAccountID) == "" || strings.TrimSpace(request.CreditAccountID) == "" || request.DebitAccountID == request.CreditAccountID || !request.Amount.IsPositive() {
		return [sha256.Size]byte{}, fmt.Errorf("invalid idempotency request")
	}
	canonical := strings.Join([]string{
		request.TenantID,
		request.ActorSubjectID,
		request.Operation,
		request.DebitAccountID,
		request.CreditAccountID,
		request.Amount.Currency().Code,
		fmt.Sprintf("%d", request.Amount.Minor()),
	}, "\n")
	return sha256.Sum256([]byte(canonical)), nil
}

type IdempotencyState string

const (
	StateInProgress IdempotencyState = "in_progress"
	StateCompleted  IdempotencyState = "completed"
	StateFailed     IdempotencyState = "failed"
)

type ExistingRequest struct {
	Fingerprint [sha256.Size]byte
	State       IdempotencyState
}

type Resolution string

const (
	ResolutionReserve Resolution = "reserve"
	ResolutionReplay  Resolution = "replay"
)

// ResolveExisting tells a repository whether it may reserve a key or must
// replay a stored final response. A same-key different-body request is never
// allowed to proceed.
func ResolveExisting(existing *ExistingRequest, requested [sha256.Size]byte) (Resolution, error) {
	if existing == nil {
		return ResolutionReserve, nil
	}
	if subtle.ConstantTimeCompare(existing.Fingerprint[:], requested[:]) != 1 {
		return "", ErrIdempotencyConflict
	}
	switch existing.State {
	case StateCompleted, StateFailed:
		return ResolutionReplay, nil
	case StateInProgress:
		return "", ErrRequestInProgress
	default:
		return "", fmt.Errorf("unknown idempotency state")
	}
}
