package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type workspaceCreateRequest struct {
	Title        string `json:"title"`
	Taxonomy     string `json:"taxonomy"`
	QueryContext struct {
		Kind       string `json:"kind"`
		RecordType string `json:"record_type"`
		Value      string `json:"value"`
	} `json:"query_context"`
	RootRecord struct {
		RecordType string `json:"record_type"`
		RecordID   string `json:"record_id"`
	} `json:"root_record"`
}

type workspaceHandoffRequest struct {
	ExpectedVersion string `json:"expected_version"`
	TargetSubjectID string `json:"target_subject_id"`
}

type workspaceStatusRequest struct {
	ExpectedVersion string `json:"expected_version"`
}

type workspaceEvidenceBundleRequest struct {
	ExpectedVersion string `json:"expected_version"`
}

func (h *InvestigationHandler) Workspaces(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeWorkspaces(writer, request, false)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	page, err := repository.ListWorkspaces(request.Context(), principal.TenantID, principal.SubjectID, access)
	if err != nil {
		httptransport.WriteError(writer, request, workspacePublicError(err))
		return
	}
	writeInvestigationJSON(writer, page)
}

func (h *InvestigationHandler) CreateWorkspace(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeWorkspaces(writer, request, true)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	var input workspaceCreateRequest
	if err := decodeSavedViewJSON(writer, request, &input); err != nil {
		writeSavedViewDecodeError(writer, request, err)
		return
	}
	if input.QueryContext.RecordType != input.RootRecord.RecordType {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	command, err := investigation.NormalizeWorkspaceCreate(investigation.WorkspaceCreate{
		TenantID: principal.TenantID, ActorID: principal.SubjectID, Title: input.Title, Taxonomy: input.Taxonomy,
		QueryKind: input.QueryContext.Kind, QueryValue: input.QueryContext.Value, RootRecordType: input.RootRecord.RecordType,
		RootRecordID: input.RootRecord.RecordID, Access: access, CorrelationID: middleware.CorrelationID(request.Context()), OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	workspace, err := repository.CreateWorkspace(request.Context(), command)
	if err != nil {
		httptransport.WriteError(writer, request, workspacePublicError(err))
		return
	}
	writer.Header().Set("Location", "/api/investigation/workspaces/"+workspace.ID)
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(workspace)
}

func (h *InvestigationHandler) Workspace(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeWorkspaces(writer, request, false)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	id := strings.ToLower(strings.TrimSpace(request.PathValue("investigationId")))
	if !canonicalInvestigationUUID.MatchString(id) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	workspace, err := repository.GetWorkspace(request.Context(), principal.TenantID, principal.SubjectID, id, access)
	if err != nil {
		httptransport.WriteError(writer, request, workspacePublicError(err))
		return
	}
	writeInvestigationJSON(writer, workspace)
}

func (h *InvestigationHandler) WorkspaceEvidenceBundle(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, ok := h.authorizeWorkspaces(writer, request, false)
	if !ok {
		return
	}
	if !principal.HasScope("exports:read") {
		writeScopeDenial(writer, request, h.audit, principal, "exports:read")
		return
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, "investigation:evidence-bundle", h.rateLimit, true) {
		return
	}
	if h.audit == nil || !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	id := strings.ToLower(strings.TrimSpace(request.PathValue("investigationId")))
	if !canonicalInvestigationUUID.MatchString(id) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	var input workspaceEvidenceBundleRequest
	if err := decodeSavedViewJSON(writer, request, &input); err != nil {
		writeSavedViewDecodeError(writer, request, err)
		return
	}
	expectedVersion, err := investigation.ParseWorkspaceVersion(input.ExpectedVersion)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	workspace, err := repository.GetWorkspace(request.Context(), principal.TenantID, principal.SubjectID, id, access)
	if err != nil {
		httptransport.WriteError(writer, request, workspacePublicError(err))
		return
	}
	currentVersion, err := investigation.ParseWorkspaceVersion(workspace.Version)
	if err != nil {
		httptransport.WriteError(writer, request, workspacePublicError(err))
		return
	}
	if expectedVersion != currentVersion {
		httptransport.WriteError(writer, request, workspacePublicError(investigation.ErrWorkspaceVersion))
		return
	}
	generatedAt := time.Now().UTC()
	correlationID := middleware.CorrelationID(request.Context())
	bundle, err := investigation.GenerateEvidenceBundle(investigation.EvidenceBundleRequest{Workspace: workspace, CorrelationID: correlationID, GeneratedAt: generatedAt})
	if err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "export_unavailable", Message: "The evidence bundle could not be generated within its safe bounds."})
		return
	}
	if err = h.audit.Record(request.Context(), db.AuditEvent{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, EventType: "investigation.evidence_bundle_generated", TargetType: "investigation_workspace", TargetID: workspace.ID, Outcome: "succeeded", CorrelationID: correlationID, OccurredAt: generatedAt,
		Metadata: map[string]string{"schema_version": investigation.EvidenceBundleSchemaVersion, "bundle_sha256": bundle.SHA256, "bundle_bytes": strconv.Itoa(len(bundle.Content)), "file_count": strconv.Itoa(bundle.FileCount), "historical_reference_rows": strconv.Itoa(bundle.ReferenceRows), "current_evidence_rows": strconv.Itoa(bundle.EvidenceRows), "workspace_version": workspace.Version, "expires_at_utc": bundle.ExpiresAt.Format(time.RFC3339Nano)},
	}); err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "export_unavailable", Message: "The evidence bundle could not establish required audit evidence."})
		return
	}
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", `attachment; filename="`+bundle.Filename+`"`)
	writer.Header().Set("Content-Length", strconv.Itoa(len(bundle.Content)))
	writer.Header().Set("Cache-Control", "no-store, private, max-age=0")
	writer.Header().Set("Pragma", "no-cache")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-LedgerSync-Bundle-Schema", investigation.EvidenceBundleSchemaVersion)
	writer.Header().Set("X-LedgerSync-Bundle-SHA256", bundle.SHA256)
	writer.Header().Set("X-LedgerSync-Bundle-Expires-At", bundle.ExpiresAt.Format(time.RFC3339Nano))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(bundle.Content)
}

func (h *InvestigationHandler) HandoffWorkspace(writer http.ResponseWriter, request *http.Request) {
	principal, access, repository, id, ok := h.authorizeWorkspaceMutation(writer, request)
	if !ok {
		return
	}
	var input workspaceHandoffRequest
	if err := decodeSavedViewJSON(writer, request, &input); err != nil {
		writeSavedViewDecodeError(writer, request, err)
		return
	}
	version, versionErr := investigation.ParseWorkspaceVersion(input.ExpectedVersion)
	target, targetErr := investigation.NormalizeWorkspaceSubject(input.TargetSubjectID)
	if versionErr != nil || targetErr != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	receipt, err := repository.HandoffWorkspace(request.Context(), investigation.WorkspaceHandoff{TenantID: principal.TenantID, ActorID: principal.SubjectID, InvestigationID: id, TargetSubjectID: target, ExpectedVersion: version, Access: access, CorrelationID: middleware.CorrelationID(request.Context()), OccurredAt: time.Now().UTC()})
	if err != nil {
		httptransport.WriteError(writer, request, workspacePublicError(err))
		return
	}
	writeInvestigationJSON(writer, receipt)
}

func (h *InvestigationHandler) CloseWorkspace(writer http.ResponseWriter, request *http.Request) {
	h.changeWorkspaceStatus(writer, request, "closed")
}
func (h *InvestigationHandler) ReopenWorkspace(writer http.ResponseWriter, request *http.Request) {
	h.changeWorkspaceStatus(writer, request, "open")
}

func (h *InvestigationHandler) changeWorkspaceStatus(writer http.ResponseWriter, request *http.Request, status string) {
	principal, access, repository, id, ok := h.authorizeWorkspaceMutation(writer, request)
	if !ok {
		return
	}
	var input workspaceStatusRequest
	if err := decodeSavedViewJSON(writer, request, &input); err != nil {
		writeSavedViewDecodeError(writer, request, err)
		return
	}
	version, err := investigation.ParseWorkspaceVersion(input.ExpectedVersion)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	receipt, err := repository.ChangeWorkspaceStatus(request.Context(), investigation.WorkspaceStatusChange{TenantID: principal.TenantID, ActorID: principal.SubjectID, InvestigationID: id, TargetStatus: status, ExpectedVersion: version, Access: access, CorrelationID: middleware.CorrelationID(request.Context()), OccurredAt: time.Now().UTC()})
	if err != nil {
		httptransport.WriteError(writer, request, workspacePublicError(err))
		return
	}
	writeInvestigationJSON(writer, receipt)
}

func (h *InvestigationHandler) authorizeWorkspaceMutation(writer http.ResponseWriter, request *http.Request) (identity.Principal, investigation.SearchAccess, investigation.WorkspaceRepository, string, bool) {
	principal, access, repository, ok := h.authorizeWorkspaces(writer, request, true)
	if !ok {
		return identity.Principal{}, investigation.SearchAccess{}, nil, "", false
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return identity.Principal{}, investigation.SearchAccess{}, nil, "", false
	}
	id := strings.ToLower(strings.TrimSpace(request.PathValue("investigationId")))
	if !canonicalInvestigationUUID.MatchString(id) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return identity.Principal{}, investigation.SearchAccess{}, nil, "", false
	}
	return principal, access, repository, id, true
}

func (h *InvestigationHandler) authorizeWorkspaces(writer http.ResponseWriter, request *http.Request, write bool) (identity.Principal, investigation.SearchAccess, investigation.WorkspaceRepository, bool) {
	if h == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("investigation handler is not configured"))
		return identity.Principal{}, investigation.SearchAccess{}, nil, false
	}
	repository, configured := h.repository.(investigation.WorkspaceRepository)
	if !configured {
		httptransport.WriteError(writer, request, errors.New("investigation workspace repository is not configured"))
		return identity.Principal{}, investigation.SearchAccess{}, nil, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, investigation.SearchAccess{}, nil, false
	}
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:investigate")
		return identity.Principal{}, investigation.SearchAccess{}, nil, false
	}
	if !principal.HasScope("investigation:read") || write && !principal.HasScope("investigation:write") {
		requiredScope := "investigation:read"
		if write {
			requiredScope = "investigation:write"
		}
		writeScopeDenial(writer, request, h.audit, principal, requiredScope)
		return identity.Principal{}, investigation.SearchAccess{}, nil, false
	}
	access := investigation.SearchAccess{Accounts: principal.HasScope("accounts:read"), Transfers: principal.HasScope("transfers:read"), Funding: principal.HasScope("funding:read"), Events: principal.HasScope("events:read"), Reconciliation: principal.HasScope("reconciliation:read"), Corrections: principal.HasScope("corrections:read")}
	if !access.Any() {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:read")
		return identity.Principal{}, investigation.SearchAccess{}, nil, false
	}
	limit, route, failClosed := h.rateLimit, "investigation:workspaces-read", false
	if write {
		limit, route, failClosed = h.workspaceWriteLimit, "investigation:workspaces-write", true
		if limit < 1 {
			limit = h.rateLimit
		}
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, route, limit, failClosed) {
		return identity.Principal{}, investigation.SearchAccess{}, nil, false
	}
	return principal, access, repository, true
}

func workspacePublicError(err error) error {
	switch {
	case errors.Is(err, investigation.ErrInvalidWorkspace):
		return httptransport.ErrBadRequest
	case errors.Is(err, investigation.ErrWorkspaceNotFound):
		return httptransport.ErrNotFound
	case errors.Is(err, investigation.ErrWorkspaceVersion):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "investigation_version_conflict", Message: "The investigation changed in another session. Read the current version before retrying."}
	case errors.Is(err, investigation.ErrWorkspaceState):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "investigation_state_conflict", Message: "The investigation is not in the required state."}
	case errors.Is(err, investigation.ErrWorkspaceLimit):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "investigation_limit_reached", Message: "Close an open investigation before creating another."}
	default:
		return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Investigation workspace evidence is unavailable."}
	}
}
