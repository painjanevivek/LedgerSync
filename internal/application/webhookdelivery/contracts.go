// Package webhookdelivery coordinates leased delivery through transport-neutral ports.
package webhookdelivery

import (
	"context"
	"encoding/json"
	"time"
)

type KeyResolver interface {
	Resolve(context.Context, string) ([]byte, error)
}

type KeyCacheObserver interface {
	ObserveWebhookKeyCacheResolve(context.Context, string, time.Duration, time.Duration)
	ObserveWebhookKeyCacheEntries(context.Context, int)
	ObserveWebhookKeyCacheEviction(context.Context, string)
}

type Delivery struct {
	ID, EventID, EventType, EndpointURL, SigningKeyReference, SigningKeyID string
	Payload                                                                json.RawMessage
	OccurredAt                                                             time.Time
	DiagnosticTraceParent                                                  string
}

type Outcome struct {
	ResponseClass string
	Retryable     bool
}
