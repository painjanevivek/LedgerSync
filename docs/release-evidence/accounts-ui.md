# Phase 6 — overview and account investigation evidence

- Overview separates customer funds from operating-controlled balances and states the aggregation rule.
- Account directory searches authorized name, account ID, and external reference, and filters account status without expanding tenant scope.
- `/accounts/{accountId}` identifies the selected account and shows status, category, external reference, exact balance, version, authoritative timestamp, and ledger history.
- Frozen accounts explain their operational impact and are excluded from transfer selectors.
- Desktop uses a comparison table; compact view uses evidence cards from the same data and semantic component tree.
- Failed balance refresh never presents an older value as current.

Automated evidence: unit financial-semantics tests, axe scan, overflow checks, and viewport journeys at 390×844 through 1920×1080. Physical-device review remains recorded as an external gate in `ui-device-matrix.md`.
