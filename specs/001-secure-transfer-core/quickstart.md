# Local Validation Quickstart

This quickstart validates the one-workstation LedgerSync product at `http://127.0.0.1:3000`. It is not a LAN, cloud, shared-host, production, or design-partner deployment procedure.

## Operator journey

1. Start Docker Desktop, then run `.\scripts\start-local.ps1` from the repository root. Require migrations, demo seed, PostgreSQL, Redis, API, worker, and web health.
2. Open `http://127.0.0.1:3000` directly. Confirm the server-controlled demo operator, isolated-demo environment, tenant context, INR/internal-only boundary, PostgreSQL authority, persisted-data statement, and safe-stop guidance.
3. Open **Accounts**. Confirm only authorized accounts appear and that balance, configuration `account_version`, financial balance `version`, audit context, and immutable history remain distinct.
4. Choose **Create account**. Complete Identity → Financial boundary → Review → Result. Confirm the fixed INR currency and exact zero opening balance; no opening-amount or direct-balance control may exist.
5. Retry the exact account command with its retained idempotency key and confirm one account/result. Change the request under that key and require `idempotency_conflict`.
6. Choose **Fund account** from the result. Confirm the normal transfer form opens with the destination selected without replacing any unresolved prior transfer intent.
7. Submit an exact internal INR transfer. If the response is uncertain, retry the identical body and key; require one transfer, one journal, two balanced postings, one movement per balance, and one saved outcome.
8. On Account Detail, freeze with a bounded reason, confirm transfer writes reject while frozen, reactivate, return funds to the source, and close only after authoritative available and ledger balances both equal exact zero. Confirm Closed is terminal and history remains visible.
9. Apply transfer `q` and financial-status filters. Confirm the server filters before pagination and binds cursors to the canonical filter. Apply Events filters to investigate delivery without inferring a financial result.
10. Run **Reconciliation**. Require a completed matched result with zero mismatches, then retain the immutable run and correlation evidence.
11. Inspect a Transfer Detail stored-evidence chain and require all seven semantic stages—request, transfer, journal/postings, balance versions, outbox, delivery, and reconciliation—with missing/unavailable/truncated states shown explicitly.
12. Inspect **Local status**, **Events**, **Developer**, and **Recovery**. Keep PostgreSQL financial truth separate from outbox delivery and Redis cache state; download only the full versioned OpenAPI contract; confirm restore/reset controls are absent from the browser.
13. Review and download transfer, account-history, and reconciliation CSV exports. Verify exact scope, active filters, 10,000-row ceiling, schema version, identifier disclosure, safe attachment filename, and exact quoted minor-unit strings. Treat CSV as evidence, never as a backup.
14. Create a protected host backup and run an isolated restore drill only through the reviewed host commands. Confirm the Recovery Center shows only the bounded evidence index and never a host path, digest, dump, credential, or browser restore action.
15. Stop with `.\scripts\stop-local.ps1`. Normal stop must preserve PostgreSQL and Redis volumes. Reset is a separate destructive operation and must never be used as routine cleanup.

## Automated acceptance

The mocked browser suites cover deterministic offline, unknown-outcome, permission, partial-evidence, forced-color, reduced-motion, and visual states. The real-stack suite under `web/tests/system` requires an explicitly isolated Compose project and proves browser → BFF → API → PostgreSQL/Redis behavior without request interception. Run it only through the acceptance harness or with every boundary variable documented in `web/tests/system/README.md`.

Automated viewport evidence covers 320, 390, 640, 768, 1024, 1366, 1440, and 1920 CSS-pixel layouts as applicable. A 640 CSS-pixel viewport is the deterministic 200%-zoom equivalent for a 1280 CSS-pixel layout, and 320 CSS pixels covers narrow reflow. Axe, keyboard, reduced-motion, forced-color, and touch-target checks are automated browser evidence.

No physical phone, tablet, browser/OS matrix, real browser zoom, NVDA, or VoiceOver pass is required or claimed for this local-only acceptance. Such manual checks may be supplementary, but they are not silently represented by emulation and do not authorize external deployment.

Required completion evidence is produced by the dedicated clean-room acceptance, recovery, capacity, security, and cleanup gates on one exact source revision. Passing individual commands or this checklist alone is not consolidated release evidence.
