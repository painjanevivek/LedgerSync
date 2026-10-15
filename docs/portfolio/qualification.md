# Local candidate qualification

R13 verifies a local review candidate. It does not approve a managed production deployment.

Use `node scripts/portfolio/run-release-candidate.mjs r13-YYYYMMDD` from the repository root. Docker Desktop must use its local `desktop-linux` builder. Ports 3102 and 3103 must be free. The normal `compose` project must remain available. The command needs local Docker access and public dependency downloads. It does not upload source, images, financial data, or secrets.

The runner copies the complete Git-listed source to a new ignored directory. Its manifest includes the current commit, index digest and each source digest. Existing media outputs remain preserved and are marked as local artifacts. They do not enter the candidate build. The production Docker context permits only the required source roots. Private state, environment files, agent instructions, dependency directories and test reports stay outside that context.

Each image has a new local tag and a source-manifest label. The normal image tags are never built. A separate Compose project uses restricted API and worker logins. Temporary database storage has an explicit owner. All services drop capabilities, use read-only root filesystems, and disable privilege escalation. Only the two frontend ports bind to host loopback. The private API and databases have no published port. The seeded records are synthetic.

The browser suite checks lifecycle, funding, exact retry after response loss, stored recovery, read evidence, exports and investigation handover through the actual BFF and API. The runner records image IDs and finite service resource samples. It verifies exact container IDs, image IDs, labels, mounts and binds before use and removal. It checks normal container identities and source preservation at completion. Private configuration, logs and source snapshots stay in ignored `data/portfolio/`. Do not publish that directory.

Use `node scripts/portfolio/run-regression.mjs` for the complete integration and fault inventories. This runner uses new tmpfs PostgreSQL/Redis targets and the dependency image identities of the preserved normal stack. It discovers the compiled test inventory. Every selected top-level test must pass exactly once. Any failure or unexpected skip fails qualification. Fixed host-coordinated capacity, recovery, optional-feature and developer profiles run separately. The standalone crash helper is exercised by its parent. A short or skipped capacity run does not qualify sustained throughput.

The existing committed-release scripts retain their clean-tree checks. The portfolio runner is a separate source-manifest qualification path. A dirty review candidate is not a signed release. After an authorized commit, repeat the committed-release and full-history CI gates before release. New R15/R16 schema and financial behavior require final regression, model, capacity and recovery checks.

## Security review rules

Go vulnerability analysis runs locally. npm audit sends public dependency metadata. Trivy receives local image archives and a public advisory cache; image analysis can run with the network disabled. Gitleaks reads sanitized source or read-only local Git history with the network disabled. Raw secret reports use full redaction. A scanner failure remains a failure until its exact finding is repaired or reviewed with a recorded reason. JSON output with exit zero is not sufficient evidence of a clean Go scan.

The R13 source and history secret findings are recorded in `docs/release-evidence/portfolio/r13/secret-review.json`. The reviewed matches are test-only idempotency identifiers, synthetic study identifiers, public content hashes, one fixed synthetic cron fixture, or incident-response prose. The original scanner outputs remain unchanged. Only exact historical finding fingerprints are added to `.gitleaksignore`; no file, directory, rule or whole-commit exception is used. Changed source and new commits require a new scan.

The production dependency set was upgraded after release-blocking advisories. Next.js is pinned to 16.4.1. The Windows command-execution advisory is documented in the [official Next.js advisory](https://github.com/vercel/next.js/security/advisories/GHSA-p293-qw3h-jr36). The BFF uses the patched OpenTelemetry 2.x API and retains its explicit privacy allowlist. Go, networking, RPC and telemetry dependencies are patched. The OpenAPI validator and manifest validator have exact direct development pins.

Five remaining npm findings in the R13 audit come from one unpatched `braces` stack-exhaustion advisory and its development-only ESLint dependency chain. The registry still offers braces 3.0.3 as its latest version at the recorded check. ESLint reads repository-controlled glob patterns during local/CI analysis. It is absent from the standalone web runtime. Do not run the lint tool as an untrusted-pattern service. Track the advisory until an upstream fix exists, and repeat the complete audit after dependency changes. This is a bounded author risk acceptance for local development tooling; it is not a clean all-dependency scan or an independent security approval.

## Limits

Resource samples are not peak memory, sustained HTTP capacity, a production SLA, or multi-host evidence. Browser accessibility and image review are automated checks and author review; no independent human assistive-technology review is claimed. Historical R07–R12 bundles retain their original source and contract versions. Final candidate evidence must state its own identity. Normal images and financial data remain on the preserved prior runtime until a separately authorized cutover.
