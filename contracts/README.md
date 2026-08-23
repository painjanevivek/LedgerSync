# LedgerSync Runtime Contracts

`contracts/openapi.yaml` is the source of truth for the private Go API. The browser-facing BFF mapping and event semantics remain documented in `specs/001-secure-transfer-core/contracts/http-api.md`. CI lints OpenAPI and compares its route and lossless-money markers with the registered MVP surface.
