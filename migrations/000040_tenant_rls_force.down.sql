-- Restore the monitored expand phase. Policies and mismatch triggers from
-- 000039 remain enabled while application coverage is repaired forward.
DO $$
DECLARE
  v_table TEXT;
BEGIN
  FOREACH v_table IN ARRAY ARRAY[
    'accounts','account_owners','account_credit_permissions',
    'transfers','transfer_velocity_events','transfer_velocity_totals',
    'funding_events','funding_velocity_events','transfer_corrections','approval_records',
    'journal_transactions','ledger_postings','audit_events',
    'opening_import_batches','opening_import_rows','opening_import_approvals','opening_import_executions',
    'account_balance_projections','account_opening_balances'
  ] LOOP
    EXECUTE format('ALTER TABLE public.%I NO FORCE ROW LEVEL SECURITY', v_table);
  END LOOP;
END
$$;

CREATE OR REPLACE FUNCTION public.tenant_context_allows_v1(row_tenant_id UUID)
RETURNS BOOLEAN
LANGUAGE plpgsql
VOLATILE
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_context TEXT := NULLIF(current_setting('ledgersync.tenant_id', true), '');
BEGIN
  IF v_context IS NULL THEN
    IF NULLIF(current_setting('ledgersync.missing_context_observed', true), '') IS NULL THEN
      PERFORM set_config('ledgersync.missing_context_observed', 'true', true);
      RAISE LOG 'ledgersync tenant context missing database=% role=% application=% operation=%',
        current_database(), current_user,
        CASE WHEN current_setting('application_name') ~ '^ledgersync-[a-z0-9_-]+-[a-zA-Z0-9._-]{1,24}$' THEN current_setting('application_name') ELSE 'unknown' END,
        CASE WHEN current_setting('ledgersync.operation', true) ~ '^[a-z][a-z0-9_]{0,47}$' THEN current_setting('ledgersync.operation', true) ELSE 'unknown' END;
    END IF;
    RETURN TRUE;
  END IF;
  RETURN row_tenant_id::text = v_context;
END
$$;

COMMENT ON FUNCTION public.tenant_context_allows_v1(UUID)
  IS 'PR010 expand policy: missing context is temporarily compatible; a present context must match';
