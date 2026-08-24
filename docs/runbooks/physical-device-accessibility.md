# Physical-device and manual accessibility evidence

This runbook turns Phase 2 into a repeatable evidence exercise. It is written so
the reviewer can execute it without understanding the LedgerSync codebase. A
passing result means that a named person observed the behavior on the stated
physical device and retained tamper-evident evidence. Browser emulation is useful
for diagnosis, but it cannot pass this gate.

## Claim boundary

This procedure verifies the operator console on physical phones, a physical
tablet, a laptop, and a wide desktop. It tests whether exact financial meaning,
focus, assistive technology, network failure, and retry safety survive the real
device environment. It does not approve accounting terminology, legal posture,
cloud security, or partner traffic.

Do not run this procedure against customer or production data. Use only the
disposable demo tenant or an explicitly approved isolated pilot test tenant.

## Roles and prerequisites

One named reviewer owns one complete run. Additional people may operate devices,
but each device row still records who observed and approved it.

Before creating a run, confirm all of the following:

- the tested Git commit is clean, pushed, and has green required CI;
- the target is the isolated LedgerSync demo/pilot environment for that commit;
- the target URL contains no token, password, query string, or fragment;
- the reviewer can access an iPhone-class device, Android phone, tablet, laptop,
  and wide desktop, or an authorized physical-device farm providing those
  devices;
- the reviewer can control network conditions and record the point at which the
  transfer request leaves the device;
- an approved evidence store can retain recordings and notes for the agreed
  pilot evidence period;
- recording is permitted and notifications, passwords, cookies, tokens, and
  unrelated personal data cannot appear in captured media.

If any item is missing, record the row as `BLOCKED`; do not substitute an
emulator, memory, or verbal confirmation.

## Create a commit-bound run package

From the repository root, with no tracked working-tree changes:

```powershell
go run ./cmd/device-evidence create `
  -reviewer "Reviewer full name" `
  -target-url "https://isolated-pilot.example.com"
```

The command creates an ignored directory under `.tmp/device-evidence/` with a
unique UTC run ID, `manifest.json`, and `checklist.md`. It binds the exercise to
the full Git SHA and rejects placeholder reviewers, credential-bearing URLs, and
dirty tracked files. The `.tmp` directory is local working material; do not add
it to Git.

Before testing, validate the untouched draft:

```powershell
go run ./cmd/device-evidence validate `
  -manifest ".tmp/device-evidence/<run-id>/manifest.json" `
  -mode draft
```

## Fixed, reversible financial fixture

Every device uses the same exact movement so results can be compared:

| Field | Forward transfer | Compensating transfer |
|---|---|---|
| Amount | `INR 1.23` (`123` minor units) | `INR 1.23` (`123` minor units) |
| Debit | Operating Reserve, `10000000-0000-4000-8000-000000000001` | Vendor Payables, `10000000-0000-4000-8000-000000000004` |
| Credit | Vendor Payables, `10000000-0000-4000-8000-000000000004` | Operating Reserve, `10000000-0000-4000-8000-000000000001` |
| Currency | `INR` | `INR` |

The compensating movement restores the net balances but does not erase history.
Both transfers remain immutable ledger evidence. Never call the compensation a
database rollback or delete either record.

Before each device row, record both balances and their versions. After the
forward movement, record the posted transfer ID, journal ID, exact source and
destination changes, visible balance versions, and transaction-history row.
After the compensation, verify the two account balances return to their recorded
starting values and the history contains both immutable movements.

For a rendering boundary check, enter `92233720368547758.07` in the exact-money
field, open review, verify the complete value is visible and readable, then
cancel. Never submit this boundary value; the pilot transfer policy should reject
it.

## Required device rows

Record exact model, OS build, browser and version, CSS viewport, orientation,
locale, display/text scaling, input method, and assistive technology. “Latest,”
“mobile,” or “Chrome” is not sufficient evidence.

1. **iPhone-class physical device:** Safari plus VoiceOver; portrait and
   landscape; browser chrome, safe areas, virtual keyboard, touch, and back
   navigation.
2. **Android physical phone:** Chrome plus TalkBack; portrait and landscape;
   browser chrome, virtual keyboard, touch, and system back behavior.
3. **Physical tablet:** the platform browser and screen reader; portrait,
   landscape, split view where supported, touch, virtual and hardware keyboard.
4. **1366-class laptop:** a 1366×768-class viewport; keyboard-only journey and a
   platform screen reader such as NVDA on Windows or VoiceOver on macOS.
5. **Wide desktop:** a 1920×1080-class viewport; keyboard-only journey and the
   platform screen reader used by the supported desktop environment.

If the pilot support matrix chooses a different browser/assistive-technology
combination, record that decision and add it; do not silently replace a required
row.

## Common journey on every device

Perform the following in order while recording:

1. Begin with a new authorized session. Verify tenant, environment, and operator
   identity appear before account balances or transfer controls.
2. Open primary navigation. Confirm the drawer/panel does not cover its close
   control, dismiss it, and confirm focus returns to the trigger when a hardware
   keyboard is available.
3. Open Accounts, search for `Operating`, apply a status/category filter, open
   Operating Reserve, copy the complete account identifier, go back, and confirm
   filter, scroll, and focus context remain understandable.
4. Read balance, ledger balance, version, last-updated evidence, and transaction
   history. Confirm an unavailable section would not be announced as zero or
   empty.
5. Open Transfers. Select the fixed source and destination and enter `1.23`.
   Rotate, resize, or enter split view before review. Confirm the draft survives.
6. Open review. Read source, destination, currency, exact amount, and the fact
   that this is an internal transfer. Cancel once and confirm nothing was posted.
7. Repeat, confirm the forward transfer, and record the confirmation, transfer
   ID, journal evidence, immediate balance, and transaction-history visibility.
8. Open transfer detail. Read posted financial state separately from downstream
   notification/delivery state. Copy the complete identifiers.
9. Post the fixed compensating transfer and verify both net balances match their
   pre-row values while both history entries remain.
10. Enter the signed-64-bit boundary display value, review without submitting,
    verify no clipping or rounding, then cancel.
11. Increase browser zoom or OS text size to 200%. At a 1280 CSS-pixel desktop
    width also test 400% zoom, which exercises a 320 CSS-pixel reflow boundary.
    Confirm all evidence and actions remain reachable without two-dimensional
    page scrolling.
12. Run the journey with the platform screen reader. Confirm headings, landmarks,
    field labels, amounts with currency, status text, error text, copy feedback,
    and the retry action are announced in a meaningful order without relying on
    color or motion.

On laptop and desktop, repeat the core path using only Tab, Shift+Tab, Enter,
Space, arrow keys where native controls require them, and Escape. A visible focus
indicator must never disappear behind sticky content or a drawer.

## Four network profiles

Use device-farm network controls, an approved test proxy, or OS/network controls.
Record the tool and measured profile. Do not enable a production-only debug
endpoint or modify financial records directly.

### 1. Normal

Use the normal test connection. The forward transfer must confirm, the immediate
balance must show the exact movement, and history/detail must identify the same
transfer. Complete the compensating transfer.

### 2. Slow

Apply at least 400 ms round-trip latency with bandwidth no faster than 1 Mbps
down/512 Kbps up, or use the closest named device-farm profile and record its
actual values. During load and submit, confirm the UI shows progress, prevents
duplicate clicks, and never describes a pending request as posted. Remove the
profile, confirm the result, and compensate.

### 3. Offline before submit

Prepare the `INR 1.23` draft, disconnect the device before pressing the final
submit action, and wait for the offline state. The write action must be disabled
or safely refused, the UI must explain that fresh confirmation is unavailable,
and no new transfer may appear after reconnection. A failed network call is not
evidence that money moved.

### 4. Lost response after submit

This is the critical retry-safety proof:

1. Start request/network telemetry and prepare the fixed forward transfer.
2. Confirm once. Use telemetry to prove the POST left the device, then interrupt
   only the return path before the response reaches the browser. If the tool
   cannot establish this ordering, mark the profile `BLOCKED` rather than PASS.
3. Observe “Result not yet confirmed” or equivalent unknown-outcome copy. The UI
   must not claim success or failure.
4. Reconnect. Use only **Retry same transfer**; do not recreate or edit the draft.
5. Record the replay response, one transfer ID, one journal, one source debit,
   one destination credit, and one visible history item for the forward intent.
6. Retry the same action once more if the UI permits. It must return the same
   committed result without a second movement.
7. Complete the separate compensating transfer and verify net balances.

## Evidence files and privacy

For every device, retain at least:

- `<run-id>_<device>_journey-recording.*`;
- `<run-id>_<device>_retry-recording.*`;
- `<run-id>_<device>_accessibility-notes.*`.

The recording must show the device identity/settings, tested URL hostname,
journey, relevant network control, visible result, and UTC time. Notes must record
screen-reader announcements and keyboard/focus observations that video alone
cannot prove.

Upload artifacts to the approved evidence store. In `manifest.json`, record each
immutable credential-free HTTPS URL, SHA-256 digest, UTC capture time, and
retention-until date. Compute the digest from the final uploaded bytes where the
store supports it, or from the exact local file before upload:

```powershell
(Get-FileHash -Algorithm SHA256 -LiteralPath '<evidence-file>').Hash.ToLower()
```

Never commit recordings, cookies, tokens, private URLs containing signatures,
personal notifications, customer data, or unredacted logs. If access-controlled
evidence uses an expiring URL, store a stable object identifier plus the approved
retrieval procedure rather than the signed query string.

## Defects and retest

Give every defect an ID, severity, status, affected device/profile/journey, clear
reproduction, expected result, observed result, and evidence reference.

- Critical or high defects are fixed immediately and receive automated
  regression coverage where reproducible.
- A closed defect requires a physical retest URL in the manifest.
- Medium/low issues may remain only if the named product/accessibility owner
  explicitly accepts them outside this technical evidence; that acceptance is a
  separate decision reference, not an implied PASS.
- Any reconciliation mismatch, duplicate movement, wrong currency/amount,
  inaccessible confirmation, or retry that creates a second transfer is
  stop-ship.

## Validate and close the run

After all observed fields are recorded, set every passing device, journey, and
network result to `PASS`, add evidence and defect/retest references, add each UTC
`completed_at`, and set the overall status to `PASS`. Then run:

```powershell
go run ./cmd/device-evidence validate `
  -manifest ".tmp/device-evidence/<run-id>/manifest.json" `
  -mode complete
```

The validator fails missing device classes, profiles, journeys, physical metadata,
assistive-technology observations, required artifact types, HTTPS references,
digests, retention dates, completion times, or open critical/high defects.

Only after validation passes should the reviewer add the run ID, immutable
evidence reference, reviewer, date, and final result to
`docs/release-evidence/ui-device-matrix.md`. `T094`, `TASK-013`, and gate `G-020`
remain open until all five rows meet this exit rule.
