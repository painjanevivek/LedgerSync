# Implementation plan: Provider-led outbound payouts

## Architecture

The Go API remains the financial command boundary. PostgreSQL owns payout,
reservation, approval, provider-attempt, webhook-event, reconciliation, journal,
outbox, and audit state. A worker delivers provider commands only after the
request/approval transaction commits. Redis remains disposable.

## Delivery order

1. Establish schema and domain invariants with test-first migration coverage.
2. Build the fake provider adapter and request/reservation flow.
3. Add dual-control approval and durable dispatch.
4. Add authenticated result processing, settlement/release logic, and
   reconciliation.
5. Add operator and BFF journeys only after private contracts are proven.
6. Attach physical-device, managed-provider, finance, legal, and pilot evidence
   only when the named external gate completes.

## Constraints

- No float values, provider calls inside transactions, editable financial
  history, or customer-controlled settlement.
- Payouts do not reuse transfer IDs or transfer records.
- The first adapter is deterministic and non-money-moving; production provider
  integration is a separately reviewed gated implementation.
