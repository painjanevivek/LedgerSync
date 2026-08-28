package operations

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

var safeIdentifier = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var ErrInvalidEventEvidence = errors.New("invalid event evidence request")

type EventFilter struct {
	Cursor, EventType, State, RelatedID, CorrelationID string
	From, To                                           time.Time
	Limit                                              int
}

type EventEvidence struct {
	EventID          string     `json:"event_id"`
	EventType        string     `json:"event_type"`
	State            string     `json:"state"`
	AggregateType    string     `json:"aggregate_type"`
	AggregateID      string     `json:"aggregate_id"`
	AggregateVersion string     `json:"aggregate_version"`
	AttemptCount     string     `json:"attempt_count"`
	TransferID       string     `json:"transfer_id,omitempty"`
	AccountID        string     `json:"account_id,omitempty"`
	CorrelationID    string     `json:"correlation_id,omitempty"`
	LastErrorCode    string     `json:"last_error_code,omitempty"`
	OccurredAt       time.Time  `json:"occurred_at"`
	AvailableAt      time.Time  `json:"available_at"`
	ClaimedUntil     *time.Time `json:"claimed_until,omitempty"`
	PublishedAt      *time.Time `json:"published_at,omitempty"`
	DeadAt           *time.Time `json:"dead_at,omitempty"`
}

type DeliveryEvidence struct {
	AttemptID     string     `json:"attempt_id"`
	Kind          string     `json:"kind"`
	State         string     `json:"state"`
	AttemptNumber string     `json:"attempt_number"`
	ResponseClass string     `json:"response_class,omitempty"`
	ErrorCode     string     `json:"error_code,omitempty"`
	DueAt         time.Time  `json:"due_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

type EventTimelineItem struct {
	Kind       string    `json:"kind"`
	OccurredAt time.Time `json:"occurred_at"`
}

type EventDetail struct {
	EventEvidence
	DeliveryAttempts          []DeliveryEvidence  `json:"delivery_attempts"`
	DeliveryAttemptsTruncated bool                `json:"delivery_attempts_truncated"`
	Timeline                  []EventTimelineItem `json:"timeline"`
}

type EventRepository interface {
	ListEvents(context.Context, string, string, EventFilter) ([]EventEvidence, string, error)
	GetEvent(context.Context, string, string, string) (EventDetail, error)
}

type EventService struct{ repository EventRepository }

func NewEventService(repository EventRepository) (*EventService, error) {
	if repository == nil {
		return nil, errors.New("event evidence repository is required")
	}
	return &EventService{repository: repository}, nil
}

func (s *EventService) List(ctx context.Context, tenantID, actorID string, filter EventFilter) ([]EventEvidence, string, error) {
	tenantID, actorID = strings.TrimSpace(tenantID), strings.TrimSpace(actorID)
	filter.EventType, filter.State = strings.TrimSpace(filter.EventType), strings.TrimSpace(filter.State)
	filter.RelatedID, filter.CorrelationID, filter.Cursor = strings.TrimSpace(filter.RelatedID), strings.TrimSpace(filter.CorrelationID), strings.TrimSpace(filter.Cursor)
	if tenantID == "" || actorID == "" || filter.Limit < 1 || filter.Limit > 100 || filter.EventType != "" && !safeIdentifier.MatchString(filter.EventType) || filter.State != "" && filter.State != "pending" && filter.State != "retrying" && filter.State != "published" && filter.State != "dead" || filter.RelatedID != "" && !canonicalUUID.MatchString(strings.ToLower(filter.RelatedID)) || filter.CorrelationID != "" && !canonicalUUID.MatchString(strings.ToLower(filter.CorrelationID)) || !filter.From.IsZero() && !filter.To.IsZero() && filter.From.After(filter.To) {
		return nil, "", ErrInvalidEventEvidence
	}
	filter.RelatedID, filter.CorrelationID = strings.ToLower(filter.RelatedID), strings.ToLower(filter.CorrelationID)
	return s.repository.ListEvents(ctx, tenantID, actorID, filter)
}

func (s *EventService) Get(ctx context.Context, tenantID, actorID, eventID string) (EventDetail, error) {
	tenantID, actorID, eventID = strings.TrimSpace(tenantID), strings.TrimSpace(actorID), strings.TrimSpace(eventID)
	if tenantID == "" || actorID == "" || !canonicalUUID.MatchString(strings.ToLower(eventID)) {
		return EventDetail{}, ErrInvalidEventEvidence
	}
	return s.repository.GetEvent(ctx, tenantID, actorID, eventID)
}
