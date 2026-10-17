package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	statement "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/statementreconciliation"
)

type StatementReconciliationRepository struct{ database *sql.DB }

func NewStatementReconciliationRepository(database *sql.DB) (*StatementReconciliationRepository, error) {
	if database == nil {
		return nil, statement.ErrInvalid
	}
	return &StatementReconciliationRepository{database}, nil
}
func statementScope(scope statement.Scope) TenantScope {
	return TenantScope{TenantID: scope.TenantID, ActorID: scope.ActorSubjectID}
}
func statementError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return statement.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch {
		case pg.Code == "42501":
			return statement.ErrForbidden
		case pg.Code == "P0002":
			return statement.ErrNotFound
		case pg.Code == "23505" || pg.Code == "40001" || pg.Code == "40P01":
			return statement.ErrConflict
		case pg.Code == "54000":
			return statement.ErrLimit
		case strings.HasPrefix(pg.Code, "22"):
			return statement.ErrInvalid
		}
	}
	return fmt.Errorf("%w: %w", statement.ErrUnavailable, err)
}
func (r *StatementReconciliationRepository) command(ctx context.Context, scope statement.Scope, query string, args ...any) (receipt statement.Receipt, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, statementScope(scope), "statement|"+scope.TenantID, 5, func(tx *sql.Tx) error {
		var raw []byte
		if queryErr := tx.QueryRowContext(ctx, query, args...).Scan(&raw); queryErr != nil {
			return queryErr
		}
		return json.Unmarshal(raw, &receipt)
	})
	return receipt, statementError(err)
}
func (r *StatementReconciliationRepository) CreateMapping(ctx context.Context, c statement.MappingCommand) (statement.Receipt, error) {
	return r.command(ctx, c.Scope, `SELECT public.controlled_statement_mapping_v1($1,$2,$3,$4,$5,$6)`, c.TenantID, c.ActorSubjectID, c.IdempotencyKey, c.CorrelationID, c.AccountID, c.SourceAccountRef)
}
func (r *StatementReconciliationRepository) AcceptImport(ctx context.Context, c statement.ImportCommand) (statement.Receipt, error) {
	return r.command(ctx, c.Scope, `SELECT public.controlled_statement_import_v1($1,$2,$3,$4,$5,$6)`, c.TenantID, c.ActorSubjectID, c.IdempotencyKey, c.CorrelationID, c.MappingID, c.Source)
}
func (r *StatementReconciliationRepository) PrepareRun(ctx context.Context, c statement.RunCommand) (statement.Receipt, error) {
	return r.command(ctx, c.Scope, `SELECT public.controlled_statement_prepare_v1($1,$2,$3,$4,$5)`, c.TenantID, c.ActorSubjectID, c.IdempotencyKey, c.CorrelationID, c.ImportID)
}
func (r *StatementReconciliationRepository) CompleteRun(ctx context.Context, scope statement.Scope, id string, result statement.MatchResult) (statement.Receipt, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return statement.Receipt{}, err
	}
	return r.command(ctx, scope, `SELECT public.controlled_statement_complete_v1($1,$2,$3,$4)`, scope.TenantID, scope.ActorSubjectID, id, raw)
}
func (r *StatementReconciliationRepository) Review(ctx context.Context, c statement.ReviewCommand) (statement.Receipt, error) {
	raw, err := json.Marshal(c.Decision)
	if err != nil {
		return statement.Receipt{}, err
	}
	return r.command(ctx, c.Scope, `SELECT public.controlled_statement_review_v1($1,$2,$3,$4,$5,$6)`, c.TenantID, c.ActorSubjectID, c.IdempotencyKey, c.CorrelationID, c.RunID, raw)
}
