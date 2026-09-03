-- Bounded operator webhook evidence reads use stable tenant/status/time and
-- endpoint-attempt access paths. These indexes do not alter delivery state.

CREATE INDEX developer_webhook_endpoints_tenant_status_updated_idx
  ON developer_webhook_endpoints (tenant_id,status,updated_at DESC,id DESC);

CREATE INDEX developer_webhook_endpoints_subscriptions_idx
  ON developer_webhook_endpoints USING GIN (subscribed_events);

CREATE INDEX delivery_attempts_webhook_endpoint_recent_idx
  ON delivery_attempts (tenant_id,endpoint_reference,created_at DESC,id DESC)
  WHERE delivery_kind='webhook';

CREATE INDEX delivery_attempts_webhook_event_endpoint_idx
  ON delivery_attempts (tenant_id,outbox_event_id,endpoint_reference)
  WHERE delivery_kind='webhook' AND outbox_event_id IS NOT NULL;
