# LedgerSync pilot graduation scorecard

Status: template; the current pilot is not eligible for graduation because no
managed design-partner traffic has started.

## Decision options

- **Graduate:** all stop-ship gates pass, evidence is complete, and approved
  partner outcomes justify controlled expansion.
- **Extend:** safety gates pass but the observation sample is insufficient;
  retain current scope/limits and set a dated evidence target.
- **Remediate:** a correctable gate fails; pause affected movement, name owner
  and due date, then repeat the relevant proof.
- **Stop:** trust, legal, product-value, or operating evidence does not support
  continuing; preserve evidence and execute the approved offboarding path.

Schedule pressure, sunk effort, or an unsigned “conditional pass” is not a fifth
decision option.

## Mandatory scorecard

| Dimension | Required evidence | Graduation threshold | Actual | Status |
|---|---|---|---|---|
| Partner sequence | One stable limited partner before partner two/three | Approved observation window complete | No partner | BLOCKED |
| Account scale | Authorized active accounts and query-plan evidence | Approved target, including 10,000-account readiness | Local synthetic only | BLOCKED |
| Meaningful traffic | Posted/rejected/retried distribution by approved use case | Signed minimum sample/window | None | BLOCKED |
| Exactness/idempotency | Duplicate movement and changed-intent conflicts | 0 duplicate movements; all conflicts explained | Repository tests pass | PARTIAL |
| Reconciliation | Persisted run IDs and mismatch investigations | 0 unexplained mismatches throughout window | Local evidence only | PARTIAL |
| RYEW/balances | RYEW violations, primary fallback, stale-current incidents | 0 RYEW/stale-current violations | Local evidence only | PARTIAL |
| Transfer latency | p50/p95/p99 and error rate at representative approved traffic | Approved SLO and headroom | Local 25 TPS envelope and 50 TPS 2× headroom pass; managed rerun pending | PARTIAL |
| Balance latency | p50/p95/p99 by cache/fallback path | Approved SLO | Local 25 TPS balance p95 159.32 ms; managed rerun pending | PARTIAL |
| Recovery | Provider PITR, achieved point, RPO/RTO, cache rebuild, reconciliation | Approved objectives met in isolation | Local logical restore only | BLOCKED |
| Security/identity | Managed OIDC/secrets/network evidence and findings | 0 open critical/high; cross-tenant tests pass | No managed environment | BLOCKED |
| Accessibility/devices | Physical matrix, defects/retests, WCAG evidence | No unresolved critical issue | Automation only | BLOCKED |
| Operations/support | Alerts, incidents, support cases, investigation time | All critical alerts owned; accepted support outcome | Tabletop not run | BLOCKED |
| Product value | Integration effort, safe first-transfer time, investigation value | Partner/product threshold approved and met | No partner evidence | BLOCKED |
| Legal/finance | Jurisdiction, currency, custody, semantics, retention approvals | Every blocking decision signed | Pending | BLOCKED |

Any BLOCKED safety/legal row prohibits graduation. A PARTIAL row may not be
upgraded from repository evidence when the threshold requires managed or partner
evidence.

## Known limitations and risk acceptance

For every limitation record: observable impact, affected partner/use case,
probability/severity, compensating control, owner, due date, evidence, accepting
authority, expiry/review date, and whether it blocks movement or only expansion.
Unowned or non-expiring critical risk cannot be accepted.

## Outcome and roadmap evidence

Before prioritizing v2, list each need with partner/source, frequency, business
impact, safety/compliance implication, alternative, estimated effort, success
metric, and decision. Repository elegance or competitor parity is not validated
customer demand.

The following always require a separate PRD, threat/risk model, legal/compliance
review, data/ledger model, operational program, and explicit approval:

- bank/payment rails, cards, external payouts, custody, settlement;
- foreign exchange or multi-currency movement;
- holds, chargebacks, scheduled/recurring transfers;
- public self-service onboarding or consumer/native applications;
- AI-generated financial decisions, postings, reconciliation conclusions, or
  recovery actions.

## Sign-off

| Authority | Name | Decision | Evidence reference | UTC date |
|---|---|---|---|---|
| Product | — | Not eligible | This scorecard | 2026-08-24 |
| Engineering | — | Not eligible | Capacity passes locally; managed, recovery, approval, device, operations, and partner gates remain | 2026-08-24 |
| Finance | — | Pending | — | — |
| Security | — | Pending | — | — |
| Operations | — | Pending | — | — |
| Legal/compliance | — | Pending | — | — |
| Partner owner | — | Pending | — | — |

TASK-022 is complete only when named authorities replace the template rows and
the decision is based on completed partner evidence.
