CREATE TABLE logical_node_clocks (
 node_id text PRIMARY KEY CHECK (length(node_id) BETWEEN 1 AND 128),
 counter bigint NOT NULL CHECK (counter >= 0),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE FUNCTION advance_logical_clock(p_node_id text,p_received bigint) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE result bigint;
BEGIN
 IF p_node_id IS NULL OR length(p_node_id) NOT BETWEEN 1 AND 128 OR p_received IS NULL OR p_received<0 THEN
  RAISE EXCEPTION 'invalid logical clock input' USING ERRCODE='22023';
 END IF;
 -- bigint overflow raises an error. A counter must never wrap or restart at zero.
 INSERT INTO logical_node_clocks(node_id,counter) VALUES(p_node_id,p_received+1)
 ON CONFLICT(node_id) DO UPDATE SET counter=greatest(logical_node_clocks.counter,p_received)+1,updated_at=clock_timestamp()
 RETURNING counter INTO result;
 RETURN result;
END $$;
REVOKE ALL ON FUNCTION advance_logical_clock(text,bigint) FROM PUBLIC;

ALTER TABLE outbox_events ADD COLUMN node_id text;
ALTER TABLE outbox_events ADD COLUMN logical_timestamp bigint CHECK (logical_timestamp>0);
ALTER TABLE outbox_events ADD COLUMN parent_reference text;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_clock_pair CHECK ((node_id IS NULL)=(logical_timestamp IS NULL));
ALTER TABLE outbox_publication_attempts ADD COLUMN node_id text;
ALTER TABLE outbox_publication_attempts ADD COLUMN receive_counter bigint CHECK(receive_counter>0);
ALTER TABLE outbox_publication_attempts ADD COLUMN send_counter bigint CHECK(send_counter>receive_counter);

CREATE FUNCTION stamp_outbox_logical_clock() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE local_node text := current_setting('application_name',true);
BEGIN
 -- Legacy connections keep nullable metadata. Do not invent historic clocks.
 IF local_node LIKE 'ledgersync/api/%' OR local_node LIKE 'ledgersync/worker/%' THEN
  NEW.node_id:=local_node;
  NEW.logical_timestamp:=advance_logical_clock(local_node,0);
  NEW.parent_reference:=CASE WHEN NEW.transfer_id IS NOT NULL THEN 'transfer/'||NEW.transfer_id::text ELSE NULL END;
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION stamp_outbox_logical_clock() FROM PUBLIC;
CREATE TRIGGER outbox_logical_clock BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION stamp_outbox_logical_clock();

CREATE FUNCTION protect_outbox_clock() RETURNS trigger LANGUAGE plpgsql SET search_path=public,pg_temp AS $$
BEGIN
 IF ROW(NEW.node_id,NEW.logical_timestamp,NEW.parent_reference) IS DISTINCT FROM ROW(OLD.node_id,OLD.logical_timestamp,OLD.parent_reference) THEN
  RAISE EXCEPTION 'immutable event clock metadata' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER outbox_clock_immutable BEFORE UPDATE ON outbox_events FOR EACH ROW EXECUTE FUNCTION protect_outbox_clock();

CREATE TABLE logical_message_receipts (
 node_id text NOT NULL,
 receive_counter bigint NOT NULL CHECK(receive_counter>0),
 tenant_id uuid NOT NULL REFERENCES tenants(id),
 event_id uuid NOT NULL REFERENCES outbox_events(id),
 transport_id text NOT NULL CHECK(length(transport_id) BETWEEN 1 AND 128),
 sender_node_id text,
 sender_counter bigint CHECK(sender_counter>0),
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(node_id,receive_counter),
 CHECK((sender_node_id IS NULL)=(sender_counter IS NULL)),
 CHECK(sender_counter IS NULL OR receive_counter>sender_counter)
);
CREATE INDEX logical_receipts_event ON logical_message_receipts(event_id,received_at DESC);
