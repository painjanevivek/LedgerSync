package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTenantRepositoriesUseScopedConnections(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, `ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY portfolio_test_scope ON accounts AS RESTRICTIVE USING (
 tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid
 AND NULLIF(current_setting('app.actor_id',true),'') IS NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(ctx, `DROP POLICY portfolio_test_scope ON accounts`); err != nil {
			t.Error(err)
		}
	})
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	session.db.SetMaxOpenConns(1)
	accountRepository, err := db.NewAccountRepository(session.db)
	if err != nil {
		t.Fatal(err)
	}
	page, err := accountRepository.ListOwnedPage(ctx, testTenantID, testActorID, accounts.Query{Limit: 10})
	if err != nil || len(page.Accounts) != 1 {
		t.Fatalf("scoped account list: count=%d err=%v", len(page.Accounts), err)
	}
	account, err := accountRepository.GetOwned(ctx, testTenantID, testActorID, testSourceID)
	if err != nil || account.AccountID != testSourceID {
		t.Fatalf("scoped account detail failed: %v", err)
	}
	balanceRepository, err := db.NewBalanceRepository(session.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := balanceRepository.Authorize(ctx, testTenantID, testActorID, testSourceID); err != nil {
		t.Fatal(err)
	}
	balance, err := balanceRepository.ReadCurrent(ctx, testTenantID, testActorID, testSourceID)
	if err != nil || balance.AccountID != testSourceID {
		t.Fatalf("scoped balance read failed: %v", err)
	}
	historyRepository, err := db.NewTransactionHistoryRepository(session.db)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := historyRepository.ListAccountHistory(ctx, testTenantID, testActorID, testSourceID, "", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := accountRepository.GetOwned(ctx, testTenantID, "not-an-owner", testSourceID); !errors.Is(err, accounts.ErrAccountNotFound) {
		t.Fatalf("actor denial failed: %v", err)
	}
	var count int
	if err := session.db.QueryRowContext(ctx, `SELECT count(*) FROM accounts`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("repository left reusable context: count=%d err=%v", count, err)
	}
}

func TestTenantTransactionContextIsolation(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)
	ctx := context.Background()
	const otherTenant = "00000000-0000-4000-8000-000000000902"
	_, err := admin.ExecContext(ctx, `
CREATE TABLE portfolio_tenant_context_probe (tenant_id uuid NOT NULL, note text NOT NULL);
ALTER TABLE portfolio_tenant_context_probe ENABLE ROW LEVEL SECURITY;
ALTER TABLE portfolio_tenant_context_probe FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON portfolio_tenant_context_probe
 USING (tenant_id = NULLIF(current_setting('app.tenant_id',true),'')::uuid
        AND NULLIF(current_setting('app.actor_id',true),'') IS NOT NULL)
 WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id',true),'')::uuid
        AND NULLIF(current_setting('app.actor_id',true),'') IS NOT NULL);
GRANT SELECT, INSERT ON portfolio_tenant_context_probe TO ledgersync_api;`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.ExecContext(ctx, `DROP TABLE portfolio_tenant_context_probe`) })
	if _, err = admin.ExecContext(ctx, `INSERT INTO portfolio_tenant_context_probe VALUES ($1,'first'),($2,'second')`, testTenantID, otherTenant); err != nil {
		t.Fatal(err)
	}
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	session.db.SetMaxOpenConns(1)
	scope := db.TenantScope{TenantID: testTenantID, ActorID: testActorID}
	assertUnscoped := func(t *testing.T) {
		t.Helper()
		var count int
		if err := session.db.QueryRowContext(ctx, `SELECT count(*) FROM portfolio_tenant_context_probe`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("unscoped read must be empty: count=%d err=%v", count, err)
		}
		var tenant, actor string
		if err := session.db.QueryRowContext(ctx, `SELECT COALESCE(current_setting('app.tenant_id',true),''), COALESCE(current_setting('app.actor_id',true),'')`).Scan(&tenant, &actor); err != nil || tenant != "" || actor != "" {
			t.Fatalf("transaction context leaked: %v", err)
		}
	}
	readOne := func(tx *sql.Tx, expected string) error {
		var tenant, actor string
		if err := tx.QueryRowContext(ctx, `SELECT tenant_id::text,current_setting('app.actor_id') FROM portfolio_tenant_context_probe`).Scan(&tenant, &actor); err != nil {
			return err
		}
		if tenant != expected || actor != testActorID {
			return errors.New("incorrect tenant or actor context")
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM portfolio_tenant_context_probe`).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("read %d tenants", count)
		}
		return nil
	}
	t.Run("missing context denies rows", assertUnscoped)
	t.Run("commit clears context", func(t *testing.T) {
		if err := db.WithTenantTransaction(ctx, session.db, scope, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error { return readOne(tx, testTenantID) }); err != nil {
			t.Fatal(err)
		}
		assertUnscoped(t)
	})
	t.Run("preexisting session context cannot survive a scoped transaction", func(t *testing.T) {
		if _, err := session.db.ExecContext(ctx, `SELECT set_config('app.tenant_id',$1,false),set_config('app.actor_id','stale-session',false)`, otherTenant); err != nil {
			t.Fatal(err)
		}
		if err := db.WithTenantTransaction(ctx, session.db, scope, nil, func(tx *sql.Tx) error { return readOne(tx, testTenantID) }); err != nil {
			t.Fatal(err)
		}
		assertUnscoped(t)
	})
	t.Run("rollback clears context and writes", func(t *testing.T) {
		if _, err := session.db.ExecContext(ctx, `SELECT set_config('app.tenant_id',$1,false),set_config('app.actor_id','stale-session',false)`, otherTenant); err != nil {
			t.Fatal(err)
		}
		stop := errors.New("expected rollback")
		err := db.WithTenantTransaction(ctx, session.db, scope, nil, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `INSERT INTO portfolio_tenant_context_probe VALUES ($1,'rolled-back')`, testTenantID); err != nil {
				return err
			}
			return stop
		})
		if !errors.Is(err, stop) {
			t.Fatal(err)
		}
		assertUnscoped(t)
		if countRows(t, admin, `SELECT count(*) FROM portfolio_tenant_context_probe WHERE note='rolled-back'`) != 0 {
			t.Fatal("rollback retained a row")
		}
	})
	t.Run("cross tenant write denied", func(t *testing.T) {
		err := db.WithTenantTransaction(ctx, session.db, scope, nil, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO portfolio_tenant_context_probe VALUES ($1,'forged')`, otherTenant)
			return err
		})
		if sqlState(err) != "42501" {
			t.Fatalf("cross-tenant write was not denied: %v", err)
		}
		assertUnscoped(t)
	})
	t.Run("cancel releases connection and context", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		err := db.WithTenantTransaction(cancelCtx, session.db, scope, nil, func(tx *sql.Tx) error {
			cancel()
			_, err := tx.ExecContext(cancelCtx, `SELECT pg_sleep(2)`)
			return err
		})
		if err == nil {
			t.Fatal("cancelled transaction succeeded")
		}
		assertUnscoped(t)
	})
	t.Run("panic rolls back", func(t *testing.T) {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("panic did not propagate")
				}
			}()
			_ = db.WithTenantTransaction(ctx, session.db, scope, nil, func(*sql.Tx) error { panic("test panic") })
		}()
		assertUnscoped(t)
	})
	t.Run("concurrent pool reuse keeps scopes separate", func(t *testing.T) {
		var workers sync.WaitGroup
		failures := make(chan error, 20)
		for i := 0; i < 20; i++ {
			tenant := testTenantID
			if i%2 == 1 {
				tenant = otherTenant
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				failures <- db.WithTenantTransaction(deadline, session.db, db.TenantScope{TenantID: tenant, ActorID: testActorID}, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error { return readOne(tx, tenant) })
			}()
		}
		workers.Wait()
		close(failures)
		for err := range failures {
			if err != nil {
				t.Error(err)
			}
		}
		assertUnscoped(t)
	})
	t.Run("serializable retry reinstalls context", func(t *testing.T) {
		if _, err := session.db.ExecContext(ctx, `SELECT set_config('app.tenant_id',$1,false),set_config('app.actor_id','stale-session',false)`, otherTenant); err != nil {
			t.Fatal(err)
		}
		attempts := 0
		err := db.WithTenantSerializableSequence(ctx, session.db, scope, "tenant-context-test", 2, func(tx *sql.Tx) error {
			attempts++
			if err := readOne(tx, testTenantID); err != nil {
				return err
			}
			if attempts == 1 {
				return &pgconn.PgError{Code: "40001", Message: "retry fixture"}
			}
			return nil
		})
		if err != nil || attempts != 2 {
			t.Fatalf("retry context: attempts=%d err=%v", attempts, err)
		}
		assertUnscoped(t)
	})
	t.Run("failed connection cleanup cannot poison the pool", func(t *testing.T) {
		err := db.WithTenantTransaction(ctx, session.db, scope, nil, func(tx *sql.Tx) error {
			var pid int
			if err := tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				return err
			}
			var stopped bool
			if err := admin.QueryRowContext(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&stopped); err != nil {
				return err
			}
			if !stopped {
				return errors.New("isolated session was not terminated")
			}
			return nil
		})
		if err == nil {
			t.Fatal("terminated connection reported a successful commit")
		}
		assertUnscoped(t)
		if err := db.WithTenantTransaction(ctx, session.db, scope, nil, func(tx *sql.Tx) error { return readOne(tx, testTenantID) }); err != nil {
			t.Fatal(err)
		}
	})
}
