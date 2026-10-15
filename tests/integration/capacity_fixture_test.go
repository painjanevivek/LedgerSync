package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/redis/go-redis/v9"
)

const capacityTenant2 = "00000000-0000-4000-8000-000000009201"

type capacityPair struct{ Shape, Tenant, Source, Destination string }
type capacityFixture struct {
	admin, api, recon *sql.DB
	service           *transfers.Service
	balances          *db.BalanceRepository
	redis             *redis.Client
	control           string
	pairs             []capacityPair
	initial           map[string]int64
}

func capacityControlDirectory(t *testing.T) string { t.Helper(); return portfolioControlDirectory(t) }

func newCapacityFixture(t *testing.T, ctx context.Context) *capacityFixture {
	t.Helper()
	control := capacityControlDirectory(t)
	_, admin := requireTransferService(t, 1_000_000)
	admin.SetMaxOpenConns(2)
	requireWorkloadRoles(t, admin)
	const c = "00000000-0000-4000-8000-000000009210"
	const d = "00000000-0000-4000-8000-000000009220"
	const e = "00000000-0000-4000-8000-000000009230"
	const f = "00000000-0000-4000-8000-000000009240"
	if _, err := admin.ExecContext(ctx, `INSERT INTO tenants(id,external_reference) VALUES($1,'synthetic-capacity-second-tenant');`, capacityTenant2); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'operator')`, capacityTenant2, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, `INSERT INTO tenant_transfer_policies(tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD',1,1000000000,5000000000,5000000000,10000000000)`, capacityTenant2); err != nil {
		t.Fatal(err)
	}
	for _, account := range []struct {
		id, tenant string
		amount     int64
		debit      bool
	}{{c, testTenantID, 1_000_000, true}, {d, testTenantID, 0, false}, {e, capacityTenant2, 1_000_000, true}, {f, capacityTenant2, 0, false}} {
		if _, err := admin.ExecContext(ctx, `INSERT INTO accounts(id,tenant_id,currency,status) VALUES($1,$2,'USD','active')`, account.id, account.tenant); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.ExecContext(ctx, `INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version) VALUES($1,$2,$2,0)`, account.id, account.amount); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.ExecContext(ctx, `INSERT INTO account_opening_balances(account_id,opening_ledger_minor) VALUES($1,$2)`, account.id, account.amount); err != nil {
			t.Fatal(err)
		}
		if account.debit {
			if _, err := admin.ExecContext(ctx, `INSERT INTO account_owners(tenant_id,account_id,subject_id,permission) VALUES($1,$2,$3,'debit')`, account.tenant, account.id, testActorID); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := admin.ExecContext(ctx, `INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id) VALUES($1,$2,$3)`, account.tenant, account.id, testActorID); err != nil {
				t.Fatal(err)
			}
		}
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	api.db.SetMaxOpenConns(4)
	recon := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_reconciliation")
	recon.db.SetMaxOpenConns(2)
	repository, err := db.NewTransferRepository(api.db, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	service, err := transfers.NewService(repository, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	balances, err := db.NewBalanceRepository(api.db)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &capacityFixture{admin: admin, api: api.db, recon: recon.db, service: service, balances: balances, redis: client, control: control, pairs: []capacityPair{{"hot", testTenantID, testSourceID, testDestinationID}, {"independent-pair", testTenantID, c, d}, {"second-tenant", capacityTenant2, e, f}}, initial: map[string]int64{testSourceID: 1_000_000, testDestinationID: 2000, c: 1_000_000, d: 0, e: 1_000_000, f: 0}}
	fixture.writeControl(t, "capacity-ready.json", map[string]any{"pid": os.Getpid(), "synthetic": true, "accounts": 6})
	deadline := time.NewTimer(45 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(control, "worker-started.json")); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-deadline.C:
			t.Fatal("native worker did not become ready")
		case <-ticker.C:
		}
	}
	return fixture
}

func (c *capacityFixture) writeControl(t *testing.T, name string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(c.control, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(encoded)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("capacity control record could not be saved")
	}
}

type capacityQueueSample struct {
	At            string  `json:"at"`
	Pending       int64   `json:"pending"`
	OldestSeconds float64 `json:"oldest_seconds"`
	Dead          int64   `json:"dead"`
	Connections   int     `json:"connections"`
	LockWaiters   int     `json:"lock_waiters"`
	DatabaseBytes int64   `json:"database_bytes"`
}

func (c *capacityFixture) sample(ctx context.Context) (capacityQueueSample, error) {
	scoped, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	sample := capacityQueueSample{At: time.Now().UTC().Format(time.RFC3339Nano)}
	err := c.admin.QueryRowContext(scoped, `SELECT count(*) FILTER(WHERE published_at IS NULL AND dead_at IS NULL),COALESCE(extract(epoch FROM clock_timestamp()-min(occurred_at) FILTER(WHERE published_at IS NULL AND dead_at IS NULL)),0)::double precision,count(*) FILTER(WHERE dead_at IS NOT NULL),(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()),(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'),pg_database_size(current_database()) FROM outbox_events`).Scan(&sample.Pending, &sample.OldestSeconds, &sample.Dead, &sample.Connections, &sample.LockWaiters, &sample.DatabaseBytes)
	return sample, err
}
