package db

import (
	"context"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
)

// Related returns a bounded set of explicit database relationships. It never
// copies amounts, balances, payloads, notes, or free-form evidence into the
// navigation response.
func (r *InvestigationRepository) Related(ctx context.Context, tenantID, actorID string, filter investigation.RelationshipFilter) (investigation.RelationshipPage, error) {
	if filter.Limit < 1 || filter.Limit > 20 || !relationshipSourceAllowed(filter.SourceType, filter.Access) {
		return investigation.RelationshipPage{}, ErrInvestigationNotFound
	}
	exists, err := r.authorizedRelationshipSource(ctx, tenantID, actorID, filter.SourceType, filter.SourceID)
	if err != nil {
		return investigation.RelationshipPage{}, err
	}
	if !exists {
		return investigation.RelationshipPage{}, ErrInvestigationNotFound
	}
	query, args := relationshipQuery(filter, tenantID, actorID)
	rows, err := r.database.QueryContext(ctx, query, args...)
	if err != nil {
		return investigation.RelationshipPage{}, fmt.Errorf("read related investigation evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := investigation.RelationshipPage{
		SourceType: filter.SourceType, SourceID: filter.SourceID,
		Relationships: make([]investigation.Relationship, 0, filter.Limit), GeneratedAt: time.Now().UTC(),
	}
	for rows.Next() {
		var item investigation.Relationship
		if err := rows.Scan(&item.RelationshipType, &item.TargetType, &item.TargetID, &item.SafeLabel, &item.Status, &item.OccurredAt); err != nil {
			return investigation.RelationshipPage{}, fmt.Errorf("scan related investigation evidence: %w", err)
		}
		item.OccurredAt = item.OccurredAt.UTC()
		item.Source, item.Freshness = "postgresql", "relationship_snapshot"
		page.Relationships = append(page.Relationships, item)
	}
	if err := rows.Err(); err != nil {
		return investigation.RelationshipPage{}, fmt.Errorf("iterate related investigation evidence: %w", err)
	}
	if len(page.Relationships) > filter.Limit {
		page.Relationships, page.Truncated = page.Relationships[:filter.Limit], true
	}
	return page, nil
}

func relationshipSourceAllowed(sourceType string, access investigation.RelationshipAccess) bool {
	switch sourceType {
	case "account":
		return access.Accounts
	case "transfer":
		return access.Transfers
	case "funding":
		return access.Funding
	case "event":
		return access.Events
	case "reconciliation_run", "reconciliation_mismatch":
		return access.Reconciliation
	case "correction":
		return access.Corrections
	default:
		return false
	}
}

func (r *InvestigationRepository) authorizedRelationshipSource(ctx context.Context, tenantID, actorID, sourceType, sourceID string) (bool, error) {
	queries := map[string]string{
		"account":                 `SELECT EXISTS(SELECT 1 FROM accounts a JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id WHERE a.tenant_id=$1 AND a.id=$3 AND owner.subject_id=$2 AND owner.permission IN ('read','debit') AND a.account_kind='customer')`,
		"transfer":                `SELECT EXISTS(SELECT 1 FROM transfers WHERE tenant_id=$1 AND id=$3)`,
		"funding":                 `SELECT EXISTS(SELECT 1 FROM funding_events WHERE tenant_id=$1 AND id=$3)`,
		"event":                   `SELECT EXISTS(SELECT 1 FROM outbox_events WHERE tenant_id=$1 AND id=$3)`,
		"reconciliation_run":      `SELECT EXISTS(SELECT 1 FROM reconciliation_runs WHERE tenant_id=$1 AND id=$3)`,
		"reconciliation_mismatch": `SELECT EXISTS(SELECT 1 FROM reconciliation_mismatches WHERE tenant_id=$1 AND id=$3)`,
		"correction":              `SELECT EXISTS(SELECT 1 FROM transfer_corrections WHERE tenant_id=$1 AND id=$3)`,
	}
	query, ok := queries[sourceType]
	if !ok {
		return false, nil
	}
	var exists bool
	if err := r.database.QueryRowContext(ctx, query, tenantID, actorID, sourceID).Scan(&exists); err != nil {
		return false, fmt.Errorf("authorize related investigation source: %w", err)
	}
	return exists, nil
}

func relationshipQuery(filter investigation.RelationshipFilter, tenantID, actorID string) (string, []any) {
	args := []any{tenantID, actorID, filter.SourceID, filter.Access.Accounts, filter.Access.Transfers, filter.Access.Funding, filter.Access.Events, filter.Access.Reconciliation, filter.Access.Corrections, filter.Limit + 1}
	queries := map[string]string{
		"account":                 accountRelationshipsSQL,
		"transfer":                transferRelationshipsSQL,
		"funding":                 fundingRelationshipsSQL,
		"event":                   eventRelationshipsSQL,
		"reconciliation_run":      reconciliationRunRelationshipsSQL,
		"reconciliation_mismatch": reconciliationMismatchRelationshipsSQL,
		"correction":              correctionRelationshipsSQL,
	}
	return queries[filter.SourceType], args
}

const relationshipSelectTail = `
SELECT relationship_type,target_type,target_id,safe_label,status,occurred_at FROM relationships
ORDER BY occurred_at DESC,relationship_type,target_id LIMIT $10`

const accountRelationshipsSQL = `WITH relationships AS (
 SELECT 'account_transaction','transfer',t.id::text,'Transfer',t.status,COALESCE(t.completed_at,t.created_at) FROM transfers t
  WHERE $5 AND t.tenant_id=$1 AND (t.debit_account_id=$3 OR t.credit_account_id=$3)
 UNION ALL SELECT 'account_funding','funding',f.id::text,'Funding record',f.status,f.updated_at FROM funding_events f
  WHERE $6 AND f.tenant_id=$1 AND f.destination_account_id=$3
 UNION ALL SELECT 'account_event','event',e.id::text,e.event_type,
  CASE WHEN e.published_at IS NOT NULL THEN 'published' WHEN e.dead_at IS NOT NULL THEN 'dead' WHEN e.attempt_count>0 THEN 'retrying' ELSE 'pending' END,e.occurred_at
  FROM outbox_events e WHERE $7 AND e.tenant_id=$1 AND e.account_id=$3
 UNION ALL SELECT 'account_mismatch','reconciliation_mismatch',m.id::text,replace(m.classification,'_',' '),'mismatch',m.created_at
  FROM reconciliation_mismatches m WHERE $8 AND m.tenant_id=$1 AND m.account_id=$3
)` + relationshipSelectTail

const transferRelationshipsSQL = `WITH relationships AS (
 SELECT CASE WHEN a.id=t.debit_account_id THEN 'transfer_source_account' ELSE 'transfer_destination_account' END,'account',a.id::text,'Account',a.status,a.created_at
 FROM transfers t JOIN accounts a ON a.tenant_id=t.tenant_id AND a.id IN (t.debit_account_id,t.credit_account_id)
 JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id AND owner.subject_id=$2 AND owner.permission IN ('read','debit')
 WHERE $4 AND t.tenant_id=$1 AND t.id=$3
 UNION ALL SELECT 'transfer_journal','journal',j.id::text,'Journal transaction','recorded',j.occurred_at FROM journal_transactions j WHERE $5 AND j.tenant_id=$1 AND j.transfer_id=$3
 UNION ALL SELECT 'journal_posting','posting',p.id::text,concat(initcap(p.direction),' posting'),'recorded',p.occurred_at
 FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE $5 AND j.tenant_id=$1 AND j.transfer_id=$3
 UNION ALL SELECT 'transfer_event','event',e.id::text,e.event_type,
  CASE WHEN e.published_at IS NOT NULL THEN 'published' WHEN e.dead_at IS NOT NULL THEN 'dead' WHEN e.attempt_count>0 THEN 'retrying' ELSE 'pending' END,e.occurred_at
 FROM outbox_events e WHERE $7 AND e.tenant_id=$1 AND e.transfer_id=$3
 UNION ALL SELECT 'transfer_delivery','delivery_attempt',d.id::text,concat(initcap(d.delivery_kind),' delivery'),d.status,COALESCE(d.completed_at,d.started_at,d.created_at)
 FROM delivery_attempts d WHERE $7 AND d.tenant_id=$1 AND d.transfer_id=$3
 UNION ALL SELECT 'transfer_mismatch','reconciliation_mismatch',m.id::text,replace(m.classification,'_',' '),'mismatch',m.created_at
 FROM reconciliation_mismatches m WHERE $8 AND m.tenant_id=$1 AND m.transfer_id=$3
 UNION ALL SELECT 'transfer_reconciliation','reconciliation_run',run.id::text,'Reconciliation run',run.status,run.completed_at
 FROM reconciliation_runs run JOIN transfers t ON t.tenant_id=run.tenant_id AND t.id=$3
 JOIN journal_transactions j ON j.tenant_id=t.tenant_id AND j.transfer_id=t.id
 WHERE $8 AND run.tenant_id=$1 AND run.ledger_watermark ~ '^[0-9]+:[0-9]+:(?:[0-9]+(?:,[0-9]+)*)?$'
  AND pg_visible_in_snapshot(t.xmin::text::xid8,run.ledger_watermark::pg_snapshot)
  AND pg_visible_in_snapshot(j.xmin::text::xid8,run.ledger_watermark::pg_snapshot)
  AND NOT EXISTS(SELECT 1 FROM ledger_postings p WHERE p.journal_transaction_id=j.id AND NOT pg_visible_in_snapshot(p.xmin::text::xid8,run.ledger_watermark::pg_snapshot))
 UNION ALL SELECT 'transfer_correction','correction',c.id::text,'Transfer correction',c.status,c.updated_at FROM transfer_corrections c
 WHERE $9 AND c.tenant_id=$1 AND (c.original_transfer_id=$3 OR c.compensation_transfer_id=$3)
)` + relationshipSelectTail

const fundingRelationshipsSQL = `WITH relationships AS (
 SELECT CASE WHEN a.id=f.destination_account_id THEN 'funding_destination_account' ELSE 'funding_system_account' END,'account',a.id::text,'Account',a.status,a.created_at
 FROM funding_events f JOIN accounts a ON a.tenant_id=f.tenant_id AND a.id IN (f.destination_account_id,f.system_account_id)
 JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id AND owner.subject_id=$2 AND owner.permission IN ('read','debit')
 WHERE $4 AND f.tenant_id=$1 AND f.id=$3
 UNION ALL SELECT 'funding_approval','approval',a.id::text,'Approval record',a.status,COALESCE(a.decided_at,a.created_at) FROM approval_records a
 WHERE $6 AND a.tenant_id=$1 AND a.target_id=$3 AND a.command_type IN ('funding','funding_compensation')
 UNION ALL SELECT 'funding_journal','journal',j.id::text,'Journal transaction','recorded',j.occurred_at FROM journal_transactions j WHERE $6 AND j.tenant_id=$1 AND j.funding_event_id=$3
 UNION ALL SELECT 'journal_posting','posting',p.id::text,concat(initcap(p.direction),' posting'),'recorded',p.occurred_at
 FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE $6 AND j.tenant_id=$1 AND j.funding_event_id=$3
 UNION ALL SELECT 'funding_event','event',e.id::text,e.event_type,
  CASE WHEN e.published_at IS NOT NULL THEN 'published' WHEN e.dead_at IS NOT NULL THEN 'dead' WHEN e.attempt_count>0 THEN 'retrying' ELSE 'pending' END,e.occurred_at
 FROM outbox_events e WHERE $7 AND e.tenant_id=$1 AND e.funding_event_id=$3
 UNION ALL SELECT CASE WHEN f.compensation_of_event_id=$3 THEN 'funding_compensation' ELSE 'funding_compensates' END,'funding',f.id::text,'Funding record',f.status,f.updated_at
 FROM funding_events f WHERE $6 AND f.tenant_id=$1 AND (f.compensation_of_event_id=$3 OR f.id=(SELECT compensation_of_event_id FROM funding_events WHERE tenant_id=$1 AND id=$3))
)` + relationshipSelectTail

const eventRelationshipsSQL = `WITH relationships AS (
 SELECT 'event_transfer','transfer',t.id::text,'Transfer',t.status,COALESCE(t.completed_at,t.created_at) FROM outbox_events e JOIN transfers t ON t.tenant_id=e.tenant_id AND t.id=e.transfer_id WHERE $5 AND e.tenant_id=$1 AND e.id=$3
 UNION ALL SELECT 'event_account','account',a.id::text,'Account',a.status,a.created_at FROM outbox_events e JOIN accounts a ON a.tenant_id=e.tenant_id AND a.id=e.account_id JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id AND owner.subject_id=$2 AND owner.permission IN ('read','debit') WHERE $4 AND e.tenant_id=$1 AND e.id=$3
 UNION ALL SELECT 'event_delivery','delivery_attempt',d.id::text,concat(initcap(d.delivery_kind),' delivery'),d.status,COALESCE(d.completed_at,d.started_at,d.created_at) FROM delivery_attempts d WHERE $7 AND d.tenant_id=$1 AND d.outbox_event_id=$3
)` + relationshipSelectTail

const reconciliationRunRelationshipsSQL = `WITH relationships AS (
 SELECT 'run_mismatch','reconciliation_mismatch',m.id::text,replace(m.classification,'_',' '),'mismatch',m.created_at FROM reconciliation_mismatches m WHERE $8 AND m.tenant_id=$1 AND m.run_id=$3
 UNION ALL SELECT 'mismatch_account','account',a.id::text,'Account',a.status,m.created_at FROM reconciliation_mismatches m JOIN accounts a ON a.tenant_id=m.tenant_id AND a.id=m.account_id JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id AND owner.subject_id=$2 AND owner.permission IN ('read','debit') WHERE $4 AND m.tenant_id=$1 AND m.run_id=$3
 UNION ALL SELECT 'mismatch_transfer','transfer',t.id::text,'Transfer',t.status,m.created_at FROM reconciliation_mismatches m JOIN transfers t ON t.tenant_id=m.tenant_id AND t.id=m.transfer_id WHERE $5 AND m.tenant_id=$1 AND m.run_id=$3
)` + relationshipSelectTail

const reconciliationMismatchRelationshipsSQL = `WITH relationships AS (
 SELECT 'mismatch_run','reconciliation_run',run.id::text,'Reconciliation run',run.status,run.completed_at FROM reconciliation_mismatches m JOIN reconciliation_runs run ON run.tenant_id=m.tenant_id AND run.id=m.run_id WHERE $8 AND m.tenant_id=$1 AND m.id=$3
 UNION ALL SELECT 'mismatch_account','account',a.id::text,'Account',a.status,m.created_at FROM reconciliation_mismatches m JOIN accounts a ON a.tenant_id=m.tenant_id AND a.id=m.account_id JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id AND owner.subject_id=$2 AND owner.permission IN ('read','debit') WHERE $4 AND m.tenant_id=$1 AND m.id=$3
 UNION ALL SELECT 'mismatch_transfer','transfer',t.id::text,'Transfer',t.status,m.created_at FROM reconciliation_mismatches m JOIN transfers t ON t.tenant_id=m.tenant_id AND t.id=m.transfer_id WHERE $5 AND m.tenant_id=$1 AND m.id=$3
)` + relationshipSelectTail

const correctionRelationshipsSQL = `WITH relationships AS (
 SELECT CASE WHEN t.id=c.original_transfer_id THEN 'correction_original_transfer' ELSE 'correction_compensation_transfer' END,'transfer',t.id::text,'Transfer',t.status,COALESCE(t.completed_at,t.created_at)
 FROM transfer_corrections c JOIN transfers t ON t.tenant_id=c.tenant_id AND t.id IN (c.original_transfer_id,c.compensation_transfer_id) WHERE $5 AND c.tenant_id=$1 AND c.id=$3
 UNION ALL SELECT 'correction_approval','approval',a.id::text,'Approval record',a.status,COALESCE(a.decided_at,a.created_at) FROM approval_records a WHERE $9 AND a.tenant_id=$1 AND a.target_id=$3 AND a.command_type='transfer_compensation'
)` + relationshipSelectTail

var _ investigation.RelationshipRepository = (*InvestigationRepository)(nil)
