# Master Phase 7 — reproducible developer contract artifacts

**Status:** `PARTIAL` — this records the generated-contract slice only. Phase 7 remains active until a partner can complete the documented lifecycle and the remaining webhook, provisioning, and recipe gates have exact-commit evidence.

## Control implemented

- `contracts/openapi.yaml` is parsed as the only generator input.
- The generator emits a versioned TypeScript operation catalogue, Go operation catalogue, manifest with the complete OpenAPI SHA-256 split into fixed-size provenance chunks, and Postman-compatible collection under `contracts/generated/`.
- `npm --prefix web run check:developer-artifacts` regenerates in check mode; the contract workflow runs it before OpenAPI lint and Go contract tests.
- Generated transports require caller-provided credentials and actor assertions. They contain no credential storage, request runner UI, or claim to move external funds.

## Local verification

- `npm --prefix web run check:developer-artifacts` passed.
- `web/node_modules/.bin/redocly.cmd lint contracts/openapi.yaml` passed.
- `go test ./tests/contract -count=1` passed.
- `npm --prefix web test` passed (95 tests).

## Scope boundary

The generator is deliberately an SDK catalogue, not a published package. SDK packaging, live signed webhook delivery, bulk provisioning, and partner recipes remain Phase 7 work and must be reviewed against the same OpenAPI source.
