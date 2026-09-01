# Investigation workspace contract

## Purpose

An investigation workspace lets an operator preserve how a case was found, reopen it against current evidence, and hand ownership to another operator without copying database screenshots or financial values. The workspace is a server-owned navigation and audit record, not a second ledger, case-note system, or financial snapshot.

## Stored data

`investigation_workspaces` stores the tenant, current owner subject, safe title, fixed taxonomy, open/closed status, optimistic version, UTC lifecycle timestamps, query kind/value, and one root record type/ID. `investigation_workspace_references` stores the root plus at most 20 canonical record references captured from the existing deterministic related-evidence service.

The workspace does not store balances, amounts, currencies, status snapshots, event payloads, request/response bodies, headers, credentials, tokens, customer names, email addresses, or free-form notes. Safe titles are capped at 80 Unicode characters and reject markup, URLs, email-shaped text, and common secret markers. Taxonomy is a fixed enum.

## Current versus historical evidence

The detail response has two deliberately separate objects:

- `historical_context` contains the original bounded query, references that are still authorized, the count of currently withheld references, and up to 100 newest immutable lifecycle events.
- `current_evidence` is generated at read time by the canonical exact-search and related-evidence repositories. Its source and freshness fields remain explicit. It is never written back into the workspace.

Listing, opening, and changing a workspace repeat tenant, owner, root-domain scope, object-existence, and account-ownership checks. Lifecycle writes perform this check again after locking the workspace inside the serializable transaction. If the root is outside the current authority, the API returns the same non-disclosing not-found response used for a missing or differently owned workspace. Historical secondary references are individually rechecked; unauthorized references are omitted and counted without returning their identifiers.

## Lifecycle and concurrency

Creation, handoff, close, and reopen use serializable repository sequences. Each mutation requires the current version and increments it exactly once. The state transitions are:

```text
create → open
open → handoff (owner changes; status remains open)
open → closed
closed → open
```

Handoff accepts an exact server-issued subject identifier, never a display name or email note. It atomically changes `owner_subject_id`; the previous owner loses access as soon as the transaction commits. The recipient must authenticate in the same tenant and hold current `investigation:read` plus the root-domain read scope before the workspace is visible. The system does not claim that a typed recipient exists before that principal authenticates; operational UI states this limitation before confirmation.

Every lifecycle transaction inserts an `audit_events` row with target type `investigation_workspace`. Audit metadata contains only taxonomy, status, workspace version, and—on creation—the captured reference count. Titles, query values, record references, and recipient subject IDs are not copied into audit metadata. The API role has insert-only authority on audit events and no delete authority on either workspace table.

## Bounds and ownership

- At most 50 open workspaces per tenant/operator. Closed history does not consume that open-case quota.
- Lists return at most 50 most recently updated authorized workspaces.
- A workspace holds at most 21 references: one root and 20 deterministic relationships.
- A detail response returns at most 20 current relationships and 100 recent lifecycle entries, with an explicit history truncation flag.
- Browser and private API request bodies are capped at 8 KiB or 4 KiB; BFF responses are capped at 256 KiB.
- Browser storage is not used for workspace records, queries, ownership, or lifecycle intents.

## API surface

- `GET/POST /api/investigation/workspaces`
- `GET /api/investigation/workspaces/{investigationId}`
- `POST /api/investigation/workspaces/{investigationId}/handoff`
- `POST /api/investigation/workspaces/{investigationId}/close`
- `POST /api/investigation/workspaces/{investigationId}/reopen`
- `POST /api/investigation/workspaces/{investigationId}/evidence-bundle`

Reads require an operator role, `investigation:read`, and at least one released record-domain read scope. Writes additionally require `investigation:write`, same-origin CSRF at the BFF, strict JSON, bounded rate limits, and a current optimistic version. Evidence-bundle generation requires `exports:read`, same-origin CSRF, current workspace version, and the same current root-domain/object authorization; it does not grant investigation mutation authority. The canonical OpenAPI 3.3.0 contract and generated SDK/Postman artifacts describe the same surface.

## Evidence bundle custody boundary

The evidence bundle is generated in memory only after the operator reviews its exact scope. It is capped at 512 KiB and contains `manifest.json`, `historical-references.csv`, `current-evidence.csv`, and `request-references.csv`. The manifest gives schema version, generated/expiry UTC, workspace and request references, row/byte counts, and SHA-256 hashes for every CSV. The complete ZIP digest is returned in a response header and independently verified by the BFF before delivery.

The archive excludes titles, safe labels, amounts, balances, currencies, payloads, bodies, headers, credentials, notes, operator labels and tenant labels. The application records one immutable successful-generation audit event before writing bytes; this truthfully proves generation and authorization without claiming that a client completed its download. If generation or audit fails, no attachment headers or archive bytes are returned. The application retains no archive after the HTTP response; the 15-minute handling expiry is recorded in the manifest and audit metadata. A downloaded file remains historical evidence regardless of its age and must never be presented as current financial authority.

## Deliberate exclusions

Free-form notes, arbitrary attachments, arbitrary URLs, collaborator lists, external email delivery, background sharing links, stored archives, public links, and vendor-hosted exports are not part of this phase. Notes require an approved privacy, retention, search, rendering/injection, legal hold, and deletion policy. The released evidence bundle is deliberately identifier-only, synchronous, bounded, audited, and unretained.
