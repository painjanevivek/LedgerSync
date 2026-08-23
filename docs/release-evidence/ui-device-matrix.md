# LedgerSync UI device matrix

Automated viewport coverage is recorded by `web/tests/e2e/responsive.spec.ts`. Real-device rows remain a release gate and must not be marked complete from emulation alone.

| Device | OS/browser | Journey | Result | Defect/retest | Reviewer/date |
|---|---|---|---|---|---|
| iPhone-class physical device | Pending | navigation, account, transfer, retry, rotation, offline | Pending external device | Required before pilot | Unassigned |
| Android phone physical device | Pending | navigation, account, transfer, virtual keyboard, offline | Pending external device | Required before pilot | Unassigned |
| Tablet physical device | Pending | portrait/landscape, filters, evidence tables/cards | Pending external device | Required before pilot | Unassigned |
| 1366-class laptop | Automated 1366×768 Chromium complete | full operator journey | Automated pass required in CI | Manual browser review pending | Unassigned |
| Wide desktop | Automated 1920×1080 Chromium complete | full operator journey | Automated pass required in CI | Manual browser review pending | Unassigned |
