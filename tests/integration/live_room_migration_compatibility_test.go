package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

// Every case creates its own database. Never seed/truncate the configured
// administrator database: it can be the preserved local runtime authority.
func liveMigrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("LEDGERSYNC_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("LEDGERSYNC_TEST_DATABASE_URL is required for isolated migration tests")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := db.OpenPool(context.Background(), db.PoolConfig{DriverName: "pgx", DSN: databaseURL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	name := fmt.Sprintf("ledgersync_live_migration_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1`, name)
		_, _ = admin.Exec(`DROP DATABASE ` + name)
	})
	parsed.Path = "/" + name
	database, err := db.OpenPool(context.Background(), db.PoolConfig{DriverName: "pgx", DSN: parsed.String()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func liveMigrationRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("LEDGERSYNC_TEST_REPOSITORY_ROOT"); root != "" {
		return root
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func liveMigrationSource(t *testing.T, beforeLive bool) fstest.MapFS {
	t.Helper()
	root := liveMigrationRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	source := fstest.MapFS{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") || beforeLive && entry.Name() >= "000037_" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, "migrations", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		source[entry.Name()] = &fstest.MapFile{Data: body}
	}
	return source
}

func applyLiveMigrationSource(t *testing.T, database *sql.DB, beforeLive bool) error {
	t.Helper()
	return db.ApplyPending(context.Background(), database, db.MigrationConfig{Source: liveMigrationSource(t, beforeLive)})
}

func provisionLiveMigrationRoles(t *testing.T, database *sql.DB) {
	t.Helper()
	if _, err := database.Exec(`SET row_security=on`); err != nil {
		t.Fatal(err)
	}
	roles, err := os.ReadFile(filepath.Join(liveMigrationRoot(t), "deploy", "postgres", "roles.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(string(roles)); err != nil {
		t.Fatal(err)
	}
}

func installLegacyLiveFixture(t *testing.T, database *sql.DB) {
	t.Helper()
	if err := applyLiveMigrationSource(t, database, true); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join(liveMigrationRoot(t), "tests", "integration", "testdata", "legacy-live-investigation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(string(fixture)); err != nil {
		t.Fatal(err)
	}
	if err := seedTransferFixture(context.Background(), database, 100_000); err != nil {
		t.Fatal(err)
	}
	seedLiveRoomRecords(t, database, true)
}

func seedLiveRoomRecords(t *testing.T, database *sql.DB, legacy bool) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO idempotency_requests(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,expires_at,created_at,request_reference)
VALUES($1,$2,'transfers.create.v1','legacy-migration-intent',decode(repeat('ab',32),'hex'),'in_progress',now()+interval '1 hour',now(),$3)`, testTenantID, testActorID, liveRequestReference); err != nil {
		t.Fatal(err)
	}
	// A historical schema fixture must not execute the current application binary.
	now := time.Now().UTC().Truncate(time.Microsecond)
	room := struct{ RoomID string }{"00000000-0000-4000-8000-000000000881"}
	const workspaceID = "00000000-0000-4000-8000-000000000882"
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO investigation_workspaces(id,tenant_id,owner_subject_id,title,taxonomy,status,query_kind,root_record_type,query_value,root_record_id,version,created_at,updated_at) VALUES($1,$2,$3,'Uncertain transfer request','transfer_delivery','open','request_reference','transfer_request',$4::text,$4::uuid,1,$5,$5)`, []any{workspaceID, testTenantID, testActorID, liveRequestReference, now}},
		{`INSERT INTO investigation_workspace_references(investigation_id,tenant_id,position,relationship_type,record_type,record_id,captured_at) VALUES($1,$2,0,'root','transfer_request',$3,$4)`, []any{workspaceID, testTenantID, liveRequestReference, now}},
		{`INSERT INTO investigation_live_rooms(id,tenant_id,investigation_id,transfer_request_reference,owner_subject_id,status,version,created_at,expires_at) VALUES($1,$2,$3,$4,$5,'waiting',1,$6,$7)`, []any{room.RoomID, testTenantID, workspaceID, liveRequestReference, testActorID, now, now.Add(30 * time.Minute)}},
		{`INSERT INTO investigation_live_room_operations(id,tenant_id,actor_subject_id,operation,idempotency_key_hash,request_fingerprint,room_id,response_body,created_at) VALUES(gen_random_uuid(),$1,$2,'start',digest('legacy-start-operation','sha256'),digest($3::text,'sha256'),$4::uuid,jsonb_build_object('room_id',$4::text,'status','waiting'),$5)`, []any{testTenantID, testActorID, liveRequestReference, room.RoomID, now}},
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	activate := `UPDATE investigation_live_rooms SET status='active',peer_subject_id='legacy-peer' WHERE id=$1`
	arguments := []any{room.RoomID}
	if legacy {
		activate = `UPDATE investigation_live_rooms SET status='active',peer_subject_id='legacy-peer',joined_at=$2,invite_redeemed_at=$2 WHERE id=$1`
		arguments = append(arguments, now)
	}
	if _, err := database.Exec(activate, arguments...); err != nil {
		t.Fatal(err)
	}
	// The legacy id is omitted on purpose: its default is part of compatibility.
	if _, err := database.Exec(`INSERT INTO investigation_live_room_findings(room_id,tenant_id,finding_code,authoritative_request_state,actor_subject_id,recorded_at)
VALUES($1,$2,'still_unresolved','in_progress',$3,$4)`, room.RoomID, testTenantID, testActorID, now); err != nil {
		t.Fatal(err)
	}
}

func liveMigrationSnapshot(t *testing.T, database *sql.DB) string {
	t.Helper()
	var snapshot string
	if err := database.QueryRow(`SELECT jsonb_build_object(
'rooms',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM investigation_live_rooms r),
'findings',(SELECT jsonb_agg(to_jsonb(f) ORDER BY room_id) FROM investigation_live_room_findings f),
'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM investigation_live_room_operations o),
'old_history',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m WHERE version IN ('000034_live_investigation_rooms.up.sql','000035_live_investigation_compatibility.up.sql')))::text`).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestLiveRoomMigrationFreshDatabase(t *testing.T) {
	database := liveMigrationDatabase(t)
	if err := applyLiveMigrationSource(t, database, false); err != nil {
		t.Fatal(err)
	}
	if got, want := countRows(t, database, `SELECT count(*) FROM schema_migrations`), len(liveMigrationSource(t, false)); got != want {
		t.Fatalf("fresh migration count=%d, want %d", got, want)
	}
	provisionLiveMigrationRoles(t, database)
	verifyLiveAttachPrivileges(t, database)
	verifyLateLiveInvite(t, database)
	verifyCompletedReferenceAttachment(t, database)
}

func verifyLateLiveInvite(t *testing.T, database *sql.DB) {
	t.Helper()
	var roomID string
	var roomExpiry time.Time
	if err := database.QueryRow(`SELECT id::text,expires_at FROM investigation_live_rooms LIMIT 1`).Scan(&roomID, &roomExpiry); err == sql.ErrNoRows {
		return // The clean schema has no room; the legacy case exercises this path.
	} else if err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	invite, err := repository.IssueLiveRoomInvite(context.Background(), testTenantID, testActorID, roomID, roomExpiry.Add(-time.Minute))
	if err != nil || !invite.ExpiresAt.Equal(roomExpiry) {
		t.Fatalf("late-room invite expiry=%s room expiry=%s error=%v", invite.ExpiresAt, roomExpiry, err)
	}
	var storedExpiry time.Time
	if err := database.QueryRow(`SELECT invite_expires_at FROM investigation_live_rooms WHERE id=$1`, roomID).Scan(&storedExpiry); err != nil || !storedExpiry.Equal(roomExpiry) {
		t.Fatalf("stored invite expiry=%s room expiry=%s error=%v", storedExpiry, roomExpiry, err)
	}
}

func verifyLiveAttachPrivileges(t *testing.T, database *sql.DB) {
	t.Helper()
	if got := countRows(t, database, `SELECT count(*) FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner
WHERE p.oid='public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)'::regprocedure
AND p.prosecdef AND r.rolname='ledgersync_migration_owner' AND 'search_path=pg_catalog, public'=ANY(p.proconfig)
AND has_function_privilege('ledgersync_api',p.oid,'EXECUTE')
AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')`); got != 1 {
		t.Fatal("request-reference function ownership or execute boundary is incorrect")
	}
	if got := countRows(t, database, `SELECT count(*) WHERE pg_has_role('ledgersync_api','ledgersync_migration_owner','MEMBER')`); got != 0 {
		t.Fatal("API must not receive membership in the migration-owner role")
	}
	if got := countRows(t, database, `SELECT count(*) WHERE has_function_privilege('ledgersync_worker','public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)','EXECUTE') OR has_function_privilege('ledgersync_support_readonly','public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)','EXECUTE')`); got != 0 {
		t.Fatal("worker/support roles must not receive attachment execute permission")
	}
}

func TestLiveRoomMigrationPreservesKnownLegacyRecordsAndHistory(t *testing.T) {
	database := liveMigrationDatabase(t)
	installLegacyLiveFixture(t, database)
	before := liveMigrationSnapshot(t, database)
	if err := applyLiveMigrationSource(t, database, false); err != nil {
		t.Fatal(err)
	}
	if after := liveMigrationSnapshot(t, database); after != before {
		t.Fatal("migration changed a legacy room, finding, operation, or historical version")
	}
	if got, want := countRows(t, database, `SELECT count(*) FROM schema_migrations`), len(liveMigrationSource(t, false))+2; got != want {
		t.Fatalf("migration count=%d, want %d including two preserved historical versions", got, want)
	}
	if got := countRows(t, database, `SELECT count(*) FROM pg_constraint WHERE conname IN ('investigation_live_rooms_current_expiry_check','investigation_live_rooms_current_invite_pair_check','investigation_live_room_findings_authoritative_check') AND convalidated`); got != 3 {
		t.Fatal("current legacy lifecycle/finding checks were not validated")
	}
	provisionLiveMigrationRoles(t, database)
	verifyLiveAttachPrivileges(t, database)
	verifyLateLiveInvite(t, database)
	for _, test := range []struct {
		name, tenant, actor, fingerprint, reference string
		allowed                                     bool
	}{
		{"same reference", testTenantID, testActorID, strings.Repeat("ab", 32), liveRequestReference, true},
		{"same reference replay", testTenantID, testActorID, strings.Repeat("ab", 32), liveRequestReference, true},
		{"wrong tenant", liveCorrelationID, testActorID, strings.Repeat("ab", 32), liveRequestReference, false},
		{"wrong actor", testTenantID, "other-actor", strings.Repeat("ab", 32), liveRequestReference, false},
		{"wrong fingerprint", testTenantID, testActorID, strings.Repeat("cd", 32), liveRequestReference, false},
		{"reference rebinding", testTenantID, testActorID, strings.Repeat("ab", 32), liveCorrelationID, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			if _, err := tx.Exec(`SET LOCAL ROLE ledgersync_api; SET LOCAL row_security=on`); err != nil {
				t.Fatal(err)
			}
			if _, scopeErr := tx.Exec("SELECT set_config('app.tenant_id',$1,true),set_config('app.actor_id',$2,true)", testTenantID, testActorID); scopeErr != nil {
				t.Fatal(scopeErr)
			}
			_, err = tx.Exec(`SELECT controlled_attach_transfer_request_reference_v1($1,$2,'legacy-migration-intent',decode($3,'hex'),$4)`, test.tenant, test.actor, test.fingerprint, test.reference)
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%v error=%v", test.allowed, err)
			}
		})
	}
	verifyCompletedReferenceAttachment(t, database)
}

func verifyCompletedReferenceAttachment(t *testing.T, database *sql.DB) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO tenants(id,external_reference) VALUES($1,'attachment-fixture') ON CONFLICT DO NOTHING`, testTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO idempotency_requests(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,response_status,response_body,expires_at,created_at,completed_at)
VALUES($1,$2,'transfers.create.v1','completed-attachment-key',decode(repeat('ef',32),'hex'),'completed',200,'{"status":"posted"}',now()+interval '1 hour',now(),now())`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := database.QueryRow(`SELECT (to_jsonb(r)-'request_reference')::text FROM idempotency_requests r WHERE idempotency_key='completed-attachment-key'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		tx, err := database.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(`SET LOCAL ROLE ledgersync_api; SET LOCAL row_security=on`); err == nil {
			if _, scopeErr := tx.Exec("SELECT set_config('app.tenant_id',$1,true),set_config('app.actor_id',$2,true)", testTenantID, testActorID); scopeErr != nil {
				t.Fatal(scopeErr)
			}
			_, err = tx.Exec(`SELECT controlled_attach_transfer_request_reference_v1($1,$2,'completed-attachment-key',decode(repeat('ef',32),'hex'),$3)`, testTenantID, testActorID, liveCorrelationID)
		}
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("completed reference attachment/replay failed: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	var after, reference string
	if err := database.QueryRow(`SELECT (to_jsonb(r)-'request_reference')::text,request_reference::text FROM idempotency_requests r WHERE idempotency_key='completed-attachment-key'`).Scan(&after, &reference); err != nil || before != after || reference != liveCorrelationID {
		t.Fatalf("attachment changed outcome or failed to retain reference: %v", err)
	}
	for _, mutation := range []string{
		`UPDATE idempotency_requests SET response_body='{"status":"changed"}' WHERE idempotency_key='completed-attachment-key'`,
		`UPDATE idempotency_requests SET request_reference=NULL WHERE idempotency_key='completed-attachment-key'`,
		`DELETE FROM idempotency_requests WHERE idempotency_key='completed-attachment-key'`,
	} {
		if _, err := database.Exec(mutation); err == nil {
			t.Fatal("resolved outcome/reference mutation was accepted")
		}
	}
	if _, err := database.Exec(`INSERT INTO idempotency_requests(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,response_status,response_body,expires_at,created_at,completed_at)
VALUES($1,$2,'transfers.create.v1','combined-mutation-key',decode(repeat('ef',32),'hex'),'completed',200,'{"status":"posted"}',now()+interval '1 hour',now(),now())`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	unusedReference := "00000000-0000-4000-8000-000000000038"
	if _, err := database.Exec(`UPDATE idempotency_requests SET response_body='{"status":"changed"}',request_reference=$1 WHERE idempotency_key='combined-mutation-key'`, unusedReference); err == nil || !strings.Contains(err.Error(), "SQLSTATE 55000") {
		t.Fatalf("reference attachment bypassed outcome checks or failed for the wrong reason: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO idempotency_requests(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,expires_at,created_at)
VALUES($1,$2,'transfers.create.v1','pending-finalization-key',decode(repeat('ef',32),'hex'),'in_progress',now()+interval '1 hour',now())`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE idempotency_requests SET state='completed',response_status=200,response_body='{"status":"posted"}',completed_at=now(),request_reference=$1 WHERE idempotency_key='pending-finalization-key'`, unusedReference); err == nil || !strings.Contains(err.Error(), "SQLSTATE 55000") {
		t.Fatalf("reference attachment was combined with outcome finalization or failed for the wrong reason: %v", err)
	}
	if _, err := database.Exec(`UPDATE idempotency_requests SET request_reference=$1 WHERE idempotency_key='pending-finalization-key'`, unusedReference); err == nil || !strings.Contains(err.Error(), "SQLSTATE 55000") {
		t.Fatalf("unresolved transfer accepted first reference-only mutation or failed for the wrong reason: %v", err)
	}
	if _, err := database.Exec(`UPDATE idempotency_requests SET state='completed',response_status=200,response_body='{"status":"posted"}',completed_at=now() WHERE idempotency_key='pending-finalization-key'`); err != nil {
		t.Fatalf("ordinary finalization was broken: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO idempotency_requests(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,response_status,response_body,expires_at,created_at,completed_at)
VALUES($1,$2,'accounts.create.v1','non-transfer-reference-key',decode(repeat('ef',32),'hex'),'completed',200,'{"status":"created"}',now()+interval '1 hour',now(),now())`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE idempotency_requests SET request_reference=$1 WHERE idempotency_key='non-transfer-reference-key'`, unusedReference); err == nil || !strings.Contains(err.Error(), "SQLSTATE 55000") {
		t.Fatalf("non-transfer command accepted reference-only mutation or failed for the wrong reason: %v", err)
	}
}

func TestLiveRoomMigrationDownRestoresStrictOutcomeGuard(t *testing.T) {
	database := liveMigrationDatabase(t)
	if err := applyLiveMigrationSource(t, database, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000039_transfer_request_reference_repair.down.sql", "000038_live_investigation_compatibility.down.sql", "000037_live_investigation_rooms.down.sql"} {
		source, err := os.ReadFile(filepath.Join(liveMigrationRoot(t), "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(string(source)); err != nil {
			t.Fatalf("isolated rollback %s: %v", name, err)
		}
	}
	if _, err := database.Exec(`INSERT INTO tenants(id,external_reference) VALUES($1,'rollback-fixture')`, testTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO idempotency_requests(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,response_status,response_body,expires_at,created_at,completed_at)
VALUES($1,$2,'transfers.create.v1','rollback-guard-key',decode(repeat('ef',32),'hex'),'completed',200,'{"status":"posted"}',now()+interval '1 hour',now(),now())`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE idempotency_requests SET response_body='{"status":"changed"}' WHERE idempotency_key='rollback-guard-key'`); err == nil || !strings.Contains(err.Error(), "resolved idempotency outcomes are immutable") {
		t.Fatalf("rollback left a broken or weakened outcome guard: %v", err)
	}
}

func TestLiveRoomMigrationReferencedTransferPostsAndReplaysOnce(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			database := liveMigrationDatabase(t)
			if legacy {
				installLegacyLiveFixture(t, database)
			}
			if err := applyLiveMigrationSource(t, database, false); err != nil {
				t.Fatal(err)
			}
			if !legacy {
				if err := seedTransferFixture(context.Background(), database, 100_000); err != nil {
					t.Fatal(err)
				}
			}
			provisionLiveMigrationRoles(t, database)
			verifyReferencedTransferPostsAndReplays(t, database)
		})
	}
}

func liveTransferService(t *testing.T, database *sql.DB) *transfers.Service {
	t.Helper()
	repository, err := db.NewTransferRepository(database, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := transfers.NewService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func verifyReferencedTransferPostsAndReplays(t *testing.T, database *sql.DB) {
	t.Helper()
	service := liveTransferService(t, database)
	journals := countRows(t, database, `SELECT count(*) FROM journal_transactions`)
	postings := countRows(t, database, `SELECT count(*) FROM ledger_postings`)
	command := transferCommand(t, "migration-reference-transfer", "10.00")
	command.RequestReference = "00000000-0000-4000-8000-000000000037"
	first, err := service.Submit(context.Background(), command)
	if err != nil || first.Result.Status != "posted" || first.Replayed {
		t.Fatalf("referenced transfer first outcome=%#v error=%v", first, err)
	}
	second, err := service.Submit(context.Background(), command)
	if err != nil || !second.Replayed || second.Result.TransferID != first.Result.TransferID {
		t.Fatalf("referenced transfer replay=%#v error=%v", second, err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM journal_transactions`); got != journals+1 {
		t.Fatalf("posting/replay journal count=%d, want %d", got, journals+1)
	}
	if got := countRows(t, database, `SELECT count(*) FROM ledger_postings`); got != postings+2 {
		t.Fatalf("posting/replay posting count=%d, want %d", got, postings+2)
	}
	var storedReference string
	if err := database.QueryRow(`SELECT request_reference::text FROM idempotency_requests WHERE idempotency_key=$1`, command.IdempotencyKey).Scan(&storedReference); err != nil || storedReference != command.RequestReference {
		t.Fatalf("retained transfer reference=%q error=%v", storedReference, err)
	}
	command.RequestReference = liveCorrelationID
	if _, err := service.Submit(context.Background(), command); err == nil {
		t.Fatal("original posted transfer accepted reference rebinding")
	}
	if got := countRows(t, database, `SELECT count(*) FROM journal_transactions`); got != journals+1 {
		t.Fatal("failed rebinding changed the posted journal")
	}
}

func preReferenceRepairSnapshot(t *testing.T, database *sql.DB) string {
	t.Helper()
	var snapshot string
	if err := database.QueryRow(`SELECT jsonb_build_object(
'history',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m WHERE version<'000039_'),
'execution_boundary',(SELECT jsonb_build_object('owner',proowner,'acl',proacl) FROM pg_proc WHERE oid='public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)'::regprocedure),
'table_acl',(SELECT relacl FROM pg_class WHERE oid='public.idempotency_requests'::regclass),
'api_migration_owner_membership',pg_has_role('ledgersync_api','ledgersync_migration_owner','MEMBER'),
'idempotency',(SELECT jsonb_agg(to_jsonb(r) ORDER BY idempotency_key) FROM idempotency_requests r),
'accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),
'balances',(SELECT jsonb_agg(to_jsonb(b) ORDER BY account_id) FROM account_balance_projections b),
'transfers',(SELECT jsonb_agg(to_jsonb(tr) ORDER BY id) FROM transfers tr),
'journals',(SELECT jsonb_agg(to_jsonb(j) ORDER BY id) FROM journal_transactions j),
'postings',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM ledger_postings p),
'rooms',(SELECT jsonb_agg(to_jsonb(rm) ORDER BY id) FROM investigation_live_rooms rm),
'findings',(SELECT jsonb_agg(to_jsonb(f) ORDER BY room_id) FROM investigation_live_room_findings f),
'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM investigation_live_room_operations o))::text`).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestLiveRoomMigrationRepairsAlreadyAppliedOriginalCurrent37(t *testing.T) {
	database := liveMigrationDatabase(t)
	source := liveMigrationSource(t, false)
	delete(source, "000039_transfer_request_reference_repair.up.sql")
	// This case must start with the original attachment body. Actor hardening
	// follows the reference repair in real migration order.
	delete(source, "000058_bind_existing_controlled_actors.up.sql")
	prior37, err := os.ReadFile(filepath.Join(liveMigrationRoot(t), "tests", "integration", "testdata", "original-current-live-investigation-37.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source["000037_live_investigation_rooms.up.sql"] = &fstest.MapFile{Data: prior37}
	if err := db.ApplyPending(context.Background(), database, db.MigrationConfig{Source: source}); err != nil {
		t.Fatal(err)
	}
	if got, want := countRows(t, database, `SELECT count(*) FROM schema_migrations`), len(source); got != want {
		t.Fatalf("original current migration count=%d, want %d", got, want)
	}
	provisionLiveMigrationRoles(t, database)
	verifyLiveAttachPrivileges(t, database)
	if err := seedTransferFixture(context.Background(), database, 100_000); err != nil {
		t.Fatal(err)
	}
	service := liveTransferService(t, database)
	if first, err := service.Submit(context.Background(), transferCommand(t, "pre39-retained-transfer", "5.00")); err != nil || first.Result.Status != "posted" {
		t.Fatalf("unreferenced original transfer failed: %v", err)
	}
	seedLiveRoomRecords(t, database, false)
	before := preReferenceRepairSnapshot(t, database)
	command := transferCommand(t, "pre39-broken-reference", "10.00")
	command.RequestReference = "00000000-0000-4000-8000-000000000039"
	if _, err := service.Submit(context.Background(), command); err == nil || (!strings.Contains(err.Error(), "SQLSTATE 55000") && !strings.Contains(err.Error(), "resolved idempotency outcomes are immutable")) {
		t.Fatalf("original current37 did not reproduce the specific reference guard defect: %v", err)
	}
	if after := preReferenceRepairSnapshot(t, database); after != before {
		t.Fatal("failed original referenced command changed retained data/history")
	}
	t.Run("unknown helper metadata stops atomically", func(t *testing.T) {
		if _, err := database.Exec(`ALTER FUNCTION public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid) STRICT`); err != nil {
			t.Fatal(err)
		}
		if err := applyLiveMigrationSource(t, database, false); err == nil || !strings.Contains(err.Error(), "incompatible transfer reference attachment helper") {
			t.Fatalf("unexpected helper metadata bypassed the repair preflight: %v", err)
		}
		if got := countRows(t, database, `SELECT count(*) FROM schema_migrations WHERE version='000039_transfer_request_reference_repair.up.sql'`); got != 0 {
			t.Fatal("failed additive repair was recorded as applied")
		}
		if after := preReferenceRepairSnapshot(t, database); after != before {
			t.Fatal("rejected additive repair changed retained data/history/privileges")
		}
		if _, err := database.Exec(`ALTER FUNCTION public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid) CALLED ON NULL INPUT`); err != nil {
			t.Fatal(err)
		}
	})
	if err := applyLiveMigrationSource(t, database, false); err != nil {
		t.Fatal(err)
	}
	if after := preReferenceRepairSnapshot(t, database); after != before {
		t.Fatal("additive repair changed pre39 financial/idempotency/live records or history")
	}
	if got, want := countRows(t, database, `SELECT count(*) FROM schema_migrations`), len(liveMigrationSource(t, false)); got != want {
		t.Fatalf("repaired migration count=%d, want %d", got, want)
	}
	if got := countRows(t, database, `SELECT count(*) FROM schema_migrations WHERE version='000039_transfer_request_reference_repair.up.sql'`); got != 1 {
		t.Fatal("additive repair did not record exactly one current39 version")
	}
	// CREATE OR REPLACE must retain provisioned ownership even before roles.sql
	// is rerun; neither API membership nor a broad new table grant is required.
	verifyLiveAttachPrivileges(t, database)
	verifyReferencedTransferPostsAndReplays(t, database)
	verifyCompletedReferenceAttachment(t, database)
	if err := applyLiveMigrationSource(t, database, false); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM schema_migrations`); got != len(liveMigrationSource(t, false)) {
		t.Fatal("repeated ApplyPending rewrote or duplicated current39 history")
	}
}

func TestLiveRoomMigrationRejectsIndexNameCollisionOnFreshDatabase(t *testing.T) {
	for _, name := range []string{"idempotency_requests_tenant_request_reference_idx", "investigation_live_rooms_active_workspace_idx", "investigation_live_rooms_active_request_idx", "investigation_live_rooms_peer_idx"} {
		t.Run(name, func(t *testing.T) {
			database := liveMigrationDatabase(t)
			if err := applyLiveMigrationSource(t, database, true); err != nil {
				t.Fatal(err)
			}
			if _, err := database.Exec(`CREATE INDEX ` + name + ` ON tenants(external_reference)`); err != nil {
				t.Fatal(err)
			}
			if err := applyLiveMigrationSource(t, database, false); err == nil {
				t.Fatal("same-name unrelated index bypassed uniqueness creation")
			}
			if got := countRows(t, database, `SELECT count(*) FROM schema_migrations WHERE version LIKE '000037%'`); got != 0 {
				t.Fatal("failed fresh migration was recorded as applied")
			}
		})
	}
}

func TestLiveRoomMigrationRejectsIncompatibleLegacyAtomically(t *testing.T) {
	for _, test := range []struct{ name, mutation string }{
		{"missing history", `DELETE FROM schema_migrations WHERE version='000035_live_investigation_compatibility.up.sql'`},
		{"extra required column", `ALTER TABLE investigation_live_rooms ADD COLUMN unknown_required TEXT NOT NULL DEFAULT 'unknown'`},
		{"wrong nullability", `ALTER TABLE investigation_live_rooms ALTER COLUMN owner_subject_id DROP NOT NULL`},
		{"missing finding default", `ALTER TABLE investigation_live_room_findings ALTER COLUMN id DROP DEFAULT`},
		{"unexpected identity collation", `ALTER TABLE investigation_live_rooms ALTER COLUMN owner_subject_id TYPE TEXT COLLATE "C"`},
		{"unexpected row policy", `ALTER TABLE investigation_live_rooms ENABLE ROW LEVEL SECURITY`},
		{"disabled finding immutability", `ALTER TABLE investigation_live_room_findings DISABLE TRIGGER investigation_live_room_findings_immutable_update`},
		{"wrong active index", `DROP INDEX investigation_live_rooms_active_request_idx; CREATE INDEX investigation_live_rooms_active_request_idx ON investigation_live_rooms(tenant_id)`},
		{"unexpected current index", `CREATE INDEX idempotency_requests_tenant_request_reference_idx ON idempotency_requests(tenant_id)`},
		{"historical expiry violation", `UPDATE investigation_live_rooms SET expires_at=created_at+interval '31 minutes'`},
		{"historical invite pair violation", `UPDATE investigation_live_rooms SET invite_token_hash=decode(repeat('ab',32),'hex'),invite_expires_at=NULL`},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := liveMigrationDatabase(t)
			installLegacyLiveFixture(t, database)
			if _, err := database.Exec(test.mutation); err != nil {
				t.Fatal(err)
			}
			before := liveMigrationSnapshot(t, database)
			if err := applyLiveMigrationSource(t, database, false); err == nil {
				t.Fatal("incompatible legacy schema/data was accepted")
			} else if test.name == "historical expiry violation" && !strings.Contains(err.Error(), "investigation_live_rooms_current_expiry_check") {
				t.Fatalf("expiry test did not reach required check validation: %v", err)
			} else if test.name == "historical invite pair violation" && !strings.Contains(err.Error(), "investigation_live_rooms_current_invite_pair_check") {
				t.Fatalf("invite-pair test did not reach required check validation: %v", err)
			}
			if liveMigrationSnapshot(t, database) != before {
				t.Fatal("failed migration changed preserved records or history")
			}
			if got := countRows(t, database, `SELECT count(*) FROM schema_migrations WHERE version LIKE '000037%' OR version LIKE '000038%'`); got != 0 {
				t.Fatal("failed migration was recorded as applied")
			}
			if got := countRows(t, database, `SELECT count(*) FROM pg_proc WHERE oid=to_regprocedure('public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)')`); got != 0 {
				t.Fatal("failed migration did not roll back the function")
			}
		})
	}
}
