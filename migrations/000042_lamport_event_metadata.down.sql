-- Stop application instances. Prefer forward repair after event publication.
DROP TABLE logical_message_receipts;
DROP TRIGGER outbox_clock_immutable ON outbox_events;
DROP FUNCTION protect_outbox_clock();
DROP TRIGGER outbox_logical_clock ON outbox_events;
DROP FUNCTION stamp_outbox_logical_clock();
ALTER TABLE outbox_publication_attempts DROP COLUMN send_counter,DROP COLUMN receive_counter,DROP COLUMN node_id;
ALTER TABLE outbox_events DROP CONSTRAINT outbox_clock_pair,DROP COLUMN logical_timestamp,DROP COLUMN parent_reference,DROP COLUMN node_id;
DROP FUNCTION advance_logical_clock(text,bigint);
DROP TABLE logical_node_clocks;
