package webhookdelivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type staticResolver struct{ key []byte }

func (r staticResolver) Resolve(context.Context, string) ([]byte, error) { return r.key, nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestDispatcherSignsBoundedWebhookRequest(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.Header.Get("X-LedgerSync-Event-Type") != "transfer.posted" || request.Header.Get("X-LedgerSync-Key-ID") != "key-001" {
			t.Fatalf("headers=%v method=%s", request.Header, request.Method)
		}
		body, _ := io.ReadAll(request.Body)
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(now.Format(time.RFC3339Nano) + ".delivery-1." + string(body) + ".key-001"))
		if got, want := request.Header.Get("X-LedgerSync-Signature"), "v1="+hex.EncodeToString(mac.Sum(nil)); got != want {
			t.Fatalf("signature=%q want=%q", got, want)
		}
		return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	dispatcher, err := NewDispatcher(client, staticResolver{key: key}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := dispatcher.Dispatch(context.Background(), Delivery{ID: "delivery-1", EventID: "event-1", EventType: "transfer.posted", EndpointURL: "https://partner.example.test/hooks", SigningKeyReference: "kms/webhooks/primary", SigningKeyID: "key-001", Payload: []byte(`{"transfer_id":"transfer-1"}`), OccurredAt: now})
	if err != nil || outcome.ResponseClass != "http_202" || outcome.Retryable {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}

func TestValidateDeliveryRejectsPrivateOrInsecureEndpoint(t *testing.T) {
	base := Delivery{ID: "delivery-1", EventID: "event-1", EventType: "transfer.posted", SigningKeyReference: "kms/webhooks/primary", SigningKeyID: "key-001", Payload: []byte(`{}`)}
	for _, endpoint := range []string{"http://partner.example.test/hooks", "https://127.0.0.1/hooks", "https://partner.example.test:8443/hooks", "https://user:pass@partner.example.test/hooks"} {
		base.EndpointURL = endpoint
		if err := validateDelivery(base); err == nil {
			t.Fatalf("endpoint %q was accepted", endpoint)
		}
	}
}
