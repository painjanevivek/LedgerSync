# New-user UX completion plan

**Date:** 2026-08-29
**Status:** Ready to implement
**Scope:** Local operator console only

## Goal

Make LedgerSync easy for a first-time local user without weakening financial checks.

The user should understand what to do next, what each field means, and which fields are required.

## What this plan changes

- Use simple, action-based language on the console.
- Show `Required` only when the server rejects a missing value.
- Show `Optional` when the server accepts a missing value.
- Add a short example or explanation below fields that need one.
- Keep advanced operational details available, but do not show them before the primary task.
- Give a new user one clear path: create an account, add a funding record, review it, then make a transfer.

## What this plan does not change

- Exact-money handling, approval rules, ledger rules, authorization, audit records, or API contracts.
- Production identity, cloud infrastructure, legal approval, partner onboarding, or external payment providers.
- A server-required field cannot be made optional only in the UI.

## Current facts

- The Funding form already marks its four server-required fields as required.
- Other console forms do not yet have one shared required/optional rule.
- The console still contains advanced operator language on Transfers, Corrections, Reconciliation, Events, Recovery, Local Status, and Developer pages.
- The local workspace uses a server-owned single-operator policy. It is suitable for a local demo, not a production approval model.

## Implementation phases

### Phase 1 — Field and language inventory

**Purpose:** decide what is required before changing labels.

1. List every visible input, select, and textarea in the console.
2. For each field, record:
   - page and field name;
   - server validation rule;
   - `Required` or `Optional` status;
   - one short helper sentence, if needed.
3. Do not mark search, filter, date-range, and investigation fields as required unless the server requires them.
4. Do not change a field to optional unless the API and server validation safely accept an empty value.
5. Create a plain-language glossary for unavoidable terms: ledger, reconciliation, approval, correction, and webhook.

**Exit check:** every visible form control has an agreed required/optional status backed by code or contract evidence.

**Commit:**

```text
docs(ux) : define required fields and plain-language terms

- record each console field and its server-backed requirement
- define simple wording for financial and operational terms
- keep financial validation and approval rules unchanged
```

### Phase 2 — Shared form and copy rules

**Purpose:** make every form follow the same simple pattern.

1. Create reusable form label, helper-text, and error components/styles.
2. Every required field shows `Required` in the label and uses the native `required` attribute where appropriate.
3. Every optional field shows `Optional` in the label.
4. Every complex field gets a short example below it.
5. Error messages say what to fix, not internal system terms.
6. Keep labels, help text, errors, and focus order accessible to keyboard and screen-reader users.

**Exit check:** one shared pattern is used by account, funding, transfer, correction, and operational forms.

**Commit:**

```text
feat(ux) : standardize required and optional form fields

- add shared labels, helper text, and error treatment
- show server-backed required and optional states clearly
- preserve accessible validation and financial input safeguards
```

### Phase 3 — First-time user journey

**Purpose:** make the first dashboard useful without demo records.

1. Keep the new-user dashboard focused on the next safe action.
2. Show the four steps in order:
   - Create an account
   - Add a funding record
   - Wait for or complete review
   - Make a transfer
3. Link each step to the real screen and show a plain reason when the next action is unavailable.
4. Keep guide content short and task-based. Put detailed explanations on the Guide page.
5. Do not show fake balances, fake approvals, or fake success states.

**Exit check:** a new local user can complete the first journey without needing to understand audit or infrastructure terms.

**Commit:**

```text
feat(onboarding) : guide new users through the first ledger tasks

- prioritize account setup, funding, review, and transfer steps
- link each step to its working console action
- keep advanced operations out of the first-task path
```

### Phase 4 — Simplify existing screens

**Purpose:** improve the pages users see after onboarding.

Apply the shared wording and field rules in this order:

1. Accounts and account creation
2. Transfers and transfer review
3. Funding details and approval actions
4. Corrections
5. Reconciliation
6. Events and Recovery
7. Local Status and Developer

For every page:

- lead with the user action and result;
- keep exact financial facts unchanged;
- move technical detail behind a `What does this mean?` disclosure or the Guide page;
- keep one dominant action per workflow stage;
- retain precise terms where an approval, correction, or irreversible action needs them.

**Exit check:** first-time pages use plain language; advanced pages are understandable without hiding important financial facts.

**Commit for each page group:**

```text
fix(ux) : simplify <page-group> for new operators

- replace internal wording with clear task-based language
- apply required and optional field labels
- preserve financial controls, accessibility, and existing behavior
```

### Phase 5 — Verification and release evidence

**Purpose:** prove the simpler UI still works safely.

1. Add tests that check each required/optional label against its real validation rule.
2. Test the new-user path with an empty local workspace.
3. Test validation errors, offline state, permission denial, unknown request outcome, and approval-required state.
4. Run lint, unit tests, build, responsive Playwright tests, accessibility tests, and visual regression tests.
5. Inspect changed visual snapshots before accepting them.
6. Test 390×844, 768×1024, 1024×768, 1280×800, 1440×900, 1920×1080, 2560×1440, 200% zoom, and 400% reflow.

**Exit check:** no page-level horizontal overflow, no broken keyboard flow, no weakened validation, and no UI claim that is unsupported by the server.

**Commit:**

```text
test(ux) : verify the new-user console journey

- test field requirement labels against server validation
- verify empty-workspace and recoverable error journeys
- confirm responsive, accessible, and reviewed visual behavior
```

## Order of work

Start with Phases 1 and 2. They prevent inconsistent required/optional labels. Then complete the new-user journey before rewriting advanced screens.

## Completion definition

This work is complete when a first-time local user can understand the primary journey, every field clearly says whether it is required or optional, and all financial checks remain unchanged.
