// Package events provides Redis Streams transport adapters. PostgreSQL remains
// authoritative; this package is deliberately disposable and carries only
// replay-safe notifications and derived projection work.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

const BalanceStream = "ledgersync:balance-events:v1"

type RedisStreams struct {
	client    redis.UniversalClient
	stream    string
	telemetry *observability.Telemetry
	maxLength int64
}

func (p *RedisStreams) WithMaxLength(maxLength int64) *RedisStreams {
	if maxLength > 0 {
		p.maxLength = maxLength
	}
	return p
}

func (p *RedisStreams) ObserveHealth(ctx context.Context, group string) error {
	depth, err := p.client.XLen(ctx, p.stream).Result()
	if err != nil {
		return fmt.Errorf("read redis stream depth: %w", err)
	}
	groups, err := p.client.XInfoGroups(ctx, p.stream).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("read redis consumer lag: %w", err)
	}
	var lag int64
	for _, current := range groups {
		if current.Name == group {
			lag = current.Lag
			break
		}
	}
	if p.telemetry != nil {
		p.telemetry.ObserveRedisStream(ctx, depth, lag)
	}
	return nil
}

func NewRedisStreams(client redis.UniversalClient, stream string, telemetry ...*observability.Telemetry) (*RedisStreams, error) {
	if client == nil {
		return nil, errors.New("redis streams client is required")
	}
	if stream == "" {
		stream = BalanceStream
	}
	return &RedisStreams{client: client, stream: stream, telemetry: firstTelemetry(telemetry)}, nil
}

func (p *RedisStreams) Publish(ctx context.Context, event outbox.Event) (err error) {
	started := time.Now()
	ctx, span := p.start(ctx, "event.redis.publish")
	defer func() { span.End(); p.observe(ctx, "publish", started, err) }()
	aggregateType, aggregateID := event.AggregateType, event.AggregateID
	if aggregateType == "" {
		aggregateType = "account_balance"
	}
	if aggregateID == "" {
		aggregateID = event.AccountID
	}
	_, err = p.client.XAdd(ctx, &redis.XAddArgs{Stream: p.stream, MaxLen: p.maxLength, Approx: p.maxLength > 0, Values: map[string]any{
		"event_id":          event.ID,
		"event_type":        event.EventType,
		"tenant_id":         event.TenantID,
		"transfer_id":       event.TransferID,
		"account_id":        event.AccountID,
		"aggregate_type":    aggregateType,
		"aggregate_id":      aggregateID,
		"aggregate_version": strconv.FormatInt(event.AggregateVersion, 10),
		"payload":           string(event.Payload),
		"occurred_at":       event.OccurredAt.UTC().Format(time.RFC3339Nano),
	}}).Result()
	if err != nil {
		return fmt.Errorf("publish redis stream event: %w", err)
	}
	return nil
}

// EnsureConsumerGroup is idempotent. The group starts at zero so a newly
// provisioned cache can rebuild from retained stream entries when available.
func (p *RedisStreams) EnsureConsumerGroup(ctx context.Context, group string) (err error) {
	started := time.Now()
	ctx, span := p.start(ctx, "event.redis.ensure_consumer_group")
	defer func() { span.End(); p.observe(ctx, "ensure_consumer_group", started, err) }()
	groups, err := p.client.XInfoGroups(ctx, p.stream).Result()
	if err == nil {
		for _, existing := range groups {
			if existing.Name == group {
				return nil
			}
		}
	} else if !strings.Contains(strings.ToLower(err.Error()), "no such key") {
		return fmt.Errorf("inspect redis consumer groups: %w", err)
	}
	err = p.client.XGroupCreateMkStream(ctx, p.stream, group, "0").Err()
	if err != nil && !strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP") {
		return fmt.Errorf("create redis consumer group: %w", err)
	}
	return nil
}

func (p *RedisStreams) ReadGroup(ctx context.Context, group, consumer string, count int64, block time.Duration) (messages []projection.Message, err error) {
	started := time.Now()
	ctx, span := p.start(ctx, "event.redis.read_group")
	defer func() { span.End(); p.observe(ctx, "read_group", started, err) }()
	if group == "" || consumer == "" || count <= 0 {
		return nil, errors.New("group, consumer, and count are required")
	}
	entries, err := p.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: group, Consumer: consumer, Streams: []string{p.stream, ">"}, Count: count, Block: block}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read redis consumer group: %w", err)
	}
	return decodeEntries(entries)
}

// RecoverPending claims messages idle longer than minIdle. This makes cache
// projection recoverable when a consumer process stops after delivery.
func (p *RedisStreams) RecoverPending(ctx context.Context, group, consumer string, minIdle time.Duration, count int64) (messages []projection.Message, err error) {
	started := time.Now()
	ctx, span := p.start(ctx, "event.redis.recover_pending")
	defer func() { span.End(); p.observe(ctx, "recover_pending", started, err) }()
	if group == "" || consumer == "" || count <= 0 {
		return nil, errors.New("group, consumer, and count are required")
	}
	entries, _, err := p.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: p.stream, Group: group, Consumer: consumer, MinIdle: minIdle, Start: "0-0", Count: count}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recover redis pending events: %w", err)
	}
	return decodeMessages(entries)
}

func (p *RedisStreams) Ack(ctx context.Context, group string, messageID string) (err error) {
	started := time.Now()
	ctx, span := p.start(ctx, "event.redis.ack")
	defer func() { span.End(); p.observe(ctx, "ack", started, err) }()
	if _, err := p.client.XAck(ctx, p.stream, group, messageID).Result(); err != nil {
		return fmt.Errorf("ack redis stream event: %w", err)
	}
	return nil
}

func (p *RedisStreams) start(ctx context.Context, name string) (context.Context, trace.Span) {
	if p.telemetry == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return p.telemetry.Start(ctx, name)
}

func (p *RedisStreams) observe(ctx context.Context, operation string, started time.Time, err error) {
	if p.telemetry != nil {
		p.telemetry.ObserveBoundary(ctx, "event", operation, started, err)
	}
}

func firstTelemetry(values []*observability.Telemetry) *observability.Telemetry {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func decodeEntries(streams []redis.XStream) ([]projection.Message, error) {
	var result []projection.Message
	for _, stream := range streams {
		decoded, err := decodeMessages(stream.Messages)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
	}
	return result, nil
}

func decodeMessages(messages []redis.XMessage) ([]projection.Message, error) {
	result := make([]projection.Message, 0, len(messages))
	for _, message := range messages {
		event, err := decodeEvent(message.Values)
		if err != nil {
			return nil, err
		}
		result = append(result, projection.Message{ID: message.ID, Event: event})
	}
	return result, nil
}

func decodeEvent(values map[string]any) (outbox.Event, error) {
	get := func(key string) (string, error) {
		value, ok := values[key]
		if !ok {
			return "", fmt.Errorf("redis stream message missing %s", key)
		}
		text, ok := value.(string)
		if !ok || text == "" {
			return "", fmt.Errorf("redis stream message has invalid %s", key)
		}
		return text, nil
	}
	id, err := get("event_id")
	if err != nil {
		return outbox.Event{}, err
	}
	eventType, err := get("event_type")
	if err != nil {
		return outbox.Event{}, err
	}
	tenantID, err := get("tenant_id")
	if err != nil {
		return outbox.Event{}, err
	}
	optional := func(key string) string {
		value, ok := values[key]
		if !ok {
			return ""
		}
		text, _ := value.(string)
		return text
	}
	transferID := optional("transfer_id")
	accountID := optional("account_id")
	versionText, err := get("aggregate_version")
	if err != nil {
		return outbox.Event{}, err
	}
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || version < 0 {
		return outbox.Event{}, errors.New("redis stream message has invalid aggregate_version")
	}
	payload, err := get("payload")
	if err != nil {
		return outbox.Event{}, err
	}
	occurredAtText, err := get("occurred_at")
	if err != nil {
		return outbox.Event{}, err
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, occurredAtText)
	if err != nil {
		return outbox.Event{}, errors.New("redis stream message has invalid occurred_at")
	}
	if !json.Valid([]byte(payload)) {
		return outbox.Event{}, errors.New("redis stream message payload is not JSON")
	}
	aggregateType, aggregateID := optional("aggregate_type"), optional("aggregate_id")
	if aggregateType == "" {
		aggregateType = "account_balance"
	}
	if aggregateID == "" {
		aggregateID = accountID
	}
	return outbox.Event{ID: id, EventType: eventType, TenantID: tenantID, TransferID: transferID, AccountID: accountID, AggregateType: aggregateType, AggregateID: aggregateID, AggregateVersion: version, Payload: []byte(payload), OccurredAt: occurredAt}, nil
}
