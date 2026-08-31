package developerplatform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
)

const (
	RegisterWebhookOperation = "webhook_register"
	VerifyWebhookOperation   = "webhook_verify"
	RotateWebhookOperation   = "webhook_signature_rotate"
	DisableWebhookOperation  = "webhook_disable"
)

var AllowedWebhookEvents = []string{
	"account.created", "account.status_changed", "correction.posted", "funding.posted",
	"reconciliation.completed", "transfer.posted", "transfer.rejected",
}

var webhookSigningKeyID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,99}$`)

type Webhook struct {
	ID                  string     `json:"webhook_id"`
	DisplayName         string     `json:"display_name"`
	EndpointURL         string     `json:"endpoint_url"`
	SubscribedEvents    []string   `json:"subscribed_events"`
	SigningKeyReference string     `json:"signing_key_reference"`
	SigningKeyID        string     `json:"signing_key_id"`
	Status              string     `json:"status"`
	Version             string     `json:"version"`
	ChallengeExpiresAt  *time.Time `json:"challenge_expires_at,omitempty"`
	VerifiedAt          *time.Time `json:"verified_at,omitempty"`
	DisabledAt          *time.Time `json:"disabled_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type WebhookPage struct {
	Items      []Webhook `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

type Delivery struct {
	AttemptID     string     `json:"attempt_id"`
	TransferID    string     `json:"transfer_id"`
	OutboxEventID string     `json:"outbox_event_id,omitempty"`
	AttemptNumber int        `json:"attempt_number"`
	Status        string     `json:"status"`
	ResponseClass string     `json:"response_class,omitempty"`
	ErrorCode     string     `json:"error_code,omitempty"`
	DueAt         time.Time  `json:"due_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type DeliveryPage struct {
	Items      []Delivery `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}
type WebhookQuery struct {
	Status, Cursor string
	Limit          int
}
type DeliveryQuery struct {
	Status, Cursor string
	Limit          int
}

type RegisterWebhookCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey     string
	DisplayName, EndpointURL, SigningKeyReference, SigningKeyID string
	SubscribedEvents                                            []string
	VerificationChallenge                                       string `json:"-"`
	ChallengeDigest                                             [sha256.Size]byte
	ChallengeExpiresAt                                          time.Time
}
type VerifyWebhookCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey, WebhookID, Challenge string
	ExpectedVersion                                                               int64
}
type RotateWebhookCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey, WebhookID, SigningKeyReference, SigningKeyID string
	ExpectedVersion                                                                                       int64
}
type DisableWebhookCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey, WebhookID, Reason string
	ExpectedVersion                                                            int64
}
type WebhookSubmission struct {
	Webhook  Webhook
	Replayed bool `json:"-"`
}

type WebhookRepository interface {
	RegisterWebhook(context.Context, RegisterWebhookCommand, [sha256.Size]byte) (WebhookSubmission, error)
	VerifyWebhook(context.Context, VerifyWebhookCommand, [sha256.Size]byte, [sha256.Size]byte) (WebhookSubmission, error)
	RotateWebhook(context.Context, RotateWebhookCommand, [sha256.Size]byte) (WebhookSubmission, error)
	DisableWebhook(context.Context, DisableWebhookCommand, [sha256.Size]byte) (WebhookSubmission, error)
	GetWebhook(context.Context, string, string) (Webhook, error)
	ListWebhooks(context.Context, string, WebhookQuery) (WebhookPage, error)
	ListWebhookDeliveries(context.Context, string, string, DeliveryQuery) (DeliveryPage, error)
}

type DeliveryReplayRepository interface {
	Inspect(context.Context, string, string) (recovery.DeadDelivery, error)
	Approve(context.Context, recovery.DeliveryApproval) error
	Replay(context.Context, recovery.DeliveryReplay) (string, error)
}

type WebhookService struct {
	repository  WebhookRepository
	replay      DeliveryReplayRepository
	environment string
	clock       func() time.Time
	random      io.Reader
}

func NewWebhookService(repository WebhookRepository, replay DeliveryReplayRepository, environment string, clock func() time.Time, random io.Reader) (*WebhookService, error) {
	if repository == nil {
		return nil, errors.New("webhook repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	if random == nil {
		random = rand.Reader
	}
	return &WebhookService{repository: repository, replay: replay, environment: strings.ToLower(strings.TrimSpace(environment)), clock: clock, random: random}, nil
}

func (s *WebhookService) Register(ctx context.Context, command RegisterWebhookCommand) (WebhookSubmission, error) {
	normalizeWebhookRegister(&command)
	if !validEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey) || len(command.DisplayName) < 1 || len(command.DisplayName) > 100 || !validWebhookURL(command.EndpointURL, s.environment) || !validWebhookEvents(command.SubscribedEvents) || !credentialReference.MatchString(command.SigningKeyReference) || !safeKeyID(command.SigningKeyID) {
		return WebhookSubmission{}, ErrInvalidCommand
	}
	challengeBytes := make([]byte, 32)
	if _, err := io.ReadFull(s.random, challengeBytes); err != nil {
		return WebhookSubmission{}, err
	}
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes)
	command.VerificationChallenge = challenge
	command.ChallengeDigest = sha256.Sum256([]byte(challenge))
	command.ChallengeExpiresAt = s.clock().UTC().Add(10 * time.Minute)
	submission, err := s.repository.RegisterWebhook(ctx, command, fingerprint(RegisterWebhookOperation, command.DisplayName, command.EndpointURL, command.SubscribedEvents, command.SigningKeyReference, command.SigningKeyID))
	return submission, err
}

func (s *WebhookService) Verify(ctx context.Context, command VerifyWebhookCommand) (WebhookSubmission, error) {
	normalizeWebhookEnvelope(&command.TenantID, &command.ActorSubjectID, &command.CorrelationID, &command.IdempotencyKey, &command.WebhookID)
	command.Challenge = strings.TrimSpace(command.Challenge)
	if !validEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey) || !canonicalUUID.MatchString(command.WebhookID) || command.ExpectedVersion < 1 || len(command.Challenge) < 32 || len(command.Challenge) > 255 {
		return WebhookSubmission{}, ErrInvalidCommand
	}
	digest := sha256.Sum256([]byte(command.Challenge))
	return s.repository.VerifyWebhook(ctx, command, fingerprint(VerifyWebhookOperation, command.WebhookID, command.ExpectedVersion, digest), digest)
}

func (s *WebhookService) Rotate(ctx context.Context, command RotateWebhookCommand) (WebhookSubmission, error) {
	normalizeWebhookEnvelope(&command.TenantID, &command.ActorSubjectID, &command.CorrelationID, &command.IdempotencyKey, &command.WebhookID)
	command.SigningKeyReference, command.SigningKeyID = strings.TrimSpace(command.SigningKeyReference), strings.TrimSpace(command.SigningKeyID)
	if !validEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey) || !canonicalUUID.MatchString(command.WebhookID) || command.ExpectedVersion < 1 || !credentialReference.MatchString(command.SigningKeyReference) || !safeKeyID(command.SigningKeyID) {
		return WebhookSubmission{}, ErrInvalidCommand
	}
	return s.repository.RotateWebhook(ctx, command, fingerprint(RotateWebhookOperation, command.WebhookID, command.ExpectedVersion, command.SigningKeyReference, command.SigningKeyID))
}

func (s *WebhookService) Disable(ctx context.Context, command DisableWebhookCommand) (WebhookSubmission, error) {
	normalizeWebhookEnvelope(&command.TenantID, &command.ActorSubjectID, &command.CorrelationID, &command.IdempotencyKey, &command.WebhookID)
	command.Reason = strings.TrimSpace(command.Reason)
	if !validEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey) || !canonicalUUID.MatchString(command.WebhookID) || command.ExpectedVersion < 1 || len(command.Reason) < 3 || len(command.Reason) > 500 {
		return WebhookSubmission{}, ErrInvalidCommand
	}
	return s.repository.DisableWebhook(ctx, command, fingerprint(DisableWebhookOperation, command.WebhookID, command.ExpectedVersion, command.Reason))
}

func (s *WebhookService) Get(ctx context.Context, tenantID, webhookID string) (Webhook, error) {
	tenantID, webhookID = strings.TrimSpace(tenantID), strings.ToLower(strings.TrimSpace(webhookID))
	if tenantID == "" || !canonicalUUID.MatchString(webhookID) {
		return Webhook{}, ErrInvalidCommand
	}
	return s.repository.GetWebhook(ctx, tenantID, webhookID)
}

func (s *WebhookService) List(ctx context.Context, tenantID string, query WebhookQuery) (WebhookPage, error) {
	tenantID, query.Status, query.Cursor = strings.TrimSpace(tenantID), strings.ToLower(strings.TrimSpace(query.Status)), strings.TrimSpace(query.Cursor)
	if query.Limit == 0 {
		query.Limit = 50
	}
	if tenantID == "" || query.Limit < 1 || query.Limit > 100 || query.Status != "" && !slices.Contains([]string{"pending_verification", "active", "disabled"}, query.Status) || len(query.Cursor) > 512 {
		return WebhookPage{}, ErrInvalidCommand
	}
	return s.repository.ListWebhooks(ctx, tenantID, query)
}

func (s *WebhookService) Deliveries(ctx context.Context, tenantID, webhookID string, query DeliveryQuery) (DeliveryPage, error) {
	tenantID, webhookID, query.Status, query.Cursor = strings.TrimSpace(tenantID), strings.ToLower(strings.TrimSpace(webhookID)), strings.ToLower(strings.TrimSpace(query.Status)), strings.TrimSpace(query.Cursor)
	if query.Limit == 0 {
		query.Limit = 50
	}
	if tenantID == "" || !canonicalUUID.MatchString(webhookID) || query.Limit < 1 || query.Limit > 100 || query.Status != "" && !slices.Contains([]string{"pending", "retrying", "delivered", "dead"}, query.Status) || len(query.Cursor) > 512 {
		return DeliveryPage{}, ErrInvalidCommand
	}
	return s.repository.ListWebhookDeliveries(ctx, tenantID, webhookID, query)
}

func (s *WebhookService) ApproveReplay(ctx context.Context, tenantID, webhookID, attemptID, actorID, reason, correlationID string) error {
	if s.replay == nil {
		return errors.New("delivery replay repository is required")
	}
	item, err := s.inspectDelivery(ctx, tenantID, webhookID, attemptID)
	if err != nil {
		return err
	}
	return s.replay.Approve(ctx, recovery.DeliveryApproval{TenantID: tenantID, AttemptID: item.AttemptID, ActorSubjectID: actorID, ReasonCode: strings.TrimSpace(reason), CorrelationID: correlationID})
}

func (s *WebhookService) ReplayDelivery(ctx context.Context, tenantID, webhookID, attemptID, actorID, correlationID string) (string, error) {
	if s.replay == nil {
		return "", errors.New("delivery replay repository is required")
	}
	item, err := s.inspectDelivery(ctx, tenantID, webhookID, attemptID)
	if err != nil {
		return "", err
	}
	return s.replay.Replay(ctx, recovery.DeliveryReplay{TenantID: tenantID, AttemptID: item.AttemptID, ActorSubjectID: actorID, CorrelationID: correlationID})
}

func (s *WebhookService) inspectDelivery(ctx context.Context, tenantID, webhookID, attemptID string) (recovery.DeadDelivery, error) {
	tenantID, webhookID, attemptID = strings.TrimSpace(tenantID), strings.ToLower(strings.TrimSpace(webhookID)), strings.ToLower(strings.TrimSpace(attemptID))
	if tenantID == "" || !canonicalUUID.MatchString(webhookID) || !canonicalUUID.MatchString(attemptID) {
		return recovery.DeadDelivery{}, ErrInvalidCommand
	}
	item, err := s.replay.Inspect(ctx, tenantID, attemptID)
	if err != nil {
		return item, err
	}
	if item.Kind != "webhook" || item.EndpointReference != webhookID {
		return recovery.DeadDelivery{}, ErrNotFound
	}
	return item, nil
}

func normalizeWebhookRegister(c *RegisterWebhookCommand) {
	c.TenantID, c.ActorSubjectID, c.CorrelationID, c.IdempotencyKey = strings.TrimSpace(c.TenantID), strings.TrimSpace(c.ActorSubjectID), strings.TrimSpace(c.CorrelationID), strings.TrimSpace(c.IdempotencyKey)
	c.DisplayName, c.EndpointURL, c.SigningKeyReference, c.SigningKeyID = strings.TrimSpace(c.DisplayName), strings.TrimSpace(c.EndpointURL), strings.TrimSpace(c.SigningKeyReference), strings.TrimSpace(c.SigningKeyID)
	c.SubscribedEvents = normalizedWebhookEvents(c.SubscribedEvents)
}
func normalizeWebhookEnvelope(tenantID, actorID, correlationID, key, webhookID *string) {
	*tenantID, *actorID, *correlationID, *key, *webhookID = strings.TrimSpace(*tenantID), strings.TrimSpace(*actorID), strings.TrimSpace(*correlationID), strings.TrimSpace(*key), strings.ToLower(strings.TrimSpace(*webhookID))
}
func normalizedWebhookEvents(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				result = append(result, v)
			}
		}
	}
	sort.Strings(result)
	return result
}
func validWebhookEvents(values []string) bool {
	if len(values) < 1 || len(values) > 32 {
		return false
	}
	for _, v := range values {
		if !slices.Contains(AllowedWebhookEvents, v) {
			return false
		}
	}
	return true
}
func safeKeyID(value string) bool { return webhookSigningKeyID.MatchString(value) }
func validWebhookURL(raw, environment string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path == "" || u.Hostname() == "" {
		return false
	}
	if u.Scheme == "https" {
		return (u.Port() == "" || u.Port() == "443") && !unsafeWebhookHost(u.Hostname())
	}
	if environment != "development" && environment != "sandbox" {
		return false
	}
	if u.Scheme != "http" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func unsafeWebhookHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast())
}
