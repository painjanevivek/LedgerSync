# Simple-first operator usability protocol

## Participants

Recruit five business or finance operators who understand money workflows but do not need to know LedgerSync internals. Do not coach product vocabulary during the tasks.

## Test setup

- Use a local test tenant with non-sensitive fixture data.
- Start every participant in Simple view with a fresh tenant/operator preference.
- Include one uncertain transfer, one review task, two accounts, and a completed transfer.
- Record task completion, assistance, wrong turns, unsafe retry intent, confidence, and difficulty.

## Tasks

1. Find the usable balance and tell the moderator how fresh it is.
2. Add money to an account and explain the expected financial effect before confirming.
3. Make a transfer and verify its result.
4. Inspect the uncertain transfer and explain what should happen next.
5. Find and complete the review task.
6. Switch to Expert view, locate the technical evidence, and copy the full record identity.

## Safety prompts

After task four, ask: “Did the transfer complete, and would you create another transfer now?” The only acceptable understanding is that completion is unknown and the existing transfer must be checked or safely retried with its existing request identity.

## Success thresholds

- At least four of five operators complete every task without assistance.
- All five correctly understand the uncertain outcome and avoid an unsafe new transfer.
- Median confidence is at least 4/5.
- Median difficulty is no more than 2/5.
- Any terminology or navigation confusion repeated by two or more participants is fixed and retested.

## Evidence record

For each participant, record only participant code, role category, task result, assistance count, observed confusion, confidence, and difficulty. Do not record account identifiers, balances, credentials, or other customer data.
