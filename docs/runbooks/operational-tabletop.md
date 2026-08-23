# LedgerSync operational tabletop

Status: executable review pack; named operations/security/finance participants
and an incident commander must run and sign it before partner traffic.

## Exercise method

For each scenario, the facilitator supplies only the observable symptoms. The
team must identify financial truth, stop/continue movement, safe customer
language, diagnostics, escalation, recovery authority, evidence retention, and
the condition for reopening. Record UTC timestamps and links; do not paste
secrets, raw tokens, or customer data.

| Scenario | Expected first decision | Required evidence before reopen |
|---|---|---|
| Transfer response lost after possible commit | Do not create a new key; query/retry the identical intent with the same key | One posted transfer or a proven rejection; ledger/reconciliation evidence |
| Reconciliation mismatch | Pause affected tenant movement and page incident/finance owners | Root cause, corrected/rebuilt projection where applicable, matched rerun, approval |
| Delivery delayed after posted transfer | Keep financial state posted; isolate downstream delivery state | Durable attempt/replay evidence; no duplicate movement; notification owner decision |
| Redis unavailable/stale | Serve version-checked primary data or mark current evidence unavailable | PostgreSQL health, cache rebuild/version proof, zero RYEW violations |
| PostgreSQL degradation | Reject/mark unknown according to commit boundary; do not infer movement | DB recovery, idempotent replay checks, reconciliation, saturation/lock evidence |
| OIDC/private credential compromise | Revoke/rotate, terminate sessions, deny production demo identity | Rotation audit, rejected old credential tests, scoped impact review |
| Provider restore required | Isolate restore; keep traffic closed | Achieved point/RPO/RTO, migration compatibility, cache rebuild, matched reconciliation, approval |

## Record template

| Field | Entry |
|---|---|
| Scenario and facilitator | Pending |
| Participants and accountable roles | Pending |
| Exercise start/end UTC | Pending |
| Initial severity and movement decision | Pending |
| Alerts/routes that fired | Pending |
| Financial truth and supporting IDs | Pending |
| Customer/partner wording | Pending |
| Escalations and decision authority | Pending |
| Evidence-retention location | Pending |
| Gaps, owners, due dates | Pending |
| Retest result and sign-off | Pending |

## Pass criteria

- The team never equates delivery/cache failure with reversed or lost money.
- Unknown outcomes always retain the original idempotency key.
- Any mismatch, authorization breach, or unsafe recovery pauses affected movement.
- Alerts reach named routes, every operational action is audited, and reopening
  requires the named incident/finance authority.
- Every discovered gap has an owner and is retested; unresolved critical gaps
  block TASK-020 partner onboarding.
