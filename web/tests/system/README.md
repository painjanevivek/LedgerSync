# Real-stack browser system tests

These tests exercise the running browser, BFF, API, PostgreSQL, and Redis stack without Playwright route interception. They are separate from the deterministic mocked journeys under `tests/e2e` because they create durable ledger records.

The lifecycle journey refuses to start unless all of these variables are set:

- `LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION=true`
- `LEDGERSYNC_SYSTEM_ISOLATED_PROJECT=true`
- `LEDGERSYNC_SYSTEM_WEB_URL` is exactly `http://127.0.0.1:3000`, `http://localhost:3000`, or `http://[::1]:3000` with no credentials, path, query, or fragment
- `LEDGERSYNC_SYSTEM_COMPOSE_PROJECT` is the isolated Compose project that owns port 3000; the normal `compose` project is rejected
- `LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID` identifies an active INR account with enough funds for the bounded transfer
- `LEDGERSYNC_SYSTEM_RUN_ID` is a unique lowercase identifier matching `[a-z0-9-]{3,32}`

Run it from `web` with `npm run test:e2e:real-stack`. Before opening a page, the harness verifies through Docker labels that the named isolated project's `web` service uniquely owns port 3000 and resolves that same project's running PostgreSQL container. The test creates one account, transfers a bounded amount into it and back out through normal transfers, and closes the account. It does not delete or rewrite existing data.

The journey first opens Close while the new account is non-zero so the dialog performs its required authoritative refresh. It proves both refreshed values are INR 1.00 and the destructive confirmation remains disabled, then sends and replays the same rejected close command through the real BFF. INR 1.00 is the local tenant policy's minimum supported transfer, so the test exercises the normal policy boundary instead of bypassing it. After returning to zero and closing, real BFF reads prove the closed account, exact zero projection, audit context, both histories, and two balanced transfer details. A final read-only `docker exec ... psql` query proves one account/owner, completed and failed idempotency outcomes, audit/outbox rows, two transfers, and four exactly balanced postings in the same isolated PostgreSQL authority.

## Accessibility and responsive coverage

Mocked acceptance journeys do not require physical devices. Playwright can set CSS viewports, touch/mobile context, device scale factor, reduced motion, forced colors, and color scheme; axe can then inspect each rendered workflow state. A 320 CSS-pixel viewport covers narrow reflow, while a 640 CSS-pixel viewport represents 200% zoom for a 1280 CSS-pixel layout. Those checks are deterministic CI evidence, but they do not replace a supplementary manual pass with real browser zoom and platform assistive technology such as NVDA or VoiceOver.
