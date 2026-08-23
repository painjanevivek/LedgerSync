\set ON_ERROR_STOP on

INSERT INTO tenants (id, external_reference) VALUES ('00000000-0000-4000-8000-000000000001', 'ledgersync-local-demo') ON CONFLICT (id) DO NOTHING;

INSERT INTO tenant_transfer_policies (tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor)
VALUES ('00000000-0000-4000-8000-000000000001','USD',1,1000000000,5000000000,5000000000,10000000000)
ON CONFLICT (tenant_id) DO UPDATE SET currency=EXCLUDED.currency,minimum_transfer_minor=EXCLUDED.minimum_transfer_minor,maximum_transfer_minor=EXCLUDED.maximum_transfer_minor,actor_rolling_24h_minor=EXCLUDED.actor_rolling_24h_minor,source_account_rolling_24h_minor=EXCLUDED.source_account_rolling_24h_minor,tenant_rolling_24h_minor=EXCLUDED.tenant_rolling_24h_minor,updated_at=now();

INSERT INTO accounts (id, tenant_id, currency, status, display_name, category, external_reference, created_at) VALUES
  ('10000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000001','USD','active','Operating Reserve','operating','OPS-RESERVE','2026-08-20T09:00:00Z'),
  ('10000000-0000-4000-8000-000000000002','00000000-0000-4000-8000-000000000001','USD','active','Customer Funds','customer_funds','CUSTOMER-FUNDS','2026-08-20T09:01:00Z'),
  ('10000000-0000-4000-8000-000000000003','00000000-0000-4000-8000-000000000001','USD','active','Payroll','payroll','PAYROLL-US','2026-08-20T09:02:00Z'),
  ('10000000-0000-4000-8000-000000000004','00000000-0000-4000-8000-000000000001','USD','active','Vendor Payables','payables','VENDOR-PAYABLES','2026-08-20T09:03:00Z'),
  ('10000000-0000-4000-8000-000000000005','00000000-0000-4000-8000-000000000001','USD','frozen','Marketing Expenses — North America','expenses','MKTG-NORTH-AMERICA','2026-08-20T09:04:00Z'),
  ('10000000-0000-4000-8000-000000000006','00000000-0000-4000-8000-000000000001','USD','active','New Project · No activity','reserve','PROJECT-EMPTY','2026-08-20T09:05:00Z')
ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name, category=EXCLUDED.category, external_reference=EXCLUDED.external_reference;

INSERT INTO account_owners (tenant_id, account_id, subject_id, permission)
SELECT '00000000-0000-4000-8000-000000000001', id, 'demo-operator', CASE WHEN status='active' THEN 'debit' ELSE 'read' END FROM accounts WHERE tenant_id='00000000-0000-4000-8000-000000000001'
ON CONFLICT (account_id, subject_id) DO NOTHING;

INSERT INTO account_credit_permissions (tenant_id,account_id,subject_id)
SELECT '00000000-0000-4000-8000-000000000001',id,'demo-operator' FROM accounts WHERE tenant_id='00000000-0000-4000-8000-000000000001' AND status='active'
ON CONFLICT (account_id,subject_id) DO NOTHING;

INSERT INTO account_balance_projections (account_id, available_minor, ledger_minor, balance_version, updated_at) VALUES
 ('10000000-0000-4000-8000-000000000001',142006419,142006419,8241193,'2026-08-20T12:00:00Z'),
 ('10000000-0000-4000-8000-000000000002',84200000,84200000,8241194,'2026-08-20T12:00:00Z'),
 ('10000000-0000-4000-8000-000000000003',21600000,21600000,8241195,'2026-08-20T12:00:00Z'),
 ('10000000-0000-4000-8000-000000000004',12500000,12500000,8241196,'2026-08-20T12:00:00Z'),
 ('10000000-0000-4000-8000-000000000005',385000,385000,8241197,'2026-08-20T12:00:00Z'),
 ('10000000-0000-4000-8000-000000000006',0,0,0,'2026-08-20T12:00:00Z')
-- Seed reruns may add missing demo rows, but must never rewind balances or
-- versions after real transfers have committed.
ON CONFLICT (account_id) DO NOTHING;

INSERT INTO account_opening_balances (account_id, opening_ledger_minor) VALUES
 ('10000000-0000-4000-8000-000000000001',129506419),('10000000-0000-4000-8000-000000000002',109200000),
 ('10000000-0000-4000-8000-000000000003',21600000),('10000000-0000-4000-8000-000000000004',0),
 ('10000000-0000-4000-8000-000000000005',385000),('10000000-0000-4000-8000-000000000006',0)
-- Opening balances are financial evidence and become immutable once created.
ON CONFLICT (account_id) DO NOTHING;

BEGIN;
SET CONSTRAINTS transfers_journal_transaction_fk DEFERRED;
INSERT INTO transfers (id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,journal_transaction_id,created_at,completed_at) VALUES
 ('20000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000001','demo-operator','10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000004',12500000,'USD','posted','30000000-0000-4000-8000-000000000001','2026-08-20T11:58:30Z','2026-08-20T11:58:34Z'),
 ('20000000-0000-4000-8000-000000000002','00000000-0000-4000-8000-000000000001','demo-operator','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001',25000000,'USD','posted','30000000-0000-4000-8000-000000000002','2026-08-20T11:47:10Z','2026-08-20T11:47:12Z')
ON CONFLICT (id) DO NOTHING;
INSERT INTO transfers (id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,rejection_code,created_at,completed_at) VALUES
 ('20000000-0000-4000-8000-000000000003','00000000-0000-4000-8000-000000000001','demo-operator','10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000005',385000,'USD','rejected','account_inactive','2026-08-20T10:22:39Z','2026-08-20T10:22:41Z')
ON CONFLICT (id) DO NOTHING;

INSERT INTO journal_transactions (id,tenant_id,transfer_id,occurred_at) VALUES
 ('30000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','2026-08-20T11:58:34Z'),
 ('30000000-0000-4000-8000-000000000002','00000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000002','2026-08-20T11:47:12Z') ON CONFLICT (id) DO NOTHING;
INSERT INTO ledger_postings (id,journal_transaction_id,account_id,direction,amount_minor,currency,occurred_at) VALUES
 ('40000000-0000-4000-8000-000000000001','30000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','debit',12500000,'USD','2026-08-20T11:58:34Z'),
 ('40000000-0000-4000-8000-000000000002','30000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000004','credit',12500000,'USD','2026-08-20T11:58:34Z'),
 ('40000000-0000-4000-8000-000000000003','30000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000002','debit',25000000,'USD','2026-08-20T11:47:12Z'),
 ('40000000-0000-4000-8000-000000000004','30000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','credit',25000000,'USD','2026-08-20T11:47:12Z') ON CONFLICT (id) DO NOTHING;

INSERT INTO reconciliation_runs (id,tenant_id,status,checked_account_count,mismatch_count,correlation_id,started_at,completed_at,details) VALUES
 ('50000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000001','matched',6,0,'60000000-0000-4000-8000-000000000001','2026-08-20T11:59:58Z','2026-08-20T12:00:00Z','{"posting_count":4,"ledger_watermark":"8241197","application_version":"demo-1","scope":"All USD demo accounts"}'),
 ('50000000-0000-4000-8000-000000000002','00000000-0000-4000-8000-000000000001','mismatch',6,1,'60000000-0000-4000-8000-000000000002','2026-08-20T10:39:58Z','2026-08-20T10:40:02Z','{"posting_count":4,"ledger_watermark":"8241196","application_version":"demo-1","scope":"All USD demo accounts","note":"Controlled historical mismatch fixture"}')
ON CONFLICT (id) DO NOTHING;
COMMIT;
