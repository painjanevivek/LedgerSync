# First ledger journey

R05 keeps the existing console style and all route URLs. Work contains Overview, Accounts, Funding, Transfers, and Approvals. Investigate contains Search, Corrections, and Internal reconciliation. Operations contains permitted local health, Events and webhooks, and Recovery. Developer contains the existing developer workspace. Help stays in the always visible Guide utility. This avoids two copies of the same tutorial link. Developer reference sections stay inside their existing page. R11 owns the optional Systems Lab boundary.

## Complete the first transfer

1. Open `/accounts/new`. Create two synthetic accounts in the same currency. Give each account a different reference. Both start at zero.
2. Open `/funding`. Use synthetic payment and evidence references in the disposable demonstration. A record is not proof of an external bank payment.
3. Ask a different permitted actor to review the record in `/approvals`. Review does not post funds. Preserve the configured review and step-up rules.
4. Open the approved record in `/funding`. Post once. Read the posted receipt and balance.
5. Open `/transfers`. Select the source and destination. Enter the exact amount. Read the review screen before confirmation.
6. Confirm once. If the result is unknown, preserve the request key and details. Check or retry that same intent. Do not create a second intent to resolve a lost response.
7. Open the receipt. Inspect the journal and both account entries. Read event delivery separately.
8. Run internal reconciliation. Its result covers a recorded snapshot. It does not prove bank settlement.

The Overview shows the next safe action after successful authorized reads. One account causes a request to create the destination. Read failure, offline state, and a reconciliation warning take priority. The Guide owns the full explanation and glossary. Optional orientation evidence remains a separate operator record; dismissing it does not prove completion.

## Balance meanings after reservations

Posted ledger equals opening funds plus immutable journal postings. Reserved funds equal active internal holds. Available funds equal posted ledger minus reserved funds. A deadline does not release funds until a terminal database transaction commits. An insufficient-funds response refers to available funds.

A transfer receipt keeps the balances saved with that original result. Its posted ledger can differ from its available amount when another hold remains active. A historical receipt without separate fields does not prove current available funds. Read the account for a new authoritative value. External statement comparison reviews supplied synthetic evidence; it cannot move money.

## Compatibility and verification

No route, filter, cursor, return URL, request key, monetary representation, or permission changed. Existing unavailable, historical, unknown, denial, and independent approval states remain. Navigation uses existing disclosure controls, keyboard behavior, focus restoration, styles, colors, and type. Tests cover desktop and compact navigation, role visibility, history, first-run text, review, unknown outcomes, reduced motion, zoom-equivalent reflow, and automated accessibility. These checks do not replace physical-device or human assistive-technology review.
