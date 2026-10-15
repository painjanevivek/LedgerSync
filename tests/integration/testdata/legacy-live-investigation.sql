-- Proven preserved PostgreSQL 16 legacy live-room schema, before current 000037.
-- Catalog definitions were captured without data, owners, ACLs, or credentials.
-- BETWEEN retains the original CHECK parse tree; pg_dump's expanded comparisons
-- can flatten its AND grouping on restore even when their meaning is unchanged.
ALTER TABLE idempotency_requests ADD COLUMN request_reference UUID;
CREATE UNIQUE INDEX idempotency_request_reference_unique ON idempotency_requests (tenant_id,request_reference);
ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_query_kind_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_query_kind_check CHECK (query_kind IN ('immutable_id','approved_reference','request_reference'));
ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_root_record_type_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_root_record_type_check CHECK (root_record_type IN ('account','transfer','transfer_request','funding','event','reconciliation_run','reconciliation_mismatch','correction'));
ALTER TABLE investigation_workspace_references DROP CONSTRAINT investigation_workspace_references_record_type_check;
ALTER TABLE investigation_workspace_references ADD CONSTRAINT investigation_workspace_references_record_type_check CHECK (record_type IN ('account','transfer','transfer_request','funding','event','reconciliation_run','reconciliation_mismatch','correction'));
INSERT INTO schema_migrations(version,applied_at) VALUES ('000034_live_investigation_rooms.up.sql','2026-09-08T05:59:55Z'),('000035_live_investigation_compatibility.up.sql','2026-09-08T12:51:07Z');
CREATE FUNCTION public.reject_live_room_finding_mutation() RETURNS trigger LANGUAGE plpgsql AS $function$
BEGIN
  RAISE EXCEPTION 'live investigation findings are append-only';
END $function$;
--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: investigation_live_room_findings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.investigation_live_room_findings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    finding_code text NOT NULL,
    authoritative_request_state text NOT NULL,
    canonical_transfer_id uuid,
    actor_subject_id text NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    CONSTRAINT investigation_live_room_findings_actor_subject_id_check CHECK (length(actor_subject_id) BETWEEN 1 AND 255 AND actor_subject_id=btrim(actor_subject_id) AND actor_subject_id !~ '[[:cntrl:]]'),
    CONSTRAINT investigation_live_room_findings_finding_code_check CHECK ((finding_code = ANY (ARRAY['confirmed_completed'::text, 'confirmed_rejected'::text, 'still_unresolved'::text, 'escalated'::text]))),
    CONSTRAINT investigation_live_room_findings_request_status_check CHECK ((authoritative_request_state = ANY (ARRAY['in_progress'::text, 'completed'::text, 'rejected'::text, 'unavailable'::text, 'expired'::text])))
);


--
-- Name: investigation_live_room_operations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.investigation_live_room_operations (
    id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    actor_subject_id text NOT NULL,
    operation text NOT NULL,
    idempotency_key_hash bytea NOT NULL,
    request_fingerprint bytea NOT NULL,
    room_id uuid NOT NULL,
    response_body jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT investigation_live_room_operations_actor_subject_id_check CHECK (((length(actor_subject_id) >= 1) AND (length(actor_subject_id) <= 255))),
    CONSTRAINT investigation_live_room_operations_idempotency_key_hash_check CHECK ((octet_length(idempotency_key_hash) = 32)),
    CONSTRAINT investigation_live_room_operations_operation_check CHECK ((operation = ANY (ARRAY['start'::text, 'join'::text, 'end'::text, 'finding'::text]))),
    CONSTRAINT investigation_live_room_operations_request_fingerprint_check CHECK ((octet_length(request_fingerprint) = 32))
);


--
-- Name: investigation_live_rooms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.investigation_live_rooms (
    id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    investigation_id uuid NOT NULL,
    transfer_request_reference uuid NOT NULL,
    owner_subject_id text NOT NULL,
    peer_subject_id text,
    status text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    invite_token_hash bytea,
    invite_expires_at timestamp with time zone,
    invite_redeemed_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL,
    joined_at timestamp with time zone,
    ended_at timestamp with time zone,
    CONSTRAINT investigation_live_rooms_check CHECK (peer_subject_id IS NULL OR (length(peer_subject_id) BETWEEN 1 AND 255 AND peer_subject_id=btrim(peer_subject_id) AND peer_subject_id !~ '[[:cntrl:]]' AND peer_subject_id<>owner_subject_id)),
    CONSTRAINT investigation_live_rooms_check1 CHECK ((invite_expires_at <= expires_at)),
    CONSTRAINT investigation_live_rooms_check2 CHECK ((((status = ANY (ARRAY['waiting'::text, 'active'::text])) AND (ended_at IS NULL)) OR ((status = ANY (ARRAY['ended'::text, 'expired'::text])) AND (ended_at IS NOT NULL)))),
    CONSTRAINT investigation_live_rooms_check3 CHECK ((((status = 'waiting'::text) AND (peer_subject_id IS NULL) AND (joined_at IS NULL) AND (invite_redeemed_at IS NULL)) OR (status <> 'waiting'::text))),
    CONSTRAINT investigation_live_rooms_invite_digest_check CHECK ((octet_length(invite_token_hash) = 32)),
    CONSTRAINT investigation_live_rooms_owner_subject_id_check CHECK (length(owner_subject_id) BETWEEN 1 AND 255 AND owner_subject_id=btrim(owner_subject_id) AND owner_subject_id !~ '[[:cntrl:]]'),
    CONSTRAINT investigation_live_rooms_status_check CHECK ((status = ANY (ARRAY['waiting'::text, 'active'::text, 'ended'::text, 'expired'::text]))),
    CONSTRAINT investigation_live_rooms_version_check CHECK ((version > 0))
);


--
-- Name: investigation_live_room_findings investigation_live_room_findings_authoritative_check; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.investigation_live_room_findings
    ADD CONSTRAINT investigation_live_room_findings_authoritative_check CHECK ((((finding_code = 'confirmed_completed'::text) AND (authoritative_request_state = 'completed'::text) AND (canonical_transfer_id IS NOT NULL)) OR ((finding_code = 'confirmed_rejected'::text) AND (authoritative_request_state = 'rejected'::text) AND (canonical_transfer_id IS NULL)) OR ((finding_code = ANY (ARRAY['still_unresolved'::text, 'escalated'::text])) AND (authoritative_request_state = ANY (ARRAY['in_progress'::text, 'unavailable'::text, 'expired'::text])) AND (canonical_transfer_id IS NULL)))) NOT VALID;


--
-- Name: investigation_live_room_findings investigation_live_room_findings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_findings
    ADD CONSTRAINT investigation_live_room_findings_pkey PRIMARY KEY (id);


--
-- Name: investigation_live_room_findings investigation_live_room_findings_room_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_findings
    ADD CONSTRAINT investigation_live_room_findings_room_id_key UNIQUE (room_id);


--
-- Name: investigation_live_room_operations investigation_live_room_opera_tenant_id_actor_subject_id_op_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_operations
    ADD CONSTRAINT investigation_live_room_opera_tenant_id_actor_subject_id_op_key UNIQUE (tenant_id, actor_subject_id, operation, idempotency_key_hash);


--
-- Name: investigation_live_room_operations investigation_live_room_operations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_operations
    ADD CONSTRAINT investigation_live_room_operations_pkey PRIMARY KEY (id);


--
-- Name: investigation_live_rooms investigation_live_rooms_id_tenant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_rooms
    ADD CONSTRAINT investigation_live_rooms_id_tenant_id_key UNIQUE (id, tenant_id);


--
-- Name: investigation_live_rooms investigation_live_rooms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_rooms
    ADD CONSTRAINT investigation_live_rooms_pkey PRIMARY KEY (id);


--
-- Name: investigation_live_rooms_active_request_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX investigation_live_rooms_active_request_idx ON public.investigation_live_rooms USING btree (tenant_id, transfer_request_reference) WHERE (status = ANY (ARRAY['waiting'::text, 'active'::text]));


--
-- Name: investigation_live_rooms_active_workspace_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX investigation_live_rooms_active_workspace_idx ON public.investigation_live_rooms USING btree (tenant_id, investigation_id) WHERE (status = ANY (ARRAY['waiting'::text, 'active'::text]));


--
-- Name: investigation_live_rooms_peer_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX investigation_live_rooms_peer_idx ON public.investigation_live_rooms USING btree (tenant_id, peer_subject_id, expires_at) WHERE (peer_subject_id IS NOT NULL);


--
-- Name: investigation_live_room_findings investigation_live_room_findings_immutable_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER investigation_live_room_findings_immutable_update BEFORE DELETE OR UPDATE ON public.investigation_live_room_findings FOR EACH ROW EXECUTE FUNCTION public.reject_live_room_finding_mutation();


--
-- Name: investigation_live_room_findings investigation_live_room_findings_room_id_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_findings
    ADD CONSTRAINT investigation_live_room_findings_room_id_tenant_id_fkey FOREIGN KEY (room_id, tenant_id) REFERENCES public.investigation_live_rooms(id, tenant_id);


--
-- Name: investigation_live_room_findings investigation_live_room_findings_transfer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_findings
    ADD CONSTRAINT investigation_live_room_findings_transfer_id_fkey FOREIGN KEY (canonical_transfer_id) REFERENCES public.transfers(id);


--
-- Name: investigation_live_room_operations investigation_live_room_operations_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_operations
    ADD CONSTRAINT investigation_live_room_operations_room_id_fkey FOREIGN KEY (room_id) REFERENCES public.investigation_live_rooms(id);


--
-- Name: investigation_live_room_operations investigation_live_room_operations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_room_operations
    ADD CONSTRAINT investigation_live_room_operations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: investigation_live_rooms investigation_live_rooms_investigation_id_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_rooms
    ADD CONSTRAINT investigation_live_rooms_investigation_id_tenant_id_fkey FOREIGN KEY (investigation_id, tenant_id) REFERENCES public.investigation_workspaces(id, tenant_id);


--
-- Name: investigation_live_rooms investigation_live_rooms_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_rooms
    ADD CONSTRAINT investigation_live_rooms_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: investigation_live_rooms investigation_live_rooms_tenant_id_request_reference_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investigation_live_rooms
    ADD CONSTRAINT investigation_live_rooms_tenant_id_request_reference_fkey FOREIGN KEY (tenant_id, transfer_request_reference) REFERENCES public.idempotency_requests(tenant_id, request_reference);


--
-- PostgreSQL database dump complete
--



SET search_path=public;
