// Package projection applies replay-safe balance notifications to disposable
// read models. PostgreSQL is still authoritative; a projection failure can only
// make a read slower, never change a financial result.
package projection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
)

const BalanceChangedEventType = "account.balance.changed.v1"

type Message struct {
	ID    string
	Event outbox.Event
}

type Stream interface {
	EnsureConsumerGroup(context.Context, string) error
	RecoverPending(context.Context, string, string, time.Duration, int64) ([]Message, error)
	ReadGroup(context.Context, string, string, int64, time.Duration) ([]Message, error)
	Ack(context.Context, string, string) error
}

type Cache interface {
	Put(context.Context, accounts.Balance) (bool, error)
}

type Config struct {
	Group       string
	Consumer    string
	BatchSize   int64
	PendingIdle time.Duration
	Block       time.Duration
}

type BalanceProjector struct {
	stream Stream
	cache  Cache
	config Config
}

func NewBalanceProjector(stream Stream, cache Cache, config Config) (*BalanceProjector, error) {
	if stream == nil || cache == nil {
		return nil, errors.New("balance stream and cache are required")
	}
	if strings.TrimSpace(config.Group) == "" || strings.TrimSpace(config.Consumer) == "" {
		return nil, errors.New("balance projector group and consumer are required")
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.PendingIdle <= 0 {
		config.PendingIdle = 30 * time.Second
	}
	if config.Block <= 0 {
		config.Block = 100 * time.Millisecond
	}
	return &BalanceProjector{stream: stream, cache: cache, config: config}, nil
}

// RunOnce recovers an abandoned pending message before reading new work. A
// message is acknowledged only after the monotonic cache write completes.
func (p *BalanceProjector) RunOnce(ctx context.Context) (int, error) {
	if err := p.stream.EnsureConsumerGroup(ctx, p.config.Group); err != nil {
		return 0, fmt.Errorf("ensure projection consumer group: %w", err)
	}
	messages, err := p.stream.RecoverPending(ctx, p.config.Group, p.config.Consumer, p.config.PendingIdle, p.config.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("recover pending balance events: %w", err)
	}
	if len(messages) == 0 {
		messages, err = p.stream.ReadGroup(ctx, p.config.Group, p.config.Consumer, p.config.BatchSize, p.config.Block)
		if err != nil {
			return 0, fmt.Errorf("read balance events: %w", err)
		}
	}
	for _, message := range messages {
		balance, err := decodeBalance(message.Event)
		if err != nil {
			return len(messages), err
		}
		if _, err := p.cache.Put(ctx, balance); err != nil {
			return len(messages), fmt.Errorf("apply balance cache projection: %w", err)
		}
		if err := p.stream.Ack(ctx, p.config.Group, message.ID); err != nil {
			return len(messages), fmt.Errorf("ack balance event: %w", err)
		}
	}
	return len(messages), nil
}

func decodeBalance(event outbox.Event) (accounts.Balance, error) {
	if event.EventType != BalanceChangedEventType || event.ID == "" || event.TenantID == "" || event.AccountID == "" || event.AggregateVersion < 0 {
		return accounts.Balance{}, errors.New("invalid balance event envelope")
	}
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.UseNumber()
	var payload struct {
		EventID        string      `json:"event_id"`
		EventType      string      `json:"event_type"`
		AccountID      string      `json:"account_id"`
		Currency       string      `json:"currency"`
		AvailableMinor json.Number `json:"available_minor"`
		BalanceVersion json.Number `json:"balance_version"`
		OccurredAt     string      `json:"occurred_at"`
	}
	if err := decoder.Decode(&payload); err != nil {
		return accounts.Balance{}, fmt.Errorf("decode balance event payload: %w", err)
	}
	if decoder.More() || payload.EventID != event.ID || payload.EventType != event.EventType || payload.AccountID != event.AccountID {
		return accounts.Balance{}, errors.New("balance event envelope does not match payload")
	}
	available, err := strconv.ParseInt(payload.AvailableMinor.String(), 10, 64)
	if err != nil || available < 0 {
		return accounts.Balance{}, errors.New("balance event has invalid available_minor")
	}
	version, err := strconv.ParseInt(payload.BalanceVersion.String(), 10, 64)
	if err != nil || version < 0 || version != event.AggregateVersion {
		return accounts.Balance{}, errors.New("balance event has invalid balance_version")
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, payload.OccurredAt)
	if err != nil || len(payload.Currency) != 3 {
		return accounts.Balance{}, errors.New("balance event has invalid currency or occurred_at")
	}
	return accounts.Balance{TenantID: event.TenantID, AccountID: event.AccountID, Currency: strings.ToUpper(payload.Currency), AvailableMinor: available, LedgerMinor: available, Version: version, AsOf: occurredAt.UTC()}, nil
}
