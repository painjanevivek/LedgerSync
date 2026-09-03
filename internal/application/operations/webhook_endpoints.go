package operations

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidWebhookEvidence = errors.New("invalid webhook evidence request")

type WebhookEndpointFilter struct {
	Cursor, Status, EventType string
	Limit                     int
}

type WebhookEndpointEvidence struct {
	EndpointID          string     `json:"endpoint_id"`
	Label               string     `json:"label"`
	Origin              string     `json:"origin"`
	Status              string     `json:"status"`
	SubscribedEvents    []string   `json:"subscribed_events"`
	RecentDeliveryState string     `json:"recent_delivery_state"`
	RecentAttemptCount  string     `json:"recent_attempt_count"`
	RecentDeadCount     string     `json:"recent_dead_count"`
	VerifiedAt          *time.Time `json:"verified_at,omitempty"`
	DisabledAt          *time.Time `json:"disabled_at,omitempty"`
	LatestDeliveryAt    *time.Time `json:"latest_delivery_at,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at"`
	EndpointURL         string     `json:"-"`
}

type WebhookDeliveryEvidence struct {
	AttemptID     string     `json:"attempt_id"`
	EventID       string     `json:"event_id,omitempty"`
	TransferID    string     `json:"transfer_id"`
	State         string     `json:"state"`
	AttemptNumber string     `json:"attempt_number"`
	ResponseClass string     `json:"response_class,omitempty"`
	ErrorCode     string     `json:"error_code,omitempty"`
	DueAt         time.Time  `json:"due_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

type WebhookEndpointPage struct {
	Items      []WebhookEndpointEvidence `json:"items"`
	NextCursor string                    `json:"next_cursor"`
}

type WebhookEndpointDetail struct {
	WebhookEndpointEvidence
	DeliveryAttempts          []WebhookDeliveryEvidence `json:"delivery_attempts"`
	DeliveryAttemptsTruncated bool                      `json:"delivery_attempts_truncated"`
}

type WebhookEndpointRepository interface {
	ListWebhookEndpoints(context.Context, string, string, WebhookEndpointFilter) (WebhookEndpointPage, error)
	GetWebhookEndpoint(context.Context, string, string, string) (WebhookEndpointDetail, error)
}

type WebhookEndpointService struct{ repository WebhookEndpointRepository }

func NewWebhookEndpointService(repository WebhookEndpointRepository) (*WebhookEndpointService, error) {
	if repository == nil {
		return nil, errors.New("webhook endpoint evidence repository is required")
	}
	return &WebhookEndpointService{repository: repository}, nil
}

func (s *WebhookEndpointService) List(ctx context.Context, tenantID, actorID string, filter WebhookEndpointFilter) (WebhookEndpointPage, error) {
	tenantID, actorID = strings.TrimSpace(tenantID), strings.TrimSpace(actorID)
	filter.Cursor, filter.Status, filter.EventType = strings.TrimSpace(filter.Cursor), strings.ToLower(strings.TrimSpace(filter.Status)), strings.TrimSpace(filter.EventType)
	if tenantID == "" || actorID == "" || filter.Limit < 1 || filter.Limit > 100 || len(filter.Cursor) > 768 || filter.Status != "" && filter.Status != "pending_verification" && filter.Status != "active" && filter.Status != "disabled" || filter.EventType != "" && !safeIdentifier.MatchString(filter.EventType) {
		return WebhookEndpointPage{}, ErrInvalidWebhookEvidence
	}
	page, err := s.repository.ListWebhookEndpoints(ctx, tenantID, actorID, filter)
	if err != nil {
		return WebhookEndpointPage{}, err
	}
	for index := range page.Items {
		sanitizeWebhookEndpoint(&page.Items[index])
	}
	return page, nil
}

func (s *WebhookEndpointService) Get(ctx context.Context, tenantID, actorID, endpointID string) (WebhookEndpointDetail, error) {
	tenantID, actorID, endpointID = strings.TrimSpace(tenantID), strings.TrimSpace(actorID), strings.ToLower(strings.TrimSpace(endpointID))
	if tenantID == "" || actorID == "" || !canonicalUUID.MatchString(endpointID) {
		return WebhookEndpointDetail{}, ErrInvalidWebhookEvidence
	}
	detail, err := s.repository.GetWebhookEndpoint(ctx, tenantID, actorID, endpointID)
	if err != nil {
		return WebhookEndpointDetail{}, err
	}
	sanitizeWebhookEndpoint(&detail.WebhookEndpointEvidence)
	return detail, nil
}

func sanitizeWebhookEndpoint(item *WebhookEndpointEvidence) {
	item.Label = safeWebhookLabel(item.Label)
	item.Origin = safeWebhookOrigin(item.EndpointURL)
	item.EndpointURL = ""
	if item.Status != "pending_verification" && item.Status != "active" && item.Status != "disabled" {
		item.Status = "unknown"
	}
	if item.RecentDeliveryState != "none" && item.RecentDeliveryState != "pending" && item.RecentDeliveryState != "retrying" && item.RecentDeliveryState != "delivered" && item.RecentDeliveryState != "dead" {
		item.RecentDeliveryState = "unknown"
	}
	for index, eventType := range item.SubscribedEvents {
		if !safeIdentifier.MatchString(eventType) {
			item.SubscribedEvents[index] = "redacted_event"
		}
	}
	item.UpdatedAt = item.UpdatedAt.UTC()
}

func safeWebhookLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || len([]rune(value)) > 100 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "Webhook endpoint"
	}
	return value
}

func safeWebhookOrigin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "unavailable"
	}
	origin := strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
	if len(origin) > 255 {
		return "unavailable"
	}
	return origin
}
