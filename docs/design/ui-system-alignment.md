# UI and system alignment

**Date:** 2026-10-10. **Owner:** Vivek Painjane. **Backup with delegated decision authority:** MasterVaik. Contact routes and authority scope are not yet recorded.

This change builds on the existing Chain of Custody design. It keeps the current Next.js and Go stack. The [system design](../architecture/ledgersync-system-design.md) records the user sequences, components, data authority, deployment topology, and production gaps.

## User path

1. Create or select an authorized account.
2. Record the external funding reference when funding is needed.
3. Review the funding record. The server applies the approval policy.
4. Post the approved journal.
5. Choose the source, destination, and exact transfer amount.
6. Review the transfer. Confirm once.
7. Read the posting result. Check delivery and reconciliation separately.
8. Keep the request reference. Use the same intent and key if the result is unknown.

Each screen now has a short task description. Definitions and supporting records use accessible disclosures. An unresolved outcome, a failed refresh, and an action prerequisite remain visible.

## Implemented changes

| Finding | Change | System rule |
|---|---|---|
| The entry path had repeated explanations and weak next-step guidance. | Overview has a state-based next action and a compact setup path. The guide follows the same order. | Step completion comes from recorded facts and explicit operator confirmations. |
| Filtering followed the account or funding records. | Account, funding, and transfer filters appear before their records. | Filtering remains server-backed and scoped. Page counts do not imply a total. |
| Account versions ran into their timestamps. | Versions and UTC source times use separate rows with a consistent gap. | The balance version and source time remain distinct evidence. |
| A transfer form appeared after its history and detail. | The form appears before history. A requested destination opens it. | Account scope must be complete. Review precedes the financial command. |
| A requested destination could disappear without a clear block. | An unavailable requested destination blocks the form. | The UI must not select another destination silently. |
| Detail refresh errors left an old transfer folio without a warning. | The folio shows its verified time and historical state. It has a focused retry. | A prior result is not a current successful read. |
| Initial offline access could show a verified empty history. | Offline and unverified reads have separate states. | A failed or absent read does not prove an empty ledger. |
| Posting and delivery copy could disagree. | Delivery warnings state the actual posting status. | Delivery retry does not alter the financial result. |
| An incomplete matched run could show a passing result. | Reconciliation uses one strict verification predicate for title, badge, and history icon. | Passing requires a completed run, ID, watermark, and zero mismatches. |
| Funding policy copy could infer production from an unreviewed record. | Before review, the UI states that server policy applies. Recorded local policy remains explicit. | The backend owns approval policy and independence checks. |
| The funding check could imply external settlement reconciliation. | It is labelled Internal journal check. | The check compares stored journal totals; it does not query a bank statement. |
| Saved work required a prior search. | Saved views and investigations can open on an empty search page. | Read permissions still apply. Panels load when opened. |
| Search and transfer account links lost investigation context. | Detail links retain a validated internal return path. | Follow-up returns to the same search or filtered register. |
| Secondary navigation and filter labels were hidden on Transfers. | Group summaries and field labels remain visible. Mobile focus includes visible summaries. | Keyboard users can reach every permitted route and field. |
| Small type and low-contrast metadata reduced readability. | Metadata is at least 12px. Shared colors meet the normal-text contrast target on assigned surfaces. | Status and freshness must remain readable on desktop and phone. |
| The worker parsed managed Redis URLs differently from the API. | Both use the shared Redis options parser. | Local host addresses and managed TLS URLs use the same connection contract. |
| Local startup found live-room tables under older migration names. | Migration 37 accepts the proven legacy shape, checks its definitions and records, and installs the current controls. | Unknown schema changes stop the migration. Existing records and migration history are preserved. |
| The finalized-request guard blocked the first transfer request reference. | Finalized transfers can attach their first reference when every other field is unchanged. Exact reference replay avoids an update; rebinding is rejected. Migration 39 also repairs databases that already applied the original migration 37. | The reference and financial outcome commit in one transaction. A retry returns the original result. |
| An invite could exceed the expiry of its room. | Stored and returned invite expiry are capped at the room expiry. | A follow-up invitation cannot extend the investigation's lifetime. |

## Verification

| Check | Result |
|---|---|
| Frontend unit suite, `npm test` | 206 passed; no failures. |
| Selected Chromium browser suite | 148 passed across 21 journey, authorization, financial-state, accessibility, and responsive test files. |
| Frontend build | Production Next.js build passed. Both local frontend images built successfully. |
| Lint | Whole-web ESLint and the final edited browser tests passed. |
| Visual review | Eight desktop previews and one phone preview checked. Captured documents have no horizontal overflow. |
| Backend packages | `go test ./internal/platform/db ./internal/platform/redisconn ./cmd/outbox-worker` passed. The worker package compiled; it has no package-local test files. |
| Database migration and transfer reference | Seven focused integration test groups passed on isolated PostgreSQL databases. They cover fresh and known-legacy schemas, posting/replay, downgrade, four index-name collisions, eleven incompatible schema/data cases, and an upgrade from the original migration 37 through migration 39. The upgrade preserves populated records, prior history, function owners, and access permissions. Unexpected helper metadata stops the upgrade atomically. |
| Diagrams | The follow-up parsed and rendered all 19 diagrams, with SVG/PNG output and text-bound checks. See the [gallery](../architecture/diagrams/index.html). |

The browser suite covers widths from 320 to 2560 CSS pixels, keyboard navigation, forced colors, reduced motion, and zoom-equivalent reflow. The eight-page accessibility scan was split into one test per page after its combined 30-second budget expired. Each page keeps the same WCAG rule tags and zero-violation assertion. The final 148-test run passed without retries.

Browser checks use explicit API fixtures. They do not establish a live financial backend, external settlement, or cloud deployment. Physical-device, Safari, and assistive-technology user review remain separate work. No user task-time improvement was measured in this change.

The [preview index](qa/redesign-20261010/README.md) links the nine initial redesign images. They contain fixture data. The [follow-up real app captures](qa/journey-20261010/README.md) show the final running UI after the journey repairs.

## Local runtime

The frontend and backend are running in the existing local Compose project. The API, outbox worker, PostgreSQL, Redis, main frontend, and peer frontend report healthy. The migration and demo-seed jobs exited successfully. The main frontend is available at `http://127.0.0.1:3000`; the peer is at `http://127.0.0.1:3001`.

The migration runner applied migrations 37, 38, and 39 while preserving the two older live-room migration records. The final database has 41 migration records. Its six accounts, four transfers, ten postings, and all three financial fingerprints are unchanged. Prior migration timestamps, named volumes, environment settings, and runtime security controls are preserved.

Login, session, account, and reconciliation reads passed through both local frontends. Referenced financial posting and exact replay passed in isolated PostgreSQL tests. The live smoke checks did not create financial records in the user's local database.

After role provisioning, the running worker reported query failures while its process probe remained healthy. Its schema and table permissions were correct. A worker restart restored successful database queries; the final outbox check found no pending or dead records. The precise connection-state cause was not established. The system action map retains the need for a progress-based worker probe.

The initial backend image build hit a dependency-download TLS error. The follow-up fixed the local builder handshake compatibility setting and passed a canonical no-cache build of all three backend images. TLS certificate and module checksum checks remain enabled. The [follow-up verification record](journey-verification-20261010.md) reports the live browser journey and its UI repairs. This result does not qualify a cloud release.

## Deployment boundary

The system document distinguishes the current local Compose topology, the configured Vercel target, and the proposed AWS target. Cloud acceptance still needs the exact database connection mode, identity and key access, controlled egress, durable recovery storage, recovery proof, and worker progress monitoring. Provider payouts remain deferred under their separate program.

Update the affected sequence and source links when a command, authorization rule, worker boundary, or deployment adapter changes.
