# Research: new-user UX completion

## Decision: use server validation as the source of truth for required fields

**Rationale:** A visual label must match actual behavior. Calling a server-required financial field optional would cause failed submissions and confusion.

**Alternatives considered:**

- Mark every field required: rejected because search, filters, and optional notes would be misleading.
- Decide only in the frontend: rejected because the backend may still reject missing values.

## Decision: simplify wording without removing financial safety terms

**Rationale:** New users need simple actions first. Some terms remain necessary for approvals, corrections, and reconciliation because they describe real financial controls.

**Alternatives considered:**

- Remove all technical wording: rejected because it could hide approval or correction consequences.
- Keep current wording: rejected because it makes the first task harder to understand.

## Decision: use progressive disclosure for advanced details

**Rationale:** The primary task stays clear while detailed explanations remain available when needed.

**Alternatives considered:**

- Hide advanced controls entirely: rejected because operators still need them.
- Keep all detail visible: rejected because it overloads new users.

## Decision: do not make production claims from local demo behavior

**Rationale:** The repository uses a server-owned local single-operator policy. Production needs separate identity, approval, tenancy, and operational evidence.

**Alternatives considered:**

- Present local approval as production-ready: rejected because it would be false.
