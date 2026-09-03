// Package recovery defines reviewed dead-work recovery without changing the
// original financial transfer or immutable event identity.
package recovery

import "time"

type DeadOutbox struct {
	EventID, TenantID, TransferID, AccountID, EventType, LastErrorCode string
	AttemptCount                                                       int
	OccurredAt, DeadAt                                                 time.Time
}

type Approval struct{ TenantID, EventID, ActorSubjectID, ReasonCode, CorrelationID string }
type Replay struct{ TenantID, EventID, ActorSubjectID, CorrelationID string }

type DeadDelivery struct {
	AttemptID, TenantID, TransferID, OutboxEventID, Kind, EndpointReference, ErrorCode string
	AttemptNumber                                                                      int
	CompletedAt                                                                        time.Time
}

type DeliveryApproval struct {
	TenantID, AttemptID, ActorSubjectID, ReasonCode, CorrelationID, IdempotencyKey string
}

type DeliveryApprovalResult struct {
	ApprovalID string
	Replayed   bool
}

type DeliveryReplay struct {
	TenantID, AttemptID, ApprovalID, ActorSubjectID, CorrelationID, IdempotencyKey string
}

type DeliveryReplayResult struct {
	DeliveryJobID string
	Replayed      bool
}
