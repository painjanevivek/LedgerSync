# Investigation workspace threat model

## Protected assets

- Tenant and operator ownership boundaries.
- Record identifiers, request references, and relationship topology.
- Immutable lifecycle/audit history.
- Current ledger, delivery, and reconciliation evidence read through the workspace.
- Recipient subject identifiers used for handoff.

## Trust boundaries

The browser is untrusted. It submits only a safe title, fixed taxonomy, original bounded query context, and one root type/ID. The BFF verifies the signed session, exact Host, same-origin CSRF for writes, method, size, schema, rate, and response shape. The private API verifies the signed actor assertion, operator role, investigation scopes, root-domain scope, and object authorization. PostgreSQL is authoritative for workspace ownership, optimistic state, captured references, and immutable audit rows.

## Threats and controls

| Threat | Control |
|---|---|
| Cross-tenant or cross-owner enumeration | Every query binds tenant and owner; malformed IDs and absent/different-owner records converge on non-disclosing not-found. |
| Creating a workspace for an inaccessible record | Server reauthorizes the root through the deterministic related-evidence boundary before the transaction. Client-supplied relationships are not accepted. |
| Scope or object-access revocation after creation | List reauthorizes the root in its bounded database query; detail repeats root object authorization and individually rechecks historical references; lifecycle writes repeat root authorization after acquiring the workspace lock. |
| Financial snapshot drift | Amounts, balances, statuses, payloads, and copied results are not persisted. Current evidence is generated on every detail read and labeled separately. |
| Notes or secrets becoming a durable exfiltration channel | There is no notes field. Strict decoders reject unknown fields. Titles are short and reject control characters, markup, URLs, email-shaped values, and common credential markers. |
| Handoff race or stale ownership | Handoff requires the current optimistic version and open state inside a serializable transaction; ownership and audit commit together. |
| Recipient obtains broader evidence than authorized | Handoff only assigns the workspace. Recipient reads still require same tenant, investigation scope, root-domain scope, and object authorization. |
| Forged or arbitrary relationship graph | Relationships come only from the bounded server-derived rail. Allowed record/relationship patterns and UUIDs are revalidated before persistence. |
| Browser persistence or stale offline mutation | No local/session/IndexedDB storage is used. Writes are disabled offline and all reads use `no-store`. |
| Oversized response or history amplification | Request/response, open workspace, relationship, reference, and history counts have explicit limits and truncation signals. |
| Audit tampering | Lifecycle audit is written in the same database transaction; the API role has no audit update/delete grant. Metadata excludes title, query, record IDs, and recipient. |
| HTML/script injection in labels | Server and BFF enforce bounded safe title/text schemas; React renders text, not HTML; CSP remains unchanged. |

## Residual risks and operating requirements

- The repository has no tenant operator-directory service, so handoff cannot prove recipient existence at entry time. Operators must use an exact server-issued subject ID. A recipient receives nothing until authenticating in the same tenant with current scopes.
- A safe-title rule reduces accidental sensitive text but cannot semantically prove that every human label is non-sensitive. Operator training and audit review remain required.
- Closed workspace retention and legal-hold duration need managed compliance policy before production rollout. Rows are intentionally not deletable by the API role.
- Support-readonly can inspect workspace tables under the existing controlled database-support model; production access must remain monitored and time bounded outside standing application credentials.
- Current-evidence reads depend on the canonical search/relationship queries. A repository error yields unavailable, never empty or financially inferred state.
