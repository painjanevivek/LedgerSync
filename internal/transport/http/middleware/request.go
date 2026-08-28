package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"

	contractassets "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/contracts"
)

type correlationKey struct{}

// CorrelationID returns the server-generated ID associated with this request.
func CorrelationID(ctx context.Context) string {
	value, _ := ctx.Value(correlationKey{}).(string)
	return value
}

// Correlation assigns an unpredictable server-generated request ID. Client IDs
// are never reflected because they are untrusted input and complicate tracing.
func Correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := newID()
		writer.Header().Set("X-Request-ID", id)
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), correlationKey{}, id)))
	})
}

// Contract identifies the reviewed API contract and the financial-data mode on
// every response, including errors and health endpoints. The mode is derived
// only from trusted process configuration; callers cannot select or override it.
func Contract(environment string, next http.Handler) http.Handler {
	mode := "production"
	if strings.EqualFold(strings.TrimSpace(environment), "development") || strings.EqualFold(strings.TrimSpace(environment), "test") {
		mode = "sandbox"
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-LedgerSync-API-Version", contractassets.Version)
		writer.Header().Set("X-LedgerSync-Mode", mode)
		next.ServeHTTP(writer, request)
	})
}

func newID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("generate correlation ID: %v", err))
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}
