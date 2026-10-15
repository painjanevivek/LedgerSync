-- Session lookup precedes tenant discovery. A transaction must supply exactly
-- the opaque token digest. Preferences use the normal tenant AND actor scope.
-- Settings prevent omitted filters; the authenticated BFF remains trusted to
-- choose claims. No setting is an identity credential for a compromised API.
DO $$ BEGIN
 IF to_regrole('ledgersync_migration_owner') IS NULL THEN
  CREATE ROLE ledgersync_migration_owner NOLOGIN NOSUPERUSER NOBYPASSRLS;
 END IF;
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname='ledgersync_migration_owner'
   AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb OR rolreplication))
   OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=to_regrole('ledgersync_migration_owner')) THEN
  RAISE EXCEPTION 'unsafe session retention owner' USING ERRCODE='42501';
 END IF;
END $$;
ALTER TABLE public.bff_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.bff_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY session_digest_scope ON public.bff_sessions
 USING (encode(token_digest,'hex')=current_setting('app.bff_session_digest',true)
     OR encode(token_digest,'hex')=current_setting('app.bff_rotation_digest',true))
 WITH CHECK (encode(token_digest,'hex')=current_setting('app.bff_session_digest',true)
     OR encode(token_digest,'hex')=current_setting('app.bff_rotation_digest',true));

ALTER TABLE public.operator_ui_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.operator_ui_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_actor_scope ON public.operator_ui_preferences
 USING (public.tenant_scope_allows(tenant_id) AND subject_id=current_setting('app.actor_id',true))
 WITH CHECK (public.tenant_scope_allows(tenant_id) AND subject_id=current_setting('app.actor_id',true));

-- Expiry cleanup exposes only a count and can remove at most 100 expired or
-- long-revoked records for the current operator. It cannot enumerate sessions.
CREATE POLICY session_retention_scope ON public.bff_sessions TO ledgersync_migration_owner
 USING (public.tenant_scope_allows(tenant_id) AND subject_id=current_setting('app.actor_id',true));
GRANT SELECT,DELETE ON public.bff_sessions TO ledgersync_migration_owner;
GRANT UPDATE(token_digest) ON public.bff_sessions TO ledgersync_migration_owner;
CREATE FUNCTION public.controlled_retire_bff_sessions_v1(p_tenant uuid,p_actor text) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE retired bigint;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true
    OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true) THEN
  RAISE EXCEPTION 'session retention scope denied' USING ERRCODE='42501';
 END IF;
 DELETE FROM public.bff_sessions WHERE token_digest IN (
  SELECT token_digest FROM public.bff_sessions
  WHERE tenant_id=p_tenant AND subject_id=p_actor
    AND (expires_at<=now() OR revoked_at<=now()-interval '1 hour')
  ORDER BY expires_at,token_digest LIMIT 100 FOR UPDATE SKIP LOCKED
 );
 GET DIAGNOSTICS retired=ROW_COUNT;
 RETURN retired;
END $$;
ALTER FUNCTION public.controlled_retire_bff_sessions_v1(uuid,text) OWNER TO ledgersync_migration_owner;
REVOKE ALL ON FUNCTION public.controlled_retire_bff_sessions_v1(uuid,text) FROM PUBLIC;
DO $$ BEGIN
 IF to_regrole('ledgersync_api') IS NOT NULL THEN
  REVOKE DELETE ON public.bff_sessions FROM ledgersync_api;
  GRANT EXECUTE ON FUNCTION public.controlled_retire_bff_sessions_v1(uuid,text) TO ledgersync_api;
 END IF;
END $$;
