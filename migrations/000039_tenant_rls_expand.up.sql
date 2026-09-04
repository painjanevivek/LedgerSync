-- Expand tenant isolation without breaking an older application revision.
-- A missing context is temporarily allowed; once present it is authoritative
-- and mismatched reads or writes fail closed. Migration 000040 removes the
-- compatibility branch after every workload path sets transaction context.

CREATE FUNCTION tenant_context_allows_v1(row_tenant_id UUID)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
SET search_path = pg_catalog, public
AS $$
  SELECT CASE
    WHEN NULLIF(current_setting('ledgersync.tenant_id', true), '') IS NULL THEN TRUE
    ELSE row_tenant_id::text = current_setting('ledgersync.tenant_id', true)
  END
$$;

CREATE FUNCTION enforce_tenant_context_if_present_v1()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_context TEXT := NULLIF(current_setting('ledgersync.tenant_id', true), '');
  v_row_tenant UUID;
BEGIN
  v_row_tenant := CASE WHEN TG_OP='DELETE' THEN OLD.tenant_id ELSE NEW.tenant_id END;
  IF v_context IS NOT NULL AND v_row_tenant::text<>v_context THEN
    RAISE EXCEPTION 'database tenant context does not match protected row'
      USING ERRCODE='42501', CONSTRAINT='tenant_context_mismatch';
  END IF;
  RETURN CASE WHEN TG_OP='DELETE' THEN OLD ELSE NEW END;
END;
$$;

COMMENT ON FUNCTION tenant_context_allows_v1(UUID)
  IS 'PR010 expand policy: missing context is temporarily compatible; a present context must match';
COMMENT ON FUNCTION enforce_tenant_context_if_present_v1()
  IS 'Rejects protected-row mutations whose tenant differs from transaction-local context';

DO $$
DECLARE
  v_table TEXT;
BEGIN
  FOREACH v_table IN ARRAY ARRAY[
    'accounts','account_owners','account_credit_permissions',
    'transfers','transfer_velocity_events','transfer_velocity_totals',
    'funding_events','funding_velocity_events','transfer_corrections','approval_records',
    'journal_transactions','ledger_postings','audit_events',
    'opening_import_batches','opening_import_rows','opening_import_approvals','opening_import_executions'
  ] LOOP
    EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',v_table);
    EXECUTE format(
      'CREATE POLICY tenant_context_expand ON public.%I AS PERMISSIVE FOR ALL USING (public.tenant_context_allows_v1(tenant_id)) WITH CHECK (public.tenant_context_allows_v1(tenant_id))',
      v_table
    );
    EXECUTE format(
      'CREATE TRIGGER enforce_tenant_context BEFORE INSERT OR UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION public.enforce_tenant_context_if_present_v1()',
      v_table
    );
  END LOOP;

  ALTER TABLE public.account_balance_projections ENABLE ROW LEVEL SECURITY;
  CREATE POLICY tenant_context_expand ON public.account_balance_projections AS PERMISSIVE FOR ALL
    USING (EXISTS (
      SELECT 1 FROM public.accounts account
       WHERE account.id=account_balance_projections.account_id
         AND public.tenant_context_allows_v1(account.tenant_id)
    ))
    WITH CHECK (EXISTS (
      SELECT 1 FROM public.accounts account
       WHERE account.id=account_balance_projections.account_id
         AND public.tenant_context_allows_v1(account.tenant_id)
    ));

  ALTER TABLE public.account_opening_balances ENABLE ROW LEVEL SECURITY;
  CREATE POLICY tenant_context_expand ON public.account_opening_balances AS PERMISSIVE FOR ALL
    USING (EXISTS (
      SELECT 1 FROM public.accounts account
       WHERE account.id=account_opening_balances.account_id
         AND public.tenant_context_allows_v1(account.tenant_id)
    ))
    WITH CHECK (EXISTS (
      SELECT 1 FROM public.accounts account
       WHERE account.id=account_opening_balances.account_id
         AND public.tenant_context_allows_v1(account.tenant_id)
    ));
END;
$$;

REVOKE ALL ON FUNCTION enforce_tenant_context_if_present_v1() FROM PUBLIC;
