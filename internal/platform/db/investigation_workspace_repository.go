package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
)

var canonicalWorkspaceID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (r *InvestigationRepository) ListWorkspaces(ctx context.Context, tenantID, actorID string, access investigation.SearchAccess) (investigation.WorkspacePage, error) {
	if r == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" || !access.Any() {
		return investigation.WorkspacePage{}, investigation.ErrInvalidWorkspace
	}
	rows, err := r.database.QueryContext(ctx, `SELECT id::text,title,taxonomy,status,version,created_at,updated_at,closed_at
FROM investigation_workspaces workspace
WHERE tenant_id=$1 AND owner_subject_id=$2 AND (
 (root_record_type='account' AND $3 AND EXISTS(SELECT 1 FROM accounts record JOIN account_owners owner ON owner.tenant_id=record.tenant_id AND owner.account_id=record.id WHERE record.tenant_id=workspace.tenant_id AND record.id=workspace.root_record_id AND owner.subject_id=$2 AND owner.permission IN ('read','debit') AND record.account_kind='customer')) OR
 (root_record_type='transfer' AND $4 AND EXISTS(SELECT 1 FROM transfers record WHERE record.tenant_id=workspace.tenant_id AND record.id=workspace.root_record_id)) OR
 (root_record_type='funding' AND $5 AND EXISTS(SELECT 1 FROM funding_events record WHERE record.tenant_id=workspace.tenant_id AND record.id=workspace.root_record_id)) OR
 (root_record_type='event' AND $6 AND EXISTS(SELECT 1 FROM outbox_events record WHERE record.tenant_id=workspace.tenant_id AND record.id=workspace.root_record_id)) OR
 (root_record_type='reconciliation_run' AND $7 AND EXISTS(SELECT 1 FROM reconciliation_runs record WHERE record.tenant_id=workspace.tenant_id AND record.id=workspace.root_record_id)) OR
 (root_record_type='reconciliation_mismatch' AND $7 AND EXISTS(SELECT 1 FROM reconciliation_mismatches record WHERE record.tenant_id=workspace.tenant_id AND record.id=workspace.root_record_id)) OR
 (root_record_type='correction' AND $8 AND EXISTS(SELECT 1 FROM transfer_corrections record WHERE record.tenant_id=workspace.tenant_id AND record.id=workspace.root_record_id))
)
ORDER BY updated_at DESC,id DESC LIMIT $9`, tenantID, actorID, access.Accounts, access.Transfers, access.Funding, access.Events, access.Reconciliation, access.Corrections, investigation.MaxInvestigationWorkspaces)
	if err != nil {
		return investigation.WorkspacePage{}, fmt.Errorf("list investigation workspaces: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := investigation.WorkspacePage{Investigations: make([]investigation.WorkspaceSummary, 0), GeneratedAt: time.Now().UTC()}
	for rows.Next() {
		var item investigation.WorkspaceSummary
		var version int64
		var closedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Title, &item.Taxonomy, &item.Status, &version, &item.CreatedAt, &item.UpdatedAt, &closedAt); err != nil {
			return investigation.WorkspacePage{}, fmt.Errorf("scan investigation workspace summary: %w", err)
		}
		item.Version = strconv.FormatInt(version, 10)
		item.CreatedAt, item.UpdatedAt = item.CreatedAt.UTC(), item.UpdatedAt.UTC()
		if closedAt.Valid {
			closed := closedAt.Time.UTC()
			item.ClosedAt = &closed
		}
		page.Investigations = append(page.Investigations, item)
	}
	if err := rows.Err(); err != nil {
		return investigation.WorkspacePage{}, fmt.Errorf("iterate investigation workspaces: %w", err)
	}
	return page, nil
}

func (r *InvestigationRepository) CreateWorkspace(ctx context.Context, command investigation.WorkspaceCreate) (investigation.Workspace, error) {
	command, err := investigation.NormalizeWorkspaceCreate(command)
	if err != nil || strings.TrimSpace(command.TenantID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.CorrelationID) == "" {
		return investigation.Workspace{}, investigation.ErrInvalidWorkspace
	}
	related, err := r.Related(ctx, command.TenantID, command.ActorID, investigation.RelationshipFilter{SourceType: command.RootRecordType, SourceID: command.RootRecordID, Limit: 20, Access: command.Access})
	if errors.Is(err, ErrInvestigationNotFound) {
		return investigation.Workspace{}, investigation.ErrWorkspaceNotFound
	}
	if err != nil {
		return investigation.Workspace{}, fmt.Errorf("authorize investigation workspace root: %w", err)
	}
	references := capturedWorkspaceReferences(command, related)
	workspaceID, err := newUUID()
	if err != nil {
		return investigation.Workspace{}, err
	}
	auditID, err := newUUID()
	if err != nil {
		return investigation.Workspace{}, err
	}
	when := workspaceTime(command.OccurredAt)
	err = WithSerializableSequence(ctx, r.database, "investigation-workspace-owner:"+command.TenantID+":"+command.ActorID, 3, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM investigation_workspaces WHERE tenant_id=$1 AND owner_subject_id=$2 AND status='open'`, command.TenantID, command.ActorID).Scan(&count); err != nil {
			return err
		}
		if count >= investigation.MaxInvestigationWorkspaces {
			return investigation.ErrWorkspaceLimit
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO investigation_workspaces(id,tenant_id,owner_subject_id,title,taxonomy,status,query_kind,root_record_type,query_value,root_record_id,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'open',$6,$7,$8,$9,1,$10,$10)`, workspaceID, command.TenantID, command.ActorID, command.Title, command.Taxonomy, command.QueryKind, command.RootRecordType, command.QueryValue, command.RootRecordID, when); err != nil {
			return err
		}
		for position, reference := range references {
			if _, err := tx.ExecContext(ctx, `INSERT INTO investigation_workspace_references(investigation_id,tenant_id,position,relationship_type,source_record_type,source_record_id,record_type,record_id,captured_at) VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')::uuid,$7,$8,$9)`, workspaceID, command.TenantID, position, reference.RelationshipType, reference.SourceRecordType, reference.SourceRecordID, reference.RecordType, reference.RecordID, when); err != nil {
				return err
			}
		}
		return insertWorkspaceAudit(ctx, tx, auditID, command.TenantID, command.ActorID, "investigation.workspace_created", workspaceID, command.CorrelationID, command.Taxonomy, "open", 1, len(references), when)
	})
	if err != nil {
		return investigation.Workspace{}, mapWorkspaceWriteError("create investigation workspace", err)
	}
	return r.GetWorkspace(ctx, command.TenantID, command.ActorID, workspaceID, command.Access)
}

func capturedWorkspaceReferences(command investigation.WorkspaceCreate, related investigation.RelationshipPage) []investigation.WorkspaceReference {
	when := workspaceTime(command.OccurredAt)
	result := []investigation.WorkspaceReference{{RelationshipType: "root", RecordType: command.RootRecordType, RecordID: command.RootRecordID, TargetPath: investigation.WorkspaceTargetPath(command.RootRecordType, command.RootRecordID), CapturedAt: when}}
	seen := map[string]struct{}{command.RootRecordType + ":" + command.RootRecordID: {}}
	for _, item := range related.Relationships {
		key := item.TargetType + ":" + item.TargetID
		if len(result) >= investigation.MaxWorkspaceReferences || !investigation.WorkspaceRecordType(item.TargetType) || !investigation.WorkspaceRecordAllowed(item.TargetType, command.Access) || !canonicalWorkspaceID.MatchString(item.TargetID) || !investigation.ValidWorkspaceRelationship(item.RelationshipType) {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, investigation.WorkspaceReference{RelationshipType: item.RelationshipType, SourceRecordType: command.RootRecordType, SourceRecordID: command.RootRecordID, RecordType: item.TargetType, RecordID: item.TargetID, TargetPath: investigation.WorkspaceTargetPath(item.TargetType, item.TargetID), CapturedAt: when})
	}
	return result
}

func (r *InvestigationRepository) GetWorkspace(ctx context.Context, tenantID, actorID, workspaceID string, access investigation.SearchAccess) (investigation.Workspace, error) {
	if !canonicalWorkspaceID.MatchString(workspaceID) || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" || !access.Any() {
		return investigation.Workspace{}, investigation.ErrWorkspaceNotFound
	}
	workspace, rootType, rootID, err := r.readWorkspaceHeader(ctx, tenantID, actorID, workspaceID)
	if errors.Is(err, sql.ErrNoRows) || !investigation.WorkspaceRecordAllowed(rootType, access) {
		return investigation.Workspace{}, investigation.ErrWorkspaceNotFound
	}
	if err != nil {
		return investigation.Workspace{}, err
	}
	exists, err := r.authorizedRelationshipSource(ctx, tenantID, actorID, rootType, rootID)
	if err != nil {
		return investigation.Workspace{}, err
	}
	if !exists {
		return investigation.Workspace{}, investigation.ErrWorkspaceNotFound
	}
	workspace.HistoricalContext.References, workspace.HistoricalContext.WithheldReferenceCount, err = r.readWorkspaceReferences(ctx, tenantID, actorID, workspaceID, access)
	if err != nil {
		return investigation.Workspace{}, err
	}
	workspace.HistoricalContext.History, workspace.HistoricalContext.HistoryTruncated, err = r.readWorkspaceHistory(ctx, tenantID, actorID, workspaceID)
	if err != nil {
		return investigation.Workspace{}, err
	}
	current, err := r.workspaceCurrentEvidence(ctx, tenantID, actorID, rootType, rootID, access)
	if err != nil {
		return investigation.Workspace{}, err
	}
	workspace.CurrentEvidence = current
	return workspace, nil
}

func (r *InvestigationRepository) readWorkspaceHeader(ctx context.Context, tenantID, actorID, workspaceID string) (investigation.Workspace, string, string, error) {
	var workspace investigation.Workspace
	var rootType, rootID string
	var version int64
	var closedAt sql.NullTime
	err := r.database.QueryRowContext(ctx, `SELECT id::text,title,taxonomy,status,query_kind,root_record_type,query_value,root_record_id::text,version,created_at,updated_at,closed_at FROM investigation_workspaces WHERE tenant_id=$1 AND owner_subject_id=$2 AND id=$3`, tenantID, actorID, workspaceID).Scan(&workspace.ID, &workspace.Title, &workspace.Taxonomy, &workspace.Status, &workspace.HistoricalContext.QueryContext.Kind, &rootType, &workspace.HistoricalContext.QueryContext.Value, &rootID, &version, &workspace.CreatedAt, &workspace.UpdatedAt, &closedAt)
	if err != nil {
		return investigation.Workspace{}, "", "", err
	}
	workspace.HistoricalContext.QueryContext.RecordType = rootType
	workspace.Version = strconv.FormatInt(version, 10)
	workspace.CreatedAt, workspace.UpdatedAt = workspace.CreatedAt.UTC(), workspace.UpdatedAt.UTC()
	if closedAt.Valid {
		closed := closedAt.Time.UTC()
		workspace.ClosedAt = &closed
	}
	return workspace, rootType, rootID, nil
}

func (r *InvestigationRepository) readWorkspaceReferences(ctx context.Context, tenantID, actorID, workspaceID string, access investigation.SearchAccess) ([]investigation.WorkspaceReference, int, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT relationship_type,COALESCE(source_record_type,''),COALESCE(source_record_id::text,''),record_type,record_id::text,captured_at FROM investigation_workspace_references WHERE tenant_id=$1 AND investigation_id=$2 ORDER BY position`, tenantID, workspaceID)
	if err != nil {
		return nil, 0, fmt.Errorf("read investigation workspace references: %w", err)
	}
	items := make([]investigation.WorkspaceReference, 0, investigation.MaxWorkspaceReferences)
	for rows.Next() {
		var item investigation.WorkspaceReference
		if err := rows.Scan(&item.RelationshipType, &item.SourceRecordType, &item.SourceRecordID, &item.RecordType, &item.RecordID, &item.CapturedAt); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	result, withheld := make([]investigation.WorkspaceReference, 0, len(items)), 0
	for _, item := range items {
		if !investigation.WorkspaceRecordAllowed(item.RecordType, access) {
			withheld++
			continue
		}
		exists, err := r.authorizedRelationshipSource(ctx, tenantID, actorID, item.RecordType, item.RecordID)
		if err != nil {
			return nil, 0, err
		}
		if !exists {
			withheld++
			continue
		}
		item.TargetPath, item.CapturedAt = investigation.WorkspaceTargetPath(item.RecordType, item.RecordID), item.CapturedAt.UTC()
		result = append(result, item)
	}
	return result, withheld, nil
}

func (r *InvestigationRepository) readWorkspaceHistory(ctx context.Context, tenantID, actorID, workspaceID string) ([]investigation.WorkspaceHistoryItem, bool, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT event_type,actor_subject_id=$2,sanitized_metadata->>'workspace_version',sanitized_metadata->>'status',occurred_at FROM audit_events WHERE tenant_id=$1 AND target_type='investigation_workspace' AND target_id=$3 AND event_type IN ('investigation.workspace_created','investigation.workspace_handed_off','investigation.workspace_closed','investigation.workspace_reopened') ORDER BY occurred_at DESC,id DESC LIMIT $4`, tenantID, actorID, workspaceID, investigation.MaxWorkspaceHistoryItems+1)
	if err != nil {
		return nil, false, fmt.Errorf("read investigation workspace history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]investigation.WorkspaceHistoryItem, 0, investigation.MaxWorkspaceHistoryItems+1)
	for rows.Next() {
		var eventType, version string
		var item investigation.WorkspaceHistoryItem
		if err := rows.Scan(&eventType, &item.ActorIsCurrentOperator, &version, &item.Status, &item.OccurredAt); err != nil {
			return nil, false, err
		}
		item.Action = strings.TrimPrefix(eventType, "investigation.workspace_")
		if _, err := investigation.ParseWorkspaceVersion(version); err != nil || item.Status != "open" && item.Status != "closed" {
			return nil, false, fmt.Errorf("invalid persisted investigation workspace history")
		}
		item.Version, item.OccurredAt = version, item.OccurredAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(items) > investigation.MaxWorkspaceHistoryItems
	if truncated {
		items = items[:investigation.MaxWorkspaceHistoryItems]
	}
	return items, truncated, nil
}

func (r *InvestigationRepository) workspaceCurrentEvidence(ctx context.Context, tenantID, actorID, rootType, rootID string, access investigation.SearchAccess) (investigation.WorkspaceCurrentEvidence, error) {
	generatedAt := time.Now().UTC()
	search, err := r.Search(ctx, tenantID, actorID, investigation.SearchFilter{Query: rootID, QueryKind: "immutable_id", Limit: 20, Access: access})
	if err != nil {
		return investigation.WorkspaceCurrentEvidence{}, err
	}
	var root *investigation.SearchResult
	for index := range search.Results {
		if search.Results[index].RecordType == rootType && search.Results[index].RecordID == rootID {
			root = &search.Results[index]
			break
		}
	}
	if root == nil {
		return investigation.WorkspaceCurrentEvidence{Relationships: []investigation.Relationship{}, GeneratedAt: generatedAt, Available: false}, nil
	}
	related, err := r.Related(ctx, tenantID, actorID, investigation.RelationshipFilter{SourceType: rootType, SourceID: rootID, Limit: 20, Access: access})
	if err != nil {
		return investigation.WorkspaceCurrentEvidence{}, err
	}
	return investigation.WorkspaceCurrentEvidence{Root: root, Relationships: related.Relationships, GeneratedAt: related.GeneratedAt, Truncated: related.Truncated, Available: true}, nil
}

func (r *InvestigationRepository) HandoffWorkspace(ctx context.Context, command investigation.WorkspaceHandoff) (investigation.WorkspaceReceipt, error) {
	target, err := investigation.NormalizeWorkspaceSubject(command.TargetSubjectID)
	if err != nil || target == command.ActorID || !command.Access.Any() || !canonicalWorkspaceID.MatchString(command.InvestigationID) || command.ExpectedVersion < 1 || strings.TrimSpace(command.TenantID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.CorrelationID) == "" {
		return investigation.WorkspaceReceipt{}, investigation.ErrInvalidWorkspace
	}
	return r.mutateWorkspace(ctx, command.TenantID, command.ActorID, command.InvestigationID, command.ExpectedVersion, command.CorrelationID, workspaceTime(command.OccurredAt), "investigation.workspace_handed_off", "open", target, command.Access)
}

func (r *InvestigationRepository) ChangeWorkspaceStatus(ctx context.Context, command investigation.WorkspaceStatusChange) (investigation.WorkspaceReceipt, error) {
	if command.TargetStatus != "open" && command.TargetStatus != "closed" || !command.Access.Any() || !canonicalWorkspaceID.MatchString(command.InvestigationID) || command.ExpectedVersion < 1 || strings.TrimSpace(command.TenantID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.CorrelationID) == "" {
		return investigation.WorkspaceReceipt{}, investigation.ErrInvalidWorkspace
	}
	eventType := "investigation.workspace_closed"
	if command.TargetStatus == "open" {
		eventType = "investigation.workspace_reopened"
	}
	return r.mutateWorkspace(ctx, command.TenantID, command.ActorID, command.InvestigationID, command.ExpectedVersion, command.CorrelationID, workspaceTime(command.OccurredAt), eventType, command.TargetStatus, "", command.Access)
}

func (r *InvestigationRepository) mutateWorkspace(ctx context.Context, tenantID, actorID, workspaceID string, expectedVersion int64, correlationID string, when time.Time, eventType, targetStatus, targetOwner string, access investigation.SearchAccess) (investigation.WorkspaceReceipt, error) {
	auditID, err := newUUID()
	if err != nil {
		return investigation.WorkspaceReceipt{}, err
	}
	receipt := investigation.WorkspaceReceipt{InvestigationID: workspaceID, Outcome: strings.TrimPrefix(eventType, "investigation.workspace_"), OccurredAt: when}
	err = WithSerializableSequence(ctx, r.database, "investigation-workspace:"+tenantID+":"+workspaceID, 3, func(tx *sql.Tx) error {
		var taxonomy, currentStatus, rootType, rootID string
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT taxonomy,status,version,root_record_type,root_record_id::text FROM investigation_workspaces WHERE tenant_id=$1 AND owner_subject_id=$2 AND id=$3 FOR UPDATE`, tenantID, actorID, workspaceID).Scan(&taxonomy, &currentStatus, &version, &rootType, &rootID)
		if errors.Is(err, sql.ErrNoRows) || !investigation.WorkspaceRecordAllowed(rootType, access) {
			return investigation.ErrWorkspaceNotFound
		}
		if err != nil {
			return err
		}
		authorized, err := authorizedRelationshipSourceWithQuerier(ctx, tx, tenantID, actorID, rootType, rootID)
		if err != nil {
			return err
		}
		if !authorized {
			return investigation.ErrWorkspaceNotFound
		}
		if version != expectedVersion {
			return investigation.ErrWorkspaceVersion
		}
		requiredStatus := "open"
		if eventType != "investigation.workspace_handed_off" && targetStatus == "open" {
			requiredStatus = "closed"
		}
		if currentStatus != requiredStatus {
			return investigation.ErrWorkspaceState
		}
		if eventType == "investigation.workspace_handed_off" {
			err := tx.QueryRowContext(ctx, `UPDATE investigation_workspaces SET owner_subject_id=$5,version=version+1,updated_at=$6 WHERE tenant_id=$1 AND owner_subject_id=$2 AND id=$3 AND version=$4 AND status='open' RETURNING status,version`, tenantID, actorID, workspaceID, expectedVersion, targetOwner, when).Scan(&currentStatus, &version)
			if err != nil {
				return err
			}
		} else {
			var closedAt any = when
			if targetStatus == "open" {
				closedAt = nil
			}
			err := tx.QueryRowContext(ctx, `UPDATE investigation_workspaces SET status=$5,closed_at=$6,version=version+1,updated_at=$7 WHERE tenant_id=$1 AND owner_subject_id=$2 AND id=$3 AND version=$4 AND status=$8 RETURNING status,version`, tenantID, actorID, workspaceID, expectedVersion, targetStatus, closedAt, when, requiredStatus).Scan(&currentStatus, &version)
			if err != nil {
				return err
			}
		}
		receipt.Version = strconv.FormatInt(version, 10)
		return insertWorkspaceAudit(ctx, tx, auditID, tenantID, actorID, eventType, workspaceID, correlationID, taxonomy, currentStatus, version, 0, when)
	})
	if err != nil {
		return investigation.WorkspaceReceipt{}, mapWorkspaceWriteError("change investigation workspace", err)
	}
	return receipt, nil
}

func insertWorkspaceAudit(ctx context.Context, tx *sql.Tx, auditID, tenantID, actorID, eventType, workspaceID, correlationID, taxonomy, status string, version int64, capturedCount int, occurredAt time.Time) error {
	metadata := map[string]string{"taxonomy": taxonomy, "status": status, "workspace_version": strconv.FormatInt(version, 10)}
	if eventType == "investigation.workspace_created" {
		metadata["captured_reference_count"] = strconv.Itoa(capturedCount)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return appendControlledAuditPayload(ctx, tx, auditID, AuditEvent{
		TenantID: tenantID, ActorSubjectID: actorID, EventType: eventType,
		TargetType: "investigation_workspace", TargetID: workspaceID, Outcome: "succeeded",
		CorrelationID: correlationID, OccurredAt: occurredAt,
	}, encoded)
}

func workspaceTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func mapWorkspaceWriteError(operation string, err error) error {
	if err == nil || errors.Is(err, investigation.ErrWorkspaceNotFound) || errors.Is(err, investigation.ErrWorkspaceVersion) || errors.Is(err, investigation.ErrWorkspaceLimit) || errors.Is(err, investigation.ErrWorkspaceState) || errors.Is(err, investigation.ErrInvalidWorkspace) {
		return err
	}
	return fmt.Errorf("%s: %w", operation, err)
}

var _ investigation.WorkspaceRepository = (*InvestigationRepository)(nil)
