# Operator capability, role, and environment matrix

**Status:** Implemented browser-safe presentation model; production role approval remains an external release gate.

## Context

LedgerSync receives roles from the identity provider and maps them to scopes in the server-owned session. The browser receives only the bounded scopes and the verified environment needed to decide what to make discoverable. Hiding a link is never treated as authorization: every direct route, BFF endpoint, and private API operation must enforce its own boundary.

## Decision rules

- A missing read capability removes the corresponding navigation link and prevents the page controller from starting its protected request.
- A read-only operator may open a readable domain route, but mutation controls remain absent or disabled according to that route's server-issued write scope.
- Local Status and local onboarding require both `environment=local` and the relevant `local:*` scope.
- Demo/local identity is rejected by server configuration in production-like environments.
- Production Administration remains unreleased and absent. An invented browser scope cannot enable it.
- Direct URLs remain safe independently of navigation. A valid session without the required scope receives an explicit denied state and no protected request; APIs still reject unauthorized calls.

## Capability matrix

| Product capability | Server-issued scope evidence | Navigation or UI result | Direct/API boundary |
|---|---|---|---|
| Read accounts | `accounts:read` | Accounts and authorized overview evidence are discoverable | Account BFF endpoints re-check the scope |
| Create/change account lifecycle | `accounts:write` | Account command controls and eligible setup actions are available | Mutation boundary checks scope, CSRF, idempotency, and version evidence |
| Read/post transfers | `transfers:read`, `transfers:write` | Transfer evidence and eligible commands are shown independently | Transfer BFF routes enforce the relevant read/write scope |
| Read/write/approve funding | `funding:read`, `funding:write`, `funding:approve` | Funding is readable; commands and Approval entry are capability-specific | Funding boundaries preserve separation of duties and posting evidence |
| Read/write/approve/post corrections | `corrections:read`, `corrections:write`, `corrections:approve` | Corrections and Approval entry appear only for relevant duties | Correction routes independently enforce each command scope |
| Read/run reconciliation | `reconciliation:read`, `reconciliation:write` | Reconciliation evidence and run controls are separated | Reconciliation BFF routes re-check scope and retry identity |
| Read events | `events:read` | Events & Webhooks is discoverable | Event BFF requests are never started without the read scope |
| Read/manage/replay webhooks | `webhooks:read`, `webhooks:write`, `webhooks:replay` | Events & Webhooks is discoverable for webhook operators | Endpoint reads and two-operator replay commands independently enforce scope, tenant, recent authentication, CSRF, rate, and retry identity |
| Read recovery evidence | `recovery:read` | Recovery appears under Platform | Recovery BFF re-checks scope and returns bounded evidence only |
| Read developer metadata | `developer:read` | Developer appears under Platform | Contract metadata/download endpoints re-check scope |
| Read local diagnostics | `local:read` plus local environment | Local Status appears under Environment | Diagnostics BFF verifies the session is the configured local identity |
| Update local onboarding | `local:write` plus local environment | Eligible onboarding confirmations can be saved | Preference mutation independently checks local write authority |
| Manage production administration | Not released | Administration is absent | `/admin` remains non-disclosing; the proposed personas, state machines, four-eyes rules, and external gates are defined in [the blocked administration boundary](../security/administration-boundary.md) |

## Navigation model

- **Work:** Overview, Accounts, Funding, Transfers, Approvals.
- **Investigate:** Corrections, Reconciliation, Events & Webhooks.
- **Platform:** Developer, Recovery.
- **Environment:** Local Status in a verified local session only. Administration is intentionally absent.
- **Utility:** Guide remains available to every authenticated operator because it does not fetch protected financial evidence.

Empty navigation groups are not rendered. The compact drawer retains its existing dialog, focus trap, Escape handling, focus restoration, and inert background behavior after filtering.

## Role examples for review

| Representative responsibility | Expected capability shape |
|---|---|
| Read-only financial auditor | Financial read routes only; no mutation or approval controls |
| Funding preparer | Funding read/write without approval unless separately granted |
| Independent finance approver | Funding or correction approval plus the minimum evidence reads required by policy |
| Reconciliation operator | Reconciliation read/write and the supporting evidence reads approved for the role |
| Delivery operator | Event and/or webhook scopes without financial posting authority |
| Local developer/operator | Local diagnostics, developer, recovery, and explicitly issued financial scopes in development only |

## Required approval before production use

Product, Operations, and Security must approve the final identity-provider role-to-scope assignments. That external approval is deliberately not represented as complete by this implementation. Any future Administration capability requires a separate privileged session contract, server authorization boundary, threat review, and release evidence.
