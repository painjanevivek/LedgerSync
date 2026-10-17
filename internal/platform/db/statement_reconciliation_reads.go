package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	statement "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/statementreconciliation"
)

func (r *StatementReconciliationRepository) Workspace(ctx context.Context, scope statement.Scope) (result statement.Workspace, err error) {
	result.Imports = []statement.ImportSummary{}
	result.Accounts = []statement.AccountOption{}
	tx, err := beginTenantRead(ctx, r.database, statementScope(scope))
	if err != nil {
		return result, statementError(err)
	}
	defer func() { _ = tx.Rollback() }()
	var mapping statement.Mapping
	err = tx.QueryRowContext(ctx, `SELECT id,account_id,source_account_ref,currency,version,created_by,created_at FROM statement_mappings WHERE tenant_id=$1`, scope.TenantID).Scan(&mapping.ID, &mapping.AccountID, &mapping.SourceAccountRef, &mapping.Currency, &mapping.Version, &mapping.CreatedBy, &mapping.CreatedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, statementError(err)
	}
	if err == nil {
		result.Mapping = &mapping
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.id,i.mapping_id,encode(i.source_sha256,'hex'),i.row_count,i.period_start,i.period_end,i.opening_minor,i.closing_minor,i.created_at,COALESCE(r.id::text,''),COALESCE(r.state,'') FROM statement_imports i LEFT JOIN statement_runs r ON r.import_id=i.id WHERE i.tenant_id=$1 ORDER BY i.created_at DESC,i.id DESC LIMIT 101`, scope.TenantID)
	if err != nil {
		return result, statementError(err)
	}
	for rows.Next() {
		var item statement.ImportSummary
		if err = rows.Scan(&item.ID, &item.MappingID, &item.SHA256, &item.RowCount, &item.PeriodStart, &item.PeriodEnd, &item.OpeningMinor, &item.ClosingMinor, &item.CreatedAt, &item.RunID, &item.RunState); err != nil {
			_ = rows.Close()
			return result, statementError(err)
		}
		result.Imports = append(result.Imports, item)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, statementError(err)
	}
	if len(result.Imports) > statement.MaxImports {
		return result, statement.ErrLimit
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,COALESCE(display_name,''),account_kind FROM accounts WHERE tenant_id=$1 AND currency='INR' AND status='active' ORDER BY id LIMIT 101`, scope.TenantID)
	if err != nil {
		return result, statementError(err)
	}
	for rows.Next() {
		var item statement.AccountOption
		if err = rows.Scan(&item.ID, &item.DisplayName, &item.Kind); err != nil {
			_ = rows.Close()
			return result, statementError(err)
		}
		result.Accounts = append(result.Accounts, item)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, statementError(err)
	}
	if len(result.Accounts) > 100 {
		result.MoreAccounts = true
		result.Accounts = result.Accounts[:100]
	}
	return result, nil
}
func loadStatementHeader(ctx context.Context, tx *sql.Tx, tenant, id string) (h statement.RunHeader, err error) {
	err = tx.QueryRowContext(ctx, `SELECT r.id,r.import_id,r.mapping_id,m.account_id,m.source_account_ref,r.state,r.rule_version,m.version,r.cutoff,i.period_start,i.period_end,encode(i.source_sha256,'hex'),i.row_count,
 (SELECT count(*) FROM statement_snapshot_items s WHERE s.run_id=r.id),
 (SELECT count(*) FROM statement_matches x WHERE x.run_id=r.id),
 i.row_count-(SELECT count(*) FROM statement_matches x WHERE x.run_id=r.id),
 (SELECT count(*) FROM statement_snapshot_items s WHERE s.run_id=r.id)-(SELECT count(*) FROM statement_matches x WHERE x.run_id=r.id)
 FROM statement_runs r JOIN statement_imports i ON i.id=r.import_id JOIN statement_mappings m ON m.id=r.mapping_id WHERE r.tenant_id=$1 AND r.id=$2`, tenant, id).Scan(&h.ID, &h.ImportID, &h.MappingID, &h.AccountID, &h.SourceAccountRef, &h.State, &h.RuleVersion, &h.MappingVersion, &h.Cutoff, &h.PeriodStart, &h.PeriodEnd, &h.SourceSHA256, &h.StatementRowCount, &h.SnapshotItemCount, &h.MatchedRowCount, &h.UnmatchedStatementCount, &h.UnmatchedLedgerCount)
	return h, statementError(err)
}
func loadStatementReviews(ctx context.Context, tx *sql.Tx, id, kind, itemID string, limit int) ([]statement.ReviewItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT v.kind,v.item_id,v.version,v.classification,v.status,COALESCE(s.external_reference,p.external_reference),COALESCE(s.direction,p.direction),COALESCE(s.amount_minor,p.amount_minor),COALESCE(s.currency,p.currency),
 v.assignee,v.reason,v.comment,v.evidence,COALESCE(m.posting_id::text,''),v.updated_by,v.updated_at
 FROM statement_review_items v JOIN statement_runs r ON r.id=v.run_id
 LEFT JOIN statement_rows s ON v.kind='statement' AND s.import_id=r.import_id AND s.row_id=v.item_id
 LEFT JOIN statement_snapshot_items p ON v.kind='ledger' AND p.run_id=v.run_id AND p.posting_id::text=v.item_id
 LEFT JOIN statement_matches m ON m.run_id=v.run_id AND ((v.kind='statement' AND m.row_id=v.item_id) OR (v.kind='ledger' AND m.posting_id::text=v.item_id))
 WHERE v.run_id=$1 AND ($2='' OR (v.kind,v.item_id)>($2,$3)) ORDER BY v.kind,v.item_id LIMIT $4`, id, kind, itemID, limit)
	if err != nil {
		return nil, statementError(err)
	}
	defer func() { _ = rows.Close() }()
	result := []statement.ReviewItem{}
	for rows.Next() {
		var item statement.ReviewItem
		if err = rows.Scan(&item.Kind, &item.ItemID, &item.Version, &item.Classification, &item.Status, &item.Reference, &item.Direction, &item.AmountMinor, &item.Currency, &item.Assignee, &item.Reason, &item.Comment, &item.Evidence, &item.LinkedPostingID, &item.UpdatedBy, &item.UpdatedAt); err != nil {
			return nil, statementError(err)
		}
		result = append(result, item)
	}
	return result, statementError(rows.Err())
}
func (r *StatementReconciliationRepository) Snapshot(ctx context.Context, scope statement.Scope, id string) (result statement.RunSnapshot, err error) {
	tx, err := beginTenantRead(ctx, r.database, statementScope(scope))
	if err != nil {
		return result, statementError(err)
	}
	defer func() { _ = tx.Rollback() }()
	result.Header, err = loadStatementHeader(ctx, tx.Tx, scope.TenantID, id)
	if err != nil {
		return result, err
	}
	result.Rows = []statement.Row{}
	result.Items = []statement.LedgerItem{}
	rows, err := tx.QueryContext(ctx, `SELECT row_id,external_reference,direction,amount_minor,currency,posted_at FROM statement_rows WHERE import_id=$1 ORDER BY row_id LIMIT 1001`, result.Header.ImportID)
	if err != nil {
		return result, statementError(err)
	}
	for rows.Next() {
		var row statement.Row
		if err = rows.Scan(&row.ID, &row.Reference, &row.Direction, &row.AmountMinor, &row.Currency, &row.PostedAt); err != nil {
			_ = rows.Close()
			return result, statementError(err)
		}
		result.Rows = append(result.Rows, row)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, statementError(err)
	}
	rows, err = tx.QueryContext(ctx, `SELECT posting_id,external_reference,direction,amount_minor,currency FROM statement_snapshot_items WHERE run_id=$1 ORDER BY posting_id LIMIT 2001`, id)
	if err != nil {
		return result, statementError(err)
	}
	for rows.Next() {
		var item statement.LedgerItem
		if err = rows.Scan(&item.ID, &item.Reference, &item.Direction, &item.AmountMinor, &item.Currency); err != nil {
			_ = rows.Close()
			return result, statementError(err)
		}
		result.Items = append(result.Items, item)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, statementError(err)
	}
	result.Reviews, err = loadStatementReviews(ctx, tx.Tx, id, "", "", statement.MaxRows+statement.MaxSnapshotItems+1)
	if len(result.Rows) > statement.MaxRows || len(result.Items) > statement.MaxSnapshotItems || len(result.Reviews) > statement.MaxRows+statement.MaxSnapshotItems {
		return result, statement.ErrLimit
	}
	return result, err
}
func (r *StatementReconciliationRepository) Page(ctx context.Context, scope statement.Scope, id, cursor string) (result statement.RunPage, err error) {
	kind, itemID := "", ""
	if cursor != "" {
		raw, decodeErr := base64.RawURLEncoding.DecodeString(cursor)
		parts := strings.SplitN(string(raw), "\x00", 2)
		if decodeErr != nil || len(raw) > 256 || len(parts) != 2 || (parts[0] != "statement" && parts[0] != "ledger") || parts[1] == "" || strings.ContainsRune(parts[1], 0) {
			return result, statement.ErrInvalid
		}
		kind, itemID = parts[0], parts[1]
	}
	tx, err := beginTenantRead(ctx, r.database, statementScope(scope))
	if err != nil {
		return result, statementError(err)
	}
	defer func() { _ = tx.Rollback() }()
	result.Run, err = loadStatementHeader(ctx, tx.Tx, scope.TenantID, id)
	if err != nil {
		return result, err
	}
	result.Items, err = loadStatementReviews(ctx, tx.Tx, id, kind, itemID, statement.ReviewPageSize+1)
	if err != nil {
		return result, err
	}
	if len(result.Items) > statement.ReviewPageSize {
		last := result.Items[statement.ReviewPageSize-1]
		result.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.Kind + "\x00" + last.ItemID))
		result.Items = result.Items[:statement.ReviewPageSize]
	}
	return result, nil
}
func (r *StatementReconciliationRepository) Source(ctx context.Context, scope statement.Scope, id string) (result statement.SourceFile, err error) {
	tx, err := beginTenantRead(ctx, r.database, statementScope(scope))
	if err != nil {
		return result, statementError(err)
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.QueryRowContext(ctx, `SELECT id,encode(source_sha256,'hex'),source_bytes FROM statement_imports WHERE tenant_id=$1 AND id=$2`, scope.TenantID, id).Scan(&result.ID, &result.SHA256, &result.Bytes)
	if len(result.Bytes) > statement.MaxSourceBytes {
		return statement.SourceFile{}, statement.ErrLimit
	}
	if err == nil {
		digest := sha256.Sum256(result.Bytes)
		if hex.EncodeToString(digest[:]) != result.SHA256 {
			return statement.SourceFile{}, statement.ErrUnavailable
		}
	}
	return result, statementError(err)
}
