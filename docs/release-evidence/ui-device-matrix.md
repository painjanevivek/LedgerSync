# LedgerSync UI device matrix

Automated viewport coverage is recorded by `web/tests/e2e/responsive.spec.ts`.
Real-device rows remain a release gate and must not be marked complete from
emulation alone. Follow the executable
[physical-device and manual accessibility runbook](../runbooks/physical-device-accessibility.md).
Use `go run ./cmd/device-evidence create` to generate the commit-bound run ID,
manifest, filenames, and checklist. A reviewer must attach immutable evidence,
record every defect, and sign the retest row; verbal confirmation is not enough.

| Device | OS/browser | Journey | Result | Run ID / immutable evidence | Defect/retest | Reviewer/date |
|---|---|---|---|---|---|---|
| iPhone-class physical device | Pending | safe area; drawer/focus; 44px targets; exact-money keyboard; rotate with draft; four network profiles/same-key retry; VoiceOver; full IDs; 200% zoom | BLOCKED — physical device/reviewer required | Pending | Required before pilot | Unassigned |
| Android phone physical device | Pending | browser chrome; drawer/back; 44px targets; numeric keyboard; rotate with draft; four network profiles/same-key retry; TalkBack; long values | BLOCKED — physical device/reviewer required | Pending | Required before pilot | Unassigned |
| Tablet physical device | Pending | portrait/landscape; filters; cards/tables; touch; keyboard; split-view/reflow; four network profiles; platform screen reader | BLOCKED — physical device/reviewer required | Pending | Required before pilot | Unassigned |
| 1366-class laptop | Automated 1366×768 Chromium complete | keyboard-only account investigation, transfer/retry, evidence copy, 200%/400% zoom/reflow, screen reader | PASS automated; manual screen-reader/browser review pending | Pending | Manual review required | Unassigned |
| Wide desktop | Automated 1920×1080 Chromium complete | complete operator journey, long identifiers, progressive 100-row history, four network profiles, screen reader | PASS automated; manual visual/accessibility review pending | Pending | Manual review required | Unassigned |

## Required test procedure for every physical row

1. Start from a new authorized session and verify tenant/environment context is
   visible before financial evidence.
2. Open navigation using touch, dismiss with its visible control and Escape when
   a hardware keyboard exists, and confirm focus returns.
3. Search accounts, open detail, copy a full identifier, return, and verify
   filter/scroll/focus context.
4. Enter the fixed exact amount, rotate or resize before confirmation, review
   source, destination, currency, amount, and idempotency intent, then cancel.
5. Execute normal, slow, offline-before-submit, and deterministically ordered
   lost-response-after-submit profiles. Verify writes disable offline and the
   unknown outcome offers only same-key retry.
6. Post the fixed compensating movement and verify net balances return while both
   immutable history entries remain.
7. Exercise 200% zoom/text sizing, 400% reflow, maximum signed-64-bit display,
   virtual keyboard, portrait/landscape or split view, and screen reader.
8. Record the manifest run ID, device model, OS build, browser/version, viewport,
   locale, immutable evidence URLs and SHA-256 digests, defects, retest result,
   reviewer, and UTC date.

## Exit rule

T094 and TASK-013 stay open until the complete manifest validator passes, all
five rows have manual evidence, and no critical/high defect remains open.
Emulation and automated screenshots may locate defects, but cannot satisfy the
physical-device claim.
