# Simple-first redesign completion matrix

This matrix records implementation and verification evidence for the LedgerSync simple-first redesign. It is intentionally separate from deployment readiness: no cloud resource, secret, deployment, commit, or push is part of this work.

## Qualification correction — 5 September 2026

The original completion labels below overstated end-to-end acceptance. The [real-stack smoke report](live-smoke-2026-09-05/report.md) is the current verification record. Four live defects were fixed, but the complete redesign is **not yet accepted**. Passing fixture tests or a Go run with harness-dependent skips does not prove live financial workflows, and the zero-vulnerability result applies only to production npm dependencies. The full npm audit has 17 findings; generated developer artifacts also fail their drift gate.

| Requirement | Implementation evidence | Verification evidence | Status |
|---|---|---|---|
| Simple view is the safe default | `ExperienceModeBoundary.tsx`, tenant/operator-scoped preferences, `/api/preferences` | `simple-first-experience.test.ts`, onboarding E2E | Complete |
| Expert view reuses the same application | Shared shell, routes, hooks, domain models, and presentation props | capability navigation and visual regression suites | Complete |
| Outcome → explanation → evidence hierarchy | `TrustStrip`, `TaskCard`, `TechnicalDetails`, presentation adapters | Home/Tasks reviewed baselines and accessibility suite | Implemented in sampled views; all-workflow acceptance pending |
| Urgent financial states remain visible | Presentation adapters mark unknown/mismatch states as attention; disclosure forces urgent content open | transfer unknown-outcome and reconciliation mismatch E2E/baselines | Complete |
| Simple Home and unified Tasks | `OverviewView.tsx`, `/tasks`, `TasksView.tsx` | Real Home/Tasks smoke; loading, error, offline, and pagination truth regressions | Partial: full attention-source aggregation not demonstrated |
| Plain-language vocabulary and exact numbers | Central presentation mappings, `Money`, `RelativeTime`, `RecordIdentity` | Exact-money/UI tests plus live Simple-view inspection | Partial: technical wording remains in account creation, funding, and Help |
| Shared action availability | `ActionAvailability.tsx` with visible reason, recovery slot, and `aria-describedby` | UI primitive, accessibility, and capability tests | Implemented; exhaustive per-action inventory acceptance remains unproven |
| 12px metadata, 14px supporting copy, 16px routine controls/body | semantic type tokens and raised legacy styles | typography contract and computed-style E2E | Complete |
| Responsive from 320px with no page overflow | feature-owned constraints, compact shell fixes, container queries | 28 real Home/Tasks/Accounts/Help checks across 320/360/390/768/1024/1280/1440; fixture route matrix | Passed tested coverage; not every real data state/browser |
| Opaque, rotating, revocable sessions | migration 000037, PostgreSQL repository, private BFF session routes, opaque cookie | Real PostgreSQL API-role create/resolve/rotate/revoke/consistency test under Linux -race; browser login/logout | Passed targeted integration; full lifecycle/retention qualification pending |
| Exact cookie lifecycle | shared runtime prefix/transport policy and cookie builders | Optimized local browser sign-in/out/re-login; OIDC/session policy tests | Local lifecycle verified; production OIDC not exercised live |
| Shared production BFF rate limiting | atomic Upstash store; local-only in-memory fallback; production fail-closed policy | rate-limit security tests and Vercel config validator | Complete; no cloud resource provisioned |
| Redacted guardrail observability | `guardrail-metrics.ts` plus session, rate-limit, step-up, and unavailable-action instrumentation | `guardrail-metrics.test.ts`, lint, and production build | Complete; emits no identifiers, amounts, tokens, or network traffic by default |
| Tenant/operator-scoped disclosure | scoped preference key and guarded storage access | progressive-disclosure and simple-first tests | Complete |
| Modular hotspots and CSS ownership | shell/navigation, transfer list/detail, correction detail splits; explicit cascade layers | lint, build, style-architecture tests | Partial decomposition; full plan coverage not established |
| Accessibility and reviewed visual states | forced-colors/reduced-motion rules and refreshed baselines | Automated accessibility checks and representative manual screenshots | Automated/sampled coverage passed; complete manual acceptance pending |
| Automated build and test gates | frontend lint/unit/build/Playwright; Go packages; config validator; dependency audit | 198 frontend unit tests; see live-smoke report for fresh suite results | Not clear: generated artifact drift and 17 development npm findings; full live backend gates outstanding |
| Go race detector | No product-code exception | Targeted PostgreSQL session/preferences integration passed with Linux -race; Windows has CGO disabled | Targeted pass; full repository race suite outstanding |
| Five moderated operator sessions | protocol in `simple-first-usability-protocol.md` | Requires representative human operators | External acceptance gate |

## Manual baseline review

Representative screenshots were inspected after regeneration: Simple Home desktop/mobile, Tasks desktop, Accounts mobile, unknown transfer mobile, mismatch mobile, Developer mobile, and the safe render-error state. The review confirmed readable hierarchy, visible risk states, no clipped page content, calm semantic color use, and preservation of technical evidence in Expert view.

## Release gate

The redesign is **not complete or approved for broad production release**. Resolve the remaining code, artifact, dependency, and live integration gates in the smoke report, then complete five moderated operator sessions against the feature acceptance thresholds. Deployment infrastructure remains intentionally unprovisioned; no deployment is requested.
