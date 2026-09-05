package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/bootstrap"
)

type stubCronRunner struct {
	results []bootstrap.WorkerCounts
	calls   int
}

func (s *stubCronRunner) RunOnce(context.Context) (bootstrap.WorkerCounts, error) {
	result := s.results[min(s.calls, len(s.results)-1)]
	s.calls++
	return result, nil
}

func TestCronDrainRejectsMissingCredential(t *testing.T) {
	runner := &stubCronRunner{results: []bootstrap.WorkerCounts{{}}}
	handler := NewCronDrainHandler("expected-secret", runner, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/internal/cron/drain", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || runner.calls != 0 {
		t.Fatalf("expected an unauthenticated rejection, status=%d calls=%d", response.Code, runner.calls)
	}
}

func TestCronDrainProcessesUntilEmptyBatch(t *testing.T) {
	runner := &stubCronRunner{results: []bootstrap.WorkerCounts{{Outbox: 2}, {WebhookDeliveries: 1}, {}}}
	handler := NewCronDrainHandler("expected-secret", runner, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/internal/cron/drain", nil)
	request.Header.Set("Authorization", "Bearer expected-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || runner.calls != 3 {
		t.Fatalf("expected three successful batches, status=%d calls=%d body=%s", response.Code, runner.calls, response.Body.String())
	}
}
