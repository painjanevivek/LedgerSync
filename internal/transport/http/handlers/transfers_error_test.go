package handlers

import (
	"errors"
	"net/http"
	"testing"

	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

func TestExhaustedSerializableConflictRequestsSafeSameKeyRetry(t *testing.T) {
	err := publicTransferError(errors.New("commit failed: SQLSTATE 40001"))
	var public *httptransport.PublicError
	if !errors.As(err, &public) {
		t.Fatalf("retryable transaction conflict was not mapped to a public availability result: %v", err)
	}
	if public.Status != http.StatusServiceUnavailable || public.Code != "transaction_conflict_retryable" {
		t.Fatalf("unexpected transaction conflict contract: %+v", public)
	}
}
