# Research and Design Decisions

| Topic | Decision | Rationale | Rejected alternative |
|---|---|---|---|
| Financial truth | PostgreSQL ledger and projections. | Transactions, constraints, recovery and reconciliation fit money. | Redis/event stream as authority. |
| Money | `BIGINT` minor units + ISO currency. | Exact values, no hidden rounding. | Float/double; arbitrary precision for normal balances. |
| Ledger | Append-only balanced postings with rebuildable balances. | Auditable and repairable by compensating entries. | Mutable balance only. |
| Concurrency | Deterministic two-account row locking in a short transaction. | Prevents overdraft/deadlock without broad serializability. | Unordered locks or remote calls in transaction. |
| Retry safety | Idempotency key, request fingerprint and saved response. | Network failures after commit do not create second transfers. | UI-only duplicate prevention. |
| Events | Transactional outbox, at-least-once worker/consumer. | A commit cannot lose its event obligation. | Direct post-commit publish; exactly-once claim. |
| RYEW | Signed required version, cache version compare, bounded primary fallback. | Constant-time freshness proof under cache lag. | Random token/stream scan/unbounded wait. |
| Public API | Same-origin Next BFF, private Go API. | Secure browser session and no raw gRPC/CORS mismatch. | Browser-to-internal-services. |
| Operations | OTel, private network, managed backup/secrets. | Detect/recover from the actual financial risks. | Kubernetes/Kafka first. |
| Local product demonstration | Production-blocked server-side demo session with deterministic PostgreSQL fixtures through real BFF/API contracts. | Makes the UI genuinely usable without weakening production OIDC or maintaining fake JSX behavior. | Hardcoded preview components; browser-only auth bypass; production demo accounts. |
| Dashboard money movement | API-first by default; dashboard transfer creation requires an explicit pilot role/policy and prepare-review-confirm flow. | Preserves the API-first boundary and limits fraud/approval surface while supporting a controlled operator use case. | Unrestricted operator transfer button; removing the transfer UX entirely. |
| Responsive architecture | One semantic component tree with shared view models, layout tokens, CSS Grid/Flexbox and component queries. | Prevents mobile/desktop financial values, authorization, states, and terminology from drifting. | Separate mobile application or duplicated responsive markup for the pilot. |
| Financial evidence UI | Reconciliation and aggregate-balance claims require approved accounting semantics and authoritative evidence. | A trust-first UI must never look more certain than the ledger can prove. | Fictional passing evidence; summing unlike account categories. |
| Frontend verification | Behavior-based Playwright matrix, axe plus manual accessibility, visual regression, performance budgets, and real-device smoke tests. | Screenshots alone do not prove that a financial workflow works on touch, keyboard, zoom, slow networks, or unknown outcomes. | Desktop happy-path screenshots as release evidence. |

Useful primary references: PostgreSQL [numeric types](https://www.postgresql.org/docs/current/datatype-numeric.html), [constraints](https://www.postgresql.org/docs/current/ddl-constraints.html), [isolation](https://www.postgresql.org/docs/current/transaction-iso.html), and [PITR](https://www.postgresql.org/docs/current/continuous-archiving.html); [Redis Streams](https://redis.io/docs/latest/develop/data-types/streams/); [OWASP API Top 10](https://owasp.org/API-Security/editions/2023/en/0x11-t10/); [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/); [NIST SSDF](https://csrc.nist.gov/pubs/sp/800/218/final).
