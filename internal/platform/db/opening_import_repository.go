package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/openingimports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
)

type OpeningImportRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewOpeningImportRepository(database *sql.DB, clock func() time.Time) (*OpeningImportRepository, error) {
	if database == nil {
		return nil, errors.New("opening import database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &OpeningImportRepository{database: database, clock: clock}, nil
}

func (repository *OpeningImportRepository) Request(ctx context.Context, manifest openingimports.PreparedManifest, actor, correlation string) (openingimports.Result, error) {
	correlationID, err := validateOpeningImportEnvelope(ctx, actor, correlation)
	if err != nil {
		return openingimports.Result{}, err
	}
	var replayed, conflicted bool
	err = repository.database.QueryRowContext(ctx, `
SELECT replayed,conflicted FROM public.controlled_request_opening_import_v1(
  $1,$2,$3,$4,$5,$6,$7,$8,$9
)`, manifest.TenantID, actor, manifest.BatchID, manifest.Currency, manifest.AccountIDs(),
		manifest.OpeningMinors(), manifest.ContentHash[:], correlationID, repository.clock().UTC()).Scan(&replayed, &conflicted)
	return openingImportResult(replayed, conflicted, err)
}

func (repository *OpeningImportRepository) Approve(ctx context.Context, manifest openingimports.PreparedManifest, actor, correlation string) (openingimports.Result, error) {
	return repository.decide(ctx, "controlled_approve_opening_import_v1", manifest, actor, correlation)
}

func (repository *OpeningImportRepository) Execute(ctx context.Context, manifest openingimports.PreparedManifest, actor, correlation string) (openingimports.Result, error) {
	return repository.decide(ctx, "controlled_execute_opening_import_v1", manifest, actor, correlation)
}

func (repository *OpeningImportRepository) decide(ctx context.Context, function string, manifest openingimports.PreparedManifest, actor, correlation string) (openingimports.Result, error) {
	correlationID, err := validateOpeningImportEnvelope(ctx, actor, correlation)
	if err != nil {
		return openingimports.Result{}, err
	}
	if function != "controlled_approve_opening_import_v1" && function != "controlled_execute_opening_import_v1" {
		return openingimports.Result{}, openingimports.ErrInvalid
	}
	var replayed, conflicted bool
	query := `SELECT replayed,conflicted FROM public.` + function + `($1,$2,$3,$4,$5,$6)`
	err = repository.database.QueryRowContext(ctx, query, manifest.TenantID, actor, manifest.BatchID,
		manifest.ContentHash[:], correlationID, repository.clock().UTC()).Scan(&replayed, &conflicted)
	return openingImportResult(replayed, conflicted, err)
}

func validateOpeningImportEnvelope(ctx context.Context, actor, correlation string) (string, error) {
	if actor == "" || strings.TrimSpace(actor) != actor {
		return "", openingimports.ErrInvalid
	}
	correlationID, err := identifier.Canonicalize(ctx, identifier.KindEvent, correlation)
	if err != nil {
		return "", openingimports.ErrInvalid
	}
	return correlationID, nil
}

func openingImportResult(replayed, conflicted bool, err error) (openingimports.Result, error) {
	if err != nil {
		return openingimports.Result{}, classifyOpeningImportError(err)
	}
	if conflicted {
		return openingimports.Result{}, openingimports.ErrConflict
	}
	return openingimports.Result{Replayed: replayed}, nil
}

func classifyOpeningImportError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return fmt.Errorf("execute controlled opening import: %w", err)
	}
	switch postgresError.ConstraintName {
	case "opening_import_input", "opening_import_rows", "opening_import_hash", "opening_import_total":
		return openingimports.ErrInvalid
	case "opening_import_caller", "opening_import_actor", "opening_import_dual_control":
		return openingimports.ErrForbidden
	case "opening_import_not_found":
		return openingimports.ErrNotFound
	case "opening_import_account_state", "opening_import_approval", "opening_import_reconciliation", "opening_import_partial":
		return openingimports.ErrConflict
	default:
		return fmt.Errorf("execute controlled opening import: %w", err)
	}
}
