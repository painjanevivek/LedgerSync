# Payout data model

| Entity | Immutable facts | Mutable workflow facts |
|---|---|---|
| `payout_intent` | tenant, source, beneficiary, INR amount, fee, policy version, requester, idempotency fingerprint | state, version, terminal timestamps |
| `payout_reservation` | payout, source, reserved amount, created reason | released/settled timestamp only |
| `payout_approval` | payout, approver, authentication time, decision, reason | none |
| `provider_payout_attempt` | payout, provider reference, idempotency reference, request digest | transport state and response timing |
| `provider_webhook_event` | provider event ID, signature metadata, payload digest, received time | processing disposition only |
| `settlement_reconciliation` | payout/provider record keys, expected and observed exact values, checked time | resolved disposition only |

Reservation is distinct from ledger posting. It reduces authoritative spendable
balance while active. Settlement creates the journal. Releasing a reservation
does not create a compensating financial journal because none existed.
