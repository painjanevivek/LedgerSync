# Redesigned LedgerSync: user journey and gap repairs

Date: 2026-10-10. App: `http://127.0.0.1:3000`. Scope: the current local INR ledger.

## Walk through the app

| Step | What the operator does | What the system proves | Diagram |
|---|---|---|---|
| 1. Sign in | Open the workspace and log in. Check the environment and operator. | Local login creates a short-lived signed session. Managed OIDC remains a separate configured production boundary. Each BFF and API request rechecks access. | Context and authentication, sections 3 and 6. |
| 2. Inspect Overview | Read operating balances, customer funds, timestamps, and latest reconciliation. | PostgreSQL supplies the financial values. Customer funds are a separate category. A recorded reconciliation describes its own snapshot. | Journey and primary reads, sections 2 and 7.5. |
| 3. Select an account | Open Accounts and inspect status, currency, balance, and versions. Create an account only when needed. | A new account starts at zero. Account configuration and financial versions have separate purposes. The command rechecks current access and account eligibility. | Account creation, section 7.1. |
| 4. Record funding | For value verified outside LedgerSync, record its account, exact amount, external reference, and supporting evidence reference. | The request stores evidence. It does not post funds or perform an external bank payment. | Funding, section 7.2. |
| 5. Approve and post | Review the funding record under the server policy. Open the approved record and post its journal. Check the receipt and balance. | Approval and posting are separate. Local policy allows the configured single operator. Production requires an independent authorized finance reviewer. Only the balanced journal changes funds. | Funding, section 7.2. |
| 6. Review a transfer | Select eligible INR source and destination accounts. Enter the exact amount. Review both accounts and the amount, then confirm once. | The BFF passes the original intent and retry identity to Go. PostgreSQL rechecks access, balance, status, currency, and policy. It commits the journal, paired postings, balance versions, retained outcome, and outbox atomically. | Transfer sequence, section 7.3. |
| 7. Inspect the result | Open the receipt, Posting evidence, and Evidence timeline and delivery. | Posting and delivery have independent states. The receipt links equal debit and credit entries and stored balance versions. Missing evidence stays missing. | Transfer and background processing, sections 7.3 and 8.1. |
| 8. Resolve uncertainty | If the result is unknown, retain the original intent, retry key, and request reference. Check status or retry exactly that request when access returns. | A lost response is not a rejection. Exact replay recovers the retained outcome without a second posting. A changed request under the same key is rejected. | Unknown result, section 7.4. |
| 9. Reconcile | Run an authorized internal comparison and inspect its run ID, scope, watermark, counts, and completion time. | The comparison uses a PostgreSQL snapshot. It does not check bank settlement or fix mismatches automatically. The main app's old run predates the inspected transfer; its evidence chain correctly reports no coverage. | Internal reconciliation, section 8.2. |
| 10. Investigate and hand off | Search an exact ID/reference. Open authoritative detail. Preserve a result as an investigation when repeatable context or handoff is needed. | Search is bounded and tenant-scoped. Return links preserve context. Saved references grant no account access. The recipient must read current authorized evidence. | Investigation, section 9. |
| 11. Inspect delivery and export | Open Events and the linked transfer. Review export scope before downloading CSV evidence. | Delivery failure does not reverse a posted transfer. CSV exports have bounded scope and are not database backups. | Background processing and investigation, sections 8.1 and 9. |
| 12. Protect and recover | Open Recovery. Use the fixed host backup command and an isolated restore drill. | The browser reads a sanitized index. Dumps remain in a protected host directory. The drill restores into a separate project and checks financial invariants and reconciliation. | Recovery and deployment, sections 10 and 11. |

The running-app review used existing authorized records. A transfer draft was opened for INR 1.00, reviewed, returned to edit, and cleared. It was not submitted. No account creation, funding, transfer, correction, or handoff command was performed in the main browser. Money-command and handoff evidence comes from the previously recorded isolated real-stack suite, not from invented writes in this walk-through.

## How the architecture supports the journey

The browser calls the Next.js same-origin BFF. It holds the browser session and checks CSRF on commands. The BFF sends a server credential and signed actor assertion to Go. Go applies route scope and domain policy; PostgreSQL enforces stored financial and authorization constraints. This is one financial authority with separate API and worker processes.

Redis stores disposable projections, streams, and live signals. Current balance reads come from PostgreSQL. A committed outbox separates the financial transaction from background publication and webhook work. A delivery retry repeats delivery work; it does not repeat the financial posting.

Local deployment publishes only the main frontend on port 3000 and optional peer on port 3001. Go, PostgreSQL, Redis, and the worker have no host-published port. The worker waits for schema, roles, and local bootstrap. Ordinary stop/restart preserves named volumes. The new readiness probe checks queue results and bounded progress, without opening a worker port.

## Repairs made in this review

| Gap | Repair |
|---|---|
| Guide skipped funding posting and transfer-result verification. | Add two explicit steps. Keep approval separate from the journal that changes the balance. |
| Guide omitted follow-up and handoff. | Add an accessible follow-up section for exact search, original intent, delivery, and investigation. Supporting evidence is described as a reference, not a browser document upload. |
| Account balance details used local time under a UTC header. | Use the shared machine-readable UTC Timestamp component. |
| Recovery declared complete evidence from a different schema and could pair different backup IDs. | Label different-schema proof as historical. Require the drill to name the exact backup and complete after its finalization. Failed refresh/offline evidence cannot keep a current-proof label. A passing label states its snapshot limit. |
| The main Recovery index contained only old backup/restore proof. | Create protected current-schema backups. Restore the latest into an isolated project, prove invariants, and expose only the sanitized receipt to the app. Existing backups remain retained. |
| A live worker could pass a process-only probe while its polls failed. | Feed the existing progress monitor into a private temporary readiness snapshot. Require successful polls on all four queues, no latest poll failure, progress within 60 seconds, and heartbeat within 15 seconds. Empty successful queues pass. Docker health and the existing status script read the result. |
| Deployment and journey drawings did not show these constraints. | Add explicit funding-receipt verification and the worker probe/bootstrap dependency. Render all 19 diagrams again. |

## Verification

- 22 focused Chromium cases passed without retries. They cover Guide order/follow-up, Asia/Kolkata versus UTC rendering, transfer review without a command, return links, stale/offline evidence, exports, recovery identity, accessibility, and responsive layout.
- All 211 frontend unit checks passed, including five added recovery-posture checks for missing/current evidence, schema differences, backup identity, chronology, and snapshot limits. Whole-web ESLint and the added test file lint passed. Main/peer production image builds and the worker image build passed.
- Focused Go worker and observability packages passed. Tests cover unstarted queues, idle success, failed/recovered polls, stalled progress, future/stale heartbeats, and malformed/missing files.
- An actual isolated worker passed idle readiness, failed its probe while remaining alive after Redis stopped, and recovered after Redis returned. Only the exact retained acceptance project was used. Test services were stopped; volumes were preserved.
- All 19 Mermaid blocks parsed and rendered. The renderer checks nonempty SVGs and text bounds; PNG, SVG, Mermaid source, and manifest are retained.
- Protected backup validation and corruption rejection passed. The latest backup is `backup-20261010T141059Z-ed1662e`, schema `000039_transfer_request_reference_repair.up.sql`, 320422 bytes. Its isolated drill restored six accounts, four transfers, and ten postings. Invalid journals, invalid posted transfers, and unexpected negative balances were all zero. Reconciliation matched with zero mismatches. Redis rebuilt three entries. Local elapsed recovery time was 36.84 seconds for this bounded drill.
- The active project identity, volumes, and financial fingerprint remained unchanged during restore. Its six accounts, four transfers, and ten postings match the pre-review fingerprint. Temporary restore resources were removed by the guarded drill. The live Recovery page names the exact latest backup and passing restore.

The backup manifest records the current Git HEAD, not an immutable build of the uncommitted working tree. Production image build results and local source changes are separate evidence. No production recovery target is inferred from one local drill.

## Remaining target decisions

Current local repairs do not establish a cloud deployment or external provider payout capability. Target owners must still select session-safe PostgreSQL pooling, controlled webhook egress, managed alert routing, identity, durable recovery storage, backup age/RPO/RTO policy, and provider contracts before release. Cross-device recovery when a browser loses its original intent remains the proposed enhancement SD-11. Current recovery preserves the original intent; it does not generate a new financial key automatically.

These constraints and proposed targets are labelled in the [system design](../architecture/ledgersync-system-design.md). Prior isolated money-command and handoff evidence remains in the [earlier journey verification](journey-verification-20261010.md). Physical-device, Safari, assistive-technology, and measured task-time evidence remain outside this local review.

## Review artifacts

- [Updated Guide capture](qa/journey-20261010/walkthrough-guide.jpg).
- [Current Recovery capture](qa/journey-20261010/walkthrough-recovery.jpg).
- [Rendered diagrams and sources](../architecture/diagrams/index.html).
- [Journey browser checks](../../web/tests/e2e/journey-alignment.spec.ts).
- [Recovery browser checks](../../web/tests/e2e/recovery-export.spec.ts).
- [Worker probe and focused tests](../../cmd/outbox-worker/health.go).
