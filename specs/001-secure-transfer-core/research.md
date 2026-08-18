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

Useful primary references: PostgreSQL [numeric types](https://www.postgresql.org/docs/current/datatype-numeric.html), [constraints](https://www.postgresql.org/docs/current/ddl-constraints.html), [isolation](https://www.postgresql.org/docs/current/transaction-iso.html), and [PITR](https://www.postgresql.org/docs/current/continuous-archiving.html); [Redis Streams](https://redis.io/docs/latest/develop/data-types/streams/); [OWASP API Top 10](https://owasp.org/API-Security/editions/2023/en/0x11-t10/); [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/); [NIST SSDF](https://csrc.nist.gov/pubs/sp/800/218/final).
