# Concurrency and snapshot evidence

## Phase 6

Run `scripts/test-unit-study-concurrency.ps1` against the isolated two-API study.
See [request and database results](concurrent-transfers.json).
Ten competing debits posted. Ten debits were rejected. The source balance reached zero.
Ten opposite transfers posted. Two requests with one key returned one transfer.
Two simultaneous reconciliation commands returned a completed run or a controlled busy result.
The final ledger matched. Every journal had two balanced postings.

The financial path uses a tenant policy advisory lock before the serializable snapshot.
Account locks use a consistent identifier order. Serialization retries have a fixed limit.
One earlier drill produced `transaction_conflict_retryable`. The repeat driver records these responses and retries the same command at most three times.
This is database-coordinated mutual exclusion. It is not a fair distributed lock algorithm.

The first integration run lacked the provisioned `ledgersync_provisioning` role.
After role setup, all six selected concurrency checks passed. The test setup command now applies migrations and roles only to the test database at port 35432.
An earlier live driver used the wrong private account route. Another result check used the wrong database column. These runner failures did not change financial controls.

## Phase 7

The new test failed before the change because snapshot delivery counts were absent.
It then passed with concurrent transfers. Manual reconciliation and replay checks also passed.
Cache loss, forged cache versions, and unavailable Redis publication checks passed.
See [live snapshot and dependency report](snapshot-report.json).

The ledger, posting checks, account scope, schema version, watermark, and outbox counts use one PostgreSQL repeatable-read transaction.
The persisted report is historical evidence. Old reports can lack delivery counts.
Current Redis and inferred worker progress observations have separate collection times.
Worker progress is inferred from committed publication records. It is not a worker heartbeat.
These observations do not form a Chandy-Lamport global snapshot.

The live probe first used an incorrect balance route. A second run assumed parsed dates were strings. Both runner errors were corrected. Their logs remain in this directory.
