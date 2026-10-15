# Replica configuration

- Each container has a unique hostname. The node name includes its service role.
- Restart a container to retain its identity. Replacing a container gives it a new identity.
- If you set `LEDGERSYNC_NODE_ID`, use a different value for each concurrent instance.
- Each API and worker uses at most four PostgreSQL connections in the study.
- Two APIs and two workers use at most 16 connections. Allow four more for a manual reconciliation command.
- The optional two schedulers will add eight connections. PostgreSQL reserves capacity for setup and operator checks.
- Signing keys come from the same protected study secret file. Do not copy them into evidence.
- PostgreSQL shares assertion replay checks, rate limits, command idempotency, and financial state.
- Run migrations once during setup. Scale existing services with `up --no-deps` after setup.
- Each study API and worker has a one-CPU limit and 256 MiB of memory. Total service limits increase when replicas increase.

The host is one failure domain. These checks do not establish availability across machines.
