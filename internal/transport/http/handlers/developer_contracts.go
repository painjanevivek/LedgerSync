package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"

	contractassets "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/contracts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

// DeveloperContractHandler serves reviewed contract assets and tenant-scoped
// observations. It does not accept an upstream URL or execute commands.
type DeveloperContractHandler struct {
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	audit         AuditRecorder
	distributed   interface {
		Read(context.Context, string, string) operations.DistributedEvidence
	}
}

func (h *DeveloperContractHandler) WithDistributedReader(reader interface {
	Read(context.Context, string, string) operations.DistributedEvidence
}) *DeveloperContractHandler {
	h.distributed = reader
	return h
}
func (h *DeveloperContractHandler) Distributed(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorizePrincipal(writer, request, "developer:distributed")
	if !ok {
		return
	}
	if request.URL.RawQuery != "" {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	if h.distributed == nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Service observations are unavailable."})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), time.Second)
	defer cancel()
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(writer).Encode(h.distributed.Read(ctx, principal.TenantID, principal.SubjectID))
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
	_, ok := h.authorizePrincipal(writer, request, route)
	return ok
}
func (h *DeveloperContractHandler) authorizePrincipal(writer http.ResponseWriter, request *http.Request, route string) (identity.Principal, bool) {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return identity.Principal{}, false
	}
	if h == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("developer contract handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, false
	}
	if identity.RequireScope(principal, "developer:read") != nil {
		writeScopeDenial(writer, request, h.audit, principal, "developer:read")
		return identity.Principal{}, false
	}
	return principal, enforceRateLimit(writer, request, h.rateLimiter, principal, route, h.rateLimit, false)
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
