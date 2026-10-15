-- Durable, immutable confirmed intent. Preparing never moves or reserves money.
CREATE TABLE transfer_recovery_intents (
 tenant_id UUID NOT NULL REFERENCES tenants(id),
 actor_subject_id TEXT NOT NULL CHECK (char_length(actor_subject_id) BETWEEN 1 AND 256),
 request_reference UUID NOT NULL,
 idempotency_key TEXT NOT NULL CHECK (idempotency_key ~ '^[!-~]{16,255}$'),
 source_account_id UUID NOT NULL,
 destination_account_id UUID NOT NULL,
 currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
 amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
 request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint)=32),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CONSTRAINT recovery_reference_unique PRIMARY KEY (tenant_id,actor_subject_id,request_reference),
 CONSTRAINT recovery_key_unique UNIQUE (tenant_id,actor_subject_id,idempotency_key),
 FOREIGN KEY (source_account_id,tenant_id) REFERENCES accounts(id,tenant_id),
 FOREIGN KEY (destination_account_id,tenant_id) REFERENCES accounts(id,tenant_id),
 CHECK (source_account_id<>destination_account_id)
);
CREATE INDEX transfer_recovery_actor_time_idx ON transfer_recovery_intents(tenant_id,actor_subject_id,created_at DESC,request_reference DESC);
CREATE TRIGGER transfer_recovery_immutable BEFORE UPDATE OR DELETE ON transfer_recovery_intents
 FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE FUNCTION controlled_prepare_transfer_recovery_v1(
 p_tenant UUID,p_actor TEXT,p_reference UUID,p_key TEXT,p_source UUID,p_destination UUID,
 p_currency TEXT,p_amount BIGINT,p_fingerprint BYTEA
) RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE v_existing public.transfer_recovery_intents; v_outcome public.idempotency_requests;
BEGIN
 IF p_reference IS NULL OR p_key IS NULL OR p_key !~ '^[!-~]{16,255}$'
    OR p_actor IS NULL OR btrim(p_actor)<>p_actor OR char_length(p_actor) NOT BETWEEN 1 AND 256
    OR p_source IS NULL OR p_destination IS NULL OR p_source=p_destination
    OR p_amount IS NULL OR p_amount<=0 OR p_currency IS NULL OR p_currency !~ '^[A-Z]{3}$'
    OR p_fingerprint IS NULL OR p_fingerprint<>digest(concat_ws(E'\n',p_tenant::text,p_actor,
       'transfers.create.v1',p_source::text,p_destination::text,p_currency,p_amount::text),'sha256') THEN
   RAISE EXCEPTION 'invalid recovery intent' USING ERRCODE='22023',CONSTRAINT='controlled_transfer_input';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role IN ('operator','finance'))
    OR NOT EXISTS(SELECT 1 FROM account_owners WHERE tenant_id=p_tenant AND subject_id=p_actor AND account_id=p_source AND permission='debit')
    OR NOT EXISTS(SELECT 1 FROM account_credit_permissions WHERE tenant_id=p_tenant AND subject_id=p_actor AND account_id=p_destination) THEN
   RAISE EXCEPTION 'recovery intent authorization denied' USING ERRCODE='42501',CONSTRAINT='controlled_transfer_actor';
 END IF;
 -- One actor/key lock serializes competing prepares. Uniqueness also protects references.
 PERFORM pg_advisory_xact_lock(hashtextextended('transfer-recovery|'||p_tenant::text||'|'||p_actor||'|'||p_key,0));
 SELECT * INTO v_existing FROM transfer_recovery_intents
 WHERE tenant_id=p_tenant AND actor_subject_id=p_actor AND (idempotency_key=p_key OR request_reference=p_reference);
 IF FOUND THEN
   IF v_existing.idempotency_key<>p_key OR v_existing.request_reference<>p_reference OR v_existing.request_fingerprint<>p_fingerprint THEN
     RAISE EXCEPTION 'recovery intent conflict' USING ERRCODE='23505',CONSTRAINT='controlled_transfer_idempotency';
   END IF;
   RETURN;
 END IF;
 SELECT * INTO v_outcome FROM idempotency_requests WHERE tenant_id=p_tenant AND actor_subject_id=p_actor
   AND operation='transfers.create.v1' AND idempotency_key=p_key;
 IF FOUND AND (v_outcome.request_fingerprint<>p_fingerprint OR
   (v_outcome.request_reference IS NOT NULL AND v_outcome.request_reference<>p_reference)) THEN
   RAISE EXCEPTION 'recovery outcome conflict' USING ERRCODE='23505',CONSTRAINT='controlled_transfer_idempotency';
 END IF;
 INSERT INTO transfer_recovery_intents(tenant_id,actor_subject_id,request_reference,idempotency_key,
   source_account_id,destination_account_id,currency,amount_minor,request_fingerprint)
 VALUES(p_tenant,p_actor,p_reference,p_key,p_source,p_destination,p_currency,p_amount,p_fingerprint);
END $$;
REVOKE ALL ON FUNCTION controlled_prepare_transfer_recovery_v1(UUID,TEXT,UUID,TEXT,UUID,UUID,TEXT,BIGINT,BYTEA) FROM PUBLIC;
DO $$ BEGIN
 IF to_regrole('ledgersync_api') IS NOT NULL THEN
   GRANT SELECT ON transfer_recovery_intents TO ledgersync_api;
   GRANT EXECUTE ON FUNCTION controlled_prepare_transfer_recovery_v1(UUID,TEXT,UUID,TEXT,UUID,UUID,TEXT,BIGINT,BYTEA) TO ledgersync_api;
 END IF;
END $$;
