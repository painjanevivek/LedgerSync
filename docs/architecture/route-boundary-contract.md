# Safe route-boundary contract

## Context

LedgerSync handles financial evidence, so an unexpected render failure must not be presented as a confirmed success, confirmed rejection, or empty record set. An unavailable route must also avoid revealing whether a record, tenant, capability, or unreleased administrative surface exists.

Phase 11 reduced the representative Overview request count from 32 to 29. That recovered enough budget to add the two justified application boundaries while retaining the hard ceiling of 32 initial requests.

## Implemented boundaries

### Application error boundary

`src/app/error.tsx` is the client boundary required by Next.js 16 for errors below the root layout. It intentionally does not read, display, serialize, or log the supplied error. The operator receives only these safe facts:

- the page was interrupted;
- no financial result is inferred;
- retrying the same page is safe;
- the Overview is the fallback destination for fresh recorded evidence.

The retry control clears the framework boundary with `reset()` and reloads the same URL. The reload is required for first-load server-render failures because clearing client state alone can retain the failed server payload. It does not create a new financial command, alter an idempotency key, or invent a result.

### Application not-found boundary

`src/app/not-found.tsx` returns the same calm public outcome for unknown paths and the intentionally unreleased `/admin` path. Its copy does not distinguish invalid identifiers, missing records, authorization failures, tenants, or unreleased capabilities. The only action returns to Overview.

## Deliberately excluded boundaries

- `global-error.tsx` is not added. There is no reproduced root-layout failure that justifies another client boundary and asset cost. The built-in framework fallback remains the last-resort root failure behavior.
- Root or route `loading.tsx` files are not added. Existing controllers preserve independent loading, unavailable, historical, and refreshing evidence. A route fallback would replace that retained evidence and weaken truthfulness during refresh.

These decisions must be revisited only with a reproduced failure or measured streaming problem, including before/after request and web-vital evidence.

## Framework guidance applied

The installed Next.js 16 package establishes that `error.tsx` must be a Client Component, that its boundary can retry through `reset`, and that `notFound()` selects the route's not-found convention while applying 404/no-index behavior. The implementation follows those installed contracts without copying the framework's default global error UI, which can expose a digest.

## Test-only failure probe

Automated browser verification needs a deterministic failure followed by a successful retry. `src/app/test-support/route-error/page.tsx` is therefore gated by all of the following:

- explicit `LEDGERSYNC_ENABLE_TEST_RENDER_FAILURE=true`;
- the development deployment mode;
- the exact Playwright loopback host `127.0.0.1:3100`;
- a bounded UUID attempt identifier.

Outside that exact conjunction it calls `notFound()` and is indistinguishable from any other unavailable route. It performs no API request and no financial mutation. A bounded in-memory attempt set makes the first render fail and the same-page retry succeed.

## Release gates

- Error source tests prove there is no `.message`, `.digest`, browser console logging, identifier, or serialization path.
- Browser tests prove 404 return navigation, same-page render recovery, no raw failure copy, and `/admin` equivalence.
- Visual and automated accessibility checks cover both unavailable and interrupted route states.
- Constrained-4G Overview measurements must remain at or below 32 initial requests and within LCP, INP, CLS, and long-task budgets.
- Static JavaScript budgets remain unchanged; a passing build alone is not sufficient.

## Measured Phase 13 result

The constrained-4G Overview run after both boundaries were added recorded 31 initial requests, including 5 initial API requests. This remains below the fixed ceiling of 32 and uses two of the three slots recovered in Phase 11.

Observed vitals remained inside the pilot contract: 1,296 ms LCP, 24 ms INP, 0.0010923108 CLS, and one 92 ms maximum long task. The production static output contained 34 JavaScript chunks totaling 1,278,305 bytes; the largest chunk was 229,156 bytes, within the 350,000-byte per-chunk and 2,000,000-byte total limits.
