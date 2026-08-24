package handlers

import (
	"errors"
	"net/http"

	contractassets "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/contracts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

// DeveloperContractHandler serves only compile-time embedded, reviewed public
// contract assets. It does not accept a target URL, headers, credentials, or a
// request body and therefore cannot become a browser-side private API runner.
type DeveloperContractHandler struct {
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	audit         AuditRecorder
}

func NewDeveloperContractHandler(provider identity.Provider) *DeveloperContractHandler {
	return &DeveloperContractHandler{identity: provider}
}

func (h *DeveloperContractHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *DeveloperContractHandler {
	h.authenticator = authenticator
	return h
}

func (h *DeveloperContractHandler) WithRateLimiter(limiter RateLimiter, limit int) *DeveloperContractHandler {
	h.rateLimiter, h.rateLimit = limiter, limit
	return h
}

func (h *DeveloperContractHandler) WithAuditRecorder(audit AuditRecorder) *DeveloperContractHandler {
	h.audit = audit
	return h
}

func (h *DeveloperContractHandler) Metadata(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request, "developer:metadata") {
		return
	}
	writeContractAsset(writer, "application/json; charset=utf-8", "", contractassets.DeveloperExamplesV1())
}

func (h *DeveloperContractHandler) OpenAPI(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request, "developer:openapi") {
		return
	}
	writeContractAsset(writer, "application/yaml; charset=utf-8", `attachment; filename="ledgersync-openapi-`+contractassets.Version+`.yaml"`, contractassets.OpenAPIYAML())
}

func (h *DeveloperContractHandler) authorize(writer http.ResponseWriter, request *http.Request, route string) bool {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return false
	}
	if h == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("developer contract handler is not configured"))
		return false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return false
	}
	if identity.RequireScope(principal, "developer:read") != nil {
		writeScopeDenial(writer, request, h.audit, principal, "developer:read")
		return false
	}
	return enforceRateLimit(writer, request, h.rateLimiter, principal, route, h.rateLimit, false)
}

func (h *DeveloperContractHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func writeContractAsset(writer http.ResponseWriter, contentType, disposition, content string) {
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if disposition != "" {
		writer.Header().Set("Content-Disposition", disposition)
	}
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(content))
}
