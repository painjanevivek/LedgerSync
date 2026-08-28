-- Apply the canonical transfer aggregate shape only after the command-schema
-- migrations have added aggregate_type, aggregate_id, and funding_event_id.
-- This keeps upgrades from the retained Phase 7 snapshot forward-compatible.
ALTER TABLE outbox_events DROP CONSTRAINT IF EXISTS outbox_command_aggregate_consistency;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_command_aggregate_consistency CHECK (
  (aggregate_type='account_balance' AND account_id IS NOT NULL AND aggregate_id=account_id
    AND ((transfer_id IS NOT NULL AND funding_event_id IS NULL) OR (transfer_id IS NULL AND funding_event_id IS NOT NULL)))
  OR (aggregate_type='account' AND transfer_id IS NULL AND funding_event_id IS NULL AND account_id IS NOT NULL AND aggregate_id=account_id)
  OR (aggregate_type='funding_event' AND transfer_id IS NULL AND funding_event_id IS NOT NULL AND aggregate_id=funding_event_id)
  OR (aggregate_type='transfer' AND transfer_id IS NOT NULL AND funding_event_id IS NULL AND account_id IS NULL AND aggregate_id=transfer_id)
  OR (aggregate_type NOT IN ('account_balance','account','funding_event','transfer') AND transfer_id IS NULL AND funding_event_id IS NULL)
);
