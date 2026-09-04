// Package identifier parses untrusted UUID text into one canonical value.
// It deliberately rejects the alternate UUID spellings accepted by many
// parsers so equivalent identifiers cannot split locks, counters, or maps.
package identifier

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"

	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid identifier")

var canonicalShape = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

// Kind is a closed, non-sensitive telemetry dimension. Raw identifier values
// must never be attached to an error, log, trace, or metric.
type Kind string

const (
	KindUnknown        Kind = "unknown"
	KindTenant         Kind = "tenant"
	KindAccount        Kind = "account"
	KindTransfer       Kind = "transfer"
	KindFunding        Kind = "funding"
	KindCorrection     Kind = "correction"
	KindWebhook        Kind = "webhook"
	KindDelivery       Kind = "delivery"
	KindEvent          Kind = "event"
	KindReconciliation Kind = "reconciliation"
	KindInvestigation  Kind = "investigation"
	KindSavedView      Kind = "saved_view"
	KindCredential     Kind = "credential"
	KindOpeningImport  Kind = "opening_import"
)

// UUID is canonical lowercase RFC 4122 text. It is comparable and therefore
// safe as a map key, and Value serializes it directly at the SQL boundary.
type UUID string

func (id UUID) String() string { return string(id) }

func (id UUID) Value() (driver.Value, error) { return string(id), nil }

func (id UUID) MarshalText() ([]byte, error) { return []byte(id), nil }

type Observer interface {
	ObserveInvalidIdentifier(context.Context, Kind)
}

type observerContextKey struct{}

func WithObserver(ctx context.Context, observer Observer) context.Context {
	if observer == nil {
		return ctx
	}
	return context.WithValue(ctx, observerContextKey{}, observer)
}

// Parse accepts canonical UUID shape with any hex letter case and returns the
// single lowercase representation. Whitespace, braces, compact forms, URNs,
// and the nil UUID are rejected.
func Parse(ctx context.Context, kind Kind, raw string) (UUID, error) {
	if !canonicalShape.MatchString(raw) {
		observeInvalid(ctx, kind)
		return "", ErrInvalid
	}
	parsed, err := uuid.Parse(raw)
	if err != nil || parsed == uuid.Nil {
		observeInvalid(ctx, kind)
		return "", ErrInvalid
	}
	return UUID(parsed.String()), nil
}

func Canonicalize(ctx context.Context, kind Kind, raw string) (string, error) {
	id, err := Parse(ctx, kind, raw)
	return id.String(), err
}

func observeInvalid(ctx context.Context, kind Kind) {
	if ctx == nil {
		return
	}
	if !validKind(kind) {
		kind = KindUnknown
	}
	if observer, ok := ctx.Value(observerContextKey{}).(Observer); ok && observer != nil {
		observer.ObserveInvalidIdentifier(ctx, kind)
	}
}

func validKind(kind Kind) bool {
	switch kind {
	case KindTenant, KindAccount, KindTransfer, KindFunding, KindCorrection,
		KindWebhook, KindDelivery, KindEvent, KindReconciliation,
		KindInvestigation, KindSavedView, KindCredential:
		return true
	case KindOpeningImport:
		return true
	default:
		return false
	}
}
