## Outcome

Describe the user or operational outcome. Keep one cohesive change per pull request.

## Scope

- Included:
- Explicitly excluded:
- Findings or work-package IDs:

## Risk

- [ ] Financial/security behavior changed
- [ ] Database schema or grants changed
- [ ] Public API, event, or stored idempotency contract changed
- [ ] Worker ownership, retry, lease, or delivery behavior changed
- [ ] No item above applies

Explain the highest credible failure or abuse scenario and its blast radius:

## Test evidence

- Tests written before implementation:
- Commands executed:
- Fault/adversarial scenario exercised:
- Checks not executed and why:

## Compatibility and migration

- Old application/new schema behavior:
- New application/old schema behavior:
- Client/event compatibility:
- Backfill or validation evidence:

## Observability and security

- New or changed signals/alerts:
- Sensitive-data and tenant-isolation review:
- Database-role or authorization evidence:

## Rollout and rollback

- Rollout/canary sequence:
- Stop conditions:
- Rollback or forward-repair sequence:
- Reconciliation/recovery evidence required:

## Reviewer independence

- [ ] The author is not the sole approving reviewer
- [ ] A CODEOWNER reviewed every affected critical boundary
- [ ] Database/ledger/security/release expertise is included where applicable

## Completion

- [ ] Documentation and contracts match behavior
- [ ] Generated artifacts are current
- [ ] No unrelated user-owned or generated files are included
- [ ] The branch is rebased/merged against the current protected base
