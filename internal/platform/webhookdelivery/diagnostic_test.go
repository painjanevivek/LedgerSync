package webhookdelivery

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"go.opentelemetry.io/otel/trace"
)

func TestDispatchPropagatesOnlyOpaqueDiagnosticContext(t *testing.T) {
	parent := "00-11111111111111111111111111111111-2222222222222222-01"
	ctx := trace.ContextWithSpanContext(context.Background(), observability.TraceContextFromHeader(parent))
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("traceparent") != parent || r.Header.Get("baggage") != "" || r.Header.Get("tracestate") != "" || r.Header.Get("X-LedgerSync-Signature") == "" {
			t.Fatal("diagnostic propagation lost or expanded the signed contract")
		}
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	dispatcher, err := NewDispatcher(client, staticResolver{key: []byte(strings.Repeat("k", 32))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = dispatcher.Dispatch(ctx, Delivery{ID: "delivery-1", EventID: "event-1", EventType: "transfer.posted", EndpointURL: "https://partner.example.test/hooks", SigningKeyReference: "synthetic-key", SigningKeyID: "key-001", Payload: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
}
