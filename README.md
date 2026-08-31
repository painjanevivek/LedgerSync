# LedgerSync

> **Every transfer is exact, explainable, and visible when it matters.**

LedgerSync is an API-first, closed-loop ledger platform for fintech and vertical-SaaS teams building wallets, credits, internal payouts, escrow-like balances, and treasury-like account systems. The pilot deliberately covers **internal, same-currency transfers between LedgerSync ledger accounts**; it is not a bank-rail, card, FX, or custody product.

**Release status:** the complete expanded **local-only product is qualified** for one Windows workstation at `http://127.0.0.1:3000`, with a clean INR workspace and no external deployment. Current-main [quality reconvergence evidence](docs/release-evidence/master-phase-1-quality.md) covers exact-commit ledger, browser, CLS, recovery, security, container, and real-stack gates. Phase 7 of the [master completion plan](docs/plans/ledgersync-master-product-system-and-website-completion-plan.md) is active; operator-console IA (M08) and unified design/accessibility (M09) remain partial. See the [master delivery register](docs/plans/ledgersync-master-progress.md) for exact phase truth. This status does not approve LAN, cloud, shared-host, pilot, or production deployment.

## Contents

- [Why LedgerSync](#why-ledgersync)
- [Numerical scorecard](#numerical-scorecard)
- [Architecture diagrams](#architecture-diagrams)
- [Correctness model](#correctness-model)
- [Read-your-writes model](#readyourwrites-model)
- [Security model](#security-model)
- [API surface](#api-surface)
- [Data model](#data-model)
- [Operations and recovery](#operations-and-recovery)
- [Quick start](#quick-start)
- [Developer and operator guides](#developer-and-operator-guides)
- [Evidence, roadmap, and license](#evidence-roadmap-and-license)

---

## Why LedgerSync

Money systems usually lose trust at their edges: a network retry doubles a debit, a cache answers before its balance projection catches up, or a support operator cannot explain a number. LedgerSync treats those as primary product requirements rather than post-launch engineering work.

| Risk | LedgerSync mechanism | Result |
|---|---|---|
| Floating-point drift | Integer minor units + ISO currency | `1250` is exact; no binary float crosses the financial path |
| Lost response / retry | Idempotency key + canonical request fingerprint | Same request replays one final result; changed intent conflicts |
| Partial write | One short PostgreSQL transaction | Transfer, journal, postings, balance versions, audit, idempotency and outbox commit together |
| Stale cache | PostgreSQL authority; Redis as disposable read cache | Current-enough cache read, authoritative fallback, or truthful unavailability |
| Unexplained balance | Immutable double-entry ledger | One debit and one credit prove each posted internal transfer |
| Cross-account access | Tenant/subject/account predicate in every protected query | Missing and inaccessible accounts get the same safe response |
| Browser/internal boundary leak | Same-origin BFF + HttpOnly session | The browser never calls PostgreSQL, Redis, or internal API ports |

```mermaid
flowchart LR
  Partner[Partner product<br/>wallet • credits • payouts] --> API[LedgerSync API / BFF]
  API --> Command{Exact transfer command}
  Command --> PG[(PostgreSQL<br/>financial authority)]
  PG --> Ledger[Immutable debit + credit]
  PG --> Projection[Balance projection<br/>version n + 1]
  PG --> Idempotency[Saved idempotency outcome]
  PG --> Outbox[Transactional outbox]
  Outbox --> Worker[Delivery worker]
  Worker --> Redis[(Redis<br/>disposable cache)]
  API --> Balance[Authorized balance read]
  Balance -->|current enough| Redis
  Balance -->|stale / delayed / absent| PG
```

---

## Numerical scorecard

### Pilot targets — not production measurements

The numbers below are deliberate launch constraints. They are **not claims of observed production performance**.

| Dimension | Number | Definition |
|---|---:|---|
| Design partners | **2–3** | Closed production pilot, not self-serve launch |
| Jurisdictions | **1** | Choose and document before pilot |
| Currencies | **1** | Internal same-currency movement only |
| Pilot account scale | **≥ 10,000** | Meaningful pilot completion threshold |
| Initial partner throughput | **≤ 25 TPS** | Enforced lower launch envelope; 50 TPS is measured local service headroom |
| Transfer latency | **p95 < 500 ms** | Healthy-dependency commit outcome target |
| Balance-read latency | **p95 < 200 ms** | Healthy-dependency cache/primary-read target |
| Reconciliation mismatches | **0** | Non-negotiable pilot exit criterion |
| Double-movement retries | **0** | Idempotency safety invariant |
| Restore drills | **≥ 1** | Completed drill required before pilot exit |

### Capacity planning arithmetic

The table converts the TPS target into volume. It is arithmetic for planning; it is **not** a load-test benchmark.

| Sustained rate | Transfers/min | Transfers/hour | Transfers/day |
|---:|---:|---:|---:|
| 10 TPS | 600 | 36,000 | 864,000 |
| 25 TPS | 1,500 | 90,000 | 2,160,000 |
| 50 TPS | 3,000 | 180,000 | 4,320,000 |

At 50 TPS, the v1 model creates a minimum of **100 ledger postings/s** and **100 account-level outbox events/s** (2 each per transfer), before audit, idempotency, index, and projection work.

```mermaid
xychart-beta
  title "Planning arithmetic: transfers per hour"
  x-axis ["10 TPS", "25 TPS", "50 TPS"]
  y-axis "Transfers/hour" 0 --> 180000
  bar [36000, 90000, 180000]
```

### Reproducible repository evidence

| Evidence | Result | What it proves |
|---|---:|---|
| Complete Phase 10 isolated acceptance | **PASS in 430.09 s** | end-to-end product, faults, recovery, cleanup, and ordinary-stack preservation |
| Five-minute mixed capacity | **7,500 transfers at 25.071 iterations/s** | exact 7,500 journals, 15,000 postings, 7,500 idempotency outcomes, zero safety drift |
| Phase 10 transfer / balance latency | **p95 212.397 / 84.403 ms** | healthy local profile remains below 500 / 200 ms gates |
| Phase 10 security qualification | **20 / 20 steps passed** | tests, vet, fuzz, coverage, dependency/history/IaC/image scans, and three SBOMs |
| Protected isolated restore | **PASS; local RTO 22.38 s** | current schema, cache rebuild, matched reconciliation, ordinary project unchanged |
| Phase 3 PostgreSQL transfer suite | **PASS in 2.613 s** | no overdraft, safe replay, conflict rejection, ledger reconciliation |
| Phase 5 PostgreSQL ownership suite | **PASS in 2.513 s** | cross-account reads denied without disclosure |
| BFF security suite | **3 / 3 tests passed** | session tamper/expiry, CSRF, response headers |
| Phase 4 fault cases | **3 scenarios passed** | delayed projection, Redis loss/rebuild, monotonic cache version |
| Ordered migrations | **29** | financial schema through lifecycle, investigation, recovery, shared replay controls, webhook verification, permanent-post retry binding, bounded approval and webhook evidence read models, and caller-owned delivery replay identity |
| Journal postings / posted transfer | **2** | one debit + one credit |
| Account events / transfer | **2** | one outbox event per affected account |
| Consistency requirement lifetime | **10 min** | signed minimum balance version |
| BFF actor assertion lifetime | **≤ 60 s** | server-to-server actor handoff |

```mermaid
pie title "Documented Phase 3–5 verification categories (10 total controls)"
  "Transfer safety" : 4
  "RYEW and cache recovery" : 3
  "Authorization and BFF security" : 3
```

The chart is evidence coverage, not an availability percentage or a production reliability score.

---

## Architecture diagrams

### 1. Transaction commit boundary

```mermaid
sequenceDiagram
  autonumber
  participant P as Partner / BFF
  participant A as Private API
  participant DB as PostgreSQL
  participant O as Outbox
  participant W as Projector
  participant R as Redis
  P->>A: POST transfer + Idempotency-Key
  A->>A: Verify actor, scope, exact amount and shape
  A->>DB: Begin serializable transaction
  DB->>DB: Reserve idempotency key and fingerprint
  DB->>DB: Lock source + destination in stable ID order
  DB->>DB: Check accounts, available funds, and exact rolling velocity
  DB->>DB: Insert transfer, journal, debit + credit
  DB->>DB: Update balance versions, audit, outcome, outbox
  DB-->>A: Commit once
  A-->>P: Final outcome + minimum balance versions
  O-->>W: Durable balance-change events
  W->>R: Monotonic cache update
```

### 2. Financial source-of-truth hierarchy

```mermaid
flowchart TB
  PG[(PostgreSQL)]:::authority --> L[Ledger postings]:::authority
  PG --> T[Transfer + idempotency outcome]:::authority
  PG --> B[Balance projection + version]:::authority
  PG --> O[Outbox event]:::authority
  O --> W[Worker]
  W --> R[(Redis cache)]:::cache
  R -. rebuild only from .-> B
  classDef authority fill:#0d2f2d,color:#fff,stroke:#10b981,stroke-width:2px;
  classDef cache fill:#5b3b08,color:#fff,stroke:#f59e0b,stroke-width:2px;
```

### 3. Read-your-writes decision

```mermaid
flowchart TD
  Q[Authorized balance request] --> Req{Signed minimum version?}
  Req -->|No| Cache{Usable cache?}
  Req -->|Yes| Version{Cache version ≥ required?}
  Version -->|Yes| Hit[Return cache answer]
  Version -->|No| Wait[Brief bounded wait]
  Wait --> Version2{Now current enough?}
  Version2 -->|Yes| Hit
  Version2 -->|No| Primary[Read PostgreSQL projection]
  Cache -->|Yes| Hit
  Cache -->|No| Primary
  Primary --> Meets{Projection meets minimum?}
  Meets -->|Yes| Truth[Return authoritative answer<br/>and refill cache opportunistically]
  Meets -->|No| Unavailable[Return truthful temporary unavailability]
```

### 4. Failure and retry state machine

```mermaid
stateDiagram-v2
  [*] --> Submitted
  Submitted --> Posted: committed transaction
  Submitted --> Rejected: validation / auth / funds
  Submitted --> InProgress: duplicate is completing
  InProgress --> Posted: retry same key
  Posted --> Posted: matching idempotent replay
  Submitted --> UnknownToClient: network response lost
  UnknownToClient --> Posted: retry same key finds outcome
  UnknownToClient --> Rejected: retry same key finds rejection
```

**Do not create a fresh transfer after a lost response. Retry the same idempotency key.**

---

## Correctness model

### Exact money

| Rule | Enforced representation | Example |
|---|---|---|
| Amount | signed integer minor units (`BIGINT`) | `1250` minor units |
| Currency | explicit uppercase ISO code | `INR` for the India pilot |
| Command amount | must be positive | `0`, `-1` rejected |
| v1 currency movement | same currency only | `INR → INR` allowed; `INR → USD` rejected |
| Float use | forbidden on the financial path | JavaScript `Number` does not represent money |

For each posted transfer \(t\) and currency \(c\):

\[
\sum \mathrm{debits}(t,c) = \sum \mathrm{credits}(t,c)
\]

For the v1 two-account transfer:

\[
\mathrm{debit}_{source} = \mathrm{credit}_{destination} = \mathrm{amount}_{minor}
\]

```mermaid
flowchart LR
  S[Source<br/>10,000] -->|Debit 1,250| J[Journal]
  J -->|Credit 1,250| D[Destination<br/>2,000]
  J --> Equal{Debit = credit?}
  Equal -->|1,250 = 1,250| Posted[Posted]
  Equal -->|not equal| Rollback[Reject / rollback]
```

The numbers in this diagram are explanatory. Posted records are append-only; corrections are new compensating entries rather than edits.

### Idempotency truth table

| Tenant + actor + operation + key | Canonical fingerprint | Result |
|---|---|---|
| First submission | New | Reserve and execute exactly once |
| Retry | Same | Replay saved final outcome |
| Key reuse | Different | `idempotency_conflict` |
| Concurrent duplicate | Same, unfinished | bounded `request_in_progress` |

---

## Read-your-writes model

LedgerSync does **not** promise that Redis always answers a balance read. It promises that a post-transfer read either reaches the committed minimum balance version or reports unavailability honestly.

| Property | Rule |
|---|---|
| Financial authority | PostgreSQL projection committed with the transfer |
| Cache | Redis; disposable and rebuildable |
| Freshness proof | signed `(tenant, account, minimum version)` requirement |
| Validity | 10 minutes |
| Cache acceptance | `cached version ≥ required version` |
| Projection delay | bounded wait, then primary fallback |
| Redis loss | PostgreSQL answer and cache refill |
| Prohibited behavior | return an older cache answer as current |

```mermaid
timeline
  title Transfer-to-visible-balance lifecycle
  T0 : Transfer request accepted
  T1 : PostgreSQL commits ledger, projection, audit and outbox
  T2 : API returns final outcome and signed minimum versions
  T3 : Worker may update Redis asynchronously
  T4 : Cache serves only a sufficiently new version
  T5 : PostgreSQL provides truthful fallback when required
```

---

## Security model

| Layer | Control | Numeric boundary |
|---|---|---:|
| Browser session | HttpOnly, `Secure` in production, `SameSite=Lax` | 30-minute max-age |
| Browser mutation | same-origin CSRF | required on every cookie-authenticated mutation |
| BFF actor handoff | HMAC-signed actor assertion | ≤ 60 seconds |
| API identity | OIDC issuer/audience/signature/expiry checks | deny invalid claims |
| Authorization | tenant + subject + account predicate in PostgreSQL | one query boundary, not UI logic |
| Scopes | `accounts:read`, `transactions:read`, `transfers:write` | deny by default |
| Errors | safe public envelope | no account-existence disclosure |
| Runtime | private network, capability drop, no-new-privileges | app containers read-only |
| Secret values | independent managed inputs | minimum 32 characters/bytes where configured |

```mermaid
flowchart LR
  U[Browser] -->|same-origin HTTPS| BFF[Next.js BFF]
  BFF -->|OIDC workload token<br/>bff:act-as-user| API[Go private API]
  BFF -->|≤60s actor assertion| API
  API -->|OIDC verifier| IDP[Managed identity provider]
  API -->|owned-account SQL predicate| PG[(PostgreSQL)]
  PG -->|owned| Data[Account / history / balance]
  PG -->|missing or inaccessible| Safe[Same safe denial]
```

```mermaid
flowchart TB
  subgraph Edge[Edge network]
    Browser[User browser]
    Web[Web BFF]
  end
  subgraph Private[Private network]
    API[API]
    Worker[Outbox worker]
    PG[(PostgreSQL)]
    Redis[(Redis)]
  end
  Browser --> Web
  Web --> API
  API --> PG
  API --> Redis
  Worker --> PG
  Worker --> Redis
  Browser -. no route .-> PG
  Browser -. no route .-> Redis
```

Administrative routes are deny-by-default until a privileged, audited operator surface exists.

---

## API surface

| BFF route | Method | Purpose | Key safety rule |
|---|---|---|---|
| `/api/me/accounts` | `GET` | List caller-owned accounts | ownership query + `accounts:read` |
| `/api/accounts/{id}/balance` | `GET` | Current-enough account balance | ownership + optional minimum version |
| `/api/accounts/{id}/transactions` | `GET` | Cursor-paginated history | ownership + page size **1–100** |
| `/api/transfers` | `POST` | Exact internal transfer | CSRF + idempotency key + exact money |

```json
{
  "sourceAccountId": "00000000-0000-0000-0000-000000000010",
  "destinationAccountId": "00000000-0000-0000-0000-000000000020",
  "amount": { "currency": "INR", "minorUnits": "1250" }
}
```

| Header | Required | Purpose |
|---|---:|---|
| `Idempotency-Key` | transfer only | safe retry and deterministic result |
| `X-CSRF-Token` | browser mutation | same-origin cookie protection |
| `Authorization` | private API | OIDC workload/user boundary |
| `X-LedgerSync-Consistency-Requirement` | optional balance read | minimum committed balance version |

See the [HTTP contract](specs/001-secure-transfer-core/contracts/http-api.md).

---

## Data model

```mermaid
erDiagram
  TENANTS ||--o{ ACCOUNTS : contains
  ACCOUNTS ||--o{ ACCOUNT_OWNERS : authorizes
  TENANTS ||--o{ TRANSFERS : scopes
  TRANSFERS ||--|| JOURNAL_TRANSACTIONS : creates
  JOURNAL_TRANSACTIONS ||--o{ LEDGER_POSTINGS : contains
  ACCOUNTS ||--|| ACCOUNT_BALANCE_PROJECTIONS : has
  TRANSFERS ||--o{ OUTBOX_EVENTS : emits
  TENANTS ||--o{ AUDIT_EVENTS : records
  TENANTS ||--o{ IDEMPOTENCY_REQUESTS : deduplicates
```

| Entity | Role | Mutability |
|---|---|---|
| `transfers` | customer-visible movement outcome | final after completion |
| `journal_transactions` | accounting group for a posted transfer | immutable |
| `ledger_postings` | debit/credit proof | append-only |
| `account_balance_projections` | authoritative current balance read model | updated with transfer transaction |
| `idempotency_requests` | fingerprint and retry outcome | final saved outcome |
| `outbox_events` | durable delivery intent | leased/retried, never financial authority |
| `audit_events` | sanitized security/operation evidence | append-only |

---

## Operations and recovery

```mermaid
flowchart LR
  Incident[Signal] --> Type{What degraded?}
  Type -->|Redis loss| Primary[Read PostgreSQL]
  Primary --> Rebuild[Rebuild cache by tenant]
  Type -->|Worker delay| Outbox[Inspect durable outbox]
  Outbox --> Recover[Recover worker]
  Type -->|Lost client response| Retry[Retry same idempotency key]
  Retry --> Replay[Replay stored outcome]
  Type -->|Balance discrepancy| Reconcile[Reconcile ledger vs projection]
  Reconcile --> Mismatch{Mismatch?}
  Mismatch -->|No| Evidence[Preserve evidence]
  Mismatch -->|Yes| Escalate[Escalate; never manually patch ledger/cache]
```

| Recommended operational metric | Formula | Decision use |
|---|---|---|
| Transfer success ratio | posted / all final outcomes | separate expected rejects from system defects |
| Idempotent replay ratio | replays / transfer submissions | identify client retry/network stress |
| Outbox delivery lag | `now - occurred_at` for unpublished event | projection-delay warning |
| RYEW primary fallback ratio | primary fallbacks / balance reads | cache efficiency without losing correctness |
| Unsatisfied minimum versions | requirement errors / balance reads | should be 0 while dependencies are healthy |
| Reconciliation mismatch count | unmatched ledger/projection accounts | release blocking when > 0 |
| Authorization denial rate | denied / protected requests | investigate abuse or broken integrations |

Runbooks: [audit events](docs/runbooks/audit-events.md) · [secret rotation](docs/runbooks/secrets-rotation.md) · [exact money ADR](docs/adr/0001-exact-minor-unit-money.md) · [immutable ledger ADR](docs/adr/0002-immutable-double-entry-ledger.md) · [outbox ADR](docs/adr/0003-transactional-outbox.md) · [RYEW ADR](docs/adr/0004-version-based-ryew.md).

The local recovery path is executable, not aspirational:

```powershell
.\scripts\backup-local.ps1 -RetentionCount 5
$backup = Get-ChildItem .\data\local-backups -Directory | Sort-Object Name -Descending | Select-Object -First 1
.\scripts\local-restore-drill.ps1 -BackupDirectory $backup.FullName
.\scripts\test-local-fault-recovery.ps1
```

The backup manifest is SHA-256-bound; restore occurs in a newly named internal
Compose project; Redis is rebuilt from PostgreSQL; and the drill fails if the
normal project's opaque financial fingerprint or named-volume set changes.

---

## Quick start

The repository-root `docker-compose.yml` is the canonical local entry point and delegates to the supported topology in `deploy/compose/docker-compose.yml`. The archived `docker-compose.legacy-demo.yml` is retained only for historical reference; it is not a supported development, test, or production path.

### Complete local operator path

1. From this repository in PowerShell, run `.\scripts\doctor-local.ps1`. It is read-only and distinguishes a missing Docker installation, stopped engine, permission failure, outdated Compose plugin, disk shortage, malformed local environment, volume state, and a port conflict.
2. Start Docker Desktop if the doctor asks you to, then run `.\scripts\start-local.ps1`.
3. Open `http://127.0.0.1:3000`, choose **Log in**, and open the clean **Overview** workspace. Local login does not require a password or external identity provider. Use **Guide** in the upper workspace bar for the account → funding → transfer → reconciliation sequence.
4. Open **Accounts**. Inspect an existing account or choose **Create account**. Creation records identity and category only: currency is fixed to INR and the exact opening balance is `INR 0.00`; there is no browser balance editor.
5. From a created account, choose **Fund account** to reuse the normal **Transfers** form with that destination selected. Choose a different active funded source, enter the exact decimal amount, review it, and post. If the result is unknown, use **Retry same transfer** so the exact body and idempotency key are retained.
6. Open the transfer record to inspect the immutable result, debit and credit postings, and seven-stage stored-evidence chain. Delivery state is separate from the PostgreSQL financial result. Use server-backed transfer filters or **Events** filters to investigate a specific identifier without changing evidence.
7. On Account Detail, use audited **Freeze account** and **Reactivate account** controls when required. Closing requires a current authoritative available and ledger balance of exact zero, a reason, and typed external-reference confirmation; Closed is terminal and history remains visible.
8. Run **Reconciliation** and require an authoritative matched result with zero mismatches. Inspect **Local status** for PostgreSQL, outbox, and Redis as separate truth domains; **Developer** for versioned examples and the OpenAPI download; and **Recovery** for current database, protected backup, and isolated-restore evidence.
9. CSV exports are bounded, tenant-authorized operational evidence. Review their scope, active filters, row ceiling, schema, and identifier disclosure before download. They are not backups. Backup and restore remain host-only operations.
10. Run `.\scripts\status-local.ps1`, create recovery evidence with `.\scripts\backup-local.ps1 -RetentionCount 5` and the isolated restore drill when needed, then run `.\scripts\stop-local.ps1` when finished.

The complete release gate is reproducible with `.\scripts\test-local-acceptance.ps1 -IncludeCapacity`; it uses unique disposable projects, preserves ordinary volumes, and restores the normal stack. Run it only when the workstation can remain available for approximately eight minutes. Security and supply-chain evidence is a separate exact-commit gate: `.\scripts\run-security-supply-chain-qualification.ps1 -Mode All -ExpectedCommit <40-character-clean-commit>`.

Normal stop preserves PostgreSQL and Redis volumes. `.\scripts\reset-local.ps1` is intentionally destructive and requires the exact confirmation documented in the [local runtime runbook](docs/runbooks/local-runtime-smoke.md).

This workflow is designed and automatically checked at CSS viewports from 320 to 1920 pixels, including keyboard, reduced-motion, forced-color, reflow, and automated WCAG checks. Those checks are browser emulation—not physical-phone, tablet, NVDA, VoiceOver, browser/OS combination, or production accessibility certification. No physical-device result is claimed for the local-only product.

| If this happens | What it means | Safe action |
|---|---|---|
| Docker unavailable | Docker may be absent, stopped, or inaccessible to this user | Run `doctor-local.ps1`; follow its classified recovery action, then rerun `start-local.ps1` |
| Port 3000 occupied | Another process owns LedgerSync's IPv4 loopback port | Keep that process untouched; stop it yourself or change its port, then rerun startup |
| Dependency still starting | PostgreSQL, Redis, API, worker, or web has not reached health | Run `status-local.ps1`, wait briefly, then read bounded output with `logs-local.ps1 -Service <name>` |
| Migration failed | The schema setup job did not complete | Read `logs-local.ps1 -Service migrate`; do not edit financial tables manually |
| Balance looks stale | Redis may be behind or disposable cache data was lost | Run `test-local-fault-recovery.ps1`; PostgreSQL remains authoritative and reconciliation rebuilds cache |
| Database unavailable | Financial truth cannot be read or changed safely | Do not retry with a new transfer key; restore PostgreSQL health, then retry the exact intent with its original key |

| Dependency | Minimum |
|---|---:|
| Go | 1.22 |
| Node.js | 20.18 |
| PostgreSQL image | 16 |
| Redis image | 7.4 |
| Docker Engine + Compose | current supported release |

```powershell
# 1. Start or recover the complete supported topology. This preserves existing data.
.\scripts\doctor-local.ps1
.\scripts\start-local.ps1

# 2. Open the product.
Start-Process http://127.0.0.1:3000

# 3. Inspect, back up, or stop it without deleting PostgreSQL/Redis volumes.
.\scripts\status-local.ps1
.\scripts\backup-local.ps1 -RetentionCount 5
.\scripts\logs-local.ps1 -Service api -Tail 100 -Since 15m
.\scripts\stop-local.ps1

# 4. Run focused developer verification when changing code.
$env:GOCACHE = "$PWD\.cache\go-build"
go test ./internal/... ./cmd/api ./tests/unit -mod=mod
npm --prefix web run test
npm --prefix web run lint
npm --prefix web run build
```

`start-local.ps1` validates PowerShell, Git, Docker, Compose 2.20+, free disk, loopback-port ownership, environment state, and Compose configuration. It waits for PostgreSQL and Redis health, requires migrations and the non-financial local workspace bootstrap to finish, verifies every long-running service, and tests the real browser/BFF read path before printing “ready.” Fresh initialization adds tenant authorization and policy boundaries but no accounts, balances, journals, transfers, or reconciliation results. API startup never mutates the financial schema. The destructive reset command is deliberately separate, reports the latest validated backup and restore-drill state, and refuses to run without the exact confirmation documented in the [local runtime runbook](docs/runbooks/local-runtime-smoke.md).

To rebuild the disposable cache from PostgreSQL projections:

```powershell
go run ./cmd/reconcile --rebuild-cache --tenant-id <tenant-uuid>
```

This command does not create, change, or delete ledger entries.

---

## Developer and operator guides

The repository deliberately separates browser usability from the financial
authority. Read these guides before connecting a design partner or operating a
pilot environment:

- [Architecture and trust boundaries](docs/architecture.md)
- [Private API integration and idempotent transfer contract](docs/api-guide.md)
- [Operator console, incident response, and release ownership](docs/operations.md)
- [Real-stack browser acceptance boundary](web/tests/system/README.md)
- [Performance measurement protocol](docs/performance-baseline.md)
- [Fault-injection scenarios](tests/fault/toxiproxy-scenarios.md)

---

## Evidence, roadmap, and license

### Evidence

- [Phase 10 — complete local-product acceptance](docs/release-evidence/local-product-phase-10-acceptance.md)
- [Local-product completion gates](docs/pilot/local-product-completion-gates.md)
- [Phase 3 — safe transfer core](docs/release-evidence/phase-3-safe-transfer-core.md)
- [Phase 4 — RYEW balance visibility](docs/release-evidence/phase-4-ryew-balance-visibility.md)
- [Phase 5 — account security](docs/release-evidence/phase-5-account-security.md)
- [Full product design brief](DESIGN.md)

![Selected LedgerSync operator-console direction](docs/design/wireframes/ledgersync-transfer-detail-high-fidelity.png)

### Implementation roadmap

```mermaid
gantt
  title LedgerSync roadmap
  dateFormat YYYY-MM-DD
  axisFormat %b
  section Completed
  Financial data foundation        :done, p1, 2026-08-01, 1d
  Safe transfer core               :done, p3, 2026-08-18, 1d
  RYEW cache and delivery          :done, p4, 2026-08-18, 1d
  Account security / BFF boundary  :done, p5, 2026-08-18, 1d
  section Next
  Recovery and operations          :active, p6, 2026-08-19, 14d
  Operator console                 :p7, after p6, 14d
  Pilot hardening                  :p8, after p7, 14d
```

| Included now | Explicitly later |
|---|---|
| exact internal same-currency ledger transfers | bank rails, card payments, FX, custody |
| API, BFF foundation, authorization and RYEW | public self-serve onboarding and end-user wallet UI |
| OIDC token validation, BFF boundary, and authorization-code-with-PKCE callback | provider-specific tenant/role/scope claim mapping approval |
| digest-bound local backup, isolated restore, cache rebuild, and fault drill | provider-backed PITR and full production SLO dashboards |

### License

Licensed under the [Apache License 2.0](LICENSE).
