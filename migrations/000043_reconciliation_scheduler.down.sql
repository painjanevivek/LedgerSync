-- Prefer forward repair. Stop all scheduler instances before rollback.
DROP TRIGGER restrict_scheduler_runs ON reconciliation_runs;
DROP TRIGGER restrict_scheduler_mismatches ON reconciliation_mismatches;
DROP TRIGGER restrict_scheduler_audit ON audit_events;
DROP FUNCTION restrict_scheduler_evidence_insert();
DROP SCHEMA ledgersync_scheduler CASCADE;
DROP FUNCTION complete_reconciliation_schedule(uuid,text,bigint,uuid,uuid,text),claim_reconciliation_schedule(uuid,text,bigint,integer),release_reconciliation_scheduler(uuid,text,bigint),renew_reconciliation_scheduler(uuid,text,bigint,integer),acquire_reconciliation_scheduler(uuid,text,integer),scheduler_tenant_allowed(uuid);
DROP TABLE reconciliation_scheduled_jobs,reconciliation_scheduler_leases,reconciliation_scheduler_tenants;
