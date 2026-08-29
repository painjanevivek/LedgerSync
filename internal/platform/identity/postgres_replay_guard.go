package identity

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const replayCleanupBatchSize = 256

// PostgresReplayGuard shares BFF assertion replay protection across every API
// process. It retains only a SHA-256 digest of the assertion ID until expiry;
// this is enough to enforce uniqueness without persisting the signed value.
type PostgresReplayGuard struct {
	database *sql.DB
	now      func() time.Time
}

func NewPostgresReplayGuard(database *sql.DB, clock func() time.Time) (*PostgresReplayGuard, error) {
	if database == nil {
		return nil, errors.New("actor assertion replay database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &PostgresReplayGuard{database: database, now: clock}, nil
}

func (g *PostgresReplayGuard) Use(ctx context.Context, assertionID string, expiresAt time.Time) error {
	if g == nil || g.database == nil || strings.TrimSpace(assertionID) == "" {
		return errAssertionReplay
	}
	now := g.now().UTC()
	expiresAt = expiresAt.UTC()
	if !expiresAt.After(now) {
		return errAssertionReplay
	}
	// Cleanup is deliberately bounded. A replay record's maximum lifetime is
	// separately capped by ActorAssertionConfig, so this table cannot become a
	// long-lived assertion archive.
	if _, err := g.database.ExecContext(ctx, `DELETE FROM bff_actor_assertion_replays WHERE assertion_digest IN (SELECT assertion_digest FROM bff_actor_assertion_replays WHERE expires_at <= $1 ORDER BY expires_at ASC LIMIT $2)`, now, replayCleanupBatchSize); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(assertionID))
	result, err := g.database.ExecContext(ctx, `INSERT INTO bff_actor_assertion_replays(assertion_digest,expires_at,created_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, digest[:], expiresAt, now)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return errAssertionReplay
	}
	return nil
}
