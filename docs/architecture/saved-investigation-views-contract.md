# Saved Investigation Views Contract

## Purpose

Saved investigation views are server-owned shortcuts to repeatable operational queries. They store a small, versioned filter definition and a human-readable name. They do not store query results. Opening a saved view navigates to a server-derived path and requests current evidence under the operator's current tenant, role, and domain scopes.

This distinction is deliberate: a saved view may remain useful while records change, but it must never become a second financial data store or a way to retain evidence after authority is removed.

## Ownership and authority

Every row is scoped by both `tenant_id` and `owner_subject_id`. The database repository includes both values in every list, update, and delete predicate. Names are unique case-insensitively only within that owner and tenant boundary.

Reading requires all of the following:

- tenant operator or tenant administrator role;
- `investigation:read`;
- at least one released domain read or approval scope; and
- access to the saved view's domain at the time of the read.

Creating, renaming, or deleting additionally requires `investigation:write`, same-origin CSRF validation at the BFF, and the matching domain scope. A saved view that is no longer authorized is omitted instead of being disclosed as inaccessible.

## Version 1 definition

`filter_schema_version` is the string `"1"`. The server accepts only the following structured fields:

| Domain | Saved fields | Explicitly not saved |
|---|---|---|
| Accounts | `status`, `category` | free-text `q`, cursor, focus |
| Transfers | identifier-shaped `q`, `accountId`, `status`, `from`, `to` | cursor, results, amounts |
| Funding | `status` | references, evidence, amounts, cursor |
| Approvals | `domain`, `status`, `age`, `requested_after`, `requested_before`, `actionable_by_me=true` | requester, cursor, decision data |
| Corrections | `status` | notes, reasons, amounts, cursor |
| Events | `eventType`, `state`, `endpointId`, `relatedId`, `correlationId`, `from`, `to` | payload, error details, cursor |
| Webhooks | `status`, `eventType` | endpoint URLs, signing material, cursor |

At least one filter and at most eight filters are required. Unknown, empty, untrimmed, malformed, contradictory, or out-of-range values fail closed. UUIDs and timestamps are canonicalized. Reversed time ranges and incompatible approval domain/status combinations are rejected.

The client sends `domain` and `filters`; it cannot send `target_path`. The application layer sorts and encodes validated filters and derives the canonical path. Persisted definitions are revalidated when read, so malformed legacy or manually altered rows do not reach the browser.

## Mutation and concurrency contract

An operator may own at most 25 saved views per tenant. Creation, rename, and delete execute in serializable transactions. Rename and delete use optimistic versions so a stale browser cannot overwrite or remove a change from another session.

- Create returns `201` and version `1`.
- Rename requires `expected_version` and returns the incremented view.
- Delete requires a quoted `If-Match` version and returns `204`.
- Duplicate names, stale versions, and the view cap return a typed `409` outcome.
- An unavailable or ambiguous write is never presented as success; the operator reloads the current server state before deciding whether to retry.

Each successful mutation writes an immutable audit event in the same database transaction. Audit metadata contains only the saved-view ID, domain, filter schema version, and view version. It intentionally excludes the user-entered name and every filter value.

## Rendering and storage contract

The save control is progressively disclosed beneath canonical list filters. Read-only users can understand the feature without seeing an enabled mutation. The management panel is loaded on Investigation search and distinguishes loading, offline, unavailable, empty, and populated states.

No saved view, result, balance, cursor, name, or mutation intent is written to `localStorage`, `sessionStorage`, IndexedDB, cookies, or another browser persistence layer. React state may hold the current rendered response only for the active page lifetime.

## Extension rule

A new filter or domain requires one atomic change set: application allowlist and canonicalizer, frontend parser/sanitizer, OpenAPI schema, domain authorization mapping, threat-model review, unit tests, adversarial response tests, and an end-to-end capture/open journey. Existing schema version 1 meanings must not be broadened silently; an incompatible change introduces a new schema version and an explicit migration or read policy.
