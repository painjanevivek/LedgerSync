# Local MVP Phase 4 — persistence and recovery evidence

**Result:** `PASSED`

**Executed:** 2026-08-24 on the local Windows/Docker Desktop workstation

**Evidence binding:** the Git commit containing this document

## What was proved

LedgerSync can preserve the workstation's PostgreSQL authority, create an
atomic digest-bound logical backup, reject corruption before database writes,
restore into a separate internal Compose project, rebuild disposable Redis
state, and recover from controlled dependency faults without changing the
authoritative financial fingerprint.

## Persistence and status contract

- Default Compose project: `compose`.
- Authoritative volume: `compose_postgres-data`.
- Disposable continuity volume: `compose_redis-data`.
- Ordinary `stop`, image rebuild, dependency restart, and full ordered restart
  preserved PostgreSQL counts and opaque balance/transfer/posting fingerprints.
- Demo seed reruns remained monotonic and did not rewind an existing projection.
- The bounded status command reported migration
  `000012_transfer_velocity_capacity.up.sql` (12 applied), zero dead outbox
  events, and the latest zero-mismatch reconciliation without printing secrets
  or raw financial rows.

## Backup evidence

The first full workstation bundle reported:

| Field | Observed |
|---|---:|
| Format | `ledgersync-local-backup/v1` |
| Accounts | 6 |
| Transfers | 140,590 |
| Immutable postings | 281,178 |
| Dump bytes | 80,016,875 |
| Migration count | 12 |
| Manifest/dump SHA-256 match | Passed |
| Atomic partial-to-final validation | Passed |
| Bounded exact-path retention | Passed |

The dump and manifest counts were read from the same exported repeatable-read
PostgreSQL snapshot, so concurrent transfers cannot create false count drift.
The dump remains under the Git-ignored `data/local-backups` directory and is
not part of this evidence or repository history.

## Isolated restore evidence

- A byte was flipped in a temporary copy; SHA-256 validation rejected it before
  any restore container, network, or volume was created.
- The valid 80 MB dump restored into project
  `ledgersync-restore-20260824114300-bc656504` with a new PostgreSQL volume and
  internal-only network.
- Restored migration version/count and all three manifest counts matched.
- Zero invalid journals, zero posted transfers without a journal, and zero
  negative projections were observed.
- Redis rebuilt to 6 account keys solely from PostgreSQL projections.
- Reconciliation run `7710a2da-2227-47fd-bcda-b7775db225a4` finished `matched`
  with 0 mismatches across 6 accounts.
- The measured local logical restore/reconcile duration was 21.77 seconds. This
  is workstation evidence, not a production RTO promise.
- The normal project's opaque fingerprint and exact volume set were unchanged,
  and the isolated project/volume/network were removed.

## Controlled fault evidence

| Fault | Required behavior | Result |
|---|---|---|
| Redis flush | Rebuild from PostgreSQL; no financial mutation | Passed; 7 operational/cache keys after rebuild |
| Redis unavailable | Authoritative PostgreSQL fallback | Passed |
| Outbox worker restart | Resume with durable outbox authority | Passed |
| API and web restart | Stateless recovery and real BFF reads | Passed |
| PostgreSQL unavailable | Sanitized, truthful temporary state | Passed; HTTP 503 `account_directory_unavailable` |
| Complete dependency stop | Compose ordering restores all services | Passed |
| Final reconciliation | Zero mismatch/dead delivery evidence | Passed; 0 mismatches, 0 dead outbox events |
| Final financial fingerprint | Identical to pre-fault authority | Passed |

Existing live fault tests additionally enforce expired worker-lease recovery,
Redis publication rescheduling without ledger mutation, duplicate projection
monotonicity, delayed read-your-writes fallback, and mismatch persistence. The
real-stack suite enforces lost-response replay after API restart.

## CI enforcement

The real-stack quality job now runs the backup, corruption guard, isolated
restore, Redis rebuild, reconciliation, controlled dependency suite, and exact
cleanup on every pushed `main` commit. The release remains blocked if any
digest, count, invariant, fingerprint, outbox, availability, or reconciliation
check fails.

## Boundary

This is a local logical backup and recovery proof. It does not claim continuous
WAL archiving, managed PITR, encrypted provider snapshots, an off-machine copy,
or a production RPO/RTO. Those remain external production-pilot gates.
