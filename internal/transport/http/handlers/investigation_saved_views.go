package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxSavedViewBodyBytes = 8 << 10

type savedViewCreateRequest struct {
	Name                string            `json:"name"`
	FilterSchemaVersion string            `json:"filter_schema_version"`
	Domain              string            `json:"domain"`
	Filters             map[string]string `json:"filters"`
}

type savedViewRenameRequest struct {
	ExpectedVersion string `json:"expected_version"`
	Name            string `json:"name"`
}

func (h *InvestigationHandler) SavedViews(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeSavedViews(writer, request, false)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	page, err := repository.ListSavedViews(request.Context(), principal.TenantID, principal.SubjectID, access)
	if err != nil {
		httptransport.WriteError(writer, request, savedViewPublicError(err))
		return
	}
	writeInvestigationJSON(writer, page)
}

func (h *InvestigationHandler) CreateSavedView(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeSavedViews(writer, request, true)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	var input savedViewCreateRequest
	if err := decodeSavedViewJSON(writer, request, &input); err != nil {
		writeSavedViewDecodeError(writer, request, err)
		return
	}
	schemaVersion, err := investigation.ParseSavedViewVersion(input.FilterSchemaVersion, false)
	if err != nil || schemaVersion != investigation.SavedViewFilterSchemaVersion {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	name, err := investigation.NormalizeSavedViewName(input.Name)
	filters, _, definitionErr := investigation.NormalizeSavedViewDefinition(input.Domain, int(schemaVersion), input.Filters)
	if err != nil || definitionErr != nil || !investigation.SavedViewDefinitionAllowed(input.Domain, filters, access) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	view, err := repository.CreateSavedView(request.Context(), investigation.SavedViewCreate{
		TenantID: principal.TenantID, ActorID: principal.SubjectID, Name: name, Domain: input.Domain,
		FilterSchemaVersion: int(schemaVersion), Filters: filters, Access: access, CorrelationID: middleware.CorrelationID(request.Context()), OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		httptransport.WriteError(writer, request, savedViewPublicError(err))
		return
	}
	writer.Header().Set("Location", "/api/investigation/saved-views/"+view.ID)
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(view)
}

func (h *InvestigationHandler) RenameSavedView(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeSavedViews(writer, request, true)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	viewID, ok := requireCanonicalIdentifier(writer, request, identifier.KindSavedView, request.PathValue("savedViewId"))
	if !ok {
		return
	}
	var input savedViewRenameRequest
	if err := decodeSavedViewJSON(writer, request, &input); err != nil {
		writeSavedViewDecodeError(writer, request, err)
		return
	}
	version, versionErr := investigation.ParseSavedViewVersion(input.ExpectedVersion, false)
	name, nameErr := investigation.NormalizeSavedViewName(input.Name)
	if versionErr != nil || nameErr != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	view, err := repository.RenameSavedView(request.Context(), investigation.SavedViewRename{TenantID: principal.TenantID, ActorID: principal.SubjectID, SavedViewID: viewID, Name: name, ExpectedVersion: version, Access: access, CorrelationID: middleware.CorrelationID(request.Context()), OccurredAt: time.Now().UTC()})
	if err != nil {
		httptransport.WriteError(writer, request, savedViewPublicError(err))
		return
	}
	writeInvestigationJSON(writer, view)
}

func (h *InvestigationHandler) DeleteSavedView(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeSavedViews(writer, request, true)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	viewID, ok := requireCanonicalIdentifier(writer, request, identifier.KindSavedView, request.PathValue("savedViewId"))
	if !ok {
		return
	}
	version, err := parseSavedViewIfMatch(request.Header.Get("If-Match"))
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	err = repository.DeleteSavedView(request.Context(), investigation.SavedViewDelete{TenantID: principal.TenantID, ActorID: principal.SubjectID, SavedViewID: viewID, ExpectedVersion: version, Access: access, CorrelationID: middleware.CorrelationID(request.Context()), OccurredAt: time.Now().UTC()})
	if err != nil {
		httptransport.WriteError(writer, request, savedViewPublicError(err))
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (h *InvestigationHandler) authorizeSavedViews(writer http.ResponseWriter, request *http.Request, write bool) (identity.Principal, investigation.SavedViewAccess, investigation.SavedViewRepository, bool) {
	if h == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("investigation handler is not configured"))
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	repository, configured := h.repository.(investigation.SavedViewRepository)
	if !configured {
		httptransport.WriteError(writer, request, errors.New("saved investigation view repository is not configured"))
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:investigate")
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	if !principal.HasScope("investigation:read") {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:read")
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	if write && !principal.HasScope("investigation:write") {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:write")
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	access := investigation.SavedViewAccess{
		Accounts: principal.HasScope("accounts:read"), Transfers: principal.HasScope("transfers:read"), Funding: principal.HasScope("funding:read"),
		FundingApprovals: principal.HasScope("funding:approve"), Corrections: principal.HasScope("corrections:read"), CorrectionApprovals: principal.HasScope("corrections:approve"),
		Events: principal.HasScope("events:read"), Webhooks: principal.HasScope("webhooks:read"),
	}
	if !access.AnyReadableDomain() {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:read")
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	limit, route, failClosed := h.rateLimit, "investigation:saved-views-read", false
	if write {
		limit, route, failClosed = h.savedViewWriteLimit, "investigation:saved-views-write", true
		if limit < 1 {
			limit = h.rateLimit
		}
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, route, limit, failClosed) {
		return identity.Principal{}, investigation.SavedViewAccess{}, nil, false
	}
	return principal, access, repository, true
}

func decodeSavedViewJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errSavedViewMediaType
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxSavedViewBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request contains more than one JSON value")
	}
	return nil
}

var errSavedViewMediaType = errors.New("saved investigation view content type must be application/json")

func writeSavedViewDecodeError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, errSavedViewMediaType) {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "Content-Type must be application/json."})
		return
	}
	httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
}

func parseSavedViewIfMatch(value string) (int64, error) {
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return 0, investigation.ErrInvalidSavedView
	}
	return investigation.ParseSavedViewVersion(value[1:len(value)-1], false)
}

func savedViewPublicError(err error) error {
	switch {
	case errors.Is(err, investigation.ErrInvalidSavedView):
		return httptransport.ErrBadRequest
	case errors.Is(err, investigation.ErrSavedViewNotFound):
		return httptransport.ErrNotFound
	case errors.Is(err, investigation.ErrSavedViewVersion):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "saved_view_version_conflict", Message: "The saved view changed in another session. Read the current version before retrying."}
	case errors.Is(err, investigation.ErrSavedViewConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "saved_view_name_conflict", Message: "A saved view with that name already exists."}
	case errors.Is(err, investigation.ErrSavedViewLimit):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "saved_view_limit_reached", Message: "The operator saved-view limit has been reached."}
	default:
		return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Saved investigation views are unavailable."}
	}
}
