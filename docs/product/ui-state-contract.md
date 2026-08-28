# LedgerSync UI state and recovery contract

This contract applies to every operator and developer surface. It prevents transport, dependency, and authorization uncertainty from being presented as financial or operational truth.

## Canonical data states

| State | Meaning | Required presentation |
|---|---|---|
| `loading` | No verified response exists yet. | Show bounded progress; do not show zero, empty, passed, or failed business data. |
| `ready-empty` | A successful authoritative response returned no records. | State the verified scope and offer one safe next action when one exists. |
| `ready-populated` | Current authoritative evidence is available. | Show its provenance, exact identifiers, and verification time where freshness matters. |
| `partial` | A successful response covers less than the product claim would require. | Preserve the bounded evidence, label the missing scope, and block aggregate conclusions or dependent commands. |
| `stale` | Previously verified evidence remains after a refresh failure or while offline. | Keep it visible, label the last verification time, and prevent it from authorizing a command that needs current evidence. |
| `unavailable` | A dependency request failed and no verified result is retained. | Describe what failed, avoid empty-state language, include the safe request reference, and offer a dependency-scoped retry. |
| `forbidden` | The verified session lacks the required scope. | Do not request or disclose the protected evidence; identify the required permission. |
| `offline` | Browser connectivity is unavailable. | Retain historical evidence, disable network actions, and explain that reconnecting does not prove a financial outcome. |
| `unknown-after-submit` | A write may have reached the authoritative service but its result is unconfirmed. | Lock the exact body and idempotency key, survive navigation or refresh, and permit only same-intent retry or authoritative evidence lookup. |

## Error content

Every dependency error must answer three questions:

- What failed or remains unknown?
- Which previously verified evidence was preserved, if any?
- What single safe action can be tried next?

Read requests carry a bounded `X-Request-ID`. The UI retains the response reference, or the locally generated request reference when no response arrives. Correlation and request identifiers may be displayed; cookies, bearer tokens, workload credentials, CSRF values, raw upstream bodies, and internal endpoints may not.

## Command and interaction rules

- A visible financial command is disabled whenever its current evidence, authorization, connectivity, or bounded-scope prerequisite is unavailable. The reason is adjacent to the control or connected with `aria-describedby`.
- A failed dependency receives its own retry. A balance retry must not reload history; an account-picker retry must not resubmit a transfer; a reconciliation-history retry must not start a run.
- A write uses an immediate in-memory in-flight lock in addition to the rendered pending state. Mouse, keyboard, touch, repeated event delivery, and rapid activation cannot create a second network submission.
- Unknown account, transfer, and reconciliation outcomes persist their exact retry identity in tenant-scoped session storage. Reloading must not generate a replacement key or unlock editing.
- Modal dialogs return focus to the control that opened them. Compact navigation returns focus to the menu trigger and traps focus while open.

## Notification persistence

LedgerSync does not use auto-dismissing toast notifications for financial outcomes, authorization failures, recovery evidence, or unknown submissions. These states remain in the document until the operator dismisses or resolves them, and critical outcomes receive programmatic focus. A transient toast may be used only for a reversible convenience acknowledgement such as “copied”; it must not be the sole carrier of business state or recovery instructions.
