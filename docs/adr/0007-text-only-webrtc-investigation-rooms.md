# ADR 0007: Text-only WebRTC investigation rooms

- Status: Accepted for local, feature-flagged verification
- Date: 2026-09-08

## Context

An operator can lose the HTTP response after a transfer request reaches the API. LedgerSync correctly treats that as an unknown outcome: the operator must check the authoritative request state and must not create a second transfer. Two authorized operators may still need to compare the submitted intent and the available evidence while that status remains unresolved.

A general chat product, a financial command channel, or a stored transcript would enlarge the trust boundary without improving ledger correctness. The collaboration path must fail independently of the transfer path.

## Decision

LedgerSync provides one optional live room for an owner-scoped uncertain transfer request:

- PostgreSQL stores the room lifecycle, two participant bindings, expiry, optimistic version, sanitized lifecycle audits, and one structured finding.
- Redis stores bounded SDP/ICE signalling and presence for 35 minutes. It stores no chat transcript or submitted intent.
- A reliable ordered WebRTC DataChannel named `ledgersync-investigation-v1` carries text and the initiating operator's submitted intent directly between browsers.
- The room is limited to two authenticated, same-tenant operators with `investigation:collaborate`, `investigation:read`, and `transfers:read`. Starting and concluding the investigation additionally require `investigation:write`.
- Messages are plain text, at most 2 KiB, not linkified, and capped at 200 in browser memory. Reloading deliberately clears them.
- Financial commands, credentials, authorization material, balances, idempotency keys, and authoritative outcomes never travel over WebRTC.
- A finding is a server-validated enum. It cannot contradict the authoritative transfer-request state.

The feature is disabled by default. Local development uses host ICE candidates. A later production decision requires shared Redis plus an approved ephemeral TURN-credential provider; missing protection fails closed. The current web boundary intentionally enables rooms only in development.

## Consequences

Collaboration outages do not change a transfer, hide its status, or disable the existing status and safe-retry paths. Redis loss ends live signalling but PostgreSQL still retains the auditable lifecycle and finding. The system does not provide transcript recovery, attachments, URLs, camera, microphone, screen sharing, or financial actions in the room.

Invite material is single-use, expires after ten minutes, travels in the URL fragment, is removed before redemption, and is never written to audit metadata. Room membership, tenant, scopes, and record ownership are rechecked on every server request.

## Rejected alternatives

- Persisted chat: rejected because it adds sensitive-data retention, discovery, moderation, and access-control obligations.
- WebSocket chat through the API: rejected for this narrow two-person aid because it centralizes message content and adds a durable messaging surface.
- Commands through the DataChannel: rejected because peer messages are not an authoritative, authenticated financial-command boundary.
- Production host-candidate-only WebRTC: rejected because reliability and network privacy require an approved TURN design before production enablement.
