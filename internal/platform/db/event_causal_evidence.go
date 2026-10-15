package db

import (
	"context"
	"database/sql"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
	"regexp"
)

var causalNodePattern = regexp.MustCompile(`^[A-Za-z0-9/_.:-]{1,128}$`)

func causalNode(value string) string {
	if !causalNodePattern.MatchString(value) {
		return "redacted"
	}
	return value
}

// Call only after the existing tenant and operator event authorization succeeds.
func (r *OperationsRepository) loadCausalEvidence(ctx context.Context, query scopedReadExecutor, tenantID, eventID string, detail *operations.EventDetail) error {
	var node, parent, stamp sql.NullString
	if err := query.QueryRowContext(ctx, `SELECT node_id,logical_timestamp::text,parent_reference FROM outbox_events WHERE tenant_id=$1 AND id=$2`, tenantID, eventID).Scan(&node, &stamp, &parent); err != nil {
		return err
	}
	if node.Valid && stamp.Valid {
		detail.EventOrder = &operations.EventOrder{NodeID: causalNode(node.String), LogicalTimestamp: stamp.String, ParentReference: parent.String}
	}
	detail.PublicationAttempts = []operations.PublicationEvidence{}
	detail.Receipts = []operations.ReceiptEvidence{}
	rows, err := query.QueryContext(ctx, `SELECT a.claim_generation::text,a.worker_id,
 CASE WHEN a.status='claimed' AND a.lease_until<=clock_timestamp() THEN 'expired' ELSE a.status END,
 a.started_at,a.lease_until,a.completed_at,a.receive_counter::text,a.send_counter::text
 FROM outbox_publication_attempts a JOIN outbox_events e ON e.id=a.event_id
 WHERE e.tenant_id=$1 AND e.id=$2 ORDER BY a.claim_generation DESC LIMIT 26`, tenantID, eventID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item operations.PublicationEvidence
		var completed sql.NullTime
		var received, sent sql.NullString
		if err := rows.Scan(&item.Generation, &item.WorkerID, &item.State, &item.StartedAt, &item.LeaseUntil, &completed, &received, &sent); err != nil {
			_ = rows.Close()
			return err
		}
		item.WorkerID = causalNode(item.WorkerID)
		item.CompletedAt = optionalDatabaseTime(completed)
		item.ReceiveCounter = received.String
		item.SendCounter = sent.String
		detail.PublicationAttempts = append(detail.PublicationAttempts, item)
	}
	err = rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if len(detail.PublicationAttempts) > 25 {
		detail.PublicationAttemptsTruncated = true
		detail.PublicationAttempts = detail.PublicationAttempts[:25]
	}
	rows, err = query.QueryContext(ctx, `SELECT node_id,sender_node_id,sender_counter::text,receive_counter::text,received_at FROM logical_message_receipts WHERE tenant_id=$1 AND event_id=$2 ORDER BY received_at DESC,node_id,receive_counter DESC LIMIT 26`, tenantID, eventID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item operations.ReceiptEvidence
		var sender, counter sql.NullString
		if err := rows.Scan(&item.NodeID, &sender, &counter, &item.ReceiveCounter, &item.ReceivedAt); err != nil {
			return err
		}
		item.NodeID = causalNode(item.NodeID)
		if sender.Valid {
			item.SenderNodeID = causalNode(sender.String)
		}
		item.SendCounter = counter.String
		detail.Receipts = append(detail.Receipts, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(detail.Receipts) > 25 {
		detail.ReceiptsTruncated = true
		detail.Receipts = detail.Receipts[:25]
	}
	return nil
}
