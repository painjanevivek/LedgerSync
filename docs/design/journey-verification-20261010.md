# TLS repair and real user journey verification

Date: 2026-10-10. Scope: the local LedgerSync app, canonical container builds, and rendered system diagrams.

## Changes made

| Observed fault | Repair | Evidence |
|---|---|---|
| A fresh pinned Go container failed a dependency download with `remote error: tls: protocol version not supported`. | Add a builder argument to the API and worker Dockerfiles. Local Compose supplies `tlsmlkem=0` for API, migration, and worker builds. Runtime images do not receive this setting. | The same fresh container passed the download with the compatibility setting. The canonical no-cache build passed for all three backend images. No host module cache or temporary build override was used for this proof. |
| Sequence message semicolons prevented Mermaid parsing. | Replace 20 message-line semicolons with commas inside diagram fences. | All 19 blocks parse and render with Mermaid 12.1.0. |
| An ER diagram initially used the browser's small default SVG size. | Set SVG width and height from its intrinsic view box. | All 19 rendered diagrams have nonzero dimensions and pass text/foreign-object bounds checks. |
| A newly created zero-balance account could not lead the operator to external funding. | Add **Record funding** for active accounts with funding read/write scopes. Name the separate incoming transfer action **Transfer into account**. | The real funding case starts without a seeded source account, follows this link, and completes request, review, and posting. |
| Funding text assumed another operator must approve and did not state the separate posting step. | Explain that the server applies the review policy. State that only journal posting changes the balance. | Real local single-operator review leaves the balance at zero. Posting changes it once. Replay adds no second journal. |
| A help-footer reason had a 2.76:1 contrast ratio on its dark surface. | Use the existing light rail text token. | Real guide state passes the same WCAG A/AA axe check after the repair. |
| Open advanced transfer filters covered **Save current filters**. | Keep the filter panel in normal page flow. Align labels and controls, retain compact tap targets, and allow responsive wrapping. | Real pointer clicks can open saved-filter capture. Structured filters persist; no forced clicks are used. Responsive and accessible browser cases pass. |
| On a phone, a selected transfer appeared after the long register. | Put the selected record first in DOM reading order. Keep it beside the register on desktop and before the register on narrow screens. | The layout assertion checks DOM order and 390/768/1366-pixel positions. The real phone capture shows the selected record. |
| Older real-stack tests assumed auto-login, always-open evidence sections, and broad text matches. The replay helper omitted a required request reference. | Use explicit login, visible detail controls, exact headings, and captured request-reference headers. Accept only bounded loopback ports and verify the selected isolated Docker owner. | Existing money, authorization, replay, database, export, and zero-violation assertions remain in place. |

The Go setting addresses a measured handshake compatibility problem. It keeps HTTPS certificate verification and Go module checksum verification enabled. Direct Dockerfile builds default to Go's normal TLS behavior; the local Compose value can be set to an empty string on a compatible network. The A/B result does not identify the exact Docker Desktop proxy fault. See the [runbook](../runbooks/local-runtime-smoke.md) and [official Go documentation](https://go.dev/doc/godebug).

## Real browser journey

The acceptance project is `ledgersync-acceptance-20261010173900-c49c3b4d`. It uses separate PostgreSQL/Redis volumes and protected local credentials. The real BFFs run on loopback ports 3102 and 3103. Docker labels bind each browser surface and database query to this project. It uses the same canonical app images as the main workspace.

All four cases passed together, serially, without retries, on the final app images:

1. **Account lifecycle:** log in; create at exact zero; replay creation; reject changed content under the original key; transfer INR 1.00 into the account; block and replay nonzero closure; freeze; reactivate; transfer the amount back; close at verified zero; run and replay reconciliation. Real API reads and scoped PostgreSQL queries prove durable results, balanced postings, audit evidence, and retained command outcomes.
2. **Follow-up and exports:** reopen those exact records; preserve server-backed filter and return context; inspect all seven evidence stages; open event timeline and delivery attempts; read Local Status, Developer/OpenAPI, and Recovery; verify transfer, account, and reconciliation CSV schema 2, attachment names, quoting, exact minor-unit strings, and `no-store`. Invalid duplicate filters and reversed date ranges are rejected. Eight real-page axe/reflow checks cover 320–1366 CSS pixels, including the 640-pixel zoom-equivalent layout.
3. **First-account funding and uncertain response:** create a zero account; record INR 2.00 funding evidence; approve under the explicit local policy; confirm zero before posting; post once; replay the funding command; reconcile the funding reference; create a destination account; post INR 1.00 with deliberate response loss; retry the same key, body, and request reference; reopen the result through search and return navigation. Each account ends with 100 minor units. Scoped SQL proves one funding journal, one transfer, and two transfer postings.
4. **Investigation and handoff:** save structured filters; preserve a matched reconciliation root; generate and download an audited ZIP; move ownership to `local-peer`; verify that the former owner's read returns `404`; reopen current evidence in the recipient's separate browser context; hand it back; verify revocation again; close and reopen the workspace. The peer receives no new account-ownership grant. An axe scan of the resulting workspace passes.

The response-loss case forwards the command to the real backend before dropping its response. It does not fabricate a financial result. The other cases use no route interception. The downloaded ZIP contains one manifest and three CSV files. All three file lengths and SHA-256 hashes match the manifest. The ZIP is retained only in ignored local test evidence.

## Protected recovery step

After the browser commands, the existing host backup script created a protected, finalized snapshot of the isolated ledger. The restore drill rejected a corrupted copy before creating its restore database. It then restored the valid snapshot into its own temporary Compose project and checked:

- 23 accounts, 24 transfers, and 58 postings restored with matching manifest counts.
- Zero unbalanced journals, zero invalid posted transfers, and zero unexpected negative balances. Funding clearing accounts retain their defined accounting exception.
- Redis rebuilt from PostgreSQL projections; 22 cache entries.
- Internal reconciliation matched with zero mismatches.
- The source acceptance project remained unchanged. The main workspace was separately fingerprinted before and after the whole verification.
- Local elapsed recovery time: 20.71 seconds for this one bounded test. This is not a production recovery target.
- The temporary restore containers, network, and volume were removed by the validated drill.

Only the sanitized recovery index was mounted read-only into the isolated API. The running Recovery page verified the finalized backup digest and passing restore result. Database dumps remained in the protected, ignored acceptance backup directory. The main API's recovery evidence index was not replaced.

## Build and regression evidence

| Check | Result |
|---|---|
| Canonical backend build | API, migration tool, and worker pass with `--no-cache`. |
| Frontend production images | Main and peer builds pass; both were restarted with the final UI. |
| Frontend unit suite | 206 passed; zero failures. |
| Whole-web ESLint | Passed, including the added real-stack tests. |
| Selected browser regression suite | 72 existing cases passed on the final app source. The added selected-record order case passed in its focused follow-up after its loaded-record locator was corrected. Together, 73 unique cases cover commands, funding, approvals, onboarding, journey alignment, filters, saved views, investigations, accessibility, and responsive layouts. |
| Real app suite | All four cases above pass together, with retries disabled. |
| Recovery | Protected isolated backup, corruption rejection, restore invariants, cache rebuild, matched reconciliation, and Recovery-page evidence passed. |
| Diagram outputs | 19 SVG, 19 PNG, and 19 Mermaid sources. All source hashes/assets match the gallery manifest. Fit/full-size switching and 320-pixel gallery reflow pass. |

The broader 148-case fixture run in the [earlier redesign record](ui-system-alignment.md) is prior evidence. The 73-case follow-up coverage is the affected regression set for these repairs. Automated browser checks do not measure user task time or replace physical-device and assistive-technology review.

## Main workspace preservation

The main app remains on [port 3000](http://127.0.0.1:3000); its optional peer remains on [port 3001](http://127.0.0.1:3001). Login, session, authorized account-list, and reconciliation-list reads pass through both frontends.

The main ledger retains six accounts, four transfers, and ten postings. Before/after fingerprints of balance projections, transfer rows, and posting rows are identical. Financial commands in this verification use only the isolated acceptance project. Existing main named volumes and protected credentials are preserved.

Actual isolated outbox records reached publication. The final worker log window has no poll errors. At the end of that validation run, the process-only Compose worker probe remained the reliability gap SD-06. The later [user journey walk-through](user-journey-walkthrough-20261010.md) implements and verifies the progress probe and updates the recovery proof. The cloud deployment and provider payout acceptance gates remain open in the [system design](../architecture/ledgersync-system-design.md).

The temporary acceptance services were stopped after review. Their test volumes, protected backup, and sanitized recovery index are retained under the isolated project. The main services remain running. No main or acceptance volume was deleted during this cleanup.

## Review artifacts

- [Rendered diagram gallery](../architecture/diagrams/index.html): sequence, context, components, data model, and local/proposed deployment diagrams.
- [Diagram manifest](../architecture/diagrams/manifest.json): source hashes, source lines, and dimensions.
- [Real app screen captures](qa/journey-20261010/README.md).
- [Real-stack test setup](../../web/tests/system/README.md).
- [Full journey test sources](../../web/tests/system/funding-transfer-journey.real-stack.spec.ts), [lifecycle and evidence](../../web/tests/system/account-lifecycle.real-stack.spec.ts), and [handoff](../../web/tests/system/investigation-handoff.real-stack.spec.ts).

Rebuild the diagrams with [the local renderer](../../scripts/render-system-diagrams.mjs). Local raw logs, backups, credentials, and database exports stay in ignored directories.
