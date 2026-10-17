# Production deployment package — local preparation

Status: local preparation verified. See the [R17 completion record](../../docs/portfolio/progress.md#r17-completion--17-october-2026) for actual collector, candidate, scan, recovery and package evidence. No cloud deployment was executed. The owner selected a production package with local execution. The package does not claim production acceptance.

## Inputs and ownership

Start with [inputs.example.json](inputs.example.json). Provider, account/project, region, domain, budget, operating owner, incident owner, issuer, backup owner, RPO and RTO are deliberately unfilled. An RPO is the maximum acceptable data-loss interval. An RTO is the target time to restore service. A responsible operator must select and verify both before deployment. No local test supplies these business decisions.

The managed acceptance list covers identity/tenant mapping, private network and TLS ingress, secret injection, backup/PITR, failover, budget/alert owners and human acceptance. The validator can pass local package structure while `deployment_ready` remains false. A readiness demand must fail while these inputs or their acceptance evidence are absent.

## Files and restrictions

[compose.yaml](compose.yaml) defines the API, worker, BFF and an explicit one-shot migration profile. It uses externally selected digest-pinned images. It does not build, seed, expose a database port, create an administrator login or deploy a provider. Containers run as named non-root users, with a read-only root, bounded temporary storage, all Linux capabilities removed, no new privileges, finite CPU/memory/PID limits, health probes and a 30-second graceful stop.

Only the BFF has a loopback host port. A provider-approved TLS ingress is still required. API access must stay private to the BFF and approved operations. Bridge networking permits egress for the selected data and identity services. Provider firewall rules must restrict that egress. A local bridge is not proof of cloud network isolation.

Use [release.env.example](release.env.example) for image identities and private environment-file locations. Do not put real secrets in tracked files. The API, worker and migration use separate restricted database logins. Provisioning, reconciliation and maintenance have separate operating roles. The migration environment is temporary privileged authority. Do not copy it into an API or worker process.

The backend and BFF supported-variable templates remain [backend.env.example](../vercel/backend.env.example) and [web/.env.example](../../web/.env.example). Production validation, OIDC, secure cookies, private assertions, rate limiting and telemetry stay enabled. A deployment must provision real identity and data-service TLS. The synthetic development issuer is only a local qualification fixture.

## Release identity

Bind each package to one exact candidate source manifest, migration manifest, API contract version, four image IDs, image scan results, SBOMs, browser results, recovery evidence and verification logs. A local image ID is not a published registry digest. Publish nothing in this task. A later operator must select a registry and retain the corresponding immutable repository digests before deployment.

The package builder writes a new private directory. It never overwrites a prior result. It rejects a failed candidate, a mismatched source hash, missing image identities, a schema/source mismatch or absent successful scans/SBOMs. Historical R13 evidence retains its actual source. It cannot qualify schema65/66 images.

## Safe startup and migration order

1. Freeze the exact release source and image identities. Complete local qualification. Keep the original images and backups.
2. Select the environment and operating owners. Verify OIDC claims, tenant mapping, workload identity, data-service TLS, ingress and restricted egress. Inject separate workload secrets.
3. Make a complete database backup. Verify the archive digest and readable object list. Restore it into a separate isolated database. Precreate all fixed policy roles as inert NOLOGIN roles. Reject unsafe roles or memberships before restore. Do not restore global role credentials.
4. Rehearse the populated upgrade on that restored copy. Retain both legacy migration records. Do not replace the migration-history count with the number of current source files. Verify all original row fingerprints and active obligations.
5. Pause new financial writes for the selected maintenance window. Keep the current worker operating until the handover point. Record active holds and queue leases from PostgreSQL. Do not release holds by changing a clock or deleting a queue record.
6. Run the migration image once with the migration-only environment. Its command applies ordered migrations, then runs the workload-role script in one transaction. Stop on any failure. The application processes do not apply migrations at startup.
7. Verify the role catalog, forced tenant policies and controlled-function EXECUTE inventory. Revoke migration authority from the running workload context.
8. Start a compatible worker and API. Check readiness, each worker queue, database access and telemetry. Start the BFF only with valid identity/assertion configuration. Run read-only checks before enabling writes.
9. Use an approved synthetic tenant to check one controlled journal, original-key replay and reconciliation. Never use an unknown customer account as a smoke-test target. Release financial writes only after the operator's go/no-go.

No command in the local package validator performs these external actions.

## Rollback and forward repair

Keep old immutable images. A binary rollback is permitted only when that binary understands the current schema, current receipt formats, active reservations, expiry obligations and pending version-2 balance events. Schema66 is forward only. A pre-reservation binary is not compatible while those obligations or events exist.

If a new process fails, pause writes and inspect the failure. Preserve the database, pending jobs and original idempotency receipts. Prefer a compatible image rollback or a forward repair. Do not drop tables, reverse ledger postings, remove legacy migration rows or erase unknown command outcomes. A restore is a separate operator decision with explicit loss/fork analysis. It is not an automatic application rollback.

After any rollback or forward repair, compare posted balances with immutable journals, compare available funds with active holds, check queue lease generations, replay original keys and verify current reads. Redis can be rebuilt from PostgreSQL. Redis loss must not erase a hold or authorize spending.

## Backup and restore duties

Select an encrypted backup destination, retention schedule, restore owner, access boundary, RPO and RTO. These remain provider inputs. Use the existing [recovery runbooks](../../docs/runbooks/) and candidate restore tooling for local proof. Keep complete archives. Include statement bytes/hashes, snapshots, matches, decisions, holds, commands, events and expiry jobs.

Restore into a new isolated target. Verify target identity before any destructive operation. Create fixed inert policy roles first. Apply object ownership and current role policy without granting workload memberships in function owners. Verify all row counts and hashes, immutable source bytes, active hold totals, original-key outcomes, cache rebuild and worker recovery. Only then consider an approved cutover. Keep the original database and backup until the operator's retention decision.

## Telemetry and incident response

[collector-image.json](collector-image.json) pins the official collector distribution. [otel-collector.yaml](otel-collector.yaml) configures a private TLS receiver, bounded memory/queue/batches, verified TLS export and attribute filtering. Certificates and keys are private operator inputs. The Go exporter supports a trusted CA file; the BFF uses Node's normal trust configuration and optional `NODE_EXTRA_CA_CERTS`. No certificate-verification bypass is supplied.

Validate the actual pinned binary before enabling telemetry. YAML parsing alone is insufficient. Then send synthetic marked attributes through a local receiver/exporter rehearsal. Verify that allowed operational fields remain, marked private fields and events are absent, TLS verification is active and the queue stays bounded. This is a selected synthetic privacy test, not proof against every future instrumentation change.

Collector loss can remove sampled diagnostic evidence. It must not change a financial result. PostgreSQL records command, audit, journal and queue truth. On a financial mismatch, pause new writes, retain evidence and run internal reconciliation. On an expiry backlog, retain active holds and restore the compatible worker; do not infer that deadline passage released funds. On an unknown response, retrieve the original intent and key. On a credential incident, revoke the affected credential, inspect its scoped audit trail and rotate the relevant key set.

Rotate database logins one workload at a time with a connection-drain window. Rotate BFF assertion keys with the existing active/previous overlap, then remove the old key after token and request lifetimes expire. Rotate consistency-token keys under their existing validity rules. Rotate webhook keys with endpoint/key-reference history intact. Rotate issuer client credentials through the provider, verify token renewal, then retire the old credential. Never edit stored financial receipts during rotation.

Use the existing alert definitions and operations dashboard as starting files. Add actual alert destinations and responsible people before activation. A dashboard with no operating owner does not establish incident coverage.

## Local checks and handover

Run these only after source work is ready. Run one heavy task at a time:

```powershell
node scripts/portfolio/verify-production-package.mjs
node scripts/portfolio/run-statement.mjs
node scripts/portfolio/run-reservations.mjs
node scripts/portfolio/run-release-candidate.mjs r17-20261016
node scripts/portfolio/verify-collector.mjs
```

The package structure check is read-only. Runtime runners use verified disposable targets and retain failed/incomplete results. The normal local database stays on its preserved schema until a separate, explicit upgrade is requested. Do not run normal startup to perform acceptance tests.

The collector command follows the official [configuration validation guidance](https://opentelemetry.io/docs/collector/configuration/). The image source is the official [0.161.0 release](https://github.com/open-telemetry/opentelemetry-collector-releases/releases/tag/v0.161.0). The registry digest was resolved without starting Docker. Managed deployment acceptance remains a later operator decision.

Span events are removed by the official [filter processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.161.0/processor/filterprocessor/README.md). The real trace rehearsal checks event removal; a successful syntax command alone does not prove it.

## Refresh scanner data without high RAM use

Use the [advisory updater](../../scripts/portfolio/update-scanner-cache.ps1) while local Docker is already running. It does not start Docker or the application. The updater and image scanner use ignored disk staging, one CPU and 1 GiB of memory. Keep at least 4 GiB of free disk. Image scans refuse an image above 2 GiB and require current advisory metadata.

Run the following command from the repository root:

```powershell
.\scripts\portfolio\update-scanner-cache.ps1
```

If the local network rejects Go's default post-quantum TLS negotiation, use the explicit TLSCompatibility switch. It changes only the public-download negotiation; certificate checks stay enabled. Keep each failed update and its output directory. Scan the exact candidate with scan-candidate-images.ps1, then pass that scan directory and candidate directory to build-production-package.mjs. The final image/SBOM bindings, rather than a tag name, establish which image was checked.

The application candidate source manifest predates the disk-staging tooling repair. The final context comparison proves that these two scanner scripts are outside the application build inputs. Their changed source identity is recorded separately; the image is not relabelled.

## Source publication and existing platform integrations

The authorized push of 8b33cd5 started a frontend preview through the existing Vercel Git integration. It reached READY before inspection. The exact new non-production preview was removed after its metadata and build events were saved locally. The existing project and production target are unchanged. At that historical follow-up, only codex/portfolio-r13-r17 was disabled. The later ordered integration disabled all five portfolio/review branches and main in both Vercel roots. That is the current policy; other branches retain their existing settings. The local application qualification and the prepared four-image package are unchanged. A future merge or another branch still requires a separate deployment decision.

A local package flag does not change an existing hosting integration. Before publishing another local-only branch, set its explicit branch rule in both applicable Vercel roots. Confirm the rule before the first push. The provider documents this control in [Git configuration](https://vercel.com/docs/project-configuration/git-configuration). The current branch guard is absent from the qualified standalone runtime and does not change application behavior.

## Current audit repair source

Main `9b2dfaf` merged all five stages and passed its recorded CI. The new uncommitted audit repairs need their own qualification. Keep the old four-image identity package and its frozen source unchanged. Do not relabel it as a build of the changed tree. See [the repair register](../../docs/portfolio/audit-remediation.md). Docker and deployments remain stopped.
