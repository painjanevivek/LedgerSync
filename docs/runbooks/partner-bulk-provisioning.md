# Bounded partner provisioning

`cmd/provision-partner` is an internal, audited tool for an approved design
partner. It is not a public signup API and it never accepts raw credentials.

## Limits and input

- One reviewed JSON configuration creates one tenant with 1–10,000 accounts.
- The configuration uses the selected pilot currency (currently INR), canonical
  integer minor-unit values, reviewed account categories, server-owned subject
  grants, and external credential references only.
- Validate the file before any mutation:

```powershell
go run ./cmd/provision-partner -action validate `
  -config docs/pilot/provisioning-example.json -pilot-currency INR
```

The command prints the immutable request fingerprint. Store that fingerprint,
the reviewed configuration location, and the approval reference outside the
repository's source tree.

## Apply and rollback

Only a trusted operator runs `apply`, with a new correlation UUID. Apply writes
the tenant, policies, accounts, permissions, credential references, and audit
records transactionally. If the onboarding must be cancelled before use, run
the explicit rollback with the same approved operator identity and a new
correlation UUID. Do not delete rows manually.

## Safety checks

1. Confirm the partner contract, tenant ID, currency, limits, roles, and
   credential references are approved.
2. Validate the file and review its fingerprint.
3. Apply once; do not run concurrent applies for the same tenant.
4. Confirm account ownership, policy, audit records, and zero/unambiguous
   opening balances through the API and reconciliation views.
5. Record support contacts and offboarding conditions before sharing any
   credential reference with the identity provider owner.
