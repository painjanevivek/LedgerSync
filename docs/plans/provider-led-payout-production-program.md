# Provider-led payout production program

**Status:** `ACTIVE — engineering foundation only`  
**Recorded:** 2026-08-29  
**Authority:** [Master delivery register](ledgersync-master-progress.md) and the approved production-completion plan.  
**Applies to:** the first provider-led outbound-payout program. It does not replace the existing closed-loop ledger until the gates below pass.

## Product boundary

LedgerSync is an India-first B2B ledger and payout-orchestration product. It
records the customer's instruction, reserves the customer's available INR
balance, enforces two-person approval, and records a provider-confirmed result.
A separately contracted, licensed provider performs the external movement of
money.

LedgerSync is not the payment provider and does not directly hold bank
credentials, customer funds, or a settlement float. It must never describe a
provider attempt as settled until the provider's authenticated evidence is
recorded and reconciliation has completed.

### Locked v1 rules

| Topic | Rule |
|---|---|
| Currency | INR only, represented as exact integer paise at rest and decimal strings over JSON. |
| Movement | Outbound payouts only. Inbound collections, cards, FX, cross-border settlement, wallets, and direct custody require separate programs. |
| Approval | Requester and approver must be different operators. Approval and provider dispatch require recent step-up authentication. |
| Reservation | A reservation lowers spendable balance but posts no journal. |
| Settlement | Only an authenticated provider-confirmed settlement creates the immutable source-to-external-payout-clearing journal. |
| Fees | Provider fee is an exact, reviewed paise value. It is charged to the source account by default and posted separately to the approved fee account. |
| Failure | A failure, cancellation, or expiry releases the reservation through an immutable event; no posted record is edited or deleted. |
| Integration | Provider calls are durable asynchronous jobs. No external HTTP call may occur while a PostgreSQL financial transaction is open. |

## Stop-ship conditions

- No contracted licensed provider, sandbox access, or legal confirmation of the
  provider-led model.
- Any duplicate provider command that could move money, unbalanced journal,
  negative available balance, tenant boundary failure, unresolved settlement
  mismatch, or stale/unauthorized approval.
- Fees, payout limits, clearing account, retention schedule, provider
  liabilities, support contacts, or incident owner have not been approved.
- A critical or high exploitable security finding, failed recovery drill, or
  unowned operational alert.

## Required external owner record

Names, approval references, and expiry dates belong in the approved external
evidence store. This repository intentionally records roles only until those
authorities provide durable evidence.

| Role | Required evidence before live payouts |
|---|---|
| Product owner | Approved scope, design partner, rollout decision, and payout limits. |
| Finance owner | Clearing/fee account mapping, fee treatment, settlement and reversal policy. |
| Security owner | Provider security review, threat model, key custody, penetration-test disposition. |
| Legal/compliance owner | India jurisdiction, provider responsibility, customer terms, retention, privacy, and regulatory perimeter conclusion. |
| Operations / incident commander | On-call rota, alert routing, provider escalation, recovery and communication drills. |
| AWS billing owner | AWS account, budget ceiling, DNS/domain ownership, and time-bounded deployment authority. |
| Provider owner | Contract, sandbox and production certification, API/SLA, webhook signing, reconciliation feed, and incident contact. |
| First design partner | Written pilot consent, named support contacts, agreed limits, and test window. |

## Engineering delivery sequence

| Phase | Repository work | External gate |
|---|---|---|
| 1 | Shared replay store, server-initiated webhook verification, recipes, bounded provisioning, managed-key adapter. | Provider-independent, but production secrets require AWS access. |
| 2 | Payout domain, migrations, fake provider adapter, contract/fault/reconciliation tests. | Provider contract before live adapter or live money. |
| 3 | Payout and approval console, accessibility, physical-device and finance terminology evidence. | T094/T095 human evidence. |
| 4 | Separate public site and qualified pilot intake. | Legal/content approval. |
| 5 | Cognito tenancy and reviewed AWS Terraform. | AWS account, budget, DNS, and approved credentials. |
| 6 | Managed telemetry, security, privacy, incident documentation and tests. | Independent penetration test and owners. |
| 7 | Managed capacity, RDS recovery, reconciliation, data-lifecycle evidence. | Managed environment and provider recovery data. |
| 8 | One-design-partner controlled pilot. | Contracted partner and provider certification. |
| 9 | Staged release and recurring operations. | Formal go/no-go approvals. |

## Evidence rule

Engineering can implement and test the provider-neutral boundary with a fake
sandbox. It cannot mark a provider, legal, AWS, device, finance, partner, or
live-money gate complete without the named authority's evidence. Every phase
commit must update the master register and attach exact-commit test evidence.

