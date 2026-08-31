# Frontend controller ownership

This document records the route-controller boundary introduced during the frontend audit remediation. App Router pages remain thin Server Components: they validate route and query input, then select one controller. Controllers own orchestration; views remain presentational; the shared session boundary owns identity, capabilities, connectivity, and sign-out only.

## Route and request matrix

| Route shape | Controller | Initial authoritative reads | Explicitly not owned |
|---|---|---|---|
| `/` | `OverviewController` | Accounts (bounded to 100), recent transfers, latest reconciliation history, and local orientation when authorized | Detail reads, mutations, exports, correction records |
| `/accounts` | `AccountsController` | Filtered 25-row account directory | Transfers, reconciliation, orientation |
| `/accounts/new` | `AccountsController` | Complete bounded account picker used to prove funding eligibility | Transfer history and reconciliation |
| `/accounts/:accountId` | `AccountsController` | Complete bounded account picker, selected account, balance, and ledger history | Unrelated transfer list and reconciliation history |
| `/transfers` | `TransfersController` | Complete bounded account picker and filtered transfer list | Reconciliation history and orientation |
| `/transfers/:transferId` | `TransfersController` | Complete bounded account picker, selected transfer, and authorized explainability chain | Unrelated transfer list and reconciliation history |
| `/reconciliation` | `ReconciliationController` | Reconciliation history | Accounts, transfers, orientation |
| `/reconciliation/:runId` | `ReconciliationController` | Selected reconciliation result | Reconciliation list, accounts, transfers |
| `/guide` | `GuideController` | None | Every financial and operational evidence graph |

Every mounted console route also consumes the single shared `/api/session` result from `ConsoleSessionBoundary`. Route transitions do not refetch that session while the root layout remains mounted.

## Domain ownership

- `useAccountWorkspace` owns account directory, detail, balance, ledger history, and account pagination state.
- `useTransferWorkspace` owns transfer list/detail, explainability, immutable-ID deduplication, and transfer pagination state.
- `useReconciliationWorkspace` owns reconciliation list/detail, immutable-ID deduplication, and observation of a completed command result.
- `useOrientationWorkspace` owns local orientation evidence and server-owned preference updates.
- Long-running financial command state remains in its dedicated account, transfer, funding, correction, or reconciliation command hook. Read hooks do not absorb mutation recovery state.
- `ConsoleSessionBoundary` must never contain balances, amounts, transfers, journals, funding records, corrections, reconciliation results, or recovery evidence.

## Change rules

1. Add a request to the narrowest domain hook or route controller that owns the returned evidence.
2. Do not create a generic cross-domain `useEntity` or global financial cache.
3. Preserve route-page validation and `return_to` context in Server Components.
4. Bind replace and append responses to their resource identity and generation.
5. Keep pagination single-flight and deduplicate only by immutable server identifiers.
6. Preserve independently verified Overview evidence when another domain fails.
7. A controller extraction is complete only when its route journey, visual baseline, accessibility checks, request count, and production build remain green.
