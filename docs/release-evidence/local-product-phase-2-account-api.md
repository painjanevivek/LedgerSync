# Local-product Phase 2 — account command API and BFF assurance

**Result:** `PASSED`

**Gate:** [LPC-020](../pilot/local-product-completion-gates.md)

**Starting point:** passed Phase 1 working tree at `8f560f3`, with migrations `000001`–`000012` frozen and additive migration `000013` already verified by LPC-010

This is the independent assurance record for the additive account-command HTTP API and browser-facing BFF. The API specialist's complete Go verification, a fresh PostgreSQL/Redis integration run, the complete web verification, an independent coordinating rerun, and a real supported-stack BFF-to-PostgreSQL lifecycle journey all passed. The verified candidate is the coordinated Phase 2 working tree based on `8f560f3`; its resulting integration commit had not been assigned when this record was updated.

## Review boundary

Phase 2 adds account create and update commands to the existing read-only account routes without expanding the local-only product boundary. The reviewed surface includes the Go routes and handler, account command service/repository, identity scope allowlists, browser-facing account routes, strict request parsers, signed demo/OIDC sessions, CSRF and fixed-origin/Host enforcement, private workload credentials and actor assertions, rate/capacity controls, public response sanitization, exact-string contracts, OpenAPI/runtime drift tests, and live PostgreSQL persistence.

Account-management UI remains LPC-030. Phase 2 exposes the safe command boundary that UI must use; it does not itself claim the create/fund/lifecycle browser experience.

## Verified checklist

### Additive routing and existing GET compatibility

- [x] `POST /api/accounts` and `PATCH /api/accounts/{accountID}` are additive and do not shadow or rename existing account, balance, transaction, transfer, reconciliation, or health routes.
- [x] Existing account GET routes retain their methods, query allowlists, pagination, authorization, exact-string money/balance-version fields, non-disclosing object behavior, rate controls, and `Cache-Control: no-store` behavior.
- [x] Unsupported command methods return the documented stable boundary and cannot reach the command repository.
- [x] The full Go command/internal/unit/contract suites and complete web suite pass alongside the new routes, providing unchanged existing-route regression evidence.

### Exact strings, configuration version, lifecycle reason, and strict input

- [x] Account configuration version, balance version, available minor amount, and ledger minor amount remain canonical decimal strings across PostgreSQL, Go, JSON, BFF sanitization, TypeScript, and OpenAPI.
- [x] Account reads now expose `account_version` additively and keep the existing balance projection `version` distinct. Live proof returned `account_version="1"` and balance `version="0"`; the UI must never substitute one for the other.
- [x] Create returns an active INR account with exact `available_minor="0"`, `ledger_minor="0"`, and `account_version="1"`, and accepts no opening balance or client-selected tenant, owner, identity, account ID, status, or version.
- [x] `expected_version` accepts only a positive canonical signed-64-bit decimal string; numeric JSON, zero, negative, exponent, decimal, leading-zero, whitespace, and overflow forms are rejected before repository use.
- [x] Create accepts only its documented identity/category/INR shape. Patch accepts exactly one complete metadata shape or one lifecycle shape; mixed, missing, null, unknown, overlength, trailing, and invalid-enum input fails without a business side effect.
- [x] Lifecycle commands require a trimmed, bounded, valid-Unicode `reason`. Control characters and missing/blank/overlength reasons are rejected, and the normalized reason participates in the idempotency fingerprint.
- [x] Go and BFF apply the same 4 KiB bound, require the JSON media type, reject trailing JSON and unknown fields, and use consistent `target_status`, `reason`, `expected_version`, and `account_version` names.

### Scope, object authorization, and private workload boundary

- [x] `accounts:write` is consistently allowlisted and enforced by demo/OIDC identity, BFF session routing, actor assertions, Go authorization, and OpenAPI. Read-only identity is insufficient for mutation.
- [x] Authentication and scope denial occur before body parsing, idempotency replay/reservation, account lookup, or repository mutation.
- [x] Creation retains the server-controlled LPC-010 operator/finance role policy; client scope alone cannot nominate tenant, owner, or authority.
- [x] Metadata and lifecycle operations re-check tenant-scoped object/debit ownership in the transactional repository boundary before replay disclosure or mutation.
- [x] Missing, malformed, cross-tenant, unauthorized, and inaccessible account objects use stable non-disclosing handling, including after authorization removal.
- [x] The private API independently requires the server workload credential and a short-lived issuer/audience-bound actor assertion derived from the signed session. Browser-controlled private headers are not forwarded.

### BFF session, CSRF, fixed Host/origin, no-store, and redaction

- [x] Browser mutations require a valid signed, unexpired session, `accounts:write`, the session-bound CSRF token, the exact configured public Origin, and a trusted fixed Host before outbound private API traffic.
- [x] DNS-rebinding Host forms, cross-origin requests, wrong CSRF tokens, malformed sessions, and missing private credentials fail closed.
- [x] The private API URL is server-configured; account IDs are encoded as one path segment and cannot redirect the proxy or select an arbitrary upstream.
- [x] Workload credentials, actor-assertion secrets, private authorization headers, raw tokens, internal hosts, stack/SQL details, bodies, and arbitrary upstream fields cannot cross the public response boundary.
- [x] BFF success and error responses use `no-store`; success/error payloads and forwarded request ID, replay, and `Retry-After` headers are closed, validated allowlists.

### Idempotency and response-unknown safety

- [x] A bounded visible-ASCII `Idempotency-Key` is required by both BFF and API before command execution.
- [x] Original create and patch commit one mutation, completed idempotency result, success audit record, and outbox event.
- [x] Exact same-intent replay returns the persisted original result and `Idempotent-Replay: true` without a duplicate account, lifecycle change, success audit, or outbox event.
- [x] Changed intent—including account, command family, expected version, normalized fields, or lifecycle reason—returns a stable conflict and cannot alter the first result.
- [x] In-progress, stable-denial replay, injected dependency failure, rollback, and same-key recovery paths are covered by the passing command/contract/integration suites.
- [x] A dispatch timeout is exposed as `504 account_command_outcome_unknown`; it does not claim definite failure and requires identical-body, identical-key retry. Pre-dispatch denial does not claim an unknown committed outcome.
- [x] The real BFF journey returned one `201` creation and the same `201` result with `Idempotent-Replay: true` on same-key retry.

### Rate, capacity, public errors, and contract drift

- [x] Account create/update use stable route-specific tenant-capacity and principal-rate controls after trusted identity and before body/repository work.
- [x] Rate/capacity denial and limiter failure fail closed with stable public semantics, bounded `Retry-After`, no-store, and no command repository side effect.
- [x] Runtime maps validation, authentication, authorization/not-found, external-reference conflict, idempotency conflict, request-in-progress, account-version conflict, invalid transition, non-zero close, rate limit, temporary unavailable, and response-unknown outcomes to tested public status/code pairs.
- [x] BFF success sanitization rejects malformed or unsafe upstream success data; error sanitization cannot reflect arbitrary upstream codes, messages, fields, or headers.
- [x] OpenAPI documents the additive routes, `accounts:write`, strict bodies, required lifecycle reason, distinct configuration/balance versions, exact-string DTOs, idempotency/replay, stable errors, rate headers, and unknown-outcome rule.
- [x] Runtime/OpenAPI contract tests pass and retain the existing GET paths and fields while proving the additive command surface.

### Real BFF → API → PostgreSQL and normal-stack proof

- [x] The supported Compose stack was rebuilt from current source and reported healthy before the live command journey.
- [x] A real demo session sent account creation through the browser-facing BFF, private workload/actor boundary, Go API, and PostgreSQL repository; no mocked fetch, service, or repository substituted for this proof.
- [x] Live create returned `201`, same-key replay returned the same `201` with `Idempotent-Replay: true`, and exact available/ledger balances remained `0`.
- [x] Live reads proved configuration `account_version=1` independently from balance projection `version=0`.
- [x] Live lifecycle commands, each with its required audited reason, progressed `active → frozen → active → closed`; the final configuration version was `4` and the zero-only terminal close succeeded.
- [x] Final normal-stack status reported schema `000013` with 13 migrations, outbox 0 pending/0 dead, and the latest reconciliation `matched` with 0 mismatches.

## Evidence reviewed

| Evidence | Result |
|---|---|
| API specialist: complete account-command, internal, Go unit, and contract suites plus `go vet` | `PASS` |
| Fresh PostgreSQL/Redis integration suite | `PASS` in 15.604 seconds |
| Web unit/security suite | `PASS` — 37/37 |
| Web lint and production build | `PASS` |
| Independent coordinating repeat of full Go tests and `go vet` | `PASS` |
| Independent coordinating repeat of web unit/security, lint, and build | `PASS` — 37/37 plus clean lint/build |
| Normal supported stack rebuilt from source | Healthy |
| Real BFF account create and exact replay | `201`; replay `201` with `Idempotent-Replay: true`; exact zero balances |
| Real BFF lifecycle with audited reasons | `active → frozen → active → closed`; final `account_version=4` |
| Final schema and migration status | `000013`; 13 migrations applied |
| Final outbox | 0 pending, 0 dead |
| Latest reconciliation | `matched`; 0 mismatches |

## Design-contract corrections completed in Phase 2

The Phase 3 preflight found that the legacy read field `version` represented balance projection version and therefore could not safely satisfy PATCH optimistic concurrency. Phase 2 corrected this additively by exposing `account_version` on authorized account reads and documenting that it is distinct from balance `version`. The live journey proved both values independently.

The design brief also requires an operator reason for freeze/lifecycle actions. Phase 2 now requires a bounded audited lifecycle `reason`, includes its normalized value in idempotency intent, and persists it with lifecycle audit/outbox evidence. Phase 3 can therefore present truthful reason-required confirmations without inventing unsupported metadata.

## Phase decision

The reviewed test and live-stack evidence closes the account-command API/BFF risks without changing the frozen financial model or expanding the local-only boundary. `LPC-020` is `PASSED`. `LPC-030` is now `READY` for the configurable account UI handoff; it must reuse these exact version, reason, authorization, idempotency, and response-unknown contracts.
