# Real-stack browser system tests

These tests exercise the running browser, BFF, API, PostgreSQL, and Redis stack without Playwright route interception. They are separate from the deterministic mocked journeys under `tests/e2e` because they create durable ledger records.

The serial acceptance journey refuses to start unless all of these variables are set:

- `LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION=true`
- `LEDGERSYNC_SYSTEM_ISOLATED_PROJECT=true`
- `LEDGERSYNC_SYSTEM_WEB_URL` is exactly `http://127.0.0.1:3000`, `http://localhost:3000`, or `http://[::1]:3000` with no credentials, path, query, or fragment
- `LEDGERSYNC_SYSTEM_COMPOSE_PROJECT` is the isolated Compose project that owns port 3000; the normal `compose` project is rejected
- `LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID` identifies an active INR account with enough funds for the bounded transfer
- `LEDGERSYNC_SYSTEM_RUN_ID` is a unique lowercase identifier matching `[a-z0-9-]{3,32}`

Run it from `web` with `npm run test:e2e:real-stack`. Before opening a page, the harness verifies through Docker labels that the named isolated project's `web` service uniquely owns port 3000 and resolves that same project's running PostgreSQL container. The journey creates one account, transfers a bounded amount into it and back out through normal transfers, and closes the account. It does not delete or rewrite existing data. Its follow-on read-only test uses those exact persisted identifiers to prove strict server-backed history filters, the seven-stage explainability chain, event list/detail, Local Status, Developer metadata/OpenAPI, Recovery evidence, and bounded transfer/account/reconciliation CSV responses.

The journey first opens Close while the new account is non-zero so the dialog performs its required authoritative refresh. It proves both refreshed values are INR 1.00 and the destructive confirmation remains disabled, then sends and replays the same rejected close command through the real BFF. INR 1.00 is the local tenant policy's minimum supported transfer, so the test exercises the normal policy boundary instead of bypassing it. After returning to zero and closing, real BFF reads prove the closed account, exact zero projection, audit context, both histories, and two balanced transfer details. A final read-only `docker exec ... psql` query proves one account/owner, completed and failed idempotency outcomes, audit/outbox rows, two transfers, and four exactly balanced postings in the same isolated PostgreSQL authority.

Account creation, each transfer, and reconciliation are replayed with their captured exact idempotency key and body. The account-create key is also reused with changed content and must return `idempotency_conflict`. Export assertions require the safe attachment family, UTF-8 CSV, schema `1`, `no-store`, fully quoted rows, and the expected exact identifiers/minor-unit strings; generated CSV is not retained by the test.

## Accessibility and responsive coverage

Neither mocked nor real-stack automated acceptance requires physical devices. Playwright sets CSS viewports and reduced motion, while axe inspects rendered real-stack workflow states. A 320 CSS-pixel viewport covers narrow reflow, while a 640 CSS-pixel viewport represents 200% zoom for a 1280 CSS-pixel layout. Those checks are deterministic browser evidence only; they are not physical-device, real browser-zoom, NVDA, VoiceOver, or production accessibility certification.

This harness is local-only. It accepts only the fixed loopback web URL, verifies the exact isolated Compose owner for port 3000, and refuses the normal Compose project. It is not a deployment, remote-environment, or shared-test runner.
