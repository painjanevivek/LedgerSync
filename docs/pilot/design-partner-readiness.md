# Design-partner pilot readiness

**Status:** technical readiness documented; customer and legal gates remain open.  
**Owner:** product owner assigns the named accountable people before any shared pilot.  
**Scope:** same-currency, closed-loop transfers between LedgerSync accounts only.

This document is the Phase 8 operating checklist. It deliberately separates
facts proven by the repository from business decisions that a deployment cannot
make for itself. A successful local build is not permission to move real money.

## What LedgerSync can safely pilot

- An authorized user can view only accounts they are allowed to access.
- An authorized user can submit an exact same-currency internal transfer.
- Retrying the same idempotency key does not create a second movement.
- The PostgreSQL ledger is the financial record; cache and event systems are
  never treated as the source of truth.
- An account balance is returned only when it meets the completed transfer's
  required version, or the product reports temporary unavailability.

The pilot does **not** include bank transfers, custody, foreign exchange,
card payments, KYC/AML operations, chargebacks, scheduled transfers, or
cross-border settlement. Those require separately approved product, legal,
security, and engineering programs.

## Required decisions before inviting a customer

The business owner must record all values below in the pilot ticket or approved
change record. Do not substitute placeholders with assumptions.

| Decision | Accountable owner | Required record |
|---|---|---|
| Two or three named design partners | Product lead | Company, use case, integration owner, support contact, pilot start/end date |
| Jurisdiction and legal entity | Legal/compliance owner | Written approval for the proposed closed-loop use case |
| Single initial currency | Finance/product owner | ISO code, decimal precision, limits, and prohibited use cases |
| Data-processing and privacy posture | Security/privacy owner | Data classification, retention, processor agreement, and incident contact |
| Identity provider | Security/platform owner | OIDC authorization-code-with-PKCE configuration, approved redirect URIs, roles/scopes |
| Hosting and recovery ownership | Platform owner | Managed PostgreSQL, secret manager, backup policy, on-call schedule |
| Commercial support model | Customer success owner | Hours, escalation route, response target, and customer communication template |

If any row is incomplete, the environment remains a demonstration or sandbox,
not a shared production pilot.

## Technical go/no-go evidence

Attach dated evidence to the release record. The required local and CI checks
are listed in [secure-transfer-core.md](../release-evidence/secure-transfer-core.md).
For the target pilot environment, add the following evidence:

1. A deployment uses managed secrets; no production secret is present in the
   repository, browser, image layer, logs, or support export.
2. The public edge exposes HTTPS only. The Go API, worker, PostgreSQL, Redis,
   diagnostics, and administrative dependencies are private.
3. A real OIDC login completes with a least-privilege role. A regular user is
   denied another tenant's account and privileged operations.
4. The latest reconciliation reports zero unexplained differences between the
   immutable ledger and balance projections.
5. The RYEW-violation metric remains zero. A cache delay returns either the
   required version or a clear temporary error, never an older balance as
   current.
6. The oldest unpublished outbox event is within the approved operational
   threshold, with no unreviewed dead event.
7. A provider-backed, isolated PostgreSQL restore drill succeeds and is
   reconciled before the environment is opened to a partner.
8. Alert delivery and the database, Redis, outbox, reconciliation, identity,
   secret-compromise, and restore runbooks have named responders.

## Daily control review during the pilot

The pilot owner holds a short daily review with finance, support, and
engineering. Record the time window, source dashboard links, reviewer, and
decision. Review:

- completed, rejected, and retried transfers;
- reconciliation result and any investigation reference;
- RYEW primary-fallback rate and violation count;
- outbox age, worker retries, dead events, and cache recovery;
- authentication/authorization denials and suspicious access patterns;
- API error rate, p95 latency, dependency health, and backup age;
- partner support tickets, integration failures, and their resolved evidence.

Use sanitized identifiers in meeting notes. Do not copy access tokens, raw
consistency requirements, personal data, or unredacted financial exports into
tickets or chat.

## Stop conditions and immediate response

Pause onboarding and new money movement immediately when any condition below
occurs. Existing ledger history is never edited to make an incident disappear;
approved corrections are compensating entries with an audit trail.

| Stop condition | First response | Required evidence before restart |
|---|---|---|
| Unexplained reconciliation mismatch | Freeze affected pilot movement, preserve evidence, run the reconciliation-mismatch runbook | Root cause, reconciled result, finance and engineering approval |
| RYEW violation or stale balance shown as current | Disable affected presentation path, retain request/event evidence, investigate cache and primary fallback | Repeat fault proof and zero violations after the fix |
| Unauthorized account access or disclosure | Revoke affected sessions/credentials, contain access, follow incident process | Security review, affected-user assessment, authorization regression test |
| Missing audit evidence | Pause the affected operation and establish the evidence gap | Reconstructed compliant audit trail and control-owner sign-off |
| Failed or stale restore proof | Do not expand the pilot; perform an isolated restore drill | Successful restore and reconciliation within the approved objective |
| Unbounded outbox backlog | Stop rollout, use the outbox runbook, and protect primary capacity | Backlog drained, consumer recovery proven, alert threshold reviewed |

## Progressive rollout sequence

1. **Sandbox integration:** partner engineers use non-production accounts and
   verify OpenAPI, idempotency, validation, and failure handling.
2. **Internal production-like rehearsal:** LedgerSync staff run the exact
   partner workflow with alerts, backups, reconciliation, and support
   escalation active.
3. **One-partner limited pilot:** enable a small, pre-agreed account set and
   transfer limit. Review controls daily.
4. **Second/third partner:** expand only after the first partner's agreed
   observation period has no unresolved stop condition.
5. **Pilot closeout:** document outcomes, capacity observations, incidents,
   customer feedback, and a decision to proceed, extend, or stop.

Feature flags may control invitation, screen copy, and non-financial rollout
presentation. They may never disable authorization, idempotency, ledger
balancing, audit recording, reconciliation, or authoritative primary fallback.

## Handoff package for each partner

- OpenAPI contract and example requests using decimal strings and an
  `Idempotency-Key` per logical transfer.
- Integration guide explaining retries after a lost response, validation
  errors, and temporary availability responses.
- Named sandbox credentials delivered through the approved secret-sharing
  channel, never email or source control.
- Support hours, escalation route, incident communication contact, and status
  update cadence.
- Written confirmation of the intentionally excluded money-movement features.

Use `partner-evidence-template.md` as the per-partner system of record. Do not
enable a partner based on a shared spreadsheet row or verbal approval alone.

## Phase completion boundary

Engineering may mark the **technical pilot-readiness work** complete when the
artifact and checks above are in place. The **live design-partner pilot phase**
is complete only after the business decisions, legal approval, named customers,
and target-environment evidence are attached by their accountable owners.
