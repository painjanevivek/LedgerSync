\set ON_ERROR_STOP on

-- A fresh workspace needs authorization and policy boundaries, but no financial
-- records. Accounts, balances, journals, transfers, and reconciliation evidence
-- are created only through the product after login.
INSERT INTO tenants (id, external_reference)
VALUES ('00000000-0000-4000-8000-000000000001', 'ledgersync-local-workspace')
ON CONFLICT (id) DO UPDATE
SET external_reference = EXCLUDED.external_reference;

INSERT INTO tenant_subject_roles (tenant_id, subject_id, role)
VALUES
  ('00000000-0000-4000-8000-000000000001', 'local-user', 'operator'),
  ('00000000-0000-4000-8000-000000000001', 'local-user', 'finance')
ON CONFLICT DO NOTHING;

INSERT INTO tenant_funding_policies (
  tenant_id,
  currency,
  mode,
  finance_activated,
  policy_version,
  per_command_minor,
  operator_rolling_24h_minor,
  tenant_rolling_24h_minor
) VALUES (
  '00000000-0000-4000-8000-000000000001',
  'INR',
  'local_demo_single_operator',
  false,
  1,
  10000000,
  50000000,
  500000000
)
ON CONFLICT (tenant_id, currency) DO UPDATE SET
  mode = EXCLUDED.mode,
  finance_activated = EXCLUDED.finance_activated,
  per_command_minor = EXCLUDED.per_command_minor,
  operator_rolling_24h_minor = EXCLUDED.operator_rolling_24h_minor,
  tenant_rolling_24h_minor = EXCLUDED.tenant_rolling_24h_minor,
  updated_at = now();

INSERT INTO tenant_transfer_policies (
  tenant_id,
  currency,
  minimum_transfer_minor,
  maximum_transfer_minor,
  actor_rolling_24h_minor,
  source_account_rolling_24h_minor,
  tenant_rolling_24h_minor
) VALUES (
  '00000000-0000-4000-8000-000000000001',
  'INR',
  100,
  10000000,
  50000000,
  50000000,
  500000000
)
ON CONFLICT (tenant_id) DO UPDATE SET
  currency = EXCLUDED.currency,
  minimum_transfer_minor = EXCLUDED.minimum_transfer_minor,
  maximum_transfer_minor = EXCLUDED.maximum_transfer_minor,
  actor_rolling_24h_minor = EXCLUDED.actor_rolling_24h_minor,
  source_account_rolling_24h_minor = EXCLUDED.source_account_rolling_24h_minor,
  tenant_rolling_24h_minor = EXCLUDED.tenant_rolling_24h_minor,
  updated_at = now();
