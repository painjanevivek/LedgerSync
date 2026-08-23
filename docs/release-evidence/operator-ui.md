# LedgerSync operator UI release evidence

Evidence date: 2026-08-23 (Asia/Calcutta)

## Implemented release candidate

- Production-blocked server demo identity and deterministic PostgreSQL seed.
- Same-origin BFF with signed session, CSRF protection, actor assertions, bounded reads, safe errors, and no-store financial responses.
- Tenant-authorized account, transfer, posting, reconciliation list/detail evidence.
- Exact-money prepare → review → confirm → success/rejection/unknown transfer journey with same-key retry.
- One responsive shell and evidence component system across compact, tablet, laptop, desktop, and wide layouts.
- Separate financial posting, downstream delivery, and reconciliation states.

## Passing automated evidence

| Suite | Result |
|---|---|
| Go unit/contract/integration/fault packages | Passed (`go test ./...`; workspace-local build cache) |
| Web lint | Passed |
| Web unit/security/semantics | 13/13 passed after Phase 3 additions |
| Next.js production build | Passed; all list/detail and BFF routes compiled |
| Playwright transfer/accessibility/responsive | 15/15 passed in the final full run |
| Static JS budget | 7 chunks; 650,147 bytes total; 229,156-byte largest chunk; limits 2,000,000 / 350,000 |
| Compose syntax | Passed (`docker compose config --quiet`) |

## Known release gates — not represented as passes

- Docker Desktop daemon was unavailable at the final full-stack checkpoint, so the new migration/seed/API/BFF stack must be rerun when the daemon is active.
- Physical iOS, Android, and tablet device evidence is pending; emulation is not substituted for physical review.
- Managed PostgreSQL PITR/restore evidence remains an external environment gate.
- Finance must approve production account-category aggregation for each design partner.
- Jurisdiction, pilot currency, custody/non-custody position, licensed-provider boundary, and partner transfer limits remain owner decisions.
- Comprehensive screenshot baselines for every error/offline/denied/mismatch state remain a release-hardening task; existing reviewed design captures are reference evidence, not a complete visual-regression matrix.
- Account API cursor pagination/back-filter preservation requires a final completion pass before claiming 10,000-account directory readiness.

## Release decision

The repository is suitable as a local engineering/design-partner demonstration candidate after the Docker smoke rerun. It is not approved for external production funds until every gate above has an owner, evidence, and sign-off.
