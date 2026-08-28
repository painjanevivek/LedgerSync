// Package observability centralizes safe logging and metrics conventions.
package observability

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

const Redacted = "[REDACTED]"

var sensitiveLogValue = regexp.MustCompile(`(?i)(?:postgres(?:ql)?://[^:\s]+:[^@\s]+@|authorization:\s*bearer\s+\S+|bearer\s+[A-Za-z0-9._~+/-]{16,})`)

// NewLogger returns a structured logger which redacts known sensitive fields
// before the configured handler receives them. Callers must still avoid logging
// secrets and raw balances in the first place.
func NewLogger(handler slog.Handler) *slog.Logger {
	return slog.New(RedactingHandler{Handler: handler})
}

type RedactingHandler struct{ slog.Handler }

func (h RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(redactAttr(attr))
		return true
	})
	return h.Handler.Handle(ctx, clean)
}

func (h RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, redactAttr(attr))
	}
	return RedactingHandler{Handler: h.Handler.WithAttrs(clean)}
}

func (h RedactingHandler) WithGroup(name string) slog.Handler {
	return RedactingHandler{Handler: h.Handler.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	if isSensitive(attr.Key) {
		return slog.String(attr.Key, Redacted)
	}
	if attr.Value.Kind() == slog.KindGroup {
		children := attr.Value.Group()
		clean := make([]any, 0, len(children))
		for _, child := range children {
			clean = append(clean, redactAttr(child))
		}
		return slog.Group(attr.Key, clean...)
	}
	if attr.Value.Kind() == slog.KindString && sensitiveLogValue.MatchString(attr.Value.String()) {
		return slog.String(attr.Key, Redacted)
	}
	return attr
}

func isSensitive(key string) bool {
	key = strings.ToLower(key)
	for _, forbidden := range []string{"password", "secret", "token", "authorization", "cookie", "session", "csrf", "consistency", "credential", "database_url", "connection_string", "dsn", "private_key", "api_key", "access_key", "balance", "amount", "email", "phone", "address", "ip_address"} {
		if strings.Contains(key, forbidden) {
			return true
		}
	}
	return false
}
