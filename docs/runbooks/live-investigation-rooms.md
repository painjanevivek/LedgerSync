# Live investigation rooms runbook

## Purpose

This runbook covers the optional, text-only collaboration room for a transfer request that remains unresolved after an authoritative status check. The room is an investigation aid; it never determines or changes the financial result.

## Local enablement

The Compose development stack enables the capability with:

```text
LEDGERSYNC_LIVE_INVESTIGATION_ENABLED=true
LEDGERSYNC_LIVE_INVESTIGATION_NAMESPACE=ledgersync:investigation-live:local
```

The API also requires its configured Redis connection. The optional `web-peer` Compose profile starts a second BFF at `http://127.0.0.1:3001` with a separate development operator in the same tenant.

Do not put invite tokens, session cookies, idempotency keys, SDP, ICE candidates, messages, or submitted intent into logs or test evidence.

## Operator path

1. Submit a transfer through the normal Details → Review → Result workflow.
2. If the result is unknown, use **Check status** first.
3. Only while the request remains unresolved, select **Start investigation**.
4. Create the single-use invitation and give it to exactly one authorized operator through an approved local test channel.
5. Compare the initiating operator's submitted intent with the read-only server evidence. Treat neither chat nor the shared intent as proof of completion.
6. Refresh authoritative evidence. Use a structured finding only when the displayed server state permits it.
7. End the room. Use the normal status or safe identical-retry control for any financial follow-up.

## Failure interpretation

| Symptom | Meaning | Safe action |
|---|---|---|
| Collaboration unavailable | Redis, signalling, browser WebRTC, or permission check failed. | Continue with the normal transfer status and safe-retry controls. Do not infer transfer failure. |
| Invite unavailable | The invite expired, was already redeemed, belongs to another tenant, or the caller is not authorized. | The owner may issue a new invite if the second identity is not already bound. |
| Disconnected | The peer channel closed or network connectivity changed. | Reload the room URL as the same bound operator; the transcript will be empty. |
| Finding rejected | The room version or authoritative request state changed. | Refresh the room and review the current server evidence. |
| Room expired | Thirty minutes elapsed. | Start a new room only if the authoritative request still remains unresolved. |

## Verification

- Confirm `/api/session` advertises `features.live_investigation: true` only in the local feature-enabled environment.
- Confirm two same-tenant authorized operators connect and a third identity receives a non-disclosing not-found response.
- Confirm page reload clears messages and displays the transcript-loss notice.
- Confirm camera and microphone permissions remain disabled and no media controls exist.
- Confirm Redis keys expire or clear while the lifecycle and structured finding remain in PostgreSQL.
- Confirm audit metadata contains only room status and version—not participant subjects, invite data, request intent, signalling, addresses, or messages.
- Confirm collaboration failure does not change the authoritative transfer response.

Production enablement is intentionally out of scope until an approved ephemeral TURN-credential service and shared Redis configuration are available and tested.
