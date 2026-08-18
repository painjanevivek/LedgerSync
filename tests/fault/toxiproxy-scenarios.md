# Toxiproxy fault scenarios

This profile is purposely isolated from the regular development stack. It lets
an operator inject dependency faults without changing application code.

```powershell
docker compose -f deploy/compose/docker-compose.fault.yml up -d
$env:LEDGERSYNC_TEST_DATABASE_URL = 'postgres://ledgersync:fault-test-only@localhost:15432/ledgersync?sslmode=disable'
$env:LEDGERSYNC_TEST_REDIS_ADDR = 'localhost:16379'
go test ./tests/fault -count=1 -v
```

Use the Toxiproxy API only during the scenario, then remove the toxic and run
the same test again. A successful recovery must never change committed ledger
postings or silently report an old balance as current.

| Scenario | API action | Expected evidence |
|---|---|---|
| PostgreSQL timeout while writing | `POST /proxies/postgres/toxics` with downstream timeout | Transfer reports no fabricated success; retry uses idempotency outcome after recovery. |
| Redis connection reset | Add a reset-peer toxic to `redis` | PostgreSQL remains authoritative; cache rebuild path succeeds. |
| Slow balance projection | Add latency to `redis` | Requirement-bearing balance read waits/falls back or returns truthful temporary unavailability. |
| Worker interruption | Disable `redis`, then re-enable it | Expired outbox lease is reclaimed; no duplicate ledger movement. |

Example temporary Redis latency:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:18474/proxies/redis/toxics -ContentType 'application/json' -Body '{"name":"delay","type":"latency","stream":"downstream","attributes":{"latency":1500,"jitter":0}}'
Invoke-RestMethod -Method Delete -Uri http://127.0.0.1:18474/proxies/redis/toxics/delay
docker compose -f deploy/compose/docker-compose.fault.yml down -v
```
