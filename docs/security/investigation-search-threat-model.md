# Investigation search security model

## Purpose

The investigation search is a locator, not a reporting database and not a broad discovery feature. It helps an authorized operator move from a known identifier to the canonical detail screen that owns the evidence. It deliberately returns no amounts, balances, journal lines, free-form event payloads, secrets, or customer-profile data.

## Authorization contract

A request must satisfy every entry condition before PostgreSQL is queried:

1. The browser has a valid server-issued session.
2. The signed actor has the `tenant:operator` or `tenant:admin` role.
3. The actor has the dedicated `investigation:read` scope.
4. The actor has at least one supported domain read scope. These scopes determine which record families can appear.
5. Account matches additionally repeat account ownership authorization in the database query.
6. The BFF and private API each apply a bounded rate limit. Production enforcement at the private API uses the shared limiter; the BFF limiter is defense in depth.

Tenant identity is taken only from the authenticated actor context. It is never accepted from a query string, form field, route parameter, or browser storage.

## Enumeration and inference threats

### Partial and fuzzy discovery

Prefix, substring, wildcard, phonetic, and free-text searches could let an operator enumerate references they did not already know. The released contract therefore accepts exactly one complete canonical UUID or one approved 8–128 character external reference. Unknown, repeated, empty, whitespace-padded, or additional query parameters fail before a protected read.

### Existence disclosure

An empty response means only that no authorized match is visible in the current tenant and scopes. The UI and API do not distinguish a missing record from an existing record outside the actor's authority. Search results use the same bounded locator shape regardless of domain.

### Cross-tenant access

Every SQL branch includes `tenant_id = authenticated_tenant`. The browser cannot submit a tenant selector. Account results also require a current ownership mapping for the authenticated subject. The database repository remains the final object-authorization boundary.

### Scope aggregation

Search does not turn one domain scope into access to another domain. Each SQL branch is enabled by a separate server-derived boolean. For example, `events:read` cannot reveal transfers, and `accounts:read` cannot reveal funding records.

### Oversized or hostile responses

The private API caps results at 20 and reads at most `limit + 1` rows to calculate truncation. The BFF stops reading an upstream body after 65,536 bytes, validates the exact JSON keys and enums, and replaces malformed upstream output with a generic unavailable response. The browser repeats the same typed validation.

### Sensitive data copied into URLs or storage

Only approved non-secret identifiers may appear as `q` in the URL. Search results remain in component memory and are not written to local storage, session storage, IndexedDB, caches, or service workers. Operators must not use personal data, access tokens, signing material, or raw payload content as a reference.

### Timing and high-rate probing

Both BFF and private API boundaries are rate limited by tenant and subject. The product presents a uniform no-authorized-match state and does not expose per-domain totals. Query indexes cover exact correlation identifiers so legitimate lookups remain predictable without enabling broad scans.

## Returned evidence contract

Every result contains only:

- record type and immutable record identifier;
- an optional typed related-record locator;
- a controlled safe label and status;
- a UTC evidence timestamp;
- source `postgresql` and freshness `search_snapshot`.

The result links to an existing canonical detail route, where that route performs its own authorization and retrieves current authoritative evidence. A locator must never be treated as proof that a financial outcome occurred.

## Operational controls

- Monitor `investigation:search` rate-limit denials and sustained unavailable responses.
- Review additions to supported record types as security-sensitive API changes.
- Keep exact-search indexes in schema migrations and test both forward and rollback migration paths.
- Do not add fuzzy search, global counts, cross-tenant administration, raw payload previews, or browser-side bulk filtering without a new threat model and explicit product/security approval.
