# Phase 12 — controlled pilot gate register

Phase 12 is an operating rollout, not a code-only phase. Engineering has prepared the local candidate, but the following approvals are required before the first external design partner:

| Gate | Required owner | Current state |
|---|---|---|
| Jurisdiction, currency, and custody positioning | Legal/compliance/product | Pending |
| Licensed banking/payment partner boundary if regulated funds are handled | Legal/compliance | Pending if applicable |
| Production OIDC tenant, role matrix, and credential rotation | Security/partner engineering | Pending |
| Managed PostgreSQL PITR configuration and isolated restore drill | SRE/security | Pending external environment |
| Finance approval of balance categories and reconciliation evidence format | Finance/controller | Pending |
| Physical-device accessibility and responsive matrix | Product/design/accessibility | Pending devices |
| 10–50 TPS production-like load and 2× headroom evidence | Engineering/SRE | Pending managed test environment |
| Design-partner support contacts, limits, incident path, and rollback criteria | Product/operations | Pending partner selection |

No code commit can truthfully close these gates. The rollout order remains internal demo → internal production-like synthetic tenant → one limited partner → second/third partner after stable reconciliation and support evidence.
