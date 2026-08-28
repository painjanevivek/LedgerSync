package handlers

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type correctionHandlerRepository struct {
	request      appcorrections.RequestCommand
	requestCalls int
	requestErr   error
}

func (r *correctionHandlerRepository) Request(_ context.Context, command appcorrections.RequestCommand, _ [sha256.Size]byte) (appcorrections.Submission, error) {
	r.request, r.requestCalls = command, r.requestCalls+1
	return appcorrections.Submission{Event: appcorrections.Event{CorrectionID: "correction-1", Status: "requested"}}, r.requestErr
}
func (*correctionHandlerRepository) Approve(context.Context, appcorrections.DecisionCommand) (appcorrections.Event, error) {
	return appcorrections.Event{}, nil
}
func (*correctionHandlerRepository) Reject(context.Context, appcorrections.DecisionCommand) (appcorrections.Event, error) {
	return appcorrections.Event{}, nil
}
func (*correctionHandlerRepository) Cancel(context.Context, appcorrections.CancelCommand) (appcorrections.Event, error) {
	return appcorrections.Event{}, nil
}
func (*correctionHandlerRepository) Post(context.Context, appcorrections.PostCommand) (appcorrections.Submission, error) {
	return appcorrections.Submission{}, nil
}
func (*correctionHandlerRepository) Get(context.Context, string, string, string) (appcorrections.Event, error) {
	return appcorrections.Event{}, nil
}
func (*correctionHandlerRepository) List(context.Context, string, string, appcorrections.Query) (appcorrections.Page, error) {
	return appcorrections.Page{}, nil
}

type correctionPrincipalProvider struct{ principal identity.Principal }

func (p correctionPrincipalProvider) Authenticate(context.Context, string) (identity.Principal, error) {
	return p.principal, nil
}

func TestCorrectionRequestUsesVerifiedStepUpAndWriteScope(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository := &correctionHandlerRepository{}
	service, err := appcorrections.NewService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	provider := correctionPrincipalProvider{principal: identity.Principal{
		SubjectID: "operator-1", TenantID: "tenant-1", AuthenticatedAt: now.Add(-time.Minute),
		Scopes: map[string]struct{}{"corrections:write": {}},
	}}
	handler := NewCorrectionHandler(service, provider)
	request := httptest.NewRequest(http.MethodPost, "/api/transfers/transfer-1/corrections", strings.NewReader(`{"reason_code":"operational_error","operator_note":"Exact reversal evidence reviewed."}`))
	request.SetPathValue("transferID", "transfer-1")
	request.Header.Set("Authorization", "Bearer verified")
	request.Header.Set("Idempotency-Key", "correction-request-0001")
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.Request)).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || repository.requestCalls != 1 || !repository.request.StepUpAuthenticatedAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("status=%d command=%#v calls=%d body=%s", response.Code, repository.request, repository.requestCalls, response.Body.String())
	}

	provider.principal.Scopes = map[string]struct{}{"corrections:read": {}}
	denied := NewCorrectionHandler(service, provider)
	deniedRequest := httptest.NewRequest(http.MethodPost, "/api/transfers/transfer-1/corrections", strings.NewReader(`{}`))
	deniedRequest.SetPathValue("transferID", "transfer-1")
	deniedResponse := httptest.NewRecorder()
	denied.Request(deniedResponse, deniedRequest)
	if deniedResponse.Code != http.StatusForbidden || repository.requestCalls != 1 {
		t.Fatalf("denied status=%d calls=%d", deniedResponse.Code, repository.requestCalls)
	}
}

func TestCorrectionStepUpFailureIsActionable(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/transfer-corrections/correction-1/approve", nil)
	httptransportError := publicCorrectionError(appcorrections.ErrStepUpRequired)
	writePublicCorrectionError(recorder, request, httptransportError)
	if recorder.Code != http.StatusPreconditionRequired || !strings.Contains(recorder.Body.String(), "step_up_required") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func writePublicCorrectionError(writer http.ResponseWriter, request *http.Request, err error) {
	// Keep this test at the real public serialization boundary.
	httptransport.WriteError(writer, request, err)
}
