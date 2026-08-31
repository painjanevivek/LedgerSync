package payouts

import (
	"context"
	"errors"
	"sync"
)

// Provider isolates an approved payment provider from financial state. The
// initial FakeProvider is deterministic and never moves money.
type Provider interface {
	CreatePayout(context.Context, CreateCommand) (ProviderResult, error)
	GetPayoutStatus(context.Context, string) (ProviderResult, error)
	VerifyProviderWebhook(context.Context, []byte, string, string) (ProviderResult, error)
	FetchSettlementRecords(context.Context, SettlementQuery) ([]SettlementRecord, error)
}
type CreateCommand struct{ IdempotencyReference, PayoutID, BeneficiaryReference, AmountMinor, FeeMinor string }
type ProviderResult struct{ ProviderReference, Status string }
type SettlementQuery struct {
	Cursor string
	Limit  int
}
type SettlementRecord struct{ ProviderReference, Status, AmountMinor, FeeMinor string }
type FakeProvider struct {
	mu     sync.Mutex
	values map[string]ProviderResult
}

func NewFakeProvider() *FakeProvider { return &FakeProvider{values: map[string]ProviderResult{}} }
func (p *FakeProvider) CreatePayout(_ context.Context, command CreateCommand) (ProviderResult, error) {
	if command.IdempotencyReference == "" || command.PayoutID == "" {
		return ProviderResult{}, errors.New("provider payout identity is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.values[command.IdempotencyReference]; ok {
		return existing, nil
	}
	result := ProviderResult{ProviderReference: "sandbox-" + command.PayoutID, Status: "pending"}
	p.values[command.IdempotencyReference] = result
	return result, nil
}
func (p *FakeProvider) GetPayoutStatus(_ context.Context, reference string) (ProviderResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, result := range p.values {
		if result.ProviderReference == reference {
			return result, nil
		}
	}
	return ProviderResult{}, errors.New("provider payout not found")
}
func (*FakeProvider) VerifyProviderWebhook(context.Context, []byte, string, string) (ProviderResult, error) {
	return ProviderResult{}, errors.New("fake provider does not accept external webhooks")
}
func (*FakeProvider) FetchSettlementRecords(context.Context, SettlementQuery) ([]SettlementRecord, error) {
	return []SettlementRecord{}, nil
}
