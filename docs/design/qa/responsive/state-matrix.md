# LedgerSync MVP route, state, and viewport contract

This matrix is the review contract for the selected LedgerSync operator UI. It prevents a visually polished happy path from hiding financial uncertainty. `Required` means the state has an automated fixture and a stored screenshot where its layout materially differs. `Shared` means the route inherits the shell-level state. `N/A` means the state would be semantically false for that screen.

## Supported viewport boundaries

| Boundary | Test size | Product reason |
|---|---:|---|
| Compact phone | 390 × 844 | One-column evidence cards, touch navigation, and no page-level horizontal overflow. |
| Tablet portrait | 768 × 1024 | Rail-to-compact navigation boundary and long financial status copy. |
| Tablet landscape / small laptop | 1024 × 768 | Dense comparison tables without removing identifiers. |
| Standard desktop | 1440 × 900 | Primary operator investigation workspace and baseline-review size. |
| Wide desktop | 1920 × 1080 | Maximum readable content width; evidence must not stretch into scanning-hostile lines. |
| Zoom / reflow | 200% and 400% | Existing accessibility tests verify 200% zoom and 400% reflow at a 320 CSS-pixel content boundary. |

## Route × state matrix

| Screen / route | Populated | Loading | Empty | Error | Offline | Permission denied | Unknown outcome | Mismatch | Baseline fixture |
|---|---|---|---|---|---|---|---|---|---|
| Shared session shell | Shared | Required | N/A | Required | Required | Required | N/A | N/A | session pending/401, browser offline |
| Overview `/` | Required | Required | Required | Required | Required | Shared | N/A | Required | normal, empty accounts, mixed currency, mismatch run |
| Account directory `/accounts` | Required | Required | Required | Required | Required | Required | N/A | N/A | bounded list, pending list, empty, 503, 403, offline |
| Account detail `/accounts/{id}` | Required | Required | Required history/audit | Required per balance/history | Required | Required per history | N/A | N/A | independent summary, balance, history, and audit fixtures |
| Transfer workspace `/transfers` | Required | Required history | Required history | Required | Required | Required write scope | Required | N/A | read/write roles, empty/error list, 504 same-key result |
| Transfer detail `/transfers/{id}` | Required | Required | Required postings for rejection | Required | Required | Shared | Required delivery state | N/A | posted + retrying delivery, rejected/no journal |
| Reconciliation list `/reconciliation` | Required | Required | Required | Required | Required | Shared | Required missing evidence | Required | matched, pending, absent, 503, mismatch |
| Reconciliation detail `/reconciliation/{id}` | Required | Required | N/A | Required | Required | Shared | Required unavailable evidence | Required | matched/mismatch detail and inaccessible record |

## Deterministic baseline set

The committed Chromium baselines cover every MVP route at 1440 × 900 plus materially different compact/tablet states: account list loading, empty, dependency error, permission denial, offline evidence, independent detail failures, transfer unknown outcome, read-only transfer controls, missing session, reconciliation mismatch, and mixed-currency aggregate refusal. Baselines are platform-qualified so Windows review and Linux CI do not silently approve each other's font-rendering differences. Functional responsive tests separately exercise all six viewport sizes so visual coverage does not replace behavior checks.

## Review and update procedure

1. Run `npm run test:visual` from `web/`. Any pixel drift fails; the test does not auto-approve it.
2. Inspect the Playwright diff and confirm financial status, exact amount, full identifier access, focus affordance, error provenance, and disabled-control explanation remain intact.
3. For an intentional change, run `npm run test:visual:update`, inspect every changed PNG, and record the reviewer, date, reason, and affected baseline in `baseline-approvals.md`.
4. Never approve a broad baseline refresh after a failing test without inspecting individual differences. Generated `playwright-report/` and `test-results/` remain disposable and ignored; reviewed PNGs are source-controlled evidence.
