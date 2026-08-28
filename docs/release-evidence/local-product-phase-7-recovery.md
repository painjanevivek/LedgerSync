# Local-product Phase 7 — recovery center and exact evidence exports

**Result:** `PASSED`

**Verified:** 2026-08-24T22:38:02Z

**Candidate:** Phase 7 working tree based on `4a6a981`; the resulting Phase 7 commit binds this evidence to the implementation.

**Boundary:** the supported single-workstation Docker Compose runtime, one INR demo tenant, a protected host backup root, uniquely named isolated restore projects, private API/PostgreSQL/Redis services, and browser access only at `http://127.0.0.1:3000`.

## Operator outcome

- `/recovery` presents three independent proofs in order: current PostgreSQL/reconciliation truth, the latest validated protected backup, and the latest passing isolated restore drill. Missing evidence remains explicitly missing; one proof never substitutes for another.
- The browser reads only a fixed, sanitized `ledgersync-recovery-evidence-index/v1` document. It receives no host path, dump filename or bytes, digest value, credential, Docker identifier, arbitrary filename, or execution capability.
- Backup, isolated restore drill, and local status commands are copy-only guidance. Restore, reset, path selection, shell execution, and volume deletion are intentionally absent from the browser.
- Transfer history, one authorized account ledger, and reconciliation evidence can be downloaded as contextual CSV exports after a review panel states scope, active filters, maximum 10,000 records, schema version, exact format, included identifiers, and the explicit fact that an export is not a backup.

## Recovery and containment controls

- Startup atomically creates a protected empty recovery index only when the exact file is absent. A valid existing index is not rewritten; a malformed existing index is rejected and byte-preserved.
- The API receives exactly one read-only bind: `recovery-evidence-index.json` at its fixed container path. The backup root, database dump, Docker socket, and host shell are not mounted.
- Backup discovery admits only immediate finalized `backup-*` directories with fixed `manifest.json`, `database.dump`, and optional restore-evidence filenames. Normalized traversal, nested paths, junctions/reparse points, symlinks, incomplete bundles, malformed JSON, unknown schemas, and digest mismatches cannot become evidence.
- Retention deletes only validated backup directories contained by the configured root. A malformed future timestamp cannot evict the newest valid backup; malformed entries remain untouched for operator investigation.
- Restore drills use a uniquely named Compose project/volume, apply current migrations, validate journals/postings/projections, rebuild Redis from PostgreSQL, reconcile, and remove only the exact isolated resources.

## Exact export controls

- Exports require `exports:read` plus `transfers:read`, `transactions:read`, or `reconciliation:read`; tenant-wide exports additionally require operator/admin role, and account history repeats object authorization at the repository boundary.
- Database iteration uses stable 250-row server-side pages. The service enforces a 10,000-row ceiling, a 10-second deadline, and request-context cancellation without assembling the full data set in browser or API memory.
- CSV is UTF-8 with deterministic versioned column order and always-quoted fields. Minor units and currency remain exact strings; operator-controlled text is line-normalized, length-bounded, and formula-injection-neutralized.
- Filenames are fixed-family UTC `v1.csv` attachments. The BFF allowlists media type, family, timestamp, schema, length, correlation, and no-store headers while discarding private or arbitrary headers.
- Append-only audit records contain export type, schema, row limit, row count, limit reached, and a filter fingerprint—never exported rows, exact amounts, currencies, credentials, or raw filters. Event lifecycle is `export.requested/completed/failed`; constrained outcomes remain `allowed/succeeded/failed`.

## Automated evidence

| Layer | Result |
|---|---|
| Go unit, contract, fault, integration, and system suite | `go test ./... -count=1` passed |
| Go static checks | `go vet ./...` passed |
| PostgreSQL export integration | Passed tenant isolation, account authorization, exact fields, paging, and reconciliation mismatch evidence |
| OpenAPI lint and route drift | Pinned `@redocly/cli@1.34.0` passed; canonical contract version `1.7.0` |
| Recovery hostile-input suite | Passed traversal, reparse, malformed, incomplete, digest, retention, startup fail-closed, and sanitization cases |
| Web unit/security | 71/71 passed |
| Focused Phase 7 security | 6/6 passed |
| Focused Phase 7 browser journeys | 5/5 passed |
| Full browser, accessibility, responsive, and visual suite | 101/101 passed with 16 workers |
| Type, lint, production build | TypeScript, ESLint, and Next.js build passed |
| Performance budget | 850,823 total JavaScript bytes; largest chunk 229,156 bytes, below 2,000,000 and 350,000-byte limits |
| Patch integrity | `git diff --check` passed |

## Live supported-stack proof

- A protected normal backup passed with schema `000015`, 7 accounts, 140,590 transfers, 281,178 postings, an 80,440,494-byte streamed dump, and a verified SHA-256 digest. The raw dump and digest remain outside Git and the browser.
- The exact candidate images rebuilt successfully. PostgreSQL, Redis, API, worker, and web became healthy; only `127.0.0.1:3000` remained published.
- Live BFF recovery returned HTTP 200 and `no-store`, with five validated retained backups, a verified digest status, passing validation status, and later the passing restore receipt.
- Live BFF transfer, account-ledger, and reconciliation exports returned HTTP 200, `text/csv`, canonical UTC v1 filenames, deterministic rows, and exact quoted INR minor-unit fields where applicable.
- PostgreSQL recorded exactly one requested and one completed summary audit for each live export: requested=`allowed`, completed=`succeeded`; completed row counts were 3 transfers, 3 account entries, and 1 reconciliation record.

## Isolated restore evidence

- The exact-tree acceptance harness passed streamed dump/digest, protected host files, sanitized recovery index, unique restore project, unchanged normal financial state, and complete cleanup.
- A normal operator restore drill then restored the newest protected backup into `ledgersync-restore-20260824223710-0ffb6b97`, applied schema `000015`, and verified 7 accounts, 140,590 transfers, and 281,178 postings.
- Invalid journals: 0; posted transfers without journal: 0; negative projections: 0; reconciliation: matched with 0 mismatches; Redis rebuild: 7 accounts.
- Local measured drill time: 31.8 seconds. This is workstation evidence, not a provider or production RTO promise.
- The restore project, containers, network, and volume were removed. The normal Compose project remained healthy at schema `000015`, outbox pending/dead `0/0`, and latest reconciliation matched with 0 mismatches.

## Failures found and remediated during root integration

The first live BFF integration attempt was intentionally treated as a stop-ship:

1. The generated index used a local numeric offset in fields named UTC while the API correctly required canonical `Z`. All recovery writers now emit canonical UTC `Z`; hostile/script tests and a fresh live index passed.
2. Export audit lifecycle strings had been passed directly into the database `outcome` column, whose established check constraint accepts only `allowed`, `denied`, `succeeded`, or `failed`. Event types retain precise lifecycle names, while outcomes now map to `allowed/succeeded/failed`; unit assertions and live PostgreSQL records prove the correction.
3. Grouped navigation exposed an axe background-resolution contrast failure for the operator subtitle. The subtitle now declares its rail background explicitly without changing the selected color; the five focused accessibility cases and full browser suite passed.

These failures were fixed and retested; they are not represented as conditional passes.

## Visual review

Three new Phase 7 baselines and twenty intentionally affected existing baselines were reviewed. Recovery uses the established navy/emerald evidence-document language and presents custody as a three-proof chain. Export dialogs preserve the same hierarchy, keep the warning amber rather than success green, and remain usable at 390×844. Existing changes are limited to the grouped Financial workspace/Local tools navigation, the Recovery destination, and Developer contract version/endpoint expansion.

## Local-only limitations

- Native browser download handoff does not prove that a user saved every final byte without buffering the file in page memory. The UI truthfully reports browser handoff; server completion and row count remain durable in audit evidence.
- Export and recovery BFF rate limits are process-local because the supported boundary has one web process.
- Local restore duration is not a cloud/provider recovery commitment. Production PITR, external object storage, managed identity, and on-call procedures remain outside this local-product gate.
