# Local capability sets

The default product supports accounts, funding, internal transfers, exact retry and recovery, corrections, internal reconciliation, approved delivery recovery, authorized exports, diagnostics and developer contracts. A live room is not required for a financial command or its recovery. Administration stays denied. Payout provider experiments have no default runtime binding and make no external payment claim.

The API and BFF accept only an empty value, `false`, or `true` for the two optional flags. An empty value means false. Invalid values stop startup. Feature settings come from the server environment. The session response exposes only the resulting safe booleans. Browser preferences do not enable a server route. Both optional sets are limited to the explicit development environment; production configuration rejects them.

| Set | LEDGERSYNC_STUDY_ENABLED | LEDGERSYNC_LIVE_INVESTIGATION_ENABLED | Result |
|---|---|---|---|
| Core / all optional features off | false | false | Main financial workflows; no Systems Lab or new live collaboration. |
| Study | true | false | Core plus read-only Systems Lab observations and retained failure proof. Dedicated study Compose adds its gateway, scheduler and replica demonstrations. |
| Collaboration | false | true | Core plus the retained two-operator WebRTC demonstration. Redis, signalling and optional peer web instance have separate duties. |
| Combined study and collaboration | true | true | Rejected. This combination has no qualification claim. |

The base Compose configuration and examples default to core. The study overlay enables the study flag on the API and BFF and disables live collaboration. The collaboration overlay enables live collaboration on the API, web and optional peer web process and disables study. Do not apply either overlay to the preserved normal environment during this programme. Use a separately qualified local project. Selecting a Compose profile alone does not override the server feature controls.

When study is off, the API distributed-observation route is absent, and both BFF observation and evidence-download routes return 404 before forwarding. Developer contract and OpenAPI reads remain core. The Systems Lab disclosure follows the server session flag. Its saved results remain historical; enabling the view does not make an old test a current health result.

When collaboration is off, room creation, joins, invitations, leave/end commands, findings, signalling and presence are denied. No new signal store is constructed. Existing room links retain authorized history. The history reader uses a tenant/actor-scoped read-only transaction and checks the stored owner/peer relation. It does not expire the room, change its version, append audit records, or read Redis presence. A stored room status or finding is labelled as stored; the request state is a separate current read. The disabled view contains no active room controls. Retention and access rules still apply; the flag never deletes evidence.

The scheduler remains a separate explicit opt-in process. Its durable lease, tenant allowlist and fencing tests remain required. A study gateway and optional peer have no place in the default money path. Their implementation and historical evidence remain available; their presence in source is not a claim that they run in the normal environment.

The request-driven cron drain is disabled unless both `CRON_SECRET` and `LEDGERSYNC_CRON_WORKER_DATABASE_URL` are configured. The secret must meet the existing 32–256-character bound. The worker URL must be distinct from the API URL. Runtime validation also requires a separate LOGIN that inherits exactly `ledgersync_worker`, has no other role membership, owns no public objects or current database, and has no superuser, bypass-RLS, role-creation, database-creation or replication capability. A different URL string cannot bypass this role check. The API owns and closes the separate worker pool. API credentials never receive global queue discovery to make cron work.

Run `node scripts/portfolio/run-optional.mjs` for the fixed disposable qualification profile. It checks restricted cron execution, read-only retained history, enabled room lifecycle, ephemeral Redis signals, scheduler fencing and tenant isolation. Core/default flag checks and direct disabled-route checks run in deterministic and browser suites. These checks do not claim a managed TURN service, arbitrary feature combinations or a physical cross-network browser call. Final integrated image and broader browser qualification belong to R13.

The fake payout provider stays in its existing application boundary because it is a preserved extension contract. It is deterministic simulation only. No migration, financial record, old specification or evidence link is deleted to shorten the default demo. The external payout programme remains deferred.
