# LedgerSync design QA

## Compared artifacts

- Canonical source: `docs/design/reference/ledgersync-overview-canonical.png`
- Verified implementation capture: `docs/design/qa/overview-implementation-browser.png`
- Normalized above-the-fold comparison: `docs/design/qa/overview-reference-vs-implementation.png`
- Supporting route captures: Accounts, Transfers, and Reconciliation under `docs/design/qa/`

The comparison normalizes the selected 1440 × 1024 source and the Codex in-app browser capture into equal 1265 × 712 above-the-fold frames. The app was also measured at an explicit 1440 × 1024 browser viewport and at 390 × 844 for responsive behavior.

## Fidelity review

| Criterion | Result | Evidence |
|---|---|---|
| Application shell | Passed | Persistent deep-navy rail, LedgerSync wordmark, environment context, four-item navigation, session/operator footer, and pale evidence canvas match the source hierarchy. |
| Overview hierarchy | Passed | Eyebrow, single serif H1, statement balance, reconciliation proof, transfer table, and trust footer appear in the same document order as the reference. |
| Financial statement | Passed | Exact tabular amount, explicit USD, account scope, authoritative timestamp, and ledger version are grouped in one restrained bordered surface. |
| Reconciliation evidence | Passed | Green verified state, mismatch count, projection explanation, checked time, and evidence action are visibly separated from the transfer table. |
| Transfer table | Passed | Immutable ID, source, destination, exact amount, financial status, delivery status, UTC time, and record action are presented as a native table with stable alignment. |
| Typography | Passed | Serif headings, bold sans statement numerals, compact sans UI text, and monospace identifiers/timestamps reproduce the selected editorial-operational character. |
| Color and elevation | Passed | Navy structure, cool canvas, white documents, cobalt links, green proof, amber delay, red rejection, fine borders, and minimal shadow match the source semantics. |
| Cross-route consistency | Passed | Overview, Accounts, Transfers, Account detail, and Reconciliation share one shell, token set, status grammar, footer, and evidence composition. |
| Responsive behavior | Passed | At compact widths the shell uses a labelled top bar and full-height focus-managed drawer; evidence becomes cards and body scroll width stays within the viewport. |
| Accessibility | Passed | Native landmarks/tables, one H1, labelled navigation, visible focus ring, icon-plus-text status, reduced-motion handling, and the automated axe check pass. |
| Interaction safety | Passed | Authenticated transfers retain exact minor-unit conversion, CSRF protection, permission gating, stored idempotency retry, unknown-result recovery, and no duplicate-submit semantics. |
| Demo truthfulness | Passed | The direct local dashboard uses a server-gated demo identity and deterministic PostgreSQL records labelled `Isolated demo`; production rejects all demo configuration and unauthenticated routes display no invented evidence. |

## Intentional deviations from the source

- The sample organization is `Meridian Labs` rather than `Acme Financial Ltd` to preserve the already selected prototype identity.
- Local mode says `Isolated demo` instead of `Production`; the browser cannot enable or select the demo identity.
- The demo operator and tenant come from server-only configuration and real database authorization paths. A verified OIDC session replaces them in production.
- The implementation uses Phosphor icons rather than approximated or handcrafted source glyphs.

## Verification evidence

- `npm run lint`: passed.
- `npm run build`: passed; Overview, Accounts, Account detail, Transfers, Reconciliation, sign-in redirect, and BFF/API routes compile.
- `npm test`: 13/13 unit, security, demo-isolation, exact-money, and financial-semantics tests passed.
- `npm run test:e2e`: 15/15 Chromium flows passed, including safe same-key retry, exact-money rejection, object detail, six primary viewports, 320 px zoom/reflow, drawer focus, forced colors, copy announcement, and axe accessibility.
- Final full-stack Docker smoke remains pending because the local Docker Desktop daemon was unavailable; this is recorded as a release gate rather than a pass.

final result: local UI quality passed; Docker, physical-device, managed-restore, finance, and pilot-owner gates remain open
