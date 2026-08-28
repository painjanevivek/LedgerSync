# Local-product Phase 6 — safe API-first developer workspace

**Result:** `PASSED`

**Verified:** 2026-08-24T21:59:28Z

**Candidate:** Phase 6 working tree based on `94b1537`; the resulting Phase 6 commit binds this evidence to the implementation.

**Boundary:** the supported single-workstation Docker Compose runtime, one INR demo tenant, a server-controlled local operator, a protected host-only private API credential, private API/PostgreSQL/Redis services, and browser access only at `http://127.0.0.1:3000`.

## Developer outcome

- `/developer` explains the local-only network boundary, distinguishes the browser BFF session from the private API development token, groups every supported account, transfer, reconciliation, operations, and developer endpoint, and exposes versioned examples derived from repository-owned contract assets.
- Exact money remains a decimal string at the HTTP boundary and integer minor units in stored result facts. No example uses floating-point money.
- Transfer and account examples state the identical-body, identical-idempotency-key retry rule. The error catalogue distinguishes safe same-key retries from conflicts that require investigation.
- The canonical OpenAPI document is available as an authenticated YAML download through the BFF. The browser receives neither the private API credential nor a general-purpose HTTP request runner.
- Correlation guidance directs operators to authorized account, transfer, reconciliation, and event evidence without claiming that a request identifier is financial proof.

## Contract and security controls

- The embedded contract version is `1.6.0`; the OpenAPI source is 47,716 bytes and the versioned example asset is 7,398 bytes, below their explicit bounded-response limits.
- Bidirectional drift validation fails if an authenticated runtime route is absent from OpenAPI or if OpenAPI advertises an unimplemented route. Every operation declares its required scope.
- Example-schema tests validate exact transfer/account payloads against the referenced OpenAPI schemas and reject numeric money.
- The private `GET /api/developer/metadata` and `GET /api/openapi.yaml` routes and their BFF counterparts require `developer:read`, use strict query allowlists, rate and response-size bounds, `no-store`, and non-disclosing denials.
- The host credential inspection command returns metadata, protected-file facts, and a SHA-256 fingerprint by default. Raw disclosure requires the explicit `-Reveal` switch and is limited to the selected private API token.
- Rotation changes only `LEDGERSYNC_DEVELOPMENT_API_TOKEN`, recreates only API and web, performs an authenticated BFF smoke test, and automatically restores the prior credential and recreates both services if activation fails.

## Automated evidence

| Layer | Result |
|---|---|
| Go unit, contract, integration, and system suite | `go test ./... -count=1` passed |
| Go static checks | `go vet ./...` passed |
| OpenAPI lint | Pinned `@redocly/cli@1.34.0` passed |
| OpenAPI/runtime drift and example-schema suites | Passed, including scope, schema-reference, exact-money, and bounded-asset assertions |
| Web unit/security | 65/65 passed |
| Full browser, accessibility, responsive, and visual suite | 94/94 passed with 16 workers |
| Focused developer browser states | 7/7 passed |
| Focused developer security suite | 5/5 passed |
| Type, lint, production build | TypeScript, ESLint, and Next.js build passed |
| Performance budget | 811,974 total JavaScript bytes; largest chunk 229,156 bytes, below 2,000,000 and 350,000-byte limits |
| Patch integrity | `git diff --check` passed |

## Live supported-stack proof

- The supported start path rebuilt all candidate images, preserved the existing PostgreSQL and Redis volumes, and returned PostgreSQL, Redis, API, worker, and web healthy at schema `000015`.
- The API image explicitly includes the embedded contract package. The web builder receives the contract fixture needed for compile-time/test validation, while the final standalone runtime image contains only its generated application assets.
- The live BFF metadata route returned HTTP 200, `Cache-Control: no-store`, contract version `1.6.0`, five endpoint groups, two versioned examples, and no credential value.
- The live OpenAPI download returned HTTP 200, `application/yaml`, and `attachment; filename="ledgersync-openapi.yaml"`; a secret-pattern check found no bearer credential or private environment-variable value.
- Final normal state remained schema `000015`, outbox pending `0`, outbox dead `0`, and latest reconciliation matched with `0` mismatches.

## Isolated credential rotation and rollback proof

The live harness required `-ConfirmIsolatedComposeCredentialRotation`, generated a uniquely named Compose project and state directory, and refused the normal `compose` project.

1. A real new private API token was written atomically, only API and web were recreated, and an authenticated BFF smoke test passed.
2. The harness then deliberately forced an activation-smoke failure.
3. The prior token was restored atomically, API and web were recreated again, and the authenticated BFF smoke test passed with the restored credential.
4. Fingerprints for every unrelated protected value were unchanged; captured output contained no raw secret.
5. Cleanup verified zero remaining acceptance containers, volumes, networks, or state directories and restored the normal stack healthy.

Two pre-rotation harness attempts failed safely during image build because the new contract assets were initially absent from the API and web build contexts. No token was rotated, cleanup still completed, both Dockerfiles were corrected, and a fresh isolated run passed the full success-and-rollback sequence. These failures are retained as evidence that packaging faults are detected before credential mutation.

## Visual review

The two new Developer baselines and the two transfer baselines affected only by the new navigation item were inspected and approved. The implementation retains the selected navy/emerald operational-console hierarchy, uses code surfaces for exact machine contracts rather than decorative terminal styling, contains long values at 320 CSS pixels and 200%-equivalent widths, and announces non-secret copy actions without placing credentials in the DOM.

## Local-only limitations and deliberate exclusions

- The BFF limiter is process-local because the supported boundary has one web process. Distributed rate limiting remains outside this local-only claim.
- The private development token is a workstation credential, not a production customer key. It is not downloadable or rotatable from the browser.
- Arbitrary request execution is intentionally absent. Copyable examples reduce integration friction without creating an SSRF, credential-exfiltration, or unrestricted mutation surface.
- Export contracts are intentionally deferred to Phase 7 so their authorization, streaming, CSV-injection, and bounded-range controls are introduced atomically.
- Windows cannot run the Go race detector for this CGO-free repository configuration; the existing pinned Linux CI race job remains the authoritative race gate.
