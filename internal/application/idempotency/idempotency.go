// Package idempotency defines operation-namespaced retry primitives shared by
// non-financial commands. The existing transfer fingerprint and operation are
// intentionally left unchanged.
package idempotency

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"hash"
	"strings"
)

var (
	ErrInvalidKey = errors.New("invalid idempotency key")
	ErrConflict   = errors.New("idempotency key belongs to a different request")
	ErrInProgress = errors.New("matching request is still in progress")
)

type State string

const (
	StateInProgress State = "in_progress"
	StateCompleted  State = "completed"
	StateFailed     State = "failed"
)

type Existing struct {
	Fingerprint [sha256.Size]byte
	State       State
}

type Resolution string

const (
	ResolutionReserve Resolution = "reserve"
	ResolutionReplay  Resolution = "replay"
)

func ValidateKey(key string) error {
	key = strings.TrimSpace(key)
	if len(key) < 16 || len(key) > 255 {
		return ErrInvalidKey
	}
	for _, character := range key {
		if character < 0x21 || character > 0x7e {
			return ErrInvalidKey
		}
	}
	return nil
}

// Fingerprint hashes an unambiguous sequence of semantic fields. Each field is
// prefixed by its byte length, so embedded separators cannot create collisions.
func Fingerprint(fields ...string) [sha256.Size]byte {
	digest := sha256.New()
	for _, field := range fields {
		writeField(digest, field)
	}
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func writeField(digest hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write([]byte(value))
}

func Resolve(existing *Existing, requested [sha256.Size]byte) (Resolution, error) {
	if existing == nil {
		return ResolutionReserve, nil
	}
	if subtle.ConstantTimeCompare(existing.Fingerprint[:], requested[:]) != 1 {
		return "", ErrConflict
	}
	switch existing.State {
	case StateCompleted, StateFailed:
		return ResolutionReplay, nil
	case StateInProgress:
		return "", ErrInProgress
	default:
		return "", errors.New("unknown idempotency state")
	}
}
