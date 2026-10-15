# ADR-004: Preserve financial outcomes while simplifying navigation

Date: 14 October 2026. Status: design accepted; screen changes pending R05.

## Outcome contract

Posted means an authoritative commit exists. Rejected means no posting for that command. Unknown means the client cannot prove the result. Preserve the complete normalised body, idempotency key, and request reference until stored evidence resolves an unknown result.

Delivery is separate from posting. Historical means a retained observation with its collection time, not current truth. Unavailable is not empty. Empty requires a successful authorised read. A trace is diagnostic evidence, not financial authority.

Keep existing error codes, exact string money, large version encodings, replay results, and read-your-writes semantics. An interface simplification must not merge these states.

## Navigation compatibility map

| Group | Existing routes to retain | Change |
|---|---|---|
| Work | `/`, `/accounts`, `/funding`, `/transfers`, `/approvals` | One next safe action and clear review. |
| Investigate | `/search`, `/corrections`, `/reconciliation`, authorised investigation details | One search-to-saved-evidence path. |
| Operations | `/local-status`, `/events`, `/webhooks`, `/recovery` | Group navigation; keep health, delivery, backup, and restore conclusions separate. |
| Developer | `/developer` and existing contract endpoints | Quickstart, Reference, Systems Lab sections. |
| Help | `/guide` | One glossary and complete first-run explanation. |

No route rename is needed for the first restructure. Preserve validated query filters, sort, pagination, return links, and existing record URLs. Require an explicit redirect map if a later packet changes a route.

Keep technical details behind accessible disclosures. Never hide amount, currency, actor, approval effect, or outcome uncertainty from a high-risk decision.
