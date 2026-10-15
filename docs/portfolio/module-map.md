# Module ownership after R04

This map explains the selected refactor. It complements the R00 inventory. It does not require a new service or a new Go module.

| Responsibility | Owner and entry point | Preserved rule and verification |
|---|---|---|
| Worker process lifetime | `cmd/outbox-worker/main.go`: `main`, `runWorker`; health file logic remains in `health.go` | Signals, dependency startup, cleanup, timers and process exit remain local. Startup errors return through resource cleanup. Health requires successful polls for all four queues. |
| Shared worker construction | `internal/bootstrap/worker.go`: `NewWorkerRunnerWithOptions` | One construction path serves continuous and request-driven adapters. The continuous process keeps its worker node identity, item/progress hooks and key-cache observer. The original constructor retains its API node default. |
| Independent queue iteration | `internal/bootstrap/worker.go`: `WorkerRunner.RunOnce` | Every queue is attempted in order. One failed queue does not starve another. Start/completion callbacks remain around each queue. Unit tests and a real restricted-worker integration test cover this contract. |
| Delivery use case and ports | `internal/application/webhookdelivery/worker.go`, `contracts.go` | Own leased delivery flow, input/output types and small interfaces. It imports no HTTP adapter. |
| Verification use case | `internal/application/webhookverification/worker.go` | Own challenge workflow and queue policy. It imports no HTTP adapter. |
| Signing-key resolution and cache | `internal/platform/webhookdelivery/key_resolver.go` | Preserve copied key bytes, bounded cache size, expiry, eviction and concurrency behavior. Existing tests moved with the implementation. |
| Signed HTTP dispatch | `internal/platform/webhookdelivery/dispatcher.go` | Preserve signatures, payload bounds, response classification and request deadlines. Application type aliases retain the port identity. |
| Outbound network policy | `internal/platform/webhookdelivery/transport.go` | Preserve TLS, redirect denial, connection bounds and public-address dialing. |
| Verification HTTP adapter | `internal/platform/webhookverification/dispatcher.go` | Preserve signed challenge and endpoint proof checks. Existing TLS tests moved with it. |
| Correction workspace state | `web/src/features/corrections/useCorrectionsWorkspace.ts` | Own list/detail requests, stale-response tokens, effects, command state, step-up and unknown-result messages. |
| Correction rendering | `web/src/features/corrections/CorrectionsConsole.tsx` | Retain the existing screen, controls, review dialog, URL filters and return context. The render body is mechanically unchanged. Existing browser tests cover posting review, step-up, filters and invalid URLs. |
| API routes and handlers | Existing `cmd/api/*_routes.go`, `internal/transport/http/handlers`, `internal/bootstrap/api.go` | Existing route/handler separation is retained. This packet does not merge distinct process and request adapter lifetimes without proof of equivalence. R11 checks the optional adapter capability matrix. |
| Financial persistence | Existing application ports and `internal/platform/db` controlled adapters | R03 remains the sole protected write boundary. R04 changes no migration, amount calculation or financial authority. |

## Selection and limits

The worker had two real construction paths with overlapping queue setup and different progress wiring. The application dispatcher also owned concrete HTTP, key cache and network policy. These were dependency and maintenance costs, not line-count targets. The correction view mixed request ownership with rendering. Those three boundaries are the selected hotspots.

The domain identifier still implements `database/sql/driver` conversion. It has no connection or query. Moving that compatibility interface would touch scanner/value callers without removing a concrete database adapter. Retain this narrow named exception. The architecture test continues to reject other domain persistence dependencies.

Four adapter/test files moved from application to platform. Their original bytes are retained in the private R04 before-copies, and the move map records their hashes. Contracts remain with the use case. No route or external contract changed. Existing source work and database records remain preserved.

The browser suite uses a separate local frontend on port 3100 and mocked responses. Its private API URL points to an unavailable loopback port. It cannot submit to the preserved financial backend. This verifies UI behavior; the exact candidate image and complete release journey remain R13 work.


## R12 example boundary

`examples/typescript/` owns the small trusted-backend client, protected local intent state, and synthetic receiver. It depends on the generated TypeScript operation catalogue and the pinned Node runtime. It has no import of internal Go implementation packages and no access to the normal financial database. The receiver SQLite state is local consumer evidence, not ledger authority. Real integration qualification lives in guarded Go integration fixtures with restricted workload sessions and a test-only local receiver transport.

The Developer UI reads a small generated retry-contract projection inside the web root. It does not import the repository SDK manifest across the Next.js build boundary. The canonical generator checks this projection with the other artifacts.
