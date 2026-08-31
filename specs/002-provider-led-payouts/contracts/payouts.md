# Payout contract outline

All money/version values are decimal strings. Every mutation requires an
idempotency key and correlation ID.

- `POST /payouts`: creates a requested/reserved payout.
- `GET /payouts`, `GET /payouts/{payoutId}`: tenant-authorized lookup.
- `POST /payouts/{payoutId}/approve`: two-person, recent-auth approval.
- `POST /payouts/{payoutId}/cancel`: requester/authorized safe cancellation.
- `POST /provider/{providerId}/payout-events`: provider-signed callback only.

Provider adapter contract: `CreatePayout`, `GetPayoutStatus`,
`VerifyProviderWebhook`, `FetchSettlementRecords`.
