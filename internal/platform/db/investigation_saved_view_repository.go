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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
)

var canonicalSavedViewID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (r *InvestigationRepository) ListSavedViews(ctx context.Context, tenantID, actorID string, access investigation.SavedViewAccess) (investigation.SavedViewPage, error) {
	if r == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" || !access.AnyReadableDomain() {
		return investigation.SavedViewPage{}, investigation.ErrInvalidSavedView
	}
	rows, err := r.database.QueryContext(ctx, `SELECT id::text,name,filter_schema_version,domain,filters,version,created_at,updated_at FROM investigation_saved_views WHERE tenant_id=$1 AND owner_subject_id=$2 ORDER BY updated_at DESC,id DESC LIMIT $3`, tenantID, actorID, investigation.MaxSavedViewsPerOperator)
	if err != nil {
		return investigation.SavedViewPage{}, fmt.Errorf("list saved investigation views: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := investigation.SavedViewPage{Views: make([]investigation.SavedView, 0), GeneratedAt: time.Now().UTC()}
	for rows.Next() {
		view, err := scanSavedView(rows)
		if err != nil {
			return investigation.SavedViewPage{}, err
		}
		if investigation.SavedViewDefinitionAllowed(view.Domain, view.Filters, access) {
			page.Views = append(page.Views, view)
		}
	}
	if err := rows.Err(); err != nil {
		return investigation.SavedViewPage{}, fmt.Errorf("iterate saved investigation views: %w", err)
	}
	return page, nil
}

func (r *InvestigationRepository) CreateSavedView(ctx context.Context, command investigation.SavedViewCreate) (investigation.SavedView, error) {
	name, err := investigation.NormalizeSavedViewName(command.Name)
	if err != nil || strings.TrimSpace(command.TenantID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.CorrelationID) == "" {
		return investigation.SavedView{}, investigation.ErrInvalidSavedView
	}
	filters, targetPath, err := investigation.NormalizeSavedViewDefinition(command.Domain, command.FilterSchemaVersion, command.Filters)
	if err != nil {
		return investigation.SavedView{}, err
	}
	if !investigation.SavedViewDefinitionAllowed(command.Domain, filters, command.Access) {
		return investigation.SavedView{}, investigation.ErrInvalidSavedView
	}
	encoded, err := json.Marshal(filters)
	if err != nil {
		return investigation.SavedView{}, investigation.ErrInvalidSavedView
	}
	viewID, err := newUUID()
	if err != nil {
		return investigation.SavedView{}, err
	}
	auditID, err := newUUID()
	if err != nil {
		return investigation.SavedView{}, err
	}
	when := savedViewTime(command.OccurredAt)
	var created investigation.SavedView
	err = WithSerializableSequence(ctx, r.database, "investigation-saved-views:"+command.TenantID+":"+command.ActorID, 3, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM investigation_saved_views WHERE tenant_id=$1 AND owner_subject_id=$2`, command.TenantID, command.ActorID).Scan(&count); err != nil {
			return err
		}
		if count >= investigation.MaxSavedViewsPerOperator {
			return investigation.ErrSavedViewLimit
		}
		var version int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO investigation_saved_views(id,tenant_id,owner_subject_id,name,filter_schema_version,domain,filters,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$8) RETURNING version,created_at,updated_at`, viewID, command.TenantID, command.ActorID, name, command.FilterSchemaVersion, command.Domain, encoded, when).Scan(&version, &created.CreatedAt, &created.UpdatedAt); err != nil {
			return err
		}
		if err := insertSavedViewAudit(ctx, tx, auditID, command.TenantID, command.ActorID, "investigation.saved_view_created", viewID, command.CorrelationID, command.Domain, command.FilterSchemaVersion, version, when); err != nil {
			return err
		}
		created.ID, created.Name, created.FilterSchemaVersion, created.Domain, created.Filters, created.TargetPath, created.Version = viewID, name, strconv.Itoa(command.FilterSchemaVersion), command.Domain, filters, targetPath, strconv.FormatInt(version, 10)
		return nil
	})
	if err != nil {
		return investigation.SavedView{}, mapSavedViewWriteError("create saved investigation view", err)
	}
	created.CreatedAt, created.UpdatedAt = created.CreatedAt.UTC(), created.UpdatedAt.UTC()
	return created, nil
}

func (r *InvestigationRepository) RenameSavedView(ctx context.Context, command investigation.SavedViewRename) (investigation.SavedView, error) {
	name, err := investigation.NormalizeSavedViewName(command.Name)
	if err != nil || !canonicalSavedViewID.MatchString(command.SavedViewID) || command.ExpectedVersion < 1 || strings.TrimSpace(command.TenantID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.CorrelationID) == "" {
		return investigation.SavedView{}, investigation.ErrInvalidSavedView
	}
	auditID, err := newUUID()
	if err != nil {
		return investigation.SavedView{}, err
	}
	when := savedViewTime(command.OccurredAt)
	var updated investigation.SavedView
	err = WithSerializableSequence(ctx, r.database, "investigation-saved-views:"+command.TenantID+":"+command.ActorID, 3, func(tx *sql.Tx) error {
		var schemaVersion int
		var rawFilters []byte
		var version int64
		err := tx.QueryRowContext(ctx, `UPDATE investigation_saved_views SET name=$4,version=version+1,updated_at=$5 WHERE tenant_id=$1 AND owner_subject_id=$2 AND id=$3 AND version=$6 RETURNING id::text,name,filter_schema_version,domain,filters,version,created_at,updated_at`, command.TenantID, command.ActorID, command.SavedViewID, name, when, command.ExpectedVersion).Scan(&updated.ID, &updated.Name, &schemaVersion, &updated.Domain, &rawFilters, &version, &updated.CreatedAt, &updated.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return savedViewMiss(ctx, tx, command.TenantID, command.ActorID, command.SavedViewID)
		}
		if err != nil {
			return err
		}
		filters, targetPath, err := decodeSavedViewDefinition(updated.Domain, schemaVersion, rawFilters)
		if err != nil {
			return err
		}
		if !investigation.SavedViewDefinitionAllowed(updated.Domain, filters, command.Access) {
			return investigation.ErrSavedViewNotFound
		}
		updated.FilterSchemaVersion, updated.Filters, updated.TargetPath, updated.Version = strconv.Itoa(schemaVersion), filters, targetPath, strconv.FormatInt(version, 10)
		return insertSavedViewAudit(ctx, tx, auditID, command.TenantID, command.ActorID, "investigation.saved_view_renamed", command.SavedViewID, command.CorrelationID, updated.Domain, schemaVersion, version, when)
	})
	if err != nil {
		return investigation.SavedView{}, mapSavedViewWriteError("rename saved investigation view", err)
	}
	updated.CreatedAt, updated.UpdatedAt = updated.CreatedAt.UTC(), updated.UpdatedAt.UTC()
	return updated, nil
}

func (r *InvestigationRepository) DeleteSavedView(ctx context.Context, command investigation.SavedViewDelete) error {
	if !canonicalSavedViewID.MatchString(command.SavedViewID) || command.ExpectedVersion < 1 || strings.TrimSpace(command.TenantID) == "" || strings.TrimSpace(command.ActorID) == "" || strings.TrimSpace(command.CorrelationID) == "" {
		return investigation.ErrInvalidSavedView
	}
	auditID, err := newUUID()
	if err != nil {
		return err
	}
	when := savedViewTime(command.OccurredAt)
	err = WithSerializableSequence(ctx, r.database, "investigation-saved-views:"+command.TenantID+":"+command.ActorID, 3, func(tx *sql.Tx) error {
		var domain string
		var schemaVersion int
		var rawFilters []byte
		err := tx.QueryRowContext(ctx, `DELETE FROM investigation_saved_views WHERE tenant_id=$1 AND owner_subject_id=$2 AND id=$3 AND version=$4 RETURNING domain,filter_schema_version,filters`, command.TenantID, command.ActorID, command.SavedViewID, command.ExpectedVersion).Scan(&domain, &schemaVersion, &rawFilters)
		if errors.Is(err, sql.ErrNoRows) {
			return savedViewMiss(ctx, tx, command.TenantID, command.ActorID, command.SavedViewID)
		}
		if err != nil {
			return err
		}
		filters, _, err := decodeSavedViewDefinition(domain, schemaVersion, rawFilters)
		if err != nil {
			return err
		}
		if !investigation.SavedViewDefinitionAllowed(domain, filters, command.Access) {
			return investigation.ErrSavedViewNotFound
		}
		return insertSavedViewAudit(ctx, tx, auditID, command.TenantID, command.ActorID, "investigation.saved_view_deleted", command.SavedViewID, command.CorrelationID, domain, schemaVersion, command.ExpectedVersion, when)
	})
	return mapSavedViewWriteError("delete saved investigation view", err)
}

type savedViewScanner interface{ Scan(...any) error }

func scanSavedView(scanner savedViewScanner) (investigation.SavedView, error) {
	var view investigation.SavedView
	var schemaVersion int
	var version int64
	var rawFilters []byte
	if err := scanner.Scan(&view.ID, &view.Name, &schemaVersion, &view.Domain, &rawFilters, &version, &view.CreatedAt, &view.UpdatedAt); err != nil {
		return investigation.SavedView{}, fmt.Errorf("scan saved investigation view: %w", err)
	}
	filters, targetPath, err := decodeSavedViewDefinition(view.Domain, schemaVersion, rawFilters)
	if err != nil {
		return investigation.SavedView{}, err
	}
	view.FilterSchemaVersion, view.Filters, view.TargetPath, view.Version = strconv.Itoa(schemaVersion), filters, targetPath, strconv.FormatInt(version, 10)
	view.CreatedAt, view.UpdatedAt = view.CreatedAt.UTC(), view.UpdatedAt.UTC()
	return view, nil
}

func decodeSavedViewDefinition(domain string, schemaVersion int, raw []byte) (map[string]string, string, error) {
	var filters map[string]string
	if err := json.Unmarshal(raw, &filters); err != nil {
		return nil, "", fmt.Errorf("decode saved investigation view filters: %w", err)
	}
	filters, targetPath, err := investigation.NormalizeSavedViewDefinition(domain, schemaVersion, filters)
	if err != nil {
		return nil, "", fmt.Errorf("validate persisted saved investigation view: %w", err)
	}
	return filters, targetPath, nil
}

func savedViewMiss(ctx context.Context, tx *sql.Tx, tenantID, actorID, viewID string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM investigation_saved_views WHERE tenant_id=$1 AND owner_subject_id=$2 AND id=$3)`, tenantID, actorID, viewID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return investigation.ErrSavedViewVersion
	}
	return investigation.ErrSavedViewNotFound
}

func insertSavedViewAudit(ctx context.Context, tx *sql.Tx, auditID, tenantID, actorID, eventType, viewID, correlationID, domain string, schemaVersion int, version int64, occurredAt time.Time) error {
	metadata, err := json.Marshal(map[string]string{"domain": domain, "filter_schema_version": strconv.Itoa(schemaVersion), "view_version": strconv.FormatInt(version, 10)})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES($1,$2,$3,$4,'investigation_saved_view',$5,'succeeded',$6,$7,$8)`, auditID, tenantID, actorID, eventType, viewID, correlationID, metadata, occurredAt)
	return err
}

func savedViewTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func mapSavedViewWriteError(operation string, err error) error {
	if err == nil || errors.Is(err, investigation.ErrSavedViewLimit) || errors.Is(err, investigation.ErrSavedViewVersion) || errors.Is(err, investigation.ErrSavedViewNotFound) || errors.Is(err, investigation.ErrInvalidSavedView) {
		return err
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return investigation.ErrSavedViewConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

var _ investigation.SavedViewRepository = (*InvestigationRepository)(nil)
