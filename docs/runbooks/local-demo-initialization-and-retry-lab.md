# Local initialization modes and retry lab

LedgerSync remains a one-workstation, loopback-only INR demonstration. The
server-controlled demo operator is available in both initialization modes;
only the initial ledger contents differ.

## Fresh initialization choices

`demo` is the default. It applies every migration and then runs the existing
deterministic, replay-safe INR seed. Use it for the guided product journey.

`empty` applies every migration but skips the deterministic seed. It is useful
for demonstrating account creation from an empty ledger. It does not remove or
zero existing accounts, transfers, journals, postings, or balances.

Choose `empty` only on a truly fresh PostgreSQL volume:

```powershell
.\scripts\start-local.ps1 -InitializationMode empty
```

The host records the selected mode in the protected local runtime state. Later
starts use that marker. A request to switch modes is rejected while the exact
Compose project's PostgreSQL volume exists.

To intentionally erase the local project and select the next fresh mode, use
the destructive host command with its exact confirmation:

```powershell
.\scripts\reset-local.ps1 `
  -Confirmation 'DELETE LEDGERSYNC LOCAL DATA' `
  -InitializationMode empty
.\scripts\start-local.ps1
```

That reset deletes the exact project's containers and PostgreSQL/Redis named
volumes. Create and validate a backup first if the data matters. The browser
cannot invoke reset, reseed, Docker, shell, or initialization-mode operations.

## Same-key retry lab

Run the retry demonstration only through its explicit isolated harness:

```powershell
.\scripts\run-local-retry-lab.ps1 -ConfirmIsolatedRetryLab
```

The harness temporarily stops the normal loopback stack, creates a uniquely
named `ledgersync-acceptance-*` project with separate state and volumes, and
uses the deterministic demo accounts. It submits one exact serialized transfer
intent, discards the successful response only after commit at the client
harness boundary, then retries the identical body and idempotency key.

The pass criteria are stored evidence: the retry returns the original transfer
ID; PostgreSQL contains exactly one transfer, one journal, and two postings;
both balances move once by the exact minor-unit amount; reconciliation has zero
mismatches; isolated resources are removed; and the normal financial
fingerprint is unchanged after restoration.

The lab does not add a browser fault toggle, API fault mode, proxy fault,
database interruption, or normal-project financial write.
