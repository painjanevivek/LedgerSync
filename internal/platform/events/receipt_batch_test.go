package events

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	"testing"
)

type batchReceiverProbe struct {
	single, batches, items int
	fail                   error
}

func (p *batchReceiverProbe) Receive(context.Context, projection.Message) error {
	p.single++
	return nil
}
func (p *batchReceiverProbe) ReceiveBatch(_ context.Context, messages []projection.Message) error {
	p.batches++
	p.items += len(messages)
	return p.fail
}

type legacyReceiverProbe struct {
	items int
	fail  error
}

func (p *legacyReceiverProbe) Receive(context.Context, projection.Message) error {
	p.items++
	return p.fail
}

func TestReceiptBatchFailureDoesNotReleaseMessagesToProjector(t *testing.T) {
	messages := make([]projection.Message, 3)
	receiver := &batchReceiverProbe{}
	stream := &RedisStreams{receiver: receiver}
	result, err := stream.recordReceipts(context.Background(), messages, nil)
	if err != nil || len(result) != 3 || receiver.batches != 1 || receiver.items != 3 || receiver.single != 0 {
		t.Fatal("bounded batch path was not used")
	}
	receiver.fail = errors.New("receipt store unavailable")
	result, err = stream.recordReceipts(context.Background(), messages, nil)
	if err == nil || result != nil {
		t.Fatal("messages were released after receipt failure")
	}
	calls := receiver.batches
	result, err = stream.recordReceipts(context.Background(), messages, errors.New("invalid message"))
	if err == nil || result != nil || receiver.batches != calls {
		t.Fatal("invalid decoded input reached receipt storage")
	}
	legacy := &legacyReceiverProbe{}
	stream.receiver = legacy
	result, err = stream.recordReceipts(context.Background(), messages, nil)
	if err != nil || len(result) != 3 || legacy.items != 3 {
		t.Fatal("legacy receiver contract changed")
	}
}
