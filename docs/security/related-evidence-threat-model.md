# Related evidence threat model

## Assets and trust boundaries

The feature can reveal that two tenant records are connected. That association may itself be sensitive even though the response contains no money or payload. Trust crosses the browser, the same-origin BFF, the signed actor assertion, the private API, and PostgreSQL.

## Principal threats and controls

### Cross-tenant or unauthorized relationship discovery

An attacker may replace the source UUID, source type, tenant, or target. Tenant identity is never caller supplied. Both server layers authenticate and authorize, the repository repeats tenant predicates, account sources/targets repeat owner checks, and source not-found responses do not distinguish missing from unauthorized evidence.

### Scope laundering through a permitted source

An operator who can read a transfer might attempt to discover corrections, events, funding, reconciliation, or accounts without those scopes. The repository receives a server-derived access envelope and places a domain-scope predicate on every target branch. The source type must also match its domain read scope.

### Heuristic or false relationships

Joining similar timestamps, amounts, notes, or JSON fields could create a misleading evidence trail. The implementation permits only schema keys and the already-defined PostgreSQL reconciliation snapshot visibility test. Transfer mismatch JSON is migrated to a tenant composite foreign key and is not queried heuristically.

### Sensitive-data aggregation

Combining otherwise authorized records could create a convenient high-value export. The response is limited to 20 navigation edges, safe labels/statuses/identifiers/timestamps, a 65,536-byte body, and `no-store`. The browser does not persist relationship responses. Money, balances, payloads, notes, evidence references, and endpoint destinations are rejected by the BFF sanitizer.

### Existence oracle and enumeration

Broad discovery is unavailable: callers must supply one complete released UUID, there are no filters or cursors, and both layers rate-limit independently from exact search. An absent edge is documented as non-conclusive.

### Client route injection

The server never returns a URL. The browser maps a fixed target-type allowlist to fixed local route templates and percent-encodes the UUID. Types without released routes render as identifiers only.

### Stale evidence treated as current

Every response is a generated relationship snapshot with a UTC timestamp and PostgreSQL source label. Offline and failed refresh states remain visibly non-current. Opening a target fetches that target’s current authoritative view; relationship metadata never substitutes for its domain values.

### Resource exhaustion

The API uses exact indexed keys, one source, a hard 20-edge bound, response byte limits, timeouts, and rate limits. Migration `000031` adds the missing transfer-mismatch and funding-journal indexes used by the relationship branches.

## Deferred risks

Free-form graph traversal, arbitrary depth, client-side graph expansion, cross-tenant joins, inferred similarity edges, and bulk relationship export are not released. They require separate product need, privacy review, query-plan evidence, and explicit global node/edge budgets.
