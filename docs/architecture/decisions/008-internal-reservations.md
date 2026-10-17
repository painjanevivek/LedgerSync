# ADR 008 — Internal full-value reservations

Status: selected; implementation and verification pending. R16 follows the R15 source work. Docker stays stopped until the code changes are ready.

A reservation holds spendable funds for one same-tenant, same-currency internal destination. It is not a journal or external authorization. Version 1 supports full capture, full release and database-clock expiry only. It excludes partial capture, FX, cards, external settlement and renewal.

`ledger = opening + immutable posting delta`. `reserved = sum(amount of active holds)`. `available = ledger - reserved`. All three use exact minor-unit integers. A hold remains active after its deadline until a controlled expiry transaction releases it. Capture is forbidden at or after the deadline. A delayed worker never makes expired funds spendable by assumption.

Create checks the current actor's debit permission, destination credit permission, active customer accounts, tenant, currency and spendable funds. It reserves funds and advances the source balance version once. Release and expiry return the reservation and advance that version once. They produce no journal. A frozen account can release or expire a hold.

Capture removes only its own reservation inside a database subtransaction and calls the existing controlled transfer function. That function rechecks current account status, authorization, amount policy and velocity. Success posts one balanced journal and advances the two affected balance versions once. The temporary release is invisible outside the transaction. Any business denial rolls back that subtransaction. The hold and funds stay reserved. The outer transaction records a durable rejected command receipt. Infrastructure and audit failures roll back the entire command.

All hold commands bind tenant, actor, operation, key and full logical intent. Original receipts are immutable. A current hold read is separate. A replayed creation receipt can say `active` after a later capture; that is the original outcome, not current state. A terminal transition can win once. Different terminal attempts after that state return conflicts.

Hold evidence is tenant scoped and append only where applicable. Ordinary API/worker roles cannot write it directly. The existing inert financial function owner owns controlled financial changes. A separate expiry queue permits bounded global discovery only through a worker-only claim function. Each claim has an owner, generation and database-clock lease. Completion checks all three under lock. Capture, release and expiry use the same account lock order.

Existing financial commands already change posted and available values by the same delta. They therefore preserve other holds. Corrections use spendable funds. Account closure requires both ledger and available to be zero; this also rejects any positive active reservation. Reconciliation must independently sum active holds and compare the available equation. Tests must cover these claims before qualification.

A balance event with different ledger and available values uses version 2 and carries exact ledger and reserved values. Historical version 1 implies no reservation. The consumer supports both. An old binary that assumes equality is not a safe rollback while any hold or version-2 event remains. Use a compatible image or a forward repair.
