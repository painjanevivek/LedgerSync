package handlers

import (
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

func canonicalIdentifier(request *http.Request, kind identifier.Kind, raw string) (string, bool) {
	parsed, ok := parseIdentifier(request, kind, raw)
	return parsed.String(), ok
}

func parseIdentifier(request *http.Request, kind identifier.Kind, raw string) (identifier.UUID, bool) {
	parsed, err := identifier.Parse(request.Context(), kind, raw)
	return parsed, err == nil
}

func requireCanonicalIdentifier(writer http.ResponseWriter, request *http.Request, kind identifier.Kind, raw string) (string, bool) {
	canonical, ok := canonicalIdentifier(request, kind, raw)
	if !ok {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
	}
	return canonical, ok
}
