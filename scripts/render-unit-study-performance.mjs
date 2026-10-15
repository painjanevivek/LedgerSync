import { readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const directory = resolve(root, "docs/release-evidence/unit-3-unit-4/performance");
const matrix = JSON.parse(await readFile(resolve(directory, "matrix.json"), "utf8"));
if (matrix.results.length !== 8 || matrix.results.some((result) => !result.correctness_passed)) throw new Error("The complete correct matrix is required.");
function metric(result, name, field) { return result.load.summary.metrics[name]?.[field] ?? 0; }
function memory(value) {
  const match = value.split("/")[0].trim().match(/^([0-9.]+)\s*(B|kB|KB|KiB|MB|MiB|GB|GiB)$/);
  if (!match) return 0;
  const factor = { B: 1, kB: 1000, KB: 1000, KiB: 1024, MB: 1e6, MiB: 1048576, GB: 1e9, GiB: 1073741824 }[match[2]];
  return Number(match[1]) * factor / 1048576;
}
const rows = matrix.results.map((result) => {
  const posted = metric(result, "ledgersync_posted_commands", "count");
  const throughput = metric(result, "ledgersync_posted_commands", "rate");
  const seconds = throughput > 0 ? posted / throughput : null;
  const samples = result.load.samples;
  const cpu = samples.map((sample) => sample.containers.filter((container) => /-(api|outbox-worker)-\d+$/.test(container.Name)).reduce((sum, container) => sum + Number.parseFloat(container.CPUPerc), 0));
  const ram = samples.map((sample) => sample.containers.filter((container) => /-(api|outbox-worker)-\d+$/.test(container.Name)).reduce((sum, container) => sum + memory(container.MemUsage), 0));
  return {
    workload: result.name.split("-")[0], api: result.api_replicas, workers: result.worker_replicas,
    posted, posted_per_second: throughput, measured_elapsed_seconds: seconds,
    p50_ms: metric(result, "ledgersync_transfer_duration", "med"), p95_ms: metric(result, "ledgersync_transfer_duration", "p(95)"),
    business_rejections: metric(result, "ledgersync_business_rejections", "count"),
    retryable_attempts: metric(result, "ledgersync_technical_attempt_failures", "count"), unresolved_errors: metric(result, "ledgersync_unexpected_outcomes", "count"),
    replays: metric(result, "ledgersync_same_key_replays", "count"),
    oldest_queue_age_seconds: Math.max(0, ...samples.map((sample) => Number(sample.database.oldest_pending_seconds))),
    peak_pending: Math.max(0, ...samples.map((sample) => sample.database.pending)),
    published_events_per_second: seconds === null ? null : (result.after.published - result.before.published) / seconds,
    peak_api_worker_cpu_percent: Math.max(0, ...cpu), peak_api_worker_memory_mib: Math.max(0, ...ram),
    api_worker_cpu_limit: result.api_replicas + result.worker_replicas, api_worker_memory_limit_mib: 256 * (result.api_replicas + result.worker_replicas),
    k6_exit_code: result.load.exit_code, correctness_passed: result.correctness_passed,
  };
});
const apiFailure = JSON.parse(await readFile(resolve(directory, "../api-failure.json"), "utf8"));
const schedulerFailure = JSON.parse(await readFile(resolve(directory, "../scheduler-takeover.json"), "utf8"));
const workerLog = await readFile(resolve(directory, "../worker-crash-verified.txt"), "utf8");
const workerRecovery = [...workerLog.matchAll(/STUDY_WORKER_RECOVERY stage=(before_publish|after_publish) recovery_ms=(\d+) deliveries=(\d+) reconciliation=matched/g)].map((match) => ({ stage: match[1], recovery_ms: Number(match[2]), deliveries: Number(match[3]), reconciliation: "matched" }));
if (workerRecovery.length !== 2 || schedulerFailure.drills.length !== 2 || apiFailure.reconciliation !== "matched") throw new Error("Complete verified recovery evidence is required.");
const recovery = { api: { captured_at: apiFailure.captured_at, detection_ms: apiFailure.detection_milliseconds, commands: apiFailure.requests.length, interrupted_attempts: apiFailure.requests.flatMap((item) => item.attempts).filter((attempt) => attempt.status !== 201).length, reconciliation: apiFailure.reconciliation }, workers: workerRecovery, scheduler: schedulerFailure.drills.map((drill) => ({ mode: drill.mode, takeover_ms: drill.takeover_ms, stale_owner_rejected: drill.stale_owner_rejected })), limits: "Worker timing includes a deliberate 600ms wait after killing the process. API and scheduler timing include Docker commands and probes. These are local controlled demonstrations." };
await writeFile(resolve(directory, "../recovery-report.json"), JSON.stringify(recovery, null, 2) + "\n");
const fixed = (number) => Number(number).toFixed(2);
let report = `# Scaling and recovery measurements\n\n## Method\n\n- Source revision: \`${matrix.source_revision}\`. The working tree contains changes.\n- Source fingerprint: \`${matrix.source_fingerprint}\`. Image IDs are in [matrix.json](performance/matrix.json).\n- Docker: ${matrix.environment.engine}; ${matrix.environment.cpu_count} logical CPUs; ${fixed(matrix.environment.memory_bytes / 1073741824)} GiB engine memory.\n- Each run uses a fresh database and cache. The seed signature is \`${matrix.seed_signature}\`.\n- Warm-up: ${matrix.warmup_seconds} seconds. Workload: ${matrix.duration_seconds} nominal seconds; ${matrix.paced_clients} paced clients.\n- The last client iteration has no trailing sleep. Use measured command throughput, not the configured client count, as the result.\n- Independent traffic uses two disjoint account pairs in one tenant. Contended traffic uses one pair. Both retain normal authentication, authorization, rate limits, exact money, idempotency, balance reads, and periodic account/history reads.\n- Each API and worker has one CPU, 256 MiB memory, and four database connections. Total limits increase with replicas. The web service and gateway retain fixed limits. PostgreSQL and Redis use the same host.\n- The scheduler is disabled during each workload. The normal workspace remains running. Background container names are recorded.\n- Each workload includes same-key replays. Logical command latency includes bounded conflict retries. The latency table includes successful replay calls.\n- Resource and queue samples are collected approximately every five seconds. Peaks can occur between samples.\n- Reconcile each ledger after queue drain. Reject a run if journals are duplicated or unbalanced, delivery stops, or a command has an unresolved result.\n\n## Results\n\n| Workload | API / worker | Posted/s | p50 ms | p95 ms | Business rejection | Retryable attempt | Unresolved error | Queue age s | CPU peak % | Memory peak MiB |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n`;
for (const row of rows) report += `| ${row.workload} | ${row.api} / ${row.workers} | ${fixed(row.posted_per_second)} | ${fixed(row.p50_ms)} | ${fixed(row.p95_ms)} | ${row.business_rejections} | ${row.retryable_attempts} | ${row.unresolved_errors} | ${fixed(row.oldest_queue_age_seconds)} | ${fixed(row.peak_api_worker_cpu_percent)} | ${fixed(row.peak_api_worker_memory_mib)} |\n`;
report += "\nCPU and memory peaks are combined API/worker samples. CPU100% is one core. The machine-readable comparison also records queue depth, publication throughput, replay counts, resource limits, and k6 threshold exit codes.\n\n## Comparison\n\n";
for (const shape of ["independent", "hot"]) {
  const baseline = rows.find((row) => row.workload === shape && row.api === 1 && row.workers === 1);
  for (const row of rows.filter((row) => row.workload === shape && (row.api !== 1 || row.workers !== 1))) {
    const change = 100 * (row.p95_ms - baseline.p95_ms) / baseline.p95_ms;
    report += `- ${shape}, ${row.api} API / ${row.workers} worker: p95 ${change >= 0 ? "increased" : "decreased"} ${fixed(Math.abs(change))}% from the one/one run. Posted throughput changed ${fixed(100 * (row.posted_per_second - baseline.posted_per_second) / baseline.posted_per_second)}%.\n`;
  }
}
report += "\n## Correctness and limits\n\nAll eight runs passed balanced-journal, unique-journal, drained-delivery, command-count, and internal reconciliation checks. Raw k6 logs retain thresholds and request failures. A threshold failure is not removed from the record.\n\nThis is a short, fixed-demand comparison on one machine. It does not measure maximum capacity, statistical significance, sustained production load, or failure across machines. Small differences can be local noise. More replicas do not establish a proportional throughput increase. Account locks and the shared tenant policy lock still limit parallel financial commands, including commands on independent accounts in one tenant. Worker replicas improve recovery options; this small workload can be served by one worker. PostgreSQL remains a common dependency.\n\n## Recovery evidence\n\nSee [API failure](api-failure.json), [worker process crashes](worker-crash-verified.txt), and [scheduler takeover](scheduler-takeover.json). Failure detection includes container command and probe overhead. Record the final rerun commands and logs in the progress record. Delivery remains at least once.\n";
await writeFile(resolve(directory, "comparison.json"), JSON.stringify({ method: matrix.limits, rows }, null, 2) + "\n");
report += `\nMeasured API detection: ${fixed(recovery.api.detection_ms)} ms. Worker recovery: ${workerRecovery.map((item) => `${item.stage} ${item.recovery_ms} ms`).join("; ")}. Scheduler takeover: ${recovery.scheduler.map((item) => `${item.mode} ${item.takeover_ms} ms`).join("; ")}. See [recovery-report.json](recovery-report.json). Worker timing includes a deliberate 600 ms wait.\n`;
await writeFile(resolve(root, "docs/release-evidence/unit-3-unit-4/performance-report.md"), report);
