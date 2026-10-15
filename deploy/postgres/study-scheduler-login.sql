-- Apply only in the isolated course deployment. No tenant is enabled here.
\set ON_ERROR_STOP on
\getenv scheduler_password LEDGERSYNC_SCHEDULER_DATABASE_PASSWORD
SELECT :'scheduler_password' ~ '^[a-f0-9]{64}$' AS scheduler_password_valid \gset
\if :scheduler_password_valid
\else
 SELECT 1/0 AS invalid_scheduler_credential;
\endif
DO $$ BEGIN CREATE ROLE ledgersync_local_scheduler LOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
ALTER ROLE ledgersync_local_scheduler WITH LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'scheduler_password';
ALTER ROLE ledgersync_local_scheduler RESET ALL;
DO $$ DECLARE membership record; BEGIN
 FOR membership IN SELECT granted.rolname AS name FROM pg_auth_members memberships JOIN pg_roles granted ON granted.oid=memberships.roleid JOIN pg_roles member ON member.oid=memberships.member WHERE member.rolname='ledgersync_local_scheduler' LOOP
  EXECUTE format('REVOKE %I FROM ledgersync_local_scheduler',membership.name);
 END LOOP;
END $$;
REVOKE ALL ON DATABASE ledgersync FROM ledgersync_local_scheduler;
REVOKE ALL ON SCHEMA public,ledgersync_scheduler FROM ledgersync_local_scheduler;
REVOKE ALL ON ALL TABLES IN SCHEMA public,ledgersync_scheduler FROM ledgersync_local_scheduler;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM ledgersync_local_scheduler;
GRANT ledgersync_scheduler TO ledgersync_local_scheduler;
