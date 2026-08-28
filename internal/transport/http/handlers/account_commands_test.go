package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type accountCommandTestRepository struct {
	createCommand accounts.CreateAccountCommand
	updateCommand accounts.UpdateAccountMetadataCommand
	statusCommand accounts.ChangeAccountStatusCommand
	submission    accounts.CommandSubmission
	err           error
}

type accountCommandTestLimiter struct {
	routes []string
	allow  bool
}

func (l *accountCommandTestLimiter) Consume(_ context.Context, _, _, route string, _ int, _ time.Duration) (db.RateLimitDecision, error) {
	l.routes = append(l.routes, route)
	if l.allow {
		return db.RateLimitDecision{Allowed: true}, nil
	}
	return db.RateLimitDecision{RetryAfter: time.Second}, nil
}

func (r *accountCommandTestRepository) Create(_ context.Context, command accounts.CreateAccountCommand, _ [sha256.Size]byte) (accounts.CommandSubmission, error) {
	r.createCommand = command
	return r.submission, r.err
}

func (r *accountCommandTestRepository) UpdateMetadata(_ context.Context, command accounts.UpdateAccountMetadataCommand, _ [sha256.Size]byte) (accounts.CommandSubmission, error) {
	r.updateCommand = command
	return r.submission, r.err
}

func (r *accountCommandTestRepository) ChangeStatus(_ context.Context, command accounts.ChangeAccountStatusCommand, _ [sha256.Size]byte) (accounts.CommandSubmission, error) {
	r.statusCommand = command
	return r.submission, r.err
}

func accountCommandTestHandler(t *testing.T, repository *accountCommandTestRepository, scopes ...string) http.Handler {
	t.Helper()
	service, err := accounts.NewCommandService(repository, func() time.Time { return time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAccountCommandHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant", Scopes: scopes})
	router := http.NewServeMux()
	router.HandleFunc("POST /api/accounts", handler.Create)
	router.HandleFunc("PATCH /api/accounts/{accountID}", handler.Patch)
	return middleware.Correlation(router)
}

func accountCommandRequest(method, target, body, key string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	return request
}

func TestCreateAccountCommandReturnsTransportDTOAndReplayHeader(t *testing.T) {
	repository := &accountCommandTestRepository{submission: accounts.CommandSubmission{Replayed: true, Result: accounts.CommandResult{
		AccountID: "account-1", TenantID: "tenant", Currency: "INR", Status: "active", DisplayName: "भारत Operations",
		Reference: "ops-inr", Category: "operating", Version: "1", AvailableMinor: "0", LedgerMinor: "0",
		CreatedAt: "2026-08-25T10:00:00Z", UpdatedAt: "2026-08-25T10:00:00Z",
	}}}
	handler := accountCommandTestHandler(t, repository, "accounts:write")
	request := accountCommandRequest(http.MethodPost, "/api/accounts", `{"display_name":"भारत Operations","external_reference":"ops-inr","category":"operating","currency":"INR"}`, "account-create-key-0001")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Idempotent-Replay") != "true" || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["account_version"] != "1" || body["external_reference"] != "ops-inr" || body["available_minor"] != "0" {
		t.Fatalf("unexpected response DTO: %#v", body)
	}
	if _, exists := body["version"]; exists {
		t.Fatalf("application version field leaked into transport DTO: %#v", body)
	}
	if _, exists := body["reference"]; exists {
		t.Fatalf("application reference field leaked into transport DTO: %#v", body)
	}
	if repository.createCommand.TenantID != "tenant" || repository.createCommand.ActorSubjectID != "operator" || repository.createCommand.IdempotencyKey != "account-create-key-0001" || repository.createCommand.CorrelationID == "" {
		t.Fatalf("unexpected create command: %#v", repository.createCommand)
	}
}

func TestPatchAccountCommandRequiresExactlyOneMutationAndCanonicalVersion(t *testing.T) {
	for name, body := range map[string]string{
		"both mutation families": `{"expected_version":"1","display_name":"Ops","external_reference":"ops-inr","category":"operating","target_status":"frozen","reason":"Routine review"}`,
		"partial metadata":       `{"expected_version":"1","display_name":"Ops"}`,
		"metadata with reason":   `{"expected_version":"1","display_name":"Ops","external_reference":"ops-inr","category":"operating","reason":"Not a lifecycle command"}`,
		"missing mutation":       `{"expected_version":"1"}`,
		"missing reason":         `{"expected_version":"1","target_status":"frozen"}`,
		"numeric version":        `{"expected_version":1,"target_status":"frozen","reason":"Routine review"}`,
		"leading zero version":   `{"expected_version":"01","target_status":"frozen","reason":"Routine review"}`,
		"unknown field":          `{"expected_version":"1","target_status":"frozen","reason":"Routine review","owner":"attacker"}`,
		"trailing JSON":          `{"expected_version":"1","target_status":"frozen","reason":"Routine review"}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			repository := &accountCommandTestRepository{}
			handler := accountCommandTestHandler(t, repository, "accounts:write")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, accountCommandRequest(http.MethodPatch, "/api/accounts/account-1", body, "account-patch-key-0001"))
			if response.Code != http.StatusBadRequest || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
			if repository.updateCommand.AccountID != "" || repository.statusCommand.AccountID != "" {
				t.Fatal("invalid patch reached repository")
			}
		})
	}
}

func TestPatchAccountCommandRoutesMetadataAndLifecycleIntent(t *testing.T) {
	for name, body := range map[string]string{
		"metadata": `{"expected_version":"7","display_name":"Payroll","external_reference":"payroll-inr","category":"payroll"}`,
		"status":   `{"expected_version":"7","target_status":"frozen","reason":"  नियमित समीक्षा  "}`,
	} {
		t.Run(name, func(t *testing.T) {
			repository := &accountCommandTestRepository{submission: accounts.CommandSubmission{Result: accounts.CommandResult{AccountID: "account-1", Version: "8"}}}
			handler := accountCommandTestHandler(t, repository, "accounts:write")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, accountCommandRequest(http.MethodPatch, "/api/accounts/account-1", body, "account-patch-key-0002"))
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if name == "metadata" && (repository.updateCommand.AccountID != "account-1" || repository.updateCommand.ExpectedVersion != 7 || repository.updateCommand.Reference != "payroll-inr") {
				t.Fatalf("unexpected metadata command: %#v", repository.updateCommand)
			}
			if name == "status" && (repository.statusCommand.AccountID != "account-1" || repository.statusCommand.ExpectedVersion != 7 || repository.statusCommand.TargetStatus != accountdomain.StatusFrozen || repository.statusCommand.Reason != "नियमित समीक्षा") {
				t.Fatalf("unexpected lifecycle command: %#v", repository.statusCommand)
			}
		})
	}
}

func TestAccountCommandDeniesMissingScopeBeforeBodyOrRepository(t *testing.T) {
	repository := &accountCommandTestRepository{}
	handler := accountCommandTestHandler(t, repository, "accounts:read")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, accountCommandRequest(http.MethodPost, "/api/accounts", `{"external_reference":"sensitive-reference"}`, "account-create-key-0003"))
	if response.Code != http.StatusForbidden || strings.Contains(response.Body.String(), "sensitive-reference") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repository.createCommand.TenantID != "" {
		t.Fatal("scope-denied mutation reached repository")
	}
}

func TestAccountCommandRejectsMissingKeyAndOversizedBody(t *testing.T) {
	for name, testCase := range map[string]struct{ key, body string }{
		"missing key": {body: `{"display_name":"Ops","external_reference":"ops-inr","category":"operating","currency":"INR"}`},
		"oversized":   {key: "account-create-key-0004", body: `{"display_name":"` + strings.Repeat("a", maxAccountCommandBodyBytes) + `","external_reference":"ops-inr","category":"operating","currency":"INR"}`},
	} {
		t.Run(name, func(t *testing.T) {
			repository := &accountCommandTestRepository{}
			handler := accountCommandTestHandler(t, repository, "accounts:write")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, accountCommandRequest(http.MethodPost, "/api/accounts", testCase.body, testCase.key))
			if response.Code != http.StatusBadRequest || repository.createCommand.TenantID != "" {
				t.Fatalf("status=%d command=%#v body=%s", response.Code, repository.createCommand, response.Body.String())
			}
		})
	}
}

func TestAccountCommandRequiresJSONMediaTypeAndAllowsCharset(t *testing.T) {
	validBody := `{"display_name":"Ops","external_reference":"ops-inr","category":"operating","currency":"INR"}`
	for name, testCase := range map[string]struct {
		contentType string
		status      int
	}{
		"missing":      {status: http.StatusUnsupportedMediaType},
		"text":         {contentType: "text/plain", status: http.StatusUnsupportedMediaType},
		"malformed":    {contentType: "application/json; charset", status: http.StatusUnsupportedMediaType},
		"json":         {contentType: "application/json", status: http.StatusCreated},
		"json charset": {contentType: "application/json; charset=utf-8", status: http.StatusCreated},
	} {
		t.Run(name, func(t *testing.T) {
			repository := &accountCommandTestRepository{}
			handler := accountCommandTestHandler(t, repository, "accounts:write")
			request := accountCommandRequest(http.MethodPost, "/api/accounts", validBody, "account-media-key-0001")
			request.Header.Set("Content-Type", testCase.contentType)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
			if testCase.status == http.StatusUnsupportedMediaType && !strings.Contains(response.Body.String(), `"code":"unsupported_media_type"`) {
				t.Fatalf("unexpected media type error: %s", response.Body.String())
			}
		})
	}
}

func TestAccountCommandUsesPrincipalAndTenantCapacityLimits(t *testing.T) {
	repository := &accountCommandTestRepository{}
	service, err := accounts.NewCommandService(repository, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	limiter := &accountCommandTestLimiter{allow: true}
	commandHandler := NewAccountCommandHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant", Scopes: []string{"accounts:write"}}).
		WithRateLimiter(limiter, 60).WithCapacityLimit(limiter, 10)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/accounts", commandHandler.Create)
	response := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(response, accountCommandRequest(http.MethodPost, "/api/accounts", `{"display_name":"Ops","external_reference":"ops-inr","category":"operating","currency":"INR"}`, "account-limit-key-0001"))
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	want := []string{"accounts:create:capacity:second", "accounts:create:capacity:minute", "accounts:create"}
	if len(limiter.routes) != len(want) {
		t.Fatalf("rate routes=%v", limiter.routes)
	}
	for index := range want {
		if limiter.routes[index] != want[index] {
			t.Fatalf("rate routes=%v, want %v", limiter.routes, want)
		}
	}
}

func TestAccountCommandRateDenialAndUnknownErrorFailClosed(t *testing.T) {
	validBody := `{"display_name":"Ops","external_reference":"ops-inr","category":"operating","currency":"INR"}`
	t.Run("rate denied", func(t *testing.T) {
		repository := &accountCommandTestRepository{}
		service, _ := accounts.NewCommandService(repository, time.Now)
		limiter := &accountCommandTestLimiter{}
		commandHandler := NewAccountCommandHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant", Scopes: []string{"accounts:write"}}).
			WithRateLimiter(limiter, 60).WithCapacityLimit(limiter, 10)
		router := http.NewServeMux()
		router.HandleFunc("POST /api/accounts", commandHandler.Create)
		response := httptest.NewRecorder()
		middleware.Correlation(router).ServeHTTP(response, accountCommandRequest(http.MethodPost, "/api/accounts", validBody, "account-limit-key-0002"))
		if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "1" || repository.createCommand.TenantID != "" {
			t.Fatalf("status=%d headers=%v command=%#v body=%s", response.Code, response.Header(), repository.createCommand, response.Body.String())
		}
	})
	t.Run("unknown dependency error", func(t *testing.T) {
		repository := &accountCommandTestRepository{err: errors.New("postgres://secret-database-url")}
		handler := accountCommandTestHandler(t, repository, "accounts:write")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, accountCommandRequest(http.MethodPost, "/api/accounts", validBody, "account-error-key-0001"))
		if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "secret-database-url") || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
		}
	})
}

func TestAccountCommandBFFWorkloadRequiresActorAssertion(t *testing.T) {
	repository := &accountCommandTestRepository{}
	service, err := accounts.NewCommandService(repository, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	provider := identity.DevelopmentProvider{SubjectID: "bff", TenantID: "system", Scopes: []string{"accounts:write", identity.BFFActorScope}}
	authenticator, err := identity.NewRequestAuthenticator(provider, "account-handler-actor-assertion-secret-long-enough")
	if err != nil {
		t.Fatal(err)
	}
	commandHandler := NewAccountCommandHandler(service, provider).WithRequestAuthenticator(authenticator)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/accounts", commandHandler.Create)
	request := accountCommandRequest(http.MethodPost, "/api/accounts", `{"display_name":"Ops","external_reference":"ops-inr","category":"operating","currency":"INR"}`, "account-assert-key-0001")
	response := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || repository.createCommand.TenantID != "" {
		t.Fatalf("status=%d command=%#v body=%s", response.Code, repository.createCommand, response.Body.String())
	}
}

func TestPublicAccountCommandErrorsAreStableAndNonDisclosing(t *testing.T) {
	for name, testCase := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"not found":             {accounts.ErrAccountNotFound, http.StatusNotFound, "not_found"},
		"reference":             {accounts.ErrAccountConflict, http.StatusConflict, "external_reference_conflict"},
		"version":               {accounts.ErrVersionConflict, http.StatusConflict, "account_version_conflict"},
		"transition":            {accounts.ErrInvalidTransition, http.StatusConflict, "invalid_account_transition"},
		"terminal":              {accounts.ErrTerminalStatus, http.StatusConflict, "invalid_account_transition"},
		"non-zero":              {accounts.ErrNonZeroClose, http.StatusUnprocessableEntity, "account_not_zero"},
		"financial unavailable": {accounts.ErrFinancialUnavailable, http.StatusServiceUnavailable, "temporary_unavailable"},
		"dependency":            {accounts.ErrCommandUnavailable, http.StatusServiceUnavailable, "temporary_unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			mapped := publicAccountCommandError(testCase.err)
			var public *httptransport.PublicError
			if !errors.As(mapped, &public) || public.Status != testCase.status || public.Code != testCase.code {
				t.Fatalf("mapped=%#v", mapped)
			}
		})
	}
}
