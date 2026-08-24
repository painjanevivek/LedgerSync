# LedgerSync operator UI release evidence

**Evidence date:** 2026-08-24 (Asia/Calcutta)

**Scope:** functional and responsive engineering release suite

**Boundary:** local/demo and CI evidence only; this file does not approve managed or partner traffic

## Implemented release candidate

- Production-blocked server demo identity and deterministic INR/PostgreSQL seed.
- Same-origin BFF with signed session, CSRF protection, short-lived actor assertions, bounded reads, safe errors, and no-store financial responses.
- Tenant-authorized account, transfer, posting, reconciliation, and evidence list/detail journeys.
- Exact-money prepare → review → confirm → posted/rejected/unknown journey with same-key retry.
- One semantic component tree across compact phone, tablet, laptop, desktop, zoom, reflow, forced-colors, reduced-motion, offline, and degraded states.
- Explicit separation of financial posting, downstream delivery, and reconciliation truth.

## Passing automated evidence

| Suite | Result |
|---|---|
| Go formatting, vet, unit, contract, integration, fault, and system packages | **PASS** — `gofmt -l cmd internal tests` empty, `go vet ./cmd/... ./internal/... ./tests/...`, and `go test ./cmd/... ./internal/... ./tests/... -count=1` passed |
| Web lint and unit/security/semantics | **PASS** — 20/20 tests |
| Next.js optimized production build | **PASS** — all operator and same-origin BFF routes compiled |
| Browser journeys | **PASS** — 45/45 on Windows Chromium and 45/45 in the pinned Linux Playwright image |
| Visual regression | **PASS** — 19/19 reviewed route/state baselines; no-update comparison passes on Windows and Linux |
| Responsive matrix | **PASS automated** — 390×844, 768×1024, 1024×768, 1366×768, 1440×900, and 1920×1080 |
| Accessibility automation | **PASS** — axe A/AA, keyboard/focus, 320 CSS-pixel reflow, increased text spacing, forced colors, reduced motion, and 44 CSS-pixel compact targets |
| Browser performance | **PASS** — throttled compact LCP/INP/CLS and progressive 100-row history checks stayed within committed budgets |
| Static JavaScript budget | **PASS** — 10 chunks, 659,957 bytes total, 229,156-byte largest chunk; limits 2,000,000 / 350,000 |
| Compose definitions | **PASS** — supported and fault profiles parse with `docker compose ... config --quiet` |
| Fresh INR real stack | **PASS** — PostgreSQL, Redis, migrations, seed, API, BFF, and worker started; same-key retry moved exactly 100 paise once; reconciliation evidence passed |
| Production demo isolation | **PASS** — unit/security tests reject production demo identity, insecure cookie weakening, and static private API credentials |

## Reviewable artifacts

- Reviewed cross-platform screenshots: [responsive baselines](../design/qa/responsive/baselines/) and [baseline approvals](../design/qa/responsive/baseline-approvals.md).
- Route/state inventory: [state matrix](../design/qa/responsive/state-matrix.md).
- Exact test and budget method: [performance baseline](../performance-baseline.md).
- Real dependency path and restart contract: [Phase 4 real-stack evidence](phase-4-real-stack.md).
- The Quality workflow uploads `operator-browser-evidence-<commit SHA>` for 30 days and `phase-0d-release-evidence-<commit SHA>` for 90 days.
- Baseline SHA `22f9fdc0ff329be2e11f845d49cf1174e5fad913` passed contract, production-path, Go/web/live-dependency, and security jobs. Its browser job exposed two stale Linux INR screenshots; this phase regenerates and reviews those exact baselines and the same pinned Linux image then passes 45/45.

## Known release gates — not represented as passes

- Physical iOS, Android, tablet, laptop, and desktop review is still required; automation/emulation does not close T094/TASK-013.
- Finance/product approval of balance terminology and aggregation is still required; T095/TASK-014 remains open.
- Security/risk approval of roles, transfer/velocity limits, and pause authority remains open.
- The measured hot-account 50 TPS result remains paused at 26 retryable serialization conflicts; Phase 1 must remediate or enforce a lower approved limit.
- Managed Cognito, renewable workload identity, private AWS infrastructure, provider PITR, secret rotation, alert routing, and the operational tabletop are not active.
- Legal/custody/retention approval, a consenting design partner, an operating window, and graduation signatures do not exist.

The authoritative owner/status/next-action list is [the pilot completion gate register](../pilot/completion-gates.md).

## Release decision

T096 is complete for the repository engineering release suite. LedgerSync is a verified local engineering MVP and controlled demonstration candidate. It is **not** approved for shared pilot traffic, regulated-fund handling, or graduation until every blocking gate in the completion register passes.
