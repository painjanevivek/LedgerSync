# Real-stack browser system tests

`transfer-recovery.real-stack.spec.ts` creates and funds two isolated accounts.
It drops a posting command before the API receives it, closes the browser context,
and signs in from a clean context. It recovers the saved original request with
explicit review and races two original retries. It then loses a real committed
response, signs in from another clean context, and opens the known receipt without
an automatic financial command. PostgreSQL proofs require two original transfers,
four balanced postings, and the exact original balances. These cases never target
the normal LedgerSync Compose project.

These tests exercise the running browser, BFF, API, PostgreSQL, and Redis stack. They are separate from the deterministic mocked journeys under `tests/e2e` because they create durable ledger records. The funding journey forwards one real transfer command, then drops its committed response to check recovery. It does not fabricate financial responses. Other real-stack cases use no route interception.

The serial acceptance journey refuses to start unless all of these variables are set:

- `LEDGERSYNC_SYSTEM_ALLOW_LEDGER_MUTATION=true`
- `LEDGERSYNC_SYSTEM_ISOLATED_PROJECT=true`
- `LEDGERSYNC_SYSTEM_WEB_URL` uses HTTP, a strict loopback host (`127.0.0.1`, `localhost`, or `[::1]`), and port `3000` or `3102`, with no credentials, path, query, or fragment
- `LEDGERSYNC_SYSTEM_COMPOSE_PROJECT` is the isolated Compose project that owns the selected port; normal `compose` and `ledgersync` projects are rejected
- `LEDGERSYNC_SYSTEM_SEEDED_SOURCE_ACCOUNT_ID` identifies an active INR account with enough funds for the lifecycle case only. The funding case creates and funds its first account through the product.
- `LEDGERSYNC_SYSTEM_RUN_ID` is a unique lowercase identifier matching `[a-z0-9-]{3,32}`

Run it from `web` with `npm run test:e2e:real-stack`. Before opening a page, the harness verifies through Docker labels that the named isolated project's `web` service uniquely owns the selected port and resolves that same project's running PostgreSQL container. Each case enters through the real local login. The lifecycle journey creates one account, transfers a bounded amount into it and back out through normal transfers, and closes the account. It does not delete or rewrite existing data. Its follow-on read-only test uses those exact persisted identifiers to prove strict server-backed history filters, the seven-stage explainability chain, event list/detail, Local Status, Developer metadata/OpenAPI, Recovery evidence, and bounded transfer/account/reconciliation CSV responses.

The journey first opens Close while the new account is non-zero so the dialog performs its required authoritative refresh. It proves both refreshed values are INR 1.00 and the destructive confirmation remains disabled, then sends and replays the same rejected close command through the real BFF. INR 1.00 is the local tenant policy's minimum supported transfer, so the test exercises the normal policy boundary instead of bypassing it. After returning to zero and closing, real BFF reads prove the closed account, exact zero projection, audit context, both histories, and two balanced transfer details. A final read-only `docker exec ... psql` query proves one account/owner, completed and failed idempotency outcomes, audit/outbox rows, two transfers, and four exactly balanced postings in the same isolated PostgreSQL authority.

Account creation, each transfer, and reconciliation are replayed with their captured exact idempotency key and body. Replay also retains the request-reference header where required. The account-create key is reused with changed content and must return `idempotency_conflict`. CSV assertions require the safe attachment family, UTF-8 CSV, schema `2`, `no-store`, fully quoted rows, and the expected exact identifiers/minor-unit strings. Investigation ZIP bundles have a separate schema `1`.

`funding-transfer-journey.real-stack.spec.ts` needs no seeded source account. It proves zero-balance creation, funding request, explicit local review policy, separate journal posting, replay without another journal, INR 1.00 transfer with response loss, exact retry, search/detail/return navigation, and read-only PostgreSQL counts.

`investigation-handoff.real-stack.spec.ts` requires the isolated `3102`/`3103` frontend pair and a matched reconciliation run from the lifecycle case. It verifies both Docker owners, uses separate browser contexts, saves structured filters, preserves a workspace, downloads its audited ZIP, hands ownership to `local-peer` and back, checks that the former owner receives `404`, and closes/reopens the workspace. Reconciliation is a tenant-authorized root; account ownership is not granted to the peer by this test.

## Accessibility and responsive coverage

Neither mocked nor real-stack automated acceptance requires physical devices. Playwright sets CSS viewports and reduced motion, while axe inspects rendered real-stack workflow states. A 320 CSS-pixel viewport covers narrow reflow, while a 640 CSS-pixel viewport represents 200% zoom for a 1280 CSS-pixel layout. Those checks are deterministic browser evidence only; they are not physical-device, real browser-zoom, NVDA, VoiceOver, or production accessibility certification.

This harness is local-only. It accepts only the bounded loopback web URLs, verifies the exact isolated Compose owner, and refuses normal Compose projects. It is not a deployment, remote-environment, or shared-test runner. New financial test records remain in the isolated volume. Stop that exact project after review; never remove normal workspace volumes.
