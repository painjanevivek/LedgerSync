# Local-product Phase 3 — configurable account UI assurance

**Result:** `PASSED`

**Gate:** [LPC-030](../pilot/local-product-completion-gates.md)

**Candidate binding:** Phase 3 working tree based on `381ba05`; this document, implementation, tests, and reviewed baselines are bound together by the resulting Phase 3 commit.

**Verified:** `2026-08-24T20:20:35Z`

## Delivered operator journey

The local operator can now complete the account lifecycle without bypassing LedgerSync's financial invariants:

1. Create an account through Identity, Financial boundary, Review, and Result stages.
2. See the server-canonical display name and lower-case external reference before submission.
3. Recover an unknown response by retrying the exact body with the same tenant-scoped idempotency key.
4. Fund the new account only through the existing transfer workflow from a different authorized active INR account.
5. Freeze and reactivate from account detail using the authoritative configuration `account_version` and a required audited reason.
6. Receive an authoritative non-zero close denial.
7. Return the account to exact zero through a second balanced transfer.
8. Close the zero-balance account using a refreshed account version, refreshed available and ledger balances, a reason, and typed full external reference.
9. Retain immutable transfer history, audit evidence, exact balances, and terminal closed status.

No account screen exposes opening balance, direct balance editing, posting editing, owner/tenant selection, or a second transfer implementation.

## Safety evidence

| Invariant | Evidence | Result |
|---|---|---|
| Exact money | Money and versions remain decimal strings in UI state, JSON, persistence, formatting, and assertions; the real journey moved exactly `100` INR minor units in each direction | `PASS` |
| Zero-start creation | Create accepts identity/category only and returns authoritative available and ledger values of `0` | `PASS` |
| Create idempotency | Same captured request and key returned HTTP 201 with `Idempotent-Replay: true` and the original account ID | `PASS` |
| Intent binding | Versioned tenant-scoped storage retains the canonical body/key only for an exact unknown-outcome retry; changed intent cannot reuse it | `PASS` |
| Funding boundary | Funding reuses `/transfers`, transfer review, exact parsing, CSRF, and transfer idempotency; no direct projection mutation exists | `PASS` |
| Double entry | PostgreSQL proof found two posted transfers, four immutable postings, and two balanced debit/credit pairs | `PASS` |
| Lifecycle concurrency | Freeze/reactivate/close use `account_version`, not balance projection `version`; stale versions refresh before a new intent | `PASS` |
| Zero-only close | Browser and API rejected the funded close; durable evidence accepted close only after both projections returned to exact zero | `PASS` |
| Auditability | One create, three successful status changes, and one denied close were retained with bounded sanitized reasons | `PASS` |
| Outbox atomicity | Durable proof found four account and four transfer outbox records; final pending/dead counts were both zero | `PASS` |
| Authorization | Account write and transfer write scopes are independent; inaccessible objects preserve non-disclosure behavior | `PASS` |
| Normal data preservation | The isolated project and volumes were removed, the supported normal project was restored, and its before/after financial fingerprint matched | `PASS` |

## UI, responsive, and accessibility evidence

- The Accounts directory exposes one `Create account` action only to `accounts:write` sessions and preserves directory query/selection context on return.
- Create, unknown outcome, replay, duplicate reference, validation, read-only, offline, and unavailable states keep entered context and announce truthful recovery.
- Lifecycle actions live on account detail, use a keyboard-modal confirmation, refresh account and balance evidence, and restore focus on close.
- Closed status is terminal while identity, balances, audits, and transfer history remain readable.
- Mobile is single-column, tablet preserves evidence order, and desktop uses the existing bounded document hierarchy.
- Controls retain 44-by-44 CSS-pixel targets; labels are permanent; errors are linked; focus transitions, reduced motion, forced colors, text spacing, rotation, 200% zoom, and 320 CSS-pixel reflow were covered.
- The established navy/emerald evidence hierarchy, restrained borders, tabular exact values, and canonical status grammar are retained. No gradients, glass effects, consumer-wallet language, or decorative metric grids were introduced.

## Automated verification

| Check | Result |
|---|---|
| `go test ./... -count=1` | all Go packages passed, including contract, fault, integration, system, and unit suites |
| `go vet ./...` | passed |
| `npm test -- --run` | 44/44 passed |
| `npm run lint` | passed |
| `npx tsc --noEmit` | passed |
| `npm run build` | passed; `/accounts/new` and all existing routes built |
| `npm run test:e2e` | 64/64 mocked browser, accessibility, responsive, and visual tests passed |
| `npm run test:visual` | 21/21 passed with no snapshot update |
| `npm run test:performance` | passed; 708,128 total JS bytes, 229,156-byte largest chunk, below 2,000,000/350,000-byte budgets |
| `scripts/test-account-product-journey.ps1` | passed in 40.34 seconds, including real browser, BFF, private API, PostgreSQL, worker, reconciliation, cleanup, and normal-project restoration |
| `git diff --check` | passed |

## Real-stack transcript

| Evidence | Value |
|---|---|
| Isolated Compose project | `ledgersync-acceptance-20260824201955-7686e97e` |
| Journey run | `p3-89bc3756c645` |
| Created/closed account | `910f0c10-6ed0-4f25-b52d-2d74cef5eeef` |
| Funding transfer | `74311f8f-55b0-4ca1-9292-976083a859b9` |
| Return transfer | `2f8c216a-537e-4129-98f3-d9cdbdd6ee9b` |
| Migration | `000013_account_lifecycle_commands.up.sql` |
| Final reconciliation | `matched`, 0 mismatches |
| Final outbox | 0 pending, 0 dead |
| Browser test | 1/1 passed in 3.0 seconds |
| Isolated cleanup | `PASS` |
| Normal project restore/fingerprint | `PASS` |

The isolated fixture identifiers are retained for traceability even though the acceptance database and volumes were safely deleted after verification.

## Reviewed visual manifest

Phase 3 approved the compact and desktop account-create review and lifecycle-confirmation baselines, plus intentional account-directory/detail baseline changes caused by the new primary and guarded actions. Windows baselines passed a clean 21-image comparison. Linux account baselines were regenerated by the pinned cross-platform visual workflow and remain commit-bound for CI comparison.

## Known local-only limitations

- This proves one Windows workstation and loopback-only Docker Compose, not external deployment or device-farm coverage.
- Demo identity is server-controlled; managed OIDC/SSO remains outside local-product completion.
- Only internal same-currency INR ledger movement is supported; bank rails, cards, FX, custody, and external settlement remain out of scope.
- The account lifecycle controls are operator workflows, not public end-user account-management APIs.

## Gate decision

LPC-030 passes. The account journey is exact, retry-safe, balanced, authorized, audited, responsive, accessible, and independently recoverable without direct balance mutation. LPC-040 may proceed.
