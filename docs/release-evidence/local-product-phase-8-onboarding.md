# Local-product Phase 8 — guided trust journey

**Result:** `PASSED`

**Verified:** 2026-08-24T23:07:12Z

**Gate:** [LPC-080](../pilot/local-product-completion-gates.md)

**Candidate:** Phase 8 working tree based on `43e7682`; the resulting Phase 8 commit binds this evidence to the implementation.

**Boundary:** the supported loopback-only Docker Compose product, one server-controlled INR demo operator, internal same-currency ledger transfers, PostgreSQL financial authority, disposable Redis, and protected host-side recovery evidence. This is not production, custody, bank-rail, managed-identity, or public-deployment evidence.

## Operator outcome

- The product still opens directly on `/`. The guide is an in-workspace, dismissible panel and can be reopened through **Local tools → Local guide**; there is no blocking landing page.
- The guide states the exact local boundary: INR only, authorized internal accounts, PostgreSQL ledger authority, persisted data, and the fixed safe-stop host command. It exposes no reset, reseed, shell, Docker, or fault-injection control.
- Seven checklist rows are backed by server-side evidence. Account creation, transfer posting, reconciliation, and protected backup may be completed only when their durable records exist. Browser-only inspection actions remain `evidence_available`; absence and dependency failure remain `missing` or `unavailable`.
- An authorized transfer shows a fixed seven-stage chain: idempotency request, transfer, journal/postings, balance versions, outbox, delivery, and reconciliation coverage. Every stage is independently bounded and can remain missing, unavailable, truncated, or out of order without being promoted to truth.
- Related account, transfer, event, and reconciliation links preserve an allowlisted return context. A global database search was not added because a uniform object-authorization and non-disclosure contract was not justified for this phase.

## Authorization and read-model controls

- `GET /api/local/orientation` requires a signed session, `local:read`, an operator/admin role at the private API, exact Host, GET with no query, a bounded operator rate, fixed response limit, timeout, and `no-store`.
- `GET /api/transfers/{transferId}/explainability` additionally requires `explainability:read`, `transfers:read`, `events:read`, and `reconciliation:read`; the repository repeats tenant and object authorization before returning any transfer evidence.
- The BFF accepts only UUID transfer identifiers and exact DTO keys. It applies semantic allowlists, size limits, item limits, stage-order validation, and hostile-field rejection; raw payloads, credentials, hosts, database details, and arbitrary URLs cannot cross the browser boundary.
- Reconciliation coverage is shown only when a stored PostgreSQL run snapshot watermark proves it covers the transfer, journal, and every returned posting. A later wall-clock timestamp alone is insufficient.
- Migration `000016_guided_read_models.up.sql` adds only bounded lookup indexes and least-privilege reads needed by the additive read model. Existing migrations remain immutable.

## Safe demo and retry proof

- Initialization mode is protected per Compose project as `demo` or `empty`. It can be selected only for a truly fresh volume; startup rejects a conflicting mode over existing data.
- Reset remains a guarded host command. Only after exact project volumes are deliberately deleted may it select the next initialization mode. The browser has no initialization authority.
- The retry lab uses a uniquely named isolated Compose project and policy-valid 137-minor-unit intent. It discards the first successful response only at the client harness boundary after the server has committed, then replays the identical serialized body and idempotency key.
- Live proof passed with an unchanged transfer ID, exactly 1 transfer, 1 journal, and 2 postings; account balances changed exactly once by 137 minor units; reconciliation returned `matched` with 0 mismatches.
- Cleanup left zero isolated containers, networks, volumes, or state directories. The normal project's financial fingerprint was unchanged and its healthy services were restored.
- Hostile identity, traversal/reparse, malformed marker, missing confirmation, mode-switch-over-data, and browser-authority cases passed. The empty branch was qualified through deterministic script and Compose tests; the live financial retry used the supported demo branch.

## Automated evidence

| Layer | Result |
|---|---|
| Go unit, contract, fault, integration, and system suite | `go test ./... -count=1` passed |
| Go static checks | `go vet ./cmd/api ./internal/... ./contracts ./tests/contract` passed |
| Full disposable PostgreSQL integration | Full suite passed in 18.719 seconds; guidance partial/missing ordering, tenant/object non-disclosure, migration, snapshot-watermark noncoverage/coverage, lifecycle ordering, and role privileges passed |
| OpenAPI lint and route drift | Redocly passed; canonical contract version `1.8.0` |
| Web unit/security | 75/75 passed |
| Phase 8 browser journeys | Orientation, seven-stage evidence, partial/out-of-order/denied, return context, and compact accessibility passed |
| Full browser, accessibility, responsive, and visual suite | 105/105 passed with 16 workers |
| Accessibility | Populated routes, keyboard flows, 320-pixel reflow, 200%/400%-equivalent zoom, text spacing, touch targets, reduced motion, and authored/forced-color states passed; 0 serious/critical axe findings |
| Type, lint, and production build | TypeScript, ESLint, and Next.js production build passed |
| Performance budget | 873,657 total JavaScript bytes; largest chunk 229,156 bytes, below 2,000,000 and 350,000-byte limits; no shipped font files |
| PowerShell demo tooling | Parser and hostile/no-authority suite passed |
| Patch integrity | `git diff --check` passed with Windows line-ending notices only |

## Live supported-stack proof

- The exact candidate images rebuilt successfully. PostgreSQL, Redis, API, worker, and web became healthy; migrations and deterministic demo seed completed with exit code 0.
- Browser/BFF smoke passed at `http://127.0.0.1:3000`; schema is `000016_guided_read_models.up.sql`; outbox is 0 pending/0 dead; latest reconciliation is matched with 0 mismatches.
- With the server-issued HttpOnly demo session, orientation returned HTTP 200 and exactly seven durable-evidence rows. The current normal data reported account inspection as evidence available, account creation completed, funding completed, transfer inspection evidence available, reconciliation completed, delivery missing, and backup completed.
- The authorized live transfer timeline returned HTTP 200 with exactly seven ordered stages. Request, transfer, journal/postings, balance versions, outbox, and reconciliation were available; delivery truthfully remained missing because no delivery attempt existed for that transfer.

## Failures found and remediated during root integration

1. The retry lab initially used 37 minor units while the seeded tenant policy requires at least 100. The harness default now uses 137; no financial policy was weakened, and the complete isolated proof passed.
2. Existing console operator text inherited the correct rail foreground but axe could not resolve its translucent ancestor background after the new guide changed full-page composition. The text now declares the rail background explicitly; all affected accessibility cases and the full browser suite passed.
3. The Event-list link assertion expected the pre-Phase-8 route. The product intentionally adds an allowlisted `/events` return context, so the test now verifies that exact encoded context for both transfer and account links.
4. The mixed-currency Overview baseline had not yet recorded the intentional dismissible guide. The new full-page image was inspected and approved; mixed-currency refusal remains visually stop-ship and no false aggregate is rendered.
5. The full disposable PostgreSQL suite exposed two older shared-tree defects: API workload-role grants omitted reconciliation command/schema privileges already required by the Phase 4 contract, and tied lifecycle audit timestamps fell back to random UUID order. Grants are now least-privilege and upgrade-safe, tied rows deterministically prefer the lifecycle change, the lifecycle case passed ten repetitions, and the complete real-PostgreSQL suite passed.

## Deliberate omissions and limitations

- There is no browser runtime fault toggle. The isolated host script proves retry safety without changing the normal API or ledger path.
- There is no global record search. Existing bounded directories, contextual evidence links, filters, and back context provide safe navigation without creating an omnipotent or disclosure-prone query surface.
- Dismissal preference is tenant-scoped browser storage, but it changes presentation only. Checklist completion never comes from local storage.
- Demo and empty initialization are local-environment choices, not tenant lifecycle features. Managed identity, production partner initialization, external devices, public networking, and provider recovery remain outside this gate.
