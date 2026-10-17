# Local quickstart

Use Windows PowerShell 7, Git, Docker Desktop with Linux containers, and the repository's pinned Go and Node runtimes. Docker Compose must be at least 2.20. Keep at least 5 GiB of free disk space. The candidate runner limits each web container to 768 MiB and other services to 512 MiB. Browser and build processes need additional host memory. These limits do not establish a minimum host-memory requirement or a setup-time promise. The first build downloads dependencies and can take several minutes.

Run commands from the repository root. First check the machine:

```powershell
.\scripts\doctor-local.ps1
.\scripts\status-local.ps1
```

The doctor is read-only. If an existing stack is healthy, open `http://127.0.0.1:3000`. Keep its data. To start an absent stack or deliberately upgrade a preserved local stack:

```powershell
.\scripts\start-local.ps1
```

Startup builds the local images and applies ordered migrations. It can change the existing local schema. Use the [runtime runbook](../runbooks/local-runtime-smoke.md) and make a valid backup before an upgrade. Do not use normal startup as an isolated acceptance test. Do not run a volume-removal command to repair a startup failure.

Use **Guide** for accounts, funding evidence, review, transfer, and internal reconciliation. A financial action in the normal console changes normal local data. Use a new disposable candidate for a repeatable synthetic demonstration:

```powershell
node scripts/portfolio/run-release-candidate.mjs r13-20261016
```

The command requires a healthy normal stack for preservation checks and free ports 3102 and 3103. It creates new image tags, a new project, synthetic data, and temporary database storage. It runs the real browser and BFF checks, checks original-key replay after an API restart, and removes only its verified temporary runtime. Read the unique `data/portfolio/r13-20261016/candidate-*/result.json`. A failure stays a failure; retain its bundle.

For an already verified candidate, inspect its retained result rather than rerun it only to obtain another green result. The [qualification guide](qualification.md) explains the distinction between an identified local candidate and a committed release.

Stop the normal stack without removing its records:

```powershell
.\scripts\stop-local.ps1
```

Use the [limitations](limitations.md) and [troubleshooting runbook](../runbooks/local-runtime-smoke.md) when a check fails. No cloud account or paid resource is required for this local path.
