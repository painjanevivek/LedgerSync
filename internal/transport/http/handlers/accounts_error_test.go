package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

func TestAccountDependencyLossIsTruthfulTemporaryUnavailability(t *testing.T) {
	err := publicAccountError(errors.New("query failed: " + accounts.ErrAccountDirectoryUnavailable.Error()))
	if _, ok := err.(*httptransport.PublicError); ok {
		t.Fatal("plain text must not be classified as the dependency sentinel")
	}

	err = publicAccountError(accounts.ErrAccountDirectoryUnavailable)
	var public *httptransport.PublicError
	if !errors.As(err, &public) {
		t.Fatalf("error=%v, want public error", err)
	}
	if public.Status != http.StatusServiceUnavailable || public.Code != "account_directory_unavailable" {
		t.Fatalf("public=%#v, want sanitized 503 account_directory_unavailable", public)
	}
}
