# Saved Investigation Views Threat Model

## Protected assets and trust boundaries

The protected assets are tenant isolation, operator privacy, domain authorization, immutable financial evidence, browser-session integrity, and audit trustworthiness. The browser is untrusted. The BFF owns same-origin session and CSRF enforcement. The private API owns role, scope, schema, and tenant decisions. PostgreSQL is the authoritative preference and audit store.

Saved views are preferences, not financial records. Their main security risk is becoming an unintended channel for data retention, cross-tenant navigation, injection, or authorization bypass.

## Threats and controls

| Threat | Control | Verification |
|---|---|---|
| A caller submits an arbitrary URL | The API accepts no target path; it derives one from a fixed domain and sorted allowlisted filters | unit and sanitizer tests reject forged targets |
| A view stores balances, results, payloads, secrets, or customer text | Exact JSON fields, domain-specific filters, small values, and `additionalProperties: false`; browser save controls omit free text and cursors | hostile-field tests and OpenAPI contract |
| A view reveals another tenant or operator | Every repository predicate includes tenant and owner; no caller-supplied tenant or owner fields | repository integration test |
| A stale session keeps opening a formerly authorized domain | Views are filtered by current server-issued domain scopes on every list and mutation | handler and application access tests |
| Cross-site creation, rename, or deletion | Same-origin Host/Origin checks and constant-time CSRF token comparison at the BFF; private API still requires write scope | BFF boundary tests |
| A stale tab overwrites or deletes a newer preference | Monotonic versions, `expected_version`, and quoted `If-Match` preconditions | conflict tests |
| Concurrent creation exceeds the cap or duplicates a name | Per-owner serializable transaction plus database unique constraint and 25-row cap | repository and integration tests |
| Names or filters leak into an audit stream | Audit metadata stores only IDs, domain, schema version, and view version | integration assertions |
| Malformed database data reaches the browser | Persisted definitions are normalized again on read; BFF performs exact response sanitization and recomputes target paths | application and frontend sanitizer tests |
| Oversized requests or responses consume resources | 4/8 KiB mutation bodies, 64 KiB upstream response cap, maximum 25 rows, maximum 8 filter keys, independent rate buckets | boundary tests |
| A browser retains evidence after logout | No local storage, session storage, IndexedDB, result snapshot, or service-worker persistence | static and browser tests |
| A saved timestamp changes time-range meaning | UTC canonicalization preserves RFC 3339 fractional seconds and validates range order | Go and TypeScript normalization tests |

## Deliberate non-features

Version 1 has no shared views, public links, team ownership, arbitrary query builder, fuzzy search, saved exports, result snapshots, account free-text search, approval requester filter, browser offline cache, or financial values. These exclusions keep the feature a navigation preference rather than a reporting or data-sharing subsystem.

## Operational response

Repeated forbidden, invalid, rate-limited, and failed mutations remain available through normal request/audit telemetry without logging view names or filter values. If malformed persisted rows are detected, operators should treat them as integrity evidence, preserve the row for investigation, and correct the source through a reviewed migration rather than weakening the reader.

## Review triggers

Security review is required before adding sharing, organization-wide ownership, arbitrary strings, URL fragments, export definitions, secret-bearing identifiers, offline persistence, or schema-version coercion. Any of those changes materially alters the data classification and abuse surface.
