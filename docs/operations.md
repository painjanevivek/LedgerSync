# Operator and pilot operations guide

## Local operator console

The supported browser entry is exactly `http://127.0.0.1:3000` on the one
Windows workstation running Docker Desktop. The server-controlled demo
operator, INR data, internal same-currency transfers, and loopback binding are
part of the boundary. This guide does not authorize LAN, cloud, shared-host, or
production deployment.

The complete local operator path, dependency restarts, protected restore, exact
cleanup, and 25 TPS profile passed [LPC-100](pilot/local-product-completion-gates.md)
on the executable candidate documented in the
[Phase 10 acceptance evidence](release-evidence/local-product-phase-10-acceptance.md).
That result is local workstation evidence, not a production SLO.

Open the dashboard directly. The local guide explains the operator identity,
fixed INR boundary, PostgreSQL authority, persisted Compose volumes, safe stop,
and host-only reset/backup actions. Dismissing the panel affects presentation
only; its checklist is rebuilt from durable API evidence and can be reopened.

### Accounts and lifecycle

- **Accounts** lists only the authorized scope. A balance that cannot meet its
  read-your-writes requirement is unavailable or explicitly historical; it is
  never silently replaced with zero.
- **Create account** records display name, external reference, and category.
  Currency is fixed to INR and the opening balance is exact zero. The browser
  has no balance mutation or opening-amount control.
- **Fund account** routes through the normal transfer form. It does not mint
  money and does not overwrite an unresolved retained transfer intent.
- Freeze, reactivate, and close require `accounts:write`, a current
  `account_version`, and a bounded audited reason. Close additionally refreshes
  the authoritative available and ledger balances, requires both to be exact
  zero, and requires typed external-reference confirmation. Closed is terminal;
  its immutable history remains readable.

### Transfers, retries, and evidence

Transfers are internal, same-currency, exact-decimal commands between active
authorized accounts. Review the exact source, destination, INR amount, and
retained retry identity before posting. When a result is not confirmed, use
**Retry same transfer**. The UI retains and resends the identical body and
idempotency key; starting a different transfer does not resolve an unknown one.

Transfer filters run on the server before pagination. A cursor belongs to the
canonical filter that produced it and must not be reused after a filter change.
Transfer Detail separates the permanent PostgreSQL financial result and debit /
credit postings from outbox/delivery state. Its seven-stage stored-evidence
chain displays missing, unavailable, truncated, and out-of-order evidence
without inventing continuity.

### Reconciliation and local investigation

- **Reconciliation** is the financial control. Only a completed matched run
  with zero mismatches is presented as passed; previous runs remain visible.
- **Local status** separates PostgreSQL financial authority, transactional
  outbox delivery, and disposable Redis cache health. A Redis or delivery
  problem does not imply that money changed.
- **Events** is a tenant-scoped, GET-only delivery investigation surface.
  Filters, attempts, timeline, and related authorized links do not replay or
  mutate an event.
- **Developer** serves versioned non-secret examples and the complete OpenAPI
  YAML. It has no arbitrary HTTP runner and never reveals the private BFF
  credential.
- **Recovery** displays current database evidence and the bounded protected
  backup/restore index. Safe commands are copy-only. Restore, reset, path
  selection, Docker, and shell execution are intentionally absent.

### Exports and safe stop

Transfer, account-history, and reconciliation CSV exports require explicit
read/export scopes and a review of the exact scope, filters, 10,000-row ceiling,
schema, and identifiers. They are bounded operational evidence, not backups.
Do not commit downloaded CSV or Playwright artifacts; they can contain local
tenant identifiers and exact demo amounts.

Use `.\scripts\stop-local.ps1` for ordinary stop; named PostgreSQL and Redis
volumes are preserved. Use `.\scripts\status-local.ps1` for bounded host status.
Backup, restore drill, retry lab, and destructive reset remain deliberate host
operations documented under `docs/runbooks/` and cannot be initiated by the
browser.

Automated acceptance covers representative 320–1920 CSS-pixel layouts,
keyboard operation, reflow, reduced motion, forced colors, touch targets, and
automated WCAG rules. This is browser emulation, not a physical-device,
real-zoom, NVDA, VoiceOver, or production accessibility certification. No such
physical-device result is claimed for the local-only product.

## Pilot release ownership

Before a shared pilot, the accountable operator must record:

1. Pilot jurisdiction and single supported currency.
2. Named OIDC provider configuration using authorization code with PKCE.
3. Managed PostgreSQL backup age and isolated restore drill evidence.
4. Reconciliation result (`0` mismatches) and RYEW violation count (`0`).
5. On-call ownership for database, Redis/outbox, identity provider, and design
   partner support.

Use the runbooks in `docs/runbooks/` for incidents. Stop pilot expansion for a
reconciliation mismatch, duplicate movement, authorization disclosure, or a
balance that was shown current without meeting its required version.
