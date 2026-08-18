# PostgreSQL outage or primary-read failure

1. Declare customer impact if primary fallback or transfer writes fail. Keep Redis read data marked non-authoritative.
2. Stop retry storms at the gateway; clients may retry only with their original idempotency key.
3. Check managed PostgreSQL health, connection pool pressure, storage/WAL capacity, and recent migration/deployment changes.
4. Do not fail over to a replica for transfer writes or requirement-bearing balance reads until the platform owner confirms the promoted primary.
5. After recovery, run tenant-scoped reconciliation for affected tenants and inspect aged/dead outbox events.
6. Rebuild Redis from PostgreSQL projections if needed, then retain correlation IDs and reconciliation evidence.
