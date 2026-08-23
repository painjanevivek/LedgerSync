# LedgerSync UI device matrix

Automated viewport coverage is recorded by `web/tests/e2e/responsive.spec.ts`.
Real-device rows remain a release gate and must not be marked complete from
emulation alone. A reviewer must attach a screenshot/video or trace reference,
record every defect, and sign the retest row; verbal confirmation is not enough.

| Device | OS/browser | Journey | Result | Defect/retest | Reviewer/date |
|---|---|---|---|---|---|
| iPhone-class physical device | Pending | safe area; drawer/focus; 44px targets; exact-money keyboard; rotate with draft; offline/same-key retry; copy full IDs; 200% zoom | BLOCKED — physical device/reviewer required | Required before pilot | Unassigned |
| Android phone physical device | Pending | browser chrome; drawer/back; 44px targets; numeric keyboard; rotate with draft; offline/same-key retry; long values | BLOCKED — physical device/reviewer required | Required before pilot | Unassigned |
| Tablet physical device | Pending | portrait/landscape; filters; cards/tables; touch; keyboard; split-view/reflow; slow network | BLOCKED — physical device/reviewer required | Required before pilot | Unassigned |
| 1366-class laptop | Automated 1366×768 Chromium complete | keyboard-only account investigation, transfer/retry, evidence copy, zoom/reflow | PASS automated; manual screen-reader/browser review pending | Manual review required | Unassigned |
| Wide desktop | Automated 1920×1080 Chromium complete | complete operator journey, long identifiers, progressive 100-row history | PASS automated; manual visual review pending | Manual review required | Unassigned |

## Required test procedure for every physical row

1. Start from a new authorized session and verify tenant/environment context is
   visible before financial evidence.
2. Open navigation using touch, dismiss with its visible control and Escape when
   a hardware keyboard exists, and confirm focus returns.
3. Search accounts, open detail, copy a full identifier, return, and verify
   filter/scroll/focus context.
4. Enter an exact amount, rotate or resize before confirmation, review source,
   destination, currency, exact amount, and idempotency intent, then cancel.
5. Simulate offline before submit and a lost response after submit. Verify writes
   disable offline and the unknown outcome offers only same-key retry.
6. Exercise 200% zoom/text sizing, long signed-64-bit amounts, virtual keyboard,
   portrait/landscape or split view, and constrained network.
7. Record device model, OS build, browser/version, viewport, locale, evidence URL,
   defects, retest result, reviewer, and UTC date.

## Exit rule

T094 and TASK-013 stay open until all five rows have manual evidence and no open
critical defect. Emulation and automated screenshots may locate defects, but
cannot satisfy the physical-device claim.
