-- An expired request stays visible. Finance can reject it. It cannot receive a
-- fresh approval that silently extends the original request window.
DO $migration$
DECLARE routine record; marker text:=E' IF p_decision=''approved'' THEN\n  UPDATE public.funding_events'; guard text; definition text;
BEGIN
 SELECT p.oid,p.prosrc INTO STRICT routine FROM pg_proc p WHERE p.oid='public.controlled_decide_funding_v1(uuid,text,uuid,text,boolean,text,uuid,timestamptz)'::regprocedure;
 IF strpos(routine.prosrc,marker)=0 THEN RAISE EXCEPTION 'unexpected funding decision body'; END IF;
 guard:=E' IF p_decision=''approved'' AND NOT EXISTS(SELECT 1 FROM public.approval_records WHERE tenant_id=p_tenant AND target_id=p_event AND status=''requested'' AND requester_subject_id=f.requester_subject_id AND created_at<=p_at AND expires_at>p_at) THEN\n  RAISE EXCEPTION ''funding request approval window expired'' USING ERRCODE=''42501'',CONSTRAINT=''controlled_funding_forbidden'';\n END IF;\n';
 definition:=replace(pg_get_functiondef(routine.oid),routine.prosrc,replace(routine.prosrc,marker,guard||marker));
 EXECUTE definition;
END $migration$;
