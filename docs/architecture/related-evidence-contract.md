# Related evidence contract

## Purpose

The related-evidence rail lets an authorized operator move from one known LedgerSync record to directly connected evidence without copying screenshots, guessing from timestamps, or searching broad text. It is navigation metadata, not a second financial read model.

## Request boundary

`GET /investigation/related/{recordType}/{recordId}` accepts one released record type and one complete UUID. It accepts no query parameters and returns at most 20 relationships. The browser BFF and private API both require:

- a valid signed identity;
- a server-issued `tenant:operator` or `tenant:admin` role;
- `investigation:read`;
- the read scope for the source domain; and
- a target-domain read scope before an edge in that domain is returned.

An absent edge does not prove that the target is absent. It can also mean that the target domain is outside the current scopes, the source is outside account ownership, the bounded response was truncated, or current evidence is unavailable.

## Relationship rules

Relationships are created only from explicit keys already owned by the authoritative PostgreSQL schema or from the existing reconciliation snapshot visibility rule:

- accounts link to their debit/credit transfers, destination funding records, account events, and account-keyed mismatches;
- transfers link to authorized account records, their journal and postings, outbox events, delivery attempts, reconciliation evidence, transfer-keyed mismatches, and correction controls;
- funding records link to authorized accounts, approval evidence, their journal and postings, outbox events, and explicit compensation records;
- outbox events link to their explicit transfer/account keys and delivery attempts;
- reconciliation runs and mismatches link through `run_id`, `account_id`, or the promoted `transfer_id` foreign key;
- correction controls link to their original and compensation transfers and explicit approval evidence.

No relationship is derived from similar text, timestamps, amounts, actor names, notes, or JSON traversal at read time.

## Reconciliation key migration

Migration `000031_reconciliation_relationship_keys` promotes the known transfer reference produced by the reconciliation worker into `reconciliation_mismatches.transfer_id`. The migration validates the UUID, tenant, and referenced transfer before creating a composite tenant foreign key. New mismatch writes populate the column directly. This closes the previous gap where a transfer relationship existed only in sanitized diagnostic JSON.

## Response semantics

Every relationship contains only:

- a versioned relationship type;
- a typed target UUID;
- a bounded safe label;
- a bounded status;
- one UTC occurrence timestamp;
- `source: postgresql`; and
- `freshness: relationship_snapshot`.

Amounts, balances, currency values, payloads, endpoint details, operator notes, document references, raw errors, and secrets are excluded. The consumer must open the canonical target view for authoritative domain values.

The browser renders loading, offline, denied/unavailable, verified-empty, populated, and truncated states separately. Records without a released detail route—such as journals, postings, delivery attempts, and approvals—remain copyable identifiers and are never linked to an invented page.

## Operational limits

- Response node/edge count: 20.
- Upstream body limit: 65,536 bytes before JSON parsing.
- Cache behavior: `no-store` end to end.
- Rate limiting: independent `investigation:relationships` bucket.
- Read behavior: GET-only and non-mutating.
- Browser persistence: none.

The linear rail is the released investigation model. A graph visualization remains deferred until the linear workflow has usage evidence and explicit node/edge budgets.
