package db

import (
	"context"
	"errors"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
)

// ReceiveBatch reuses one scoped transaction for each contiguous tenant group.
// It retains transport order and calls the same controlled function for every
// receipt. A failed group rolls back; earlier tenant groups can already commit.
// Redis messages remain pending until all receipt groups and cache writes pass.
func (r *LogicalClockRepository) ReceiveBatch(ctx context.Context, messages []projection.Message) error {
	if len(messages) > 100 {
		return errors.New("logical receipt batch exceeds 100 messages")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for start := 0; start < len(messages); {
		end := start + 1
		for end < len(messages) && messages[end].Event.TenantID == messages[start].Event.TenantID {
			end++
		}
		if err := r.receiveTenantBatch(ctx, messages[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (r *LogicalClockRepository) receiveTenantBatch(ctx context.Context, messages []projection.Message) error {
	tx, err := beginTenantTransaction(ctx, r.database, TenantScope{TenantID: messages[0].Event.TenantID, ActorID: "system:" + r.nodeID}, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, message := range messages {
		sender := message.Event.Delivery
		if sender == nil {
			sender = message.Event.Causal
		}
		var senderNode, senderCounter any
		if sender != nil {
			senderNode = sender.NodeID
			senderCounter = sender.Timestamp
		}
		if _, err := tx.ExecContext(ctx, `SELECT public.controlled_receive_message_v1($1,$2,$3,$4,$5,$6)`, message.Event.TenantID, r.nodeID, message.Event.ID, message.ID, senderNode, senderCounter); err != nil {
			return err
		}
	}
	return tx.Commit()
}
