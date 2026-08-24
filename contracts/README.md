# LedgerSync Runtime Contracts

`contracts/openapi.yaml` is the source of truth for the private Go API. `contracts/developer-examples.v1.json` is the bounded, non-secret source for developer workspace examples and retry guidance. Both files are embedded in the API binary and exposed through authenticated, read-only contract routes; no runtime filesystem lookup or request runner is involved.

The browser-facing BFF mapping and event semantics remain documented in `specs/001-secure-transfer-core/contracts/http-api.md`. CI lints OpenAPI, resolves every local reference, compares routes and response DTO fields with the registered runtime surface, and validates the versioned examples against their request schemas.
