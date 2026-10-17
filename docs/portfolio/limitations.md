# Limits and release decisions

The permitted target is a local technical portfolio with synthetic test data. A passing phase is a local engineering decision. It is not permission to use real customer funds or to expose a shared service.

| Area | Current limit | Required next evidence |
|---|---|---|
| Financial scope | Same-currency internal ledger; no bank rails, cards, FX, or external payout | Separate product, finance, legal, and integration review |
| External statement | Verified bounded unquoted CSV v1; synthetic INR sample; one immutable mapping per tenant | Provider-specific formats, multiple mappings/currencies and actual external evidence need separate qualification |
| Reservations | Verified full internal capture/release and fenced expiry; no partial capture or renewal | New obligation types require state, race, model, recovery, API and UI verification |
| Identity | Local synthetic identities and restricted workload tests | Actual provider, issuer, tenant mapping, revocation, and managed pool checks |
| Tenant isolation | Forced row policies plus trusted tenant and actor context | A fully compromised shared workload remains outside that isolation guarantee |
| Delivery | At-least-once delivery with bounded retries and fencing | Receiver deduplication and actual destination operating checks |
| Capacity | One-host component workload, recorded versions and resources | HTTP/BFF, target workload, multi-host, and managed-service measurement |
| Recovery | Synthetic candidate restore and supported historical fixtures | Actual managed backup, restore, failover, retention, and operating owner |
| UI acceptance | Automated keyboard, reflow, accessibility, and reviewed images | Physical devices and independent assistive-technology review |
| Optional features | Core, study, or collaboration as separate supported sets | No combined-set or arbitrary optional combination claim |
| Security tooling | Runtime dependencies and images have recorded checks; development-only braces risk is bounded | Track upstream lint-tool advisory and rescan changed source/dependencies |
| Release | Identified local source/image qualification | Clean committed release gates, exact manifest, and later deployment approval |

Raw evidence and failed runs remain in ignored local storage. Public documents contain bounded summaries and checksums. Do not publish database backups, source snapshots with local configuration, opaque sessions, credentials, full scanner matches, or private operator data.

The [qualification guide](qualification.md) states scanner rules. The [issue register](issues.md) retains failures and repair conditions. The [master register](../plans/ledgersync-master-progress.md) retains commercial and managed-environment gates. R17 prepares a package locally. Provider account, region, domain, budget, operating owner, and actual managed checks remain later inputs.

The pinned Linux browser target passed the final unchanged performance budgets. The native Windows four-times CPU-throttled preflight measured LCP at 2864 ms against 2500 ms and remains unqualified. Its 30-request count, interaction and layout checks passed. This is P-106 in the issue register. Do not extend the Linux timing claim to native Windows or physical devices.
