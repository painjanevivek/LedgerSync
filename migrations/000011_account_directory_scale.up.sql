-- Stable bounded account-directory reads for the 10,000-account pilot target.
-- Existing tenant/subject ownership indexes remain the authorization entry point.

CREATE INDEX accounts_tenant_created_stable_idx
  ON accounts (tenant_id,created_at ASC,id ASC)
  INCLUDE (currency,status,display_name,category,external_reference);

CREATE INDEX accounts_tenant_status_created_stable_idx
  ON accounts (tenant_id,status,created_at ASC,id ASC);

CREATE INDEX accounts_tenant_category_created_stable_idx
  ON accounts (tenant_id,(COALESCE(category,'operating')),created_at ASC,id ASC);

CREATE INDEX accounts_tenant_display_name_prefix_idx
  ON accounts (tenant_id,(lower(COALESCE(display_name,''))) text_pattern_ops);

CREATE INDEX accounts_tenant_external_reference_prefix_idx
  ON accounts (tenant_id,(lower(COALESCE(external_reference,''))) text_pattern_ops);

CREATE INDEX audit_events_account_context_idx
  ON audit_events (tenant_id,target_id,occurred_at DESC,id DESC)
  WHERE target_type='account';
