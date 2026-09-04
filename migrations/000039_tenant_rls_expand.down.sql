DO $$
DECLARE
  v_table TEXT;
BEGIN
  FOREACH v_table IN ARRAY ARRAY[
    'accounts','account_owners','account_credit_permissions',
    'account_balance_projections','account_opening_balances',
    'transfers','transfer_velocity_events','transfer_velocity_totals',
    'funding_events','funding_velocity_events','transfer_corrections','approval_records',
    'journal_transactions','ledger_postings','audit_events',
    'opening_import_batches','opening_import_rows','opening_import_approvals','opening_import_executions'
  ] LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_context_expand ON public.%I',v_table);
    EXECUTE format('DROP TRIGGER IF EXISTS enforce_tenant_context ON public.%I',v_table);
    EXECUTE format('ALTER TABLE public.%I DISABLE ROW LEVEL SECURITY',v_table);
  END LOOP;
END;
$$;

DROP FUNCTION enforce_tenant_context_if_present_v1();
DROP FUNCTION tenant_context_allows_v1(UUID);
