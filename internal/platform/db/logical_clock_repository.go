package db

import (
	"context"
	"database/sql"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
)

// LogicalClockRepository persists atomic per-node Lamport updates. Clock
// records are investigation evidence. They do not order financial commands.
type LogicalClockRepository struct {
	database *sql.DB
	nodeID   string
}

func NewLogicalClockRepository(database *sql.DB, nodeID string) (*LogicalClockRepository, error) {
	if database == nil || len(nodeID) < 1 || len(nodeID) > 128 {
		return nil, errors.New("database and bounded node identity are required")
	}
	return &LogicalClockRepository{database: database, nodeID: nodeID}, nil
}
func (r *LogicalClockRepository) PreparePublication(ctx context.Context, owner string, event outbox.Event) (outbox.Event, error) {
	tx, err := beginTenantTransaction(ctx, r.database, TenantScope{TenantID: event.TenantID, ActorID: owner}, nil)
	if err != nil {
		return event, err
	}
	defer func() { _ = tx.Rollback() }()
	var node, parent sql.NullString
	var stamp sql.NullInt64
	var sent int64
	var diagnostic sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT prepared.node_id,prepared.logical_timestamp,prepared.parent_reference,prepared.send_counter,event.diagnostic_traceparent
FROM public.controlled_prepare_publication_v1($1,$2,$3,$4) prepared
JOIN public.outbox_events event ON event.id=$1 AND event.tenant_id=$5`, event.ID, owner, event.ClaimGeneration, r.nodeID, event.TenantID).Scan(&node, &stamp, &parent, &sent, &diagnostic); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return event, ErrStaleClaim
		}
		return event, err
	}
	event.DiagnosticTraceParent = diagnostic.String
	if err := tx.Commit(); err != nil {
		return event, err
	}
	if node.Valid && stamp.Valid {
		event.Causal = &outbox.CausalMetadata{NodeID: node.String, Timestamp: stamp.Int64, ParentReference: parent.String}
	} else {
		event.Causal = nil
	}
	event.Delivery = &outbox.CausalMetadata{NodeID: r.nodeID, Timestamp: sent, ParentReference: event.ID}
	return event, nil
}
func (r *LogicalClockRepository) Receive(ctx context.Context, message projection.Message) error {
	return r.ReceiveBatch(ctx, []projection.Message{message})
}

// Counters survive process restart because no process-local counter is used.
func (r *LogicalClockRepository) Advance(ctx context.Context, incoming int64) (int64, error) {
	var value int64
	err := r.database.QueryRowContext(ctx, `SELECT advance_logical_clock($1,$2)`, r.nodeID, incoming).Scan(&value)
	return value, err
}
