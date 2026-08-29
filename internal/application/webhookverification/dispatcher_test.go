package webhookverification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
)

func TestDispatcherRequiresSignedEndpointProof(t *testing.T) {
	key := []byte("verification-test-key-that-is-long-enough-for-hmac")
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	challenge := "challenge-challenge-challenge-challenge"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-LedgerSync-Verification-ID") != "job-1" || r.Header.Get("X-LedgerSync-Signature") == "" {
			t.Error("missing signed verification request")
		}
		w.Header().Set(verificationProofHeader, "v1="+testHMAC(key, challenge))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resolver, err := webhookdelivery.NewStaticKeyResolver(map[string][]byte{"secret/verification": key})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher, err := NewDispatcher(server.Client(), resolver, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := dispatcher.Verify(context.Background(), Job{ID: "job-1", EndpointURL: server.URL, SigningKeyReference: "secret/verification", SigningKeyID: "key-1", Challenge: []byte(challenge), ExpiresAt: now.Add(time.Minute), AttemptNumber: 1})
	if err != nil || outcome.ResponseClass != "http_204" || outcome.Retryable {
		t.Fatalf("outcome=%#v err=%v", outcome, err)
	}
}

func TestDispatcherRejectsMissingProof(t *testing.T) {
	key := []byte("verification-test-key-that-is-long-enough-for-hmac")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	resolver, _ := webhookdelivery.NewStaticKeyResolver(map[string][]byte{"secret/verification": key})
	dispatcher, _ := NewDispatcher(server.Client(), resolver, time.Now)
	outcome, err := dispatcher.Verify(context.Background(), Job{ID: "job-1", EndpointURL: server.URL, SigningKeyReference: "secret/verification", SigningKeyID: "key-1", Challenge: []byte("challenge-challenge-challenge-challenge"), ExpiresAt: time.Now().Add(time.Minute), AttemptNumber: 1})
	if err == nil || outcome.ResponseClass != "invalid_proof" || outcome.Retryable {
		t.Fatalf("outcome=%#v err=%v", outcome, err)
	}
}

func testHMAC(key []byte, value string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
