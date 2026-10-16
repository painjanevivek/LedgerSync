# Final phase trace — 17 October 2026

Use this document with the [phase register](../../../portfolio/progress.md). The detailed external plan retains all 18 phases and their original work steps. The tables below connect the final three phases to concrete results.

## R15: compare supplied statement evidence

| Plan steps | What a person can do | Source and proof |
|---|---|---|
| 1–4 | Read one exact UTF-8 CSV format. Check source, period, direction, currency and opening/closing totals. Select one fixed authorized INR account mapping. | [Format and bounds](../../../portfolio/statement-reconciliation.md), [application module](../../../../internal/application/statementreconciliation/) |
| 5–8 | Preview the complete file, then accept immutable bytes and SHA-256. Retry the original import after a lost response. A bad row rejects the complete import. | Protected source, stable row IDs, atomic commands; restricted-role and cancellation checks |
| 9–12 | Compare a saved ledger cutoff with the source period. Keep ambiguous candidates separate. Prevent reuse of a source row or posting. | Saved mapping/rule versions, snapshot items, exact unique matching and database consumption constraints |
| 13–16 | Assign and review a difference with a reason, comment and internal evidence reference. Stale decisions fail. A manual link cannot alter money or mismatch amount/currency/direction. | Optimistic review versions, immutable review events, original command receipts and no-financial-mutation checks |
| 17–20 | Keep external comparison separate from internal reconciliation. Download audited bounded evidence. Retain prior files and decisions. | Complete API/BFF/browser journey, formula-safe comparison export, original-source digest check and full restore |

The first version has one unquoted CSV format, at most 1000 rows and 1 MiB per file, one immutable INR mapping and at most 100 imports per tenant. It does not parse bank PDFs or submit settlement instructions. Retention limits do not authorize deleting existing evidence.

## R16: reserve and resolve an internal amount

| Plan steps | What a person can do | Source and proof |
|---|---|---|
| 1–4 | Read posted ledger, reserved and available amounts separately. Reserve an exact full amount and preserve the original command receipt. | [Reservation rules](../../../portfolio/reservations.md), [application module](../../../../internal/application/reservations/), exact-int64 and 128-step reference-model checks |
| 5–8 | Capture, release or expire once. Recheck account status and policy. Recover a denied capture without releasing its hold. Resolve capture/expiry races with one terminal outcome. | Controlled transactions, ordered account locks, worker-only owner/generation leases, audit and durable events |
| 9–10 | Use scoped API and browser controls. Keep unknown outcomes recoverable. Export current obligations after an audit write. Restore obligations and rebuild mixed event projections. | Final real-stack journey, full regression, separate bounded CSV snapshot and 76-table restore |
| 11–12 | Upgrade a clean or supported prior schema without turning old transfers into holds. Keep compatible rollback images. Recheck the affected core. | Supported migration/cancellation profile, version-1/version-2 event rules, 256-operation core model, three final capacity/recovery rounds |

Available equals posted ledger minus active holds. A passed deadline does not release money. One terminal database transaction must commit first. Only capture posts a journal. Full release and expiry leave posted history unchanged. The first version excludes partial capture, renewal, cards, FX and external authorization.

## R17: prepare deployment locally

| Plan step | Delivered result | Remaining external duty |
|---|---|---|
| 1 | Versioned secret-free inputs, separate workload environments and validators | Select provider, account, region, issuer, domain, budget and responsible people |
| 2 | Four bounded non-root workloads; exact local images, schema66 and contract 3.9.0; scans/SBOMs and identity package | Publish reviewed immutable registry digests and configure actual private services |
| 3 | Explicit migration profile and synthetic data confined to disposable test startup | Rehearse the approved managed maintenance window and backup |
| 4 | Startup, upgrade, compatible rollback, forward repair, backup/restore, rotation, incident and shutdown procedures | Named operators must validate those procedures in the selected environment |
| 5 | Fresh local candidate, verified TLS/collector rehearsal, dependency failure, readiness recovery, replay and graceful shutdown | Actual managed identity, pooling, ingress/egress, backup/PITR and failover |
| 6 | Reviewable local manifest and completed phase evidence | Independent human acceptance and a later managed go/no-go decision |

The [deployment guide](../../../../deploy/production/README.md) defines these duties. Static package validation and actual Compose rendering passed. A demand for managed readiness correctly failed while inputs remained unfilled. Scanner staging uses private disk with one CPU and 1 GiB memory. The updater and all four exact image scans passed; application budgets were unchanged.

## Final capacity results

| Run | Steady offered / new transfers | Steady command p95 ms | Steady oldest queue s | Final journals | Recovery |
|---|---|---|---|---|---|
| demo-20261016220328-be1862ea | 3000 / 2640 | 13.96 | 0.106 | 3168 | Passed |
| demo-20261016221041-bbdc1e0d | 3000 / 2640 | 13.29 | 0.104 | 3168 | Passed |
| demo-20261016221755-dcd99449 | 3000 / 2640 | 13.27 | 0.106 | 3168 | Passed |

All three runs use source manifest a9798c8ce5fb4bb4f51a60b64d16e05203190d4b104b1e5d1e71c943a99ec542. Each steady stage lasts 300 seconds at ten offered commands per second. A replay or rejected command is not a new transfer. These are one-host Go component results with six synthetic accounts in two tenants. The actual native worker is included; HTTP/BFF and active-hold throughput are excluded. The historical R09 chart keeps its original source and values.

## Preserved boundaries

The normal database stays on migration64 with 66 retained history rows. The final candidate uses 66 current source migrations and ends at migration66. Those counts describe different histories. The normal images, named volumes and financial data remain. All 122 private media artifacts remain local. Source snapshots, raw reports, credentials and backups are not publication artifacts.

P-062 remains a bounded development-tool advisory. P-082 retains its process deviation. P-106 remains an unqualified native Windows throttled timing. Provider, independent accessibility and managed operating gates remain open. These limits do not become production approval when the local source is committed.
