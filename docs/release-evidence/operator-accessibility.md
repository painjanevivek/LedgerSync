# Phase 3 operator accessibility and interaction evidence

## Scope

This evidence covers the authenticated overview, account directory/detail, transfer workspace/detail, and reconciliation list/detail. It proves deterministic browser behavior; it does not substitute emulation for the Phase 6 physical-device gate or claim that automated rules discover every experiential barrier.

## Automated evidence

| Control | Evidence | Expected result |
|---|---|---|
| WCAG A/AA rules | Axe scans on every populated MVP route, including WCAG 2.2 AA tags | Zero detected violations |
| Keyboard and focus | Skip navigation, compact menu Enter/Escape/restoration, transfer review focus, keyboard confirm, retry action | Logical focus; unknown result announced as status |
| Retry safety | Two keyboard submissions after an unknown response | Same non-empty idempotency key on both requests |
| Exact money | Maximum signed-64-bit minor-unit amount entered as decimal text | Exact `USD 92233720368547758.07`; no floating point or truncation |
| Rotation / virtual keyboard contract | `inputmode=decimal`, portrait → landscape → portrait | Input state preserved; no horizontal page overflow |
| Zoom and reflow | 200% browser zoom plus 320 CSS-pixel boundary | Financial evidence and actions remain reachable |
| Increased text spacing | WCAG line, paragraph, letter, and word-spacing override | Account evidence remains visible with no page overflow |
| Forced colors / reduced motion | Chromium media emulation plus authored-color axe rerun | Status remains operable; zero detected authored-color violations |
| Touch target | Compact menu and primary transfer actions | At least 44 × 44 CSS pixels |
| Independent truth states | Balance succeeds while history fails; balance and history fail independently | No failure is relabeled as empty or current |

Run from `web/`:

```text
npm run lint
npm run build
npx playwright test tests/e2e/accessibility.spec.ts tests/e2e/transfer.spec.ts tests/e2e/account-directory.spec.ts --workers=1
```

## Manual review checklist

- Read the unknown-outcome message without color: it states that the result is unconfirmed and instructs the operator to retry the same transfer.
- Confirm posted ledger status and downstream delivery status are announced as separate text.
- Confirm full identifiers remain copyable even where visual rows use abbreviated links.
- Confirm error, offline, denied, empty, and mismatch copy has a distinct semantic role and does not infer a zero balance or passing control.
- Confirm compact navigation closes with Escape and returns focus to the menu trigger.
- Confirm no motion is required to understand balance, transfer, or reconciliation state.

## Known limitations and external gate

- Browser emulation cannot validate VoiceOver/TalkBack pronunciation, OS browser chrome, real safe-area insets, physical touch accuracy, or virtual-keyboard resizing. These remain explicitly pending in the physical-device matrix.
- Axe cannot prove accounting language is correct. Aggregate ownership and status terms remain subject to finance/operations approval.
- The local demo identity is intentionally non-production. Managed OIDC keyboard and screen-reader behavior must be repeated with the selected provider.
