-- Stop all workers before downgrade. Prefer a forward repair after use.
DROP TABLE outbox_publication_attempts;
ALTER TABLE webhook_endpoint_verification_jobs DROP COLUMN claim_generation;
ALTER TABLE webhook_delivery_jobs DROP COLUMN claim_generation;
ALTER TABLE outbox_events DROP COLUMN claim_generation;
