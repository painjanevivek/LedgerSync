# Phase 7 — Managed environment gate status

Evidence date: 2026-08-24

## Repository controls ready

- Production API and BFF configuration already reject demo identity, insecure
  OIDC URLs, weak/missing signing material, and static private API tokens.
- The managed pilot preflight schema and CLI now fail closed on missing identity,
  secrets, network, backup, observability, security, and provider-restore proof.
- Unknown manifest fields and trailing JSON are rejected; output contains only
  status, revision, and whether restore proof was required.
- Unit tests prove deployment preflight cannot be misrepresented as the final
  provider-backed restore gate.

## External evidence still required

TASK-016, TASK-017, and TASK-018 remain open. No managed IdP, secret manager,
isolated cloud network, PostgreSQL continuous archive, alert destination, or
provider restore was supplied in this workspace. The example manifest is
intentionally invalid and contains no secrets. It must never be cited as proof
that those controls are active.

Phase 8 partner onboarding remains blocked until Phase 6 approvals exist and
`pilot-preflight --require-restore` passes against reviewed external evidence.
