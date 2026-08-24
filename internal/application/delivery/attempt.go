// Package delivery owns downstream notification evidence. Financial posting,
// outbox publication, cache projection, and recipient delivery are deliberately
// separate state machines.
package delivery

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRetrying  Status = "retrying"
	StatusDelivered Status = "delivered"
	StatusDead      Status = "dead"
)

type Attempt struct {
	ID, TenantID, TransferID, OutboxEventID string
	Kind, EndpointReference                 string
	AttemptNumber                           int
	Status                                  Status
	ResponseClass, SanitizedErrorCode       string
	DueAt, StartedAt, CompletedAt           time.Time
}

type Store interface {
	Record(context.Context, Attempt) error
}

func Validate(attempt Attempt) error {
	if strings.TrimSpace(attempt.ID) == "" || strings.TrimSpace(attempt.TenantID) == "" || strings.TrimSpace(attempt.TransferID) == "" || strings.TrimSpace(attempt.EndpointReference) == "" || attempt.AttemptNumber < 1 || attempt.DueAt.IsZero() {
		return errors.New("delivery attempt identity, endpoint, number, and due time are required")
	}
	if attempt.Kind != "webhook" && attempt.Kind != "notification" {
		return errors.New("unsupported delivery kind")
	}
	switch attempt.Status {
	case StatusPending, StatusRetrying:
		if !attempt.CompletedAt.IsZero() {
			return errors.New("incomplete delivery attempt cannot have completion time")
		}
	case StatusDelivered, StatusDead:
		if attempt.CompletedAt.IsZero() {
			return errors.New("final delivery attempt requires completion time")
		}
	default:
		return errors.New("unsupported delivery status")
	}
	return nil
}
