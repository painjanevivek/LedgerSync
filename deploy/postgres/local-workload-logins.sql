-- Local Compose only. Passwords are read from the protected runtime.env by
-- psql and are never stored in this file or passed as command-line arguments.
\set ON_ERROR_STOP on
\getenv api_password LEDGERSYNC_API_DATABASE_PASSWORD
\getenv worker_password LEDGERSYNC_WORKER_DATABASE_PASSWORD

SELECT :'api_password' ~ '^[a-f0-9]{64}$'
   AND :'worker_password' ~ '^[a-f0-9]{64}$'
   AND :'api_password' <> :'worker_password' AS workload_passwords_valid \gset
\if :workload_passwords_valid
\else
  \echo 'Local workload database credentials are missing, malformed, or identical.'
  SELECT 1 / 0 AS invalid_workload_credentials;
\endif

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'ledgersync_local_api') THEN
    CREATE ROLE ledgersync_local_api LOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'ledgersync_local_worker') THEN
    CREATE ROLE ledgersync_local_worker LOGIN;
  END IF;
END $$;

ALTER ROLE ledgersync_local_api WITH LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'api_password';
ALTER ROLE ledgersync_local_worker WITH LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'worker_password';
ALTER ROLE ledgersync_local_api RESET ALL;
ALTER ROLE ledgersync_local_worker RESET ALL;

-- These repository-owned identities receive authority only through one group
-- membership. Remove stale memberships before re-granting the intended role.
DO $$
DECLARE
  membership record;
BEGIN
  FOR membership IN
    SELECT member.rolname AS member_name, granted.rolname AS granted_name
    FROM pg_auth_members memberships
    JOIN pg_roles granted ON granted.oid = memberships.roleid
    JOIN pg_roles member ON member.oid = memberships.member
    WHERE member.rolname IN ('ledgersync_local_api', 'ledgersync_local_worker')
  LOOP
    EXECUTE format('REVOKE %I FROM %I', membership.granted_name, membership.member_name);
  END LOOP;
END $$;

REVOKE ALL PRIVILEGES ON DATABASE ledgersync FROM ledgersync_local_api, ledgersync_local_worker;
REVOKE ALL PRIVILEGES ON SCHEMA public FROM ledgersync_local_api, ledgersync_local_worker;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM ledgersync_local_api, ledgersync_local_worker;
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM ledgersync_local_api, ledgersync_local_worker;
REVOKE ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public FROM ledgersync_local_api, ledgersync_local_worker;

GRANT ledgersync_api TO ledgersync_local_api;
GRANT ledgersync_worker TO ledgersync_local_worker;
