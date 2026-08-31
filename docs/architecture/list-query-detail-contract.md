# LedgerSync list, query, pagination, and detail contract

**Status:** Phase 10 convergence contract. The table below describes the behavior that exists, the behavior that is intentionally not applicable, and the remaining work. A capability is not treated as released merely because a control can be drawn in the browser.

## Why this contract exists

An operator should be able to open a list, narrow it, copy the URL, inspect one record, and return to the same investigation without guessing whether the system silently changed the scope. LedgerSync therefore treats the URL, browser-to-BFF request, private API validation, database query, cursor, detail link, and export as one end-to-end contract.

In plain language:

- a filter changes the complete server-side result, not only the rows already visible in the browser;
- a cursor means “continue this exact query after this exact record,” not “page two” in a mutable offset list;
- “5 on this page” is not the same as “5 in total”;
- an empty authorized result, denied access, offline state, upstream failure, stale retained evidence, and an invalid shared URL are different states;
- a detail link carries a bounded `return_to` value so Back returns to the same filtered investigation;
- an export must apply the same authorized filter semantics as the visible list or be explicitly unavailable.

## Rules shared by every domain

1. Only the parameters in this document are accepted. Unknown, repeated, empty, oversized, or malformed values are rejected before a protected read starts.
2. Browser pages and BFF routes use the same names and limits. Private APIs validate them again; the database never receives caller-authored SQL, column names, or sort expressions.
3. All current sort orders are fixed and server-owned. No sort control is shown until the backend supports and cursor-binds that sort.
4. Cursors are opaque, bounded to 2,048 characters at the browser boundary, signed or fingerprint-bound where the repository supports filters, and never decoded by the UI.
5. A changed filter creates a new evidence resource. Prior rows are cleared while that new query loads. A refresh of the same query may retain the last verified page, visibly marked historical.
6. “Next page” is single-flight. The control is disabled while a continuation is in progress, and duplicate records are not appended.
7. Every list states its default order and labels the count as page-only unless an authoritative total is returned.
8. Search is exact ID or an explicitly documented normalized/prefix match. Broad substring search over identity or sensitive fields is prohibited without an enumeration review.
9. Dates are UTC ISO dates/timestamps with an inclusive lower and upper bound as documented by the private API.
10. CSV is formula-injection safe, bounded, tenant-authorized, and generated from the server query. “Not applicable” below means the UI must not imply that an export exists.
11. Detail routes validate immutable identifiers and accept only one bounded local `return_to` path for their own route family.
12. All lists must remain bounded with a 10,000-record fixture. DOM and long-task measurement precede any virtualization decision.

## Per-domain matrix

| Domain | Fixed default sort | Approved searchable/filter fields | Pagination and count | CSV | Detail and return context | Current convergence state |
|---|---|---|---|---|---|---|
| Approvals | Actionable first, then oldest `requested_at`, immutable ID, domain | Exact domain, domain-qualified status, exact requester subject, age bucket, requested UTC date range, actionable-by-me | Opaque cursor; exact `page_count`; no total implied | Not applicable to the decision queue: bulk decision evidence could invite stale/offline review; underlying funding/correction records remain individually auditable | Funding/correction detail with exact filtered `/approvals` return URL | Converged for the released query surface: strict page rejection, URL restoration, link pagination, count semantics and detail return context are implemented |
| Funding | Newest `requested_at`, then immutable ID | Exact status only in the released private API | Opaque cursor; page count is derived from returned rows; no total | Not released: an exact filtered funding export needs a reviewed schema and authorization contract | Funding detail with exact status/cursor return context | Converged for the released query surface: page and BFF validation, URL filters, link pagination, count semantics and return context are implemented |
| Corrections | Newest `requested_at`, then immutable ID | Exact correction status only in the released private API | Opaque cursor; page count is derived; no total | Not released: transfer/correction linkage and approval evidence need a dedicated reviewed export schema | Correction detail with exact status/cursor return context | Converged for the released query surface: page and BFF validation, URL filters, link pagination, count semantics and return context are implemented |
| Reconciliation | Newest `completed_at`, then immutable ID | No list filters in the released read API. Run ID is supported only by the exact export/detail contract | Opaque cursor; page count is derived; no total | Released, bounded server export for all authorized runs or one immutable run; cursor selects a visible page and does not narrow the all-runs export | Run detail with exact cursor-page return URL | Converged for the released query surface: strict page/BFF validation, URL continuation, count and order labels, export scope, and return context are implemented |
| Transfers | Newest effective completion time, then immutable ID | Normalized bounded transfer/account identifier `q`, exact account ID, exact financial status, inclusive UTC from/to | Filter-bound opaque cursor; page count is derived; no total | Released; status, account, query, and UTC date range exactly match the visible query while cursor is excluded as page position | Transfer detail with exact filtered cursor-page return URL | Converged for the released query surface: strict page/BFF/export validation, all filters, URL continuation, count/order labels, export parity, and return context are implemented |
| Events | Newest `occurred_at`, then immutable ID | Exact event type, exact state, exact webhook endpoint ID, exact related ID, exact correlation ID, inclusive UTC from/to | Filter-bound opaque cursor; page count is derived; no total | Not released: an event export requires payload-free column review and retention policy | Event detail with exact filtered `/events` return URL; endpoint/financial links remain separate authorities | Converged for the released query surface: strict page/BFF validation, URL filters, link pagination, count/order labels, and exact return context are implemented |
| Webhook endpoints | Newest `updated_at`, then immutable ID | Exact endpoint status and exact subscribed event type | Filter-bound opaque cursor; page count is derived; no total | Not applicable to delivery recovery in the browser; endpoint evidence is bounded and secrets must remain server-side | Endpoint detail with exact filtered `/webhooks` return URL; attempts link to events and financial records | Converged for the released query surface: strict page/BFF validation, URL filters, link pagination, count/order labels, and exact return context are implemented |
| Accounts | Oldest `created_at`, then immutable ID | Exact ID, normalized display-name/external-reference prefix, exact status, exact category | Opaque cursor bound to order; page count is derived; no total | List export is not released; an individual account has a bounded exact ledger-history export | Account detail/focus with exact filtered `/accounts` return URL | Strong reference; query validation, default-sort label, and page-count wording remain |

## Query names and bounds

| Domain | Browser/BFF parameters | Semantic bounds |
|---|---|---|
| Approvals | `domain`, `status`, `requester`, `age`, `requested_after`, `requested_before`, `actionable_by_me`, `cursor`, BFF-only `limit` | domains `funding|correction`; allowlisted domain-qualified statuses; requester ≤255; date `YYYY-MM-DD`; boolean exactly `true`; cursor ≤2,048; limit fixed by UI and bounded by API |
| Funding | `status`, `cursor`, BFF-only `limit` | allowlisted funding lifecycle status; cursor ≤2,048 |
| Corrections | `status`, `cursor`, BFF-only `limit` | allowlisted correction lifecycle status; cursor ≤2,048 |
| Reconciliation | `cursor`, BFF-only `limit` | cursor ≤2,048 |
| Transfers | `q`, `accountId`, `status`, `from`, `to`, `cursor`, BFF-only `limit`; create-flow-only `destination`; detail-only `return_to` | query ≤128; canonical account UUID; allowlisted transfer status; valid UTC timestamp; cursor ≤2,048; bounded same-origin return path |
| Events | `eventType`, `state`, `endpointId`, `relatedId`, `correlationId`, `from`, `to`, `cursor`, BFF-only `limit` | safe event identifier; allowlisted delivery state; canonical UUIDs; valid UTC timestamp; cursor ≤2,048 |
| Webhook endpoints | `status`, `eventType`, `cursor`, BFF-only `limit` | allowlisted endpoint status; safe event identifier; cursor ≤2,048 |
| Accounts | `q`, `status`, `category`, `cursor`, optional UI `focus`, BFF-only `limit` | query ≤128 with escaped prefix semantics; allowlisted status/category; cursor ≤2,048; focus is a canonical UUID and does not broaden the private query |

## Implementation sequence and acceptance evidence

The convergence order is deliberately approvals, funding, corrections, reconciliation, transfers, events/webhooks, then accounts. Each domain change must include:

- page-parser tests for unknown, repeated, oversized, empty, and malformed values;
- BFF tests proving the same rejection and proving no upstream request occurred;
- backend integration tests proving filters execute before pagination and cursors cannot be replayed under another query;
- browser tests for URL restoration, clear-all, detail return, page-only count, loading, empty, denied, offline, same-query historical refresh, and different-query reset;
- an exact export comparison when CSV is released, or an explicit user-facing/documented reason when it is not;
- a 10,000-record bounded-page check and accessibility/DOM budget evidence before exit.

Phase 10 is complete only when every row above is marked converged by executable evidence. This document records intended truth; it does not turn a partial row into a completed capability.
