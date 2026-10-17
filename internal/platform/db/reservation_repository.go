package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	hold "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reservations"
)

type ReservationRepository struct{ database *sql.DB }

func NewReservationRepository(database *sql.DB) (*ReservationRepository, error) {
	if database == nil {
		return nil, hold.ErrInvalid
	}
	return &ReservationRepository{database}, nil
}
func reservationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return hold.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch {
		case pg.Code == "42501":
			return hold.ErrForbidden
		case pg.Code == "P0002":
			return hold.ErrNotFound
		case pg.Code == "23505" || pg.Code == "55000" || pg.Code == "40001" || pg.Code == "40P01":
			return hold.ErrConflict
		case pg.Code == "54000":
			return hold.ErrLimit
		case strings.HasPrefix(pg.Code, "22"):
			return hold.ErrInvalid
		}
	}
	return fmt.Errorf("%w: %w", hold.ErrUnavailable, err)
}
func (r *ReservationRepository) command(ctx context.Context, m hold.Meta, query string, args ...any) (result hold.Receipt, err error) {
	err = WithTenantSerializableSequence(ctx, r.database, TenantScope{m.TenantID, m.ActorSubjectID}, "transfer-policy|"+m.TenantID, 5, func(tx *sql.Tx) error {
		var raw []byte
		if e := tx.QueryRowContext(ctx, query, args...).Scan(&raw); e != nil {
			return e
		}
		return json.Unmarshal(raw, &result)
	})
	return result, reservationError(err)
}
func (r *ReservationRepository) Create(ctx context.Context, c hold.Create) (hold.Receipt, error) {
	return r.command(ctx, c.Meta, `SELECT public.controlled_create_hold_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`, c.TenantID, c.ActorSubjectID, c.IdempotencyKey, c.CorrelationID, c.DebitAccountID, c.CreditAccountID, c.AmountMinor, c.Currency, c.TTLSeconds)
}
func (r *ReservationRepository) Decide(ctx context.Context, c hold.Decide) (hold.Receipt, error) {
	return r.command(ctx, c.Meta, `SELECT public.controlled_decide_hold_v1($1,$2,$3,$4,$5,$6)`, c.TenantID, c.ActorSubjectID, c.IdempotencyKey, c.CorrelationID, c.HoldID, c.Decision)
}

const holdReadColumns = `h.id,h.debit_account_id,h.credit_account_id,h.amount_minor::text,h.currency,h.state,h.version::text,h.created_by,h.created_at,h.expires_at,h.completed_at,COALESCE(h.transfer_id::text,'')`
const holdReadVisibility = `EXISTS(SELECT 1 FROM account_owners o WHERE o.tenant_id=h.tenant_id AND o.subject_id=$2 AND o.account_id IN (h.debit_account_id,h.credit_account_id) AND o.permission IN ('read','debit'))`

func scanReservation(row interface{ Scan(...any) error }) (result hold.Hold, err error) {
	err = row.Scan(&result.ID, &result.DebitAccountID, &result.CreditAccountID, &result.AmountMinor, &result.Currency, &result.State, &result.Version, &result.CreatedBy, &result.CreatedAt, &result.ExpiresAt, &result.CompletedAt, &result.TransferID)
	return result, reservationError(err)
}
func (r *ReservationRepository) Read(ctx context.Context, s hold.Scope, id string) (hold.Hold, error) {
	tx, err := beginTenantRead(ctx, r.database, TenantScope{s.TenantID, s.ActorSubjectID})
	if err != nil {
		return hold.Hold{}, reservationError(err)
	}
	defer func() { _ = tx.Rollback() }()
	return scanReservation(tx.QueryRowContext(ctx, `SELECT `+holdReadColumns+` FROM account_holds h WHERE h.tenant_id=$1 AND `+holdReadVisibility+` AND h.id=$3`, s.TenantID, s.ActorSubjectID, id))
}
func (r *ReservationRepository) List(ctx context.Context, s hold.Scope, cursor string) (result hold.Page, err error) {
	result.Holds = []hold.Hold{}
	var before time.Time
	var id string
	if cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(cursor)
		parts := strings.Split(string(raw), "|")
		if e != nil || len(parts) != 2 || !hold.ValidID(parts[1]) {
			return result, hold.ErrInvalid
		}
		before, e = time.Parse(time.RFC3339Nano, parts[0])
		if e != nil || before.IsZero() {
			return result, hold.ErrInvalid
		}
		id = parts[1]
	}
	tx, err := beginTenantRead(ctx, r.database, TenantScope{s.TenantID, s.ActorSubjectID})
	if err != nil {
		return result, reservationError(err)
	}
	defer func() { _ = tx.Rollback() }()
	query := `SELECT ` + holdReadColumns + ` FROM account_holds h WHERE h.tenant_id=$1 AND ` + holdReadVisibility
	args := []any{s.TenantID, s.ActorSubjectID}
	if cursor != "" {
		query += ` AND (h.created_at,h.id)<($3,$4)`
		args = append(args, before, id)
	}
	query += ` ORDER BY h.created_at DESC,h.id DESC LIMIT ` + strconv.Itoa(hold.PageSize+1)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return result, reservationError(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		item, e := scanReservation(rows)
		if e != nil {
			return result, e
		}
		result.Holds = append(result.Holds, item)
	}
	if err = rows.Err(); err != nil {
		return result, reservationError(err)
	}
	if len(result.Holds) > hold.PageSize {
		last := result.Holds[hold.PageSize-1]
		result.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID))
		result.Holds = result.Holds[:hold.PageSize]
	}
	return result, nil
}

// ReservationExpiryWorker discovers only bounded leased jobs through the queue
// capability. Each financial completion uses an explicit scoped transaction.
type ReservationExpiryWorker struct {
	database *sql.DB
	owner    string
	batch    int
	OnItem   func(string)
}

func NewReservationExpiryWorker(database *sql.DB, owner string, batch int) (*ReservationExpiryWorker, error) {
	if database == nil || strings.TrimSpace(owner) == "" || len(owner) > 256 || batch < 1 || batch > 100 {
		return nil, hold.ErrInvalid
	}
	return &ReservationExpiryWorker{database: database, owner: owner, batch: batch}, nil
}
func (w *ReservationExpiryWorker) RunOnce(ctx context.Context) (int, error) {
	rows, err := w.database.QueryContext(ctx, `SELECT hold_id,tenant_id,generation FROM public.controlled_claim_hold_expiry_v1($1,$2,30)`, w.batch, w.owner)
	if err != nil {
		return 0, err
	}
	type claim struct {
		id, tenant string
		generation int64
	}
	var claims []claim
	for rows.Next() {
		var c claim
		if err = rows.Scan(&c.id, &c.tenant, &c.generation); err != nil {
			_ = rows.Close()
			return 0, err
		}
		claims = append(claims, c)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	var failures []error
	completed := 0
	for _, c := range claims {
		if w.OnItem != nil {
			w.OnItem(c.id)
		}
		err = WithTenantSerializableSequence(ctx, w.database, TenantScope{c.tenant, "system:hold-expiry"}, "transfer-policy|"+c.tenant, 5, func(tx *sql.Tx) error {
			var changed bool
			return tx.QueryRowContext(ctx, `SELECT public.controlled_expire_hold_v1($1,$2,$3,$4)`, c.tenant, c.id, w.owner, c.generation).Scan(&changed)
		})
		if err != nil {
			failures = append(failures, err)
		} else {
			completed++
		}
	}
	return completed, errors.Join(failures...)
}
