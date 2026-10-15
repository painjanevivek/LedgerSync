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
	receiver  interface {
		Receive(context.Context, projection.Message) error
	}
}

func (p *RedisStreams) WithReceiver(receiver interface {
	Receive(context.Context, projection.Message) error
}) *RedisStreams {
	p.receiver = receiver
	return p
}
func (p *RedisStreams) recordReceipts(ctx context.Context, messages []projection.Message, err error) ([]projection.Message, error) {
	if err != nil {
		return nil, err
	}
	if p.receiver != nil {
		if batch, ok := p.receiver.(interface {
			ReceiveBatch(context.Context, []projection.Message) error
		}); ok {
			if err := batch.ReceiveBatch(ctx, messages); err != nil {
				return nil, fmt.Errorf("persist logical receive batch: %w", err)
			}
			return messages, nil
		}
		for _, message := range messages {
			if err := p.receiver.Receive(ctx, message); err != nil {
				return nil, fmt.Errorf("persist logical receive evidence: %w", err)
			}
		}
	}
	return messages, nil
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
	ctx, span := p.telemetry.StartLinked(ctx, "event.redis.publish", event.DiagnosticTraceParent)
	defer func() { p.observe(ctx, "publish", started, err); span.End() }()
	aggregateType, aggregateID := event.AggregateType, event.AggregateID
	if aggregateType == "" {
		aggregateType = "account_balance"
	}
	if aggregateID == "" {
		aggregateID = event.AccountID
	}
	values := map[string]any{
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
	}
	if parent := observability.TraceParent(ctx); parent != "" {
		values["diagnostic_traceparent"] = parent
	}
	if event.Causal != nil {
		values["origin_node_id"] = event.Causal.NodeID
		values["origin_logical_timestamp"] = strconv.FormatInt(event.Causal.Timestamp, 10)
		values["parent_reference"] = event.Causal.ParentReference
	}
	if event.Delivery != nil {
		values["delivery_node_id"] = event.Delivery.NodeID
		values["delivery_logical_timestamp"] = strconv.FormatInt(event.Delivery.Timestamp, 10)
	}
	_, err = p.client.XAdd(ctx, &redis.XAddArgs{Stream: p.stream, MaxLen: p.maxLength, Approx: p.maxLength > 0, Values: values}).Result()
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
	defer func() { p.observe(ctx, "ensure_consumer_group", started, err); span.End() }()
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
	defer func() { p.observe(ctx, "read_group", started, err); span.End() }()
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
	messages, decodeErr := decodeEntries(entries)
	return p.recordReceipts(ctx, messages, decodeErr)
}

// RecoverPending claims messages idle longer than minIdle. This makes cache
// projection recoverable when a consumer process stops after delivery.
func (p *RedisStreams) RecoverPending(ctx context.Context, group, consumer string, minIdle time.Duration, count int64) (messages []projection.Message, err error) {
	started := time.Now()
	ctx, span := p.start(ctx, "event.redis.recover_pending")
	defer func() { p.observe(ctx, "recover_pending", started, err); span.End() }()
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
	messages, decodeErr := decodeMessages(entries)
	return p.recordReceipts(ctx, messages, decodeErr)
}

func (p *RedisStreams) Ack(ctx context.Context, group string, messageID string) (err error) {
	started := time.Now()
	ctx, span := p.start(ctx, "event.redis.ack")
	defer func() { p.observe(ctx, "ack", started, err); span.End() }()
	if _, err := p.client.XAck(ctx, p.stream, group, messageID).Result(); err != nil {
		return fmt.Errorf("ack redis stream event: %w", err)
	}
	return nil
}

func (p *RedisStreams) start(ctx context.Context, name string) (context.Context, trace.Span) {
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
	origin, err := decodeCausal(values, "origin", optional("parent_reference"))
	if err != nil {
		return outbox.Event{}, err
	}
	delivery, err := decodeCausal(values, "delivery", id)
	if err != nil {
		return outbox.Event{}, err
	}
	parent := optional("diagnostic_traceparent")
	if !observability.TraceContextFromHeader(parent).IsValid() {
		parent = ""
	}
	return outbox.Event{ID: id, EventType: eventType, TenantID: tenantID, TransferID: transferID, AccountID: accountID, AggregateType: aggregateType, AggregateID: aggregateID, AggregateVersion: version, Payload: []byte(payload), OccurredAt: occurredAt, Causal: origin, Delivery: delivery, DiagnosticTraceParent: parent}, nil
}

func decodeCausal(values map[string]any, prefix, parent string) (*outbox.CausalMetadata, error) {
	nodeValue, hasNode := values[prefix+"_node_id"]
	stampValue, hasStamp := values[prefix+"_logical_timestamp"]
	if !hasNode && !hasStamp {
		return nil, nil
	}
	node, ok := nodeValue.(string)
	if !ok || node == "" || len(node) > 128 || len(parent) > 128 {
		return nil, errors.New("invalid causal node metadata")
	}
	text, ok := stampValue.(string)
	if !ok || !hasStamp {
		return nil, errors.New("invalid causal counter metadata")
	}
	stamp, err := strconv.ParseInt(text, 10, 64)
	if err != nil || stamp < 1 {
		return nil, errors.New("invalid causal counter metadata")
	}
	return &outbox.CausalMetadata{NodeID: node, Timestamp: stamp, ParentReference: parent}, nil
}
