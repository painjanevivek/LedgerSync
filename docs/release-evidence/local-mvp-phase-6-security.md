# Local MVP Phase 6 — identity and security boundary evidence

**Result:** `PASSED`

**Executed:** 2026-08-24 on the local Windows workstation and Docker Desktop

**Evidence binding:** the Git commit containing this document

## Boundary proved

LedgerSync remains a sign-in-free demo only inside one explicit development environment on one workstation. The browser reaches only `127.0.0.1:3000`; API, worker, PostgreSQL, Redis, migration, and seed services publish no host ports. This is not an OIDC, shared-user, public-network, custody, or production-deployment claim.

## Security review and remediation

A standard Codex Security source scan was completed against Phase 5 commit `2f9f1edfff68f6cef592d3a2b205ac195b995e47` with scan ID `e99cf188-fd3a-44f0-9670-fe9efbfe9088`. It reported one high and four medium findings. Phase 6 remediated all five:

| Finding | Correction | Verification |
|---|---|---|
| `csf_925b0b2eda8090d13e907fd9` — cached balance owner bypass | PostgreSQL account authorization now precedes every Redis cache disclosure | Warm-cache non-owner unit test plus PostgreSQL owner/non-owner integration test |
| `csf_09ca70cd00202d69f59b0239` — destination balance disclosure | Transfer success returns only balances and RYEW versions the initiating actor may read | Atomic transfer integration test proves destination balance/version is absent while both postings still commit |
| `csf_6e6617dd213e4184e48a1986` — fail-open demo environment | Demo settings require the affirmative `development` deployment label | Missing, staging, preview, `prod`, and production negative tests |
| `csf_6a53075893a8d391506554a7` — host-derived origin fallback | Cookie mutations require a fixed valid public origin and matching request origin/host | Cross-origin, missing-origin, and rebound-host tests |
| `csf_c63af3d6594d4913cc5d085d` — linear replay cleanup | Replay expiry uses a bounded min-heap with incremental cleanup instead of a full map scan | Duplicate, expiry, capacity, and full Go test/race-compatible paths |

Tenant-wide transfer and reconciliation evidence also now requires a server-owned `tenant:operator` or `tenant:admin` role in addition to its named read scope. The approved loopback demo still auto-creates its explicit demo operator session; that is deliberate local product behavior and is not presented as shared authentication.

## Generated secrets and runtime isolation

The startup command generated six independent 32-byte secrets in ignored `data/local-runtime/runtime.env`, removed inherited Windows ACLs, and rotated the existing PostgreSQL role over stdin without printing the value or resetting its named volume. Tracked Compose has no fixed credential fallback.

Effective inspection covered seven created services:

| Control | Observed result |
|---|---|
| Runtime users | `postgres`, `redis`, or `ledgersync`; no implicit root user |
| Root filesystem | Read-only for PostgreSQL, Redis, API, worker, web, migration, and seed |
| Linux capabilities | `ALL` dropped for every created service |
| Privilege escalation | `no-new-privileges:true` for every created service |
| Writable paths | Named data volumes plus bounded `tmpfs` paths only |
| Host publication | Web only, exactly `127.0.0.1:3000` |
| Networks | Browser edge plus Docker-internal private service network |
| Image inputs | PostgreSQL, Redis, Python diagnostic, Go builder/runtime, Alpine runtime, and Node builder/runtime pinned by digest |

A disposable clean project proved that hardened non-root PostgreSQL and Redis can initialize fresh named volumes, after which all 12 migrations and the deterministic demo seed completed. The disposable containers and volumes were removed after the pass; the authoritative `compose` volumes were untouched.

## Browser, API, and evidence controls

- Signed browser sessions now authenticate the exact encoded payload, so property insertion order cannot invalidate a legitimate future OIDC session.
- Session cookies remain HttpOnly and SameSite; `Secure=false` is permitted only in explicit non-production local HTTP mode.
- Cookie-authenticated writes require a configured exact origin, matching request host/origin, and constant-time CSRF token comparison.
- BFF financial JSON is limited to 16 KiB; private API JSON remains strictly decoded and limited to 64 KiB. Server and upstream timeouts remain bounded.
- PostgreSQL-backed principal/tenant rate limits return stable `429` and `Retry-After`; shared per-tenant second/minute capacity limits remain enforced.
- Tenant-wide investigation routes require both the narrow read scope and server-owned operator/admin role.
- Go structured logs, audit metadata, and bounded PowerShell logs redact credentials, cookies, sessions, CSRF, consistency data, database URLs/DSNs, PII field families, and raw balance/amount fields.

## Reproducible results

| Check | Observed result |
|---|---|
| Full Go suite | Passed: all internal, contract, fault, integration, system, and unit packages |
| Web lint | Passed |
| Web unit/security suite | 25 passed |
| Next.js production build | Passed |
| JavaScript/font budgets | Passed; 663,477 total JS bytes, 229,156 largest chunk, no webfonts |
| Functional/responsive/accessibility/state/visual browser suite | 48 passed |
| Isolated throttled browser performance suite | 2 passed; LCP 376 ms, observed interaction 24 ms, CLS 0.0777, longest task 91 ms |
| Phase 6 live boundary script | Passed: generated secrets, seven hardened containers, loopback-only port, redacted logs, authenticated reads |
| Live mutation boundary | Same-origin session/CSRF reached API validation; cross-origin request returned 403; invalid payload moved no money |
| Fresh-volume hardened startup | Passed: PostgreSQL, Redis, migrations, and seed |
| Go vulnerability scan | 0 called vulnerabilities; one imported package finding is not reachable from this code |
| Production npm audit | 0 vulnerabilities |
| Docker Scout image scan | API 0 high/critical, worker 0 high/critical, web 0 high/critical |
| Current tracked-source secret signatures | No private-key, cloud-key, live-token, or former fixed-development credential signature found |
| Live operational state | Migration 12, healthy stack, zero dead/pending outbox work, latest reconciliation matched with zero mismatches |

The repository CI independently retains history-aware secret scanning, infrastructure scanning, high/critical image gates, SBOM generation, and build provenance. No local result weakens those gates.
