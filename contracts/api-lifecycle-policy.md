# LedgerSync API lifecycle policy

LedgerSync uses semantic API contract versions. Backward-compatible fields and operations increment the minor version, compatible clarifications and fixes increment the patch version, and an intentional breaking contract increments the major version. The reviewed version in `contracts/openapi.yaml`, the embedded `contractassets.Version`, generated SDK manifests, examples, and Postman collection must be identical.

## Supported window

- The current major version and its immediately preceding minor release are supported during the pilot.
- A minor release remains supported for at least 180 days after its successor is published unless a critical security issue requires an earlier stop.
- Removing or changing an operation, required scope, field meaning, exact-money representation, idempotency behavior, or webhook signature contract is a breaking change.
- Additive response fields are compatible; clients must ignore unknown response fields while requests remain strict.

## Response contract

Every API response, including errors, carries:

- `X-Request-ID`: an unpredictable server-generated correlation identifier.
- `X-LedgerSync-API-Version`: the reviewed semantic contract version.
- `X-LedgerSync-Mode`: `sandbox` for local/test data or `production` for production financial evidence. Request headers cannot override this value.

Every bounded-rate response that rejects work carries `Retry-After`. Retrying a mutation after a timeout or unknown response must use the identical body and `Idempotency-Key`; a changed intent must use a new key only after the original outcome is inspected.

## Deprecation

LedgerSync publishes a changelog entry before deprecating an operation. A deprecated operation is marked in OpenAPI and returns RFC-compatible `Deprecation` and `Sunset` headers plus a `Link` to migration guidance. The announced sunset is at least 180 days after notice. Security removal may be faster only with a named incident decision and partner notification.

## Generated artifacts

SDKs, examples, and collections are generated only from the reviewed OpenAPI file. CI regenerates them in a clean workspace and fails on any diff. Generated code is never edited by hand, never contains credentials, and always labels examples as ledger recording rather than external-funds movement.
