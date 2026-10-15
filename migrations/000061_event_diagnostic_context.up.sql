-- Diagnostic context carries no tenant, actor, amount or authorization data.
-- Historical rows remain NULL. Financial and public event payloads do not change.
ALTER TABLE outbox_events ADD COLUMN diagnostic_traceparent text
 CHECK (diagnostic_traceparent IS NULL OR diagnostic_traceparent ~ '^00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]$');

CREATE FUNCTION preserve_event_diagnostic_context() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE parent text;
BEGIN
 IF TG_OP='INSERT' THEN
  parent:=current_setting('app.diagnostic_traceparent',true);
  NEW.diagnostic_traceparent:=CASE WHEN parent ~ '^00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]$'
    AND substring(parent from 4 for 32)<>repeat('0',32)
    AND substring(parent from 37 for 16)<>repeat('0',16) THEN parent ELSE NULL END;
 ELSIF NEW.diagnostic_traceparent IS DISTINCT FROM OLD.diagnostic_traceparent THEN
  RAISE EXCEPTION 'immutable diagnostic origin' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION preserve_event_diagnostic_context() FROM PUBLIC;
CREATE TRIGGER outbox_diagnostic_context BEFORE INSERT OR UPDATE ON outbox_events
FOR EACH ROW EXECUTE FUNCTION preserve_event_diagnostic_context();
