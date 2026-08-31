# Production administration boundary — design contract

**Status:** `BLOCKED` for implementation. This document is a security and product design input, not proof that production administration exists.

**Blocking dependencies:** M11 managed identity and tenancy, M12 production infrastructure, named production ownership, external security acceptance, and approved legal/compliance controls.

## Purpose and non-disclosure rule

LedgerSync administration will govern tenant and operator lifecycle. It must never grant money-movement authority implicitly. Until every blocking dependency is proven, the console has no Administration navigation, `/admin` returns the same not-found experience as an unknown route, and no browser or private administration API is shipped. A role or scope invented in browser state cannot change that result.

The eventual boundary must satisfy all three layers independently:

1. The server-issued session may disclose that administration exists only to an approved production administrator.
2. Every BFF route must re-authenticate the privileged session, require recent step-up, validate same-origin CSRF, and expose a fixed operation rather than a generic proxy.
3. Every private API command must derive tenant, actor, allowed transition, and approval policy from server-owned records and commit immutable audit evidence atomically.

## Proposed personas

These are proposed duties, not production assignments.

| Persona | Intended duty | Explicit exclusions |
|---|---|---|
| Platform identity administrator | Connect an approved identity subject to a managed tenant and initiate operator lifecycle commands | No transfer, funding, correction, replay, or unilateral grant approval |
| Tenant access administrator | Request tenant-scoped operator invitations, suspension, revocation, and least-privilege grants | No platform-wide tenant discovery; no self-approval; no implicit financial scope |
| Security approver | Independently approve high-risk grants, tenant reactivation, recovery access, and break-glass use | Cannot execute the command they approved |
| Compliance audit reader | Read bounded immutable administration evidence and approved exports | No lifecycle mutation, credential material, raw tokens, or unrestricted identity search |
| Recovery approver | Approve a time-bound, case-bound recovery identity or access restoration | No standing access; no approve-and-execute; no ledger mutation |

No production persona is considered owned until Product, Security, Operations, and Legal record a named accountable owner and review interval.

## Separation-of-duty policy

| Command | Requester | Independent approval | Executor | Required controls |
|---|---|---|---|---|
| Create or reactivate tenant | Platform identity administrator | Security approver | Different platform identity administrator or automated reconciler | Recent MFA step-up, four-eyes, immutable request key, approved legal/contract reference |
| Suspend tenant | Platform identity administrator or incident commander | Security approver, except a separately approved emergency containment policy | Different executor | Bounded reason taxonomy, impact preview, incident/case reference, no ledger deletion |
| Invite or reprovision operator | Tenant access administrator | Required when proposed grants contain write, approval, replay, export, or administration authority | Different tenant/platform administrator | Managed-identity proof, expiry, tenant binding, exact grant preview |
| Suspend or revoke operator | Tenant access administrator or security operator | Required for recovery accounts and administrators | Different executor for privileged identities | Session/token revocation evidence, effective-at timestamp, retry-safe command |
| Grant or revoke scope | Tenant access administrator | Security approver for privileged scopes | Different executor | Exact before/after grants, policy version, reason, expiry, object relationship |
| Activate recovery access | Recovery approver | Second security approver | Different executor | Case-bound, time-bound, monitored, automatically expires, no financial row mutation |

Self-approval, self-grant, wildcard grants, `platform:root`, unbounded tenant search, and a combined approve-and-execute endpoint are prohibited.

## Proposed lifecycle state machines

### Tenant lifecycle

```text
requested -> under_review -> provisioning -> active
active -> suspended -> active
active|suspended -> closure_pending -> closed
```

- `requested`: an approved commercial/legal reference exists, but no runtime tenant exists.
- `under_review`: Security, Product, Operations, and required legal checks are pending.
- `provisioning`: an idempotent managed workflow is creating server-owned records; the tenant cannot transact.
- `active`: explicitly approved capabilities may be used.
- `suspended`: new commands fail closed; evidence and legally required recovery remain available to separately authorized readers.
- `closure_pending`: writes remain disabled while retention, export, obligations, and owner approval are checked.
- `closed`: irreversible business closure marker; financial and audit evidence remains governed by retention policy.

There is no direct `requested -> active`, `suspended -> closed`, or `closed -> active` transition. Closure never deletes journals, postings, approvals, audit events, or required delivery evidence.

### Operator lifecycle

```text
invited -> provisioned -> active
active -> suspended -> active
invited|provisioned|active|suspended -> revoked
invited -> expired
```

- Invitations are single-purpose, tenant-bound, expiring, and never expose whether another tenant or subject exists.
- Provisioning requires managed identity proof and a reviewed grant set before activation.
- Suspension blocks new privileged use without rewriting historical actor evidence.
- Revocation is terminal for that operator binding and must revoke active sessions/tokens with measured propagation evidence.
- Re-entry after revocation creates a new reviewed binding; it does not resurrect the old record.

### Grant lifecycle

```text
requested -> approved -> applied -> revoked
requested|approved -> rejected
approved|applied -> expired
```

Each grant binds a tenant, subject, exact scope, object relationship, requester, independent approver where required, policy version, reason code, request key, expiry, and evidence timestamps. A changed grant set is a new intent; it cannot reuse an idempotency key from another request.

## Step-up, four-eyes, and recovery requirements

- Sensitive administration requires a managed-provider MFA event whose authentication time is no older than the approved window. Repository fixtures or local login cannot prove this.
- Approval and execution must use separate actors and separate stable request keys. Retrying an unknown response reuses the exact intent and key.
- Approval expires after a policy-defined period and cannot authorize a materially changed target, grant set, tenant state, or reason.
- Recovery identities are disabled by default, named, time-bound, alerting-enabled, and automatically revoked. They cannot receive financial write authority through an administration shortcut.
- Break-glass use requires a case reference, two-person activation, narrow duration, live monitoring, post-use credential/session invalidation, and a retrospective review.

## Enumeration and privacy boundary

- Unknown, cross-tenant, unauthorized, revoked, and intentionally hidden administration objects return the same non-disclosing result.
- Search requires an exact approved tenant reference or exact subject reference; no prefix, wildcard, email-domain, or global people directory is exposed to tenant administrators.
- Errors never reveal whether a subject belongs to another tenant, whether an invitation exists, or which administrator owns a tenant.
- Browser data excludes raw identity-provider claims, token identifiers, credential references, private emails unless policy requires them, secrets, and infrastructure identifiers.
- Rate limits apply per privileged subject, tenant, operation, and source risk signal in shared storage; process-local limits are insufficient for production.

## Immutable audit and export contract

Every proposed command records an append-only event with the allowlisted fields below. Audit persistence must be atomic with the authoritative state transition or the command fails.

- event ID, UTC occurred-at time, tenant ID, actor subject ID, actor duty, and authenticated-at evidence reference;
- command type, target type, target immutable ID, prior state, resulting state, and policy version;
- stable request key outcome, bounded reason code, approval ID and independent approver subject where required;
- sanitized correlation/request reference and deployment/release identity;
- for grants, sorted exact scopes, object relationship, expiry, and a digest of the reviewed before/after set;
- no token, secret, raw identity-provider payload, unrestricted personal data, or caller-supplied free-form log message.

Administration exports are asynchronous, case-bound, authorized separately from mutation, schema-versioned, size/time bounded, encrypted, expiring, access-audited, and contain only the approved tenant scope. CSV cells are injection-safe. Export generation cannot discover cross-tenant records through counts, filenames, timing, or error differences.

## Future API/UI constraints

The eventual API shape remains a design proposal until M11/M12 approve a versioned contract:

- purpose-specific list/detail/command routes; never a generic admin query or command runner;
- opaque cursors, bounded filters, no fabricated totals, and exact tenant predicates;
- `404`-equivalent non-disclosure for hidden objects and `409` for safe state/idempotency conflicts only after authorization;
- explicit preview/review before mutations, no optimistic success, and unknown-outcome recovery with the exact key;
- progressive disclosure of operational impact, never secret material or raw provider payloads;
- navigation appears only for a server-issued production privileged-session capability after external review acceptance.

## Required evidence before unblocking

- Real Cognito authorization-code + PKCE + MFA and recent-auth evidence for the exact production app client.
- Managed tenant/operator/grant schemas, migration review, immutable audit storage, shared limits, and revocation propagation.
- Negative isolation matrix across tenants, subjects, duties, objects, direct URLs, APIs, exports, cursors, and timing/error behavior.
- Four-eyes concurrency, expiry, replay, unknown-response, and changed-intent conflict tests.
- Reviewed AWS private networking, WAF, KMS/secrets, logging, alerting, backup/PITR, and break-glass controls.
- External security review acceptance, named operational ownership, legal/compliance approval, and exercised incident/recovery runbooks.

Until all evidence exists, Phase 9 remains `BLOCKED`; design completeness must not be reported as implementation completeness.
