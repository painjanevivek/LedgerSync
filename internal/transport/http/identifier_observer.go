package httptransport

import (
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
)

// WithIdentifierObserver makes privacy-safe invalid-identifier telemetry
// available to every authenticated route without coupling domain parsing to a
// concrete metrics implementation.
func WithIdentifierObserver(next http.Handler, observer identifier.Observer) http.Handler {
	if observer == nil {
		return next
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(writer, request.WithContext(identifier.WithObserver(request.Context(), observer)))
	})
}
