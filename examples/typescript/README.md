# One trusted backend integration

This example shows how to use the current LedgerSync contract without a browser credential. It creates exact-zero accounts, records synthetic funding, uses a separate finance reviewer, posts funding, sends an internal transfer, reads current balances, and reconciles. It also receives real signed verification and delivery requests in the isolated qualification.

The example never moves external funds. Its automated reviewer is a synthetic test identity. It is not evidence of a human approval, bank settlement, custody, or a production deployment.

## 1. Check the prerequisites

Use the repository checkout, Node **24.12.0**, the Go version in `go.mod`, and local Docker Desktop. The example has no external runtime packages. Its lockfile records that fact. Run `npm ci --ignore-scripts --offline` in this directory, then `npm test`. Node runs erasable TypeScript directly. Do not add TypeScript features that need transpilation.

The receiver and intent store use Node's built-in SQLite API. This API is experimental in the pinned runtime. The tests qualify this exact runtime for the local example. It is not the LedgerSync financial authority; PostgreSQL remains that authority. A shared production receiver needs its own supported storage and operating qualification.

For static type checks, install the repository's locked web development tools, then run `node web/node_modules/typescript/bin/tsc -p examples/typescript/tsconfig.json` from the repository root. These tools are not runtime dependencies of the example.

## 2. Run the complete disposable check

The fixed host runner reuses only the immutable PostgreSQL/Redis dependency image identities from the preserved local stack. That stack must be present and healthy. It does not serve this example's API requests and is not seeded or migrated by the check.

From the repository root, run:

```powershell
node scripts/portfolio/run-developer.mjs
```

The runner creates new temporary databases and a new loopback Redis allocation. It applies the current migrations and grants there. The fixture starts real HTTP handlers with a synthetic local identity provider and the real request authenticator. Each Node process has new private configuration. The API pool has only the restricted API role. Verification and delivery use a separate restricted worker login.

The check executes this sequence:

1. Create two INR accounts through the HTTP API. Both must start at exactly zero.
2. Request funding of `1000` minor units with a synthetic evidence reference. The requester's self-approval must fail.
3. Read the request as a different finance principal. Perform the separately recorded synthetic approval, then post it with a persisted command key.
4. Register a synthetic HTTPS receiver. Let the real verification worker send a signed challenge. The receiver must prove key control before the endpoint becomes active.
5. Persist the complete `2.50` INR transfer intent and its key. Send it once. Discard the actual successful response.
6. Start a different Node process. Load the saved intent, original credential binding and key. The retry must return the original posted result.
7. Send a changed body under that key and require HTTP 409. Send numeric money and require HTTP 400. Use a reader credential for a write and require HTTP 403.
8. Read authoritative balances: `750` and `250` minor units. Run internal reconciliation and require zero mismatches.
9. Repeat only the original safe replay until the real database rate limiter returns HTTP 429 with `Retry-After`. Do not create extra transfers to test throttling.
10. Let the actual delivery worker reach a receiver that returns HTTP 503. Retain its retry evidence. Remove the host-only failure marker and require a later delivery to succeed.
11. Terminate the owned receiver process. Restart it with the same protected SQLite state. Send the same signed event under a new delivery ID. It must acknowledge the duplicate without a second business effect.
12. Confirm exactly two balanced journals and four postings for this new tenant. Verify artifact hashes, source/index preservation, normal container identities, and exact cleanup.

The test-only HTTP transport maps the fixed `https://receiver.example.test/hooks` destination to the owned loopback Node receiver. That adapter exists only in Go `_test.go` code. It does not change production destination validation or transport. The receiver listens only on `127.0.0.1` and has no remote fault-control route.

The resulting private bundle is under `data/portfolio/demo-runs/<run-id>/`. Inspect the manifest, named JSON test output, and `example-*.txt` logs. Run `node scripts/portfolio/verify-demo.mjs data/portfolio/demo-runs/<run-id>` to recheck its integrity. Do not pass `--save-summary`: the failure-demonstration browser summary belongs to its own fixed profile.

This is a clean disposable data/configuration check with separate Node processes. It does not claim a second human reviewer, a fresh physical-machine install, a complete candidate API container, or an external OAuth provider deployment. Elapsed time in the named test covers fixture setup and the executed integration flow, not toolchain installation. Keep failed runs when a guide or fixture needs correction.

## 3. Separate the authentication lanes

Browser code uses the same-origin BFF session and CSRF protection. Never place a workload token, signing key, actor-assertion secret, or this configuration file in browser storage or a public environment variable.

The primary example uses **own-principal backend credentials**. Each approved token acts as its own issuer-provided subject in one configured tenant. Its scopes, database role membership and account authority must permit the requested action. A separate finance identity reviews funding. A browser session token is not a substitute. Obtain credentials through the approved identity/provisioning owner and protected local host tooling. The qualification runner creates synthetic equivalents automatically; no source-controlled token is needed.

A workload with `bff:act-as-user` is a different lane. It must send a fresh short-lived actor assertion on every request. The generated client accepts an assertion factory for a trusted delegating backend. A static assertion is a single-call compatibility option and cannot be reused as a session. The durable example deliberately rejects actor-delegating bindings; it does not distribute the BFF signing secret or add browser impersonation.

## 4. Prepare protected configuration

Keep configuration and state below an ignored private directory, such as `data/portfolio/manual-example/`, or a protected operator directory. Apply the operating system's user-only access policy. Unix mode bits are not a complete Windows ACL policy. Do not check in tokens, SQLite databases, WAL/SHM files, receipts, or receiver keys.

Create a private JSON configuration with `statePath` as an absolute SQLite path, `baseUrl` as the approved API base including `/api`, and a `principals` map. Each principal alias has its own protected `token` value. The aliases used by the complete check are `operator`, `reviewer`, and `reader`. Do not copy a working configuration into this README. Use HTTPS for a remote approved backend; plain HTTP is limited to an explicit loopback test server.

To run the optional local receiver, add `receiver` with absolute `statePath`, `keyId`, and `keyBase64`. The chosen key must contain at least 32 bytes and must correspond to the endpoint's key-manager reference. Optional absolute `readyPath` and `failurePath` are local host-fixture controls. The failure file makes this local receiver return 503; there is no HTTP control endpoint. Never reuse a qualification key outside its isolated environment.

Run `node examples/typescript/src/receiver-main.ts <private-config.json>` from the repository root. The receiver allocates a loopback port and reports it. Use the qualification transport for the synthetic local check. Do not weaken production SSRF protection to register a private receiver.

## 5. Persist before each financial send

Create a private intent JSON file. It must contain `key`, `principal`, `baseUrl`, `operationId`, `path`, and `body`. Choose a visible ASCII key of 16–255 characters once. Use `createAccount`, `requestFunding`, `postFundingEvent`, or `createTransfer` as appropriate. The example also supports a recorded reconciliation command and webhook registration. Keep exact money and versions as JSON strings.

For an internal transfer, use the canonical fields `source_account_id`, `destination_account_id`, `amount`, and `currency`. For funding, use `destination_account_id`, `amount_minor`, `currency`, `external_reference`, and `evidence_reference`. Account creation accepts INR and exact zero only; it has no opening-money field.

```powershell
node examples/typescript/src/main.ts prepare <private-config.json> <private-intent.json>
node examples/typescript/src/main.ts inspect <private-config.json> <original-key>
node examples/typescript/src/main.ts send <private-config.json> <original-key>
```

Preparation commits the complete canonical body, operation, API destination, principal alias and a non-secret credential digest before the first network call. A changed intent cannot replace the old key. Each explicit send makes at most one HTTP attempt. The client has a five-second sample timeout and a 64 KiB response bound. It does not generate another key or automatically retry an error.

An unknown response keeps the original persisted body and key. A new process can inspect or resend that exact command. Changing the configured credential or destination stops the retry before network dispatch. This prevents an alias from silently moving the same unresolved command to a different principal or ledger. Credential rotation for an unresolved command requires approved identity/outcome recovery; the example does not automatically rebind it.

An approval is not a blindly retried transfer. Inspect the funding event as the separate reviewer, record the review intent before the one-time approval call, and inspect current state if its response is lost. The qualification records a synthetic review only. A real operator must follow the applicable evidence-review and approval policy. Posting uses its own persisted idempotency key.

For HTTP 400, correct the invalid request through a deliberate new command after inspecting the original. For 403, stop and resolve authority. For 409, inspect the original intent; do not overwrite its key. For 429, honor the bounded `Retry-After` and preserve the original command. For a 503 or transport timeout, inspect the operation's contract before an explicit retry. Reconciliation has its own unknown-result rule; inspect its run state instead of applying the transfer retry rule indiscriminately. The unit tests use a labelled local HTTP fixture for these generic client error cases; the full integration exercises real authority, validation, conflict and rate-limit responses.

## 6. Receive signed events safely

Verify the HMAC over the exact raw bytes. The message is `timestamp + "." + deliveryID + "." + rawBody + "." + keyID`. The signature is lowercase hexadecimal SHA-256 with the `v1=` prefix. For endpoint verification, use the verification ID in place of the delivery ID. After valid verification, return `X-LedgerSync-Verification: v1=<HMAC-SHA256 of the raw challenge text>`.

The receiver rejects unknown keys, ambiguous duplicate protocol headers, invalid signatures, bodies over 256 KiB, and timestamps outside its explicit five-minute local replay window. It compares event ID and type headers with their signed body fields. A retry can have a new delivery ID and timestamp while it retains the original event ID and bytes.

One SQLite transaction records event identity, raw-byte digest, delivery identity and the example business effect. Duplicate acknowledgements do not repeat that effect. A valid signature over a different body with an existing event ID is a conflict. State uses WAL and full synchronization. The local example stops at 10,000 events or 20,000 deliveries and requires an explicit archive/retention decision; it never deletes deduplication evidence automatically.

The real delivery check selects `transfer.posted`. Each transfer is an immutable aggregate. The separate ordering tests use signed synthetic `account.balance.changed.v1` fixtures and compare exact `balance_version` strings with `BigInt`; they do not claim that this additional event subscription is enabled by the selected API workflow. Older versions cannot replace newer state. Neither receiver state nor event arrival order authorizes a financial command.

## 7. Keep contracts reproducible

The canonical API is `contracts/openapi.yaml`. Generate with `node scripts/generate-developer-artifacts.mjs` and verify with the same command plus `--check`. Fix client transport code in `scripts/templates/typescript-client.ts.txt`; do not patch generated output alone. The TypeScript client is a small operation catalogue and bounded HTTP adapter, not a complete generated domain model. Existing Go and Postman artifacts remain.

Keep installation, identity setup, private paths, selected subscription, error handling and cleanup explicit when another engineer follows this guide. The fixed local check is repeatable evidence; any future independent human trial must record its actual missing instructions and elapsed time separately.
