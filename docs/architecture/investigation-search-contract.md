# Exact investigation search contract

## What the operator sees

`/search` provides one keyboard-accessible form and a scoped navigation entry. The operator enters a complete known record ID or approved external reference. LedgerSync then shows a small list of typed locators with status, UTC time, source, and freshness. Selecting a locator opens the existing detail screen that owns that evidence.

The page uses progressive rendering: session and authorization settle first, then the bounded lookup runs, and only validated results render. Loading, offline, denied, unavailable, empty, truncated, and populated states have separate language so a temporary failure is never presented as an empty financial result.

## Ownership boundaries

- `web/src/lib/page-query/investigation-search.ts` owns the shareable page URL contract.
- `web/src/lib/investigation-search-boundary.ts` owns BFF method, host, role, scope, and rate-limit checks.
- `web/src/lib/api/investigation-search.ts` owns the strict browser-safe response DTO and byte-bounded upstream reader.
- `internal/transport/http/handlers/investigation.go` owns private API validation and authorization.
- `internal/platform/db/investigation_repository.go` owns tenant- and object-scoped exact SQL lookup.
- Existing domain detail routes remain authoritative for full evidence.

## Deliberate limits

- One query value; no compound query language.
- Exact UUID or approved reference only.
- Default 10 and maximum 20 locators.
- No browser-side bulk dataset fetch or filtering.
- No saved results and no result persistence.
- No money, balances, ledger postings, raw payloads, or secrets in locator responses.
- No released relationship graph in this slice; related evidence is a separate phase with its own deterministic edge contract.

## Extension rule

A new record family requires all of the following in one reviewed change: a server scope decision, tenant predicate, object-authorization decision, indexed exact lookup, safe-label/status mapping, DTO enum update, canonical-route mapping, threat-model update, and positive/negative tests. If any item is absent, the record family remains unavailable rather than falling back to untyped output.
