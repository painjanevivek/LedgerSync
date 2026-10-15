# ADR-003: Core capabilities and the optional Systems Lab

Date: 14 October 2026. Status: design accepted; runtime changes deferred to R05/R07/R11.

Keep Accounts, Funding, Transfers, Approvals, Corrections, Internal Reconciliation, Events/Webhooks, Recovery, Search, and Developer in the core portfolio scope.

Existing live investigation already has a default-off server setting, a Next.js boundary, and denial tests. R11 must verify and consolidate those boundaries. It must not add a duplicate feature flag. The study scheduler also already requires explicit enablement. The normal configuration keeps the scheduler off.

Systems Lab presents samples and historical evidence. Failure orchestration stays in a host-only runner. Never give browser requests arbitrary Docker, shell, file, network, or container control. Isolated run identity and synthetic data are mandatory before fault injection.

The server owns feature availability; the session and scopes own user authority. A visible menu item proves neither. Disabled collaboration must block direct room, join, signalling, and presence calls. Preserve old room evidence and normal unknown-transfer recovery.

Payout provider code is a simulation. Keep it outside default claims. Administration deliberately returns not-found. No privileged admin feature is added by this plan.

Supported future combinations are core, core plus Systems Lab, and core plus explicitly enabled collaboration. Validate combinations rather than implying every flag combination is supported.

No current feature is disabled or removed by R00/R01. R11 owns runtime changes and dependency review.
