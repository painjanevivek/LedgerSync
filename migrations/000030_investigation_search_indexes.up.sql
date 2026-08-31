CREATE INDEX audit_events_tenant_correlation_search_idx
  ON audit_events (tenant_id,correlation_id,occurred_at DESC,id DESC)
  INCLUDE (target_type,target_id,outcome);

CREATE INDEX reconciliation_runs_tenant_correlation_search_idx
  ON reconciliation_runs (tenant_id,correlation_id,completed_at DESC,id DESC)
  INCLUDE (status);

CREATE INDEX funding_events_tenant_correlation_search_idx
  ON funding_events (tenant_id,correlation_id,updated_at DESC,id DESC)
  INCLUDE (destination_account_id,status);

CREATE INDEX transfer_corrections_tenant_correlation_search_idx
  ON transfer_corrections (tenant_id,correlation_id,updated_at DESC,id DESC)
  INCLUDE (original_transfer_id,status);
