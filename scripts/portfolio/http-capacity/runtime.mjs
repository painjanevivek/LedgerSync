import { sampleSQL, financialSQL } from './queries.mjs';
import { execFileSync, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createHash, randomBytes } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { createServer } from 'node:net';
import { cpus, arch, platform, hostname, totalmem, freemem, loadavg } from 'node:os';
import { setTimeout as sleep } from 'node:timers/promises';
import { performance } from 'node:perf_hooks';
import { repository, captureCandidateSource, sourceIdentity } from '../candidate-source.mjs';
import { capacitySeedSQL, composeCapacityCase, collectorConfiguration } from './compose.mjs';
import { fixedBudgets, validateImageSet } from './definition.mjs';
import { assertCapacityTarget, validateFinancialSnapshot, parseMemory } from './boundary.mjs';
import { CapacityWorkload } from './driver.mjs';
import { assessSteadyMeasurement } from './measurement.mjs';
import { verifyCapacityCaseEvidence, createCaseWriter } from './evidence.mjs';
import { summarizeCapacityComparisons } from '../capacity-matrix.mjs';
const hash = value => createHash('sha256').update(value).digest('hex');
const executeAsync = promisify(execFile);
const envKeys = new Set(['PATH', 'PATHEXT', 'SYSTEMROOT', 'WINDIR', 'COMSPEC', 'TEMP', 'TMP', 'USERPROFILE', 'HOME', 'LOCALAPPDATA', 'APPDATA', 'PROGRAMFILES', 'PROGRAMFILES(X86)', 'PROGRAMDATA', 'SYSTEMDRIVE', 'DOCKER_CONFIG']);
const cleanEnvironment = () => Object.fromEntries(Object.entries(process.env).filter(([key]) => envKeys.has(key.toUpperCase())));
const save = (root, name, value) => writeFileSync(join(root, name), typeof value === 'string' ? value : JSON.stringify(value, null, 2) + '\n', {
    flag: 'wx',
    mode: 0o600
});
const identity = c => ({
    id: c.Id,
    image: c.Image,
    running: c.State.Running,
    paused: c.State.Paused,
    mounts: c.Mounts,
    project: c.Config.Labels?.['com.docker.compose.project'],
    started_at: c.State.StartedAt,
    finished_at: c.State.FinishedAt
});
async function freePort() {
    return new Promise((resolve, reject) => {
        const server = createServer();
        server.once('error', reject);
        server.listen(0, '127.0.0.1', () => {
            const port = server.address().port;
            server.close(error => error ? reject(error) : resolve(port));
        });
    });
}
export class DockerCapacityRuntime {
    constructor({ signal, runtimeApproved = false } = {}) {
        this.signal = signal;
        this.runtimeApproved = runtimeApproved;
        this.environment = cleanEnvironment();
        this.suffix = randomBytes(4).toString('hex');
        this.stamp = new Date().toISOString().replace(/\D/g, '').slice(0, 14);
        this.root = join(repository, 'data/portfolio', `r09-${this.stamp.slice(0, 8)}`, `candidate-${this.stamp}-${this.suffix}`);
        this.images = {};
        this.context = null;
    }
    command(args, { input, timeout = 30000 } = {}) {
        this.signal?.throwIfAborted();
        if (this.caseDeadline) {
            const remaining = this.caseDeadline - Date.now();
            if (remaining <= 0)
                throw Error('Capacity case wall limit exceeded');
            timeout = Math.min(timeout, remaining);
        }
        try {
            return execFileSync('docker', this.context ? ['--context', this.context, ...args] : args, {
                cwd: repository,
                env: this.environment,
                encoding: 'utf8',
                windowsHide: true,
                input,
                timeout,
                maxBuffer: 4 * 1024 * 1024,
                stdio: ['pipe', 'pipe', 'pipe']
            }).trim();
        }
        catch (error) {
            throw Error(`Capacity Docker command failed (${args[0]}, exit ${error.status ?? 'timeout'}); private output was withheld`);
        }
    }
    async commandAsync(args, { timeout = 10000 } = {}) {
        this.signal?.throwIfAborted();
        if (this.caseDeadline) {
            const remaining = this.caseDeadline - Date.now();
            if (remaining <= 0)
                throw Error('Capacity case wall limit exceeded');
            timeout = Math.min(timeout, remaining);
        }
        try {
            const { stdout } = await executeAsync('docker', this.context ? ['--context', this.context, ...args] : args, {
                cwd: repository,
                env: this.environment,
                encoding: 'utf8',
                windowsHide: true,
                timeout,
                maxBuffer: 4 * 1024 * 1024
            });
            return stdout.trim();
        }
        catch (error) {
            throw Error(`Capacity sample command failed (${args[0]}, exit ${error.code ?? 'timeout'}); private output was withheld`);
        }
    }
    inspect(id) {
        return JSON.parse(this.command(['inspect', id]))[0];
    }
    normal() {
        const ids = this.command(['ps', '-aq']).split(/\s+/).filter(Boolean);
        return ids.map(id => identity(this.inspect(id))).sort((a, b) => a.id.localeCompare(b.id));
    }
    verify(state, target, options) {
        return assertCapacityTarget(this.inspect(target.id), target, options);
    }
    async prepare() {
        if (this.runtimeApproved !== true || process.env.LEDGERSYNC_CAPACITY_RUNTIME_APPROVED !== 'true')
            throw Error('Explicit later runtime approval is required before Docker access');
        this.context = this.command(['context', 'show']);
        const context = JSON.parse(this.command(['context', 'inspect', this.context]))[0], endpoint = context.Endpoints?.docker?.Host;
        if (!/^npipe:\/+\.\/pipe\/docker(?:DesktopLinuxEngine|_engine)$/.test(endpoint ?? '') && !/^unix:\/\/(?:\/var\/run|\/run\/user\/\d+)\/docker.sock$/.test(endpoint ?? ''))
            throw Error('Capacity execution requires a local Docker endpoint');
        if (this.command(['version', '--format', '{{.Server.Os}}']) !== 'linux')
            throw Error('Capacity images require the local Linux engine');
        this.normalBefore = this.normal();
        this.candidate = captureCandidateSource(this.root);
        this.before = this.candidate.identity;
        save(this.root, 'normal-before.json', this.normalBefore);
        this.machine = {
            hostname: hostname(),
            platform: platform(),
            arch: arch(),
            cpu_model: cpus()[0]?.model,
            cpu_count: cpus().length,
            memory_bytes: totalmem()
        };
        this.machineSHA = hash(JSON.stringify(this.machine));
        save(this.root, 'machine.json', this.machine);
        for (const service of ['api', 'outbox-worker', 'web', 'migrate']) {
            const tag = `ledgersync-capacity-${service}:${this.stamp}-${this.suffix}`, dockerfile = service === 'migrate' ? 'api' : service;
            const args = ['build', '--label', `ledgersync.source-manifest=${this.candidate.manifest_sha256}`, '--label', `org.opencontainers.image.revision=${this.before.head}`, '--tag', tag, '--file', join(this.candidate.snapshot, `deploy/docker/${dockerfile}.Dockerfile`)];
            if (service === 'migrate')
                args.push('--target', 'database-tooling');
            if (service !== 'web')
                args.push('--build-arg', 'GO_BUILD_GODEBUG=tlsmlkem=0');
            args.push(this.candidate.snapshot);
            save(this.root, `build-${service}.txt`, this.command(args, {
                timeout: 1200000
            }));
            const image = JSON.parse(this.command(['image', 'inspect', tag]))[0];
            if (image.Config.Labels?.['ledgersync.source-manifest'] !== this.candidate.manifest_sha256)
                throw Error('Built image source differs');
            this.images[service] = image.Id;
        }
        const normalCompose = readFileSync(join(this.candidate.snapshot, 'deploy/compose/docker-compose.yml'), 'utf8');
        for (const service of ['postgres', 'redis']) {
            const match = new RegExp(`  ${service}:\\r?\\n    image: ([^\\r\\n]+)`).exec(normalCompose);
            if (!match || !new RegExp(`^${service}:.*@sha256:[a-f0-9]{64}$`).test(match[1]))
                throw Error('Pinned dependency image absent');
            this.command(['pull', match[1]], {
                timeout: 180000
            });
            this.images[service] = JSON.parse(this.command(['image', 'inspect', match[1]]))[0].Id;
        }
        const collector = JSON.parse(readFileSync(join(this.candidate.snapshot, 'deploy/production/collector-image.json'), 'utf8')).image;
        if (!/^otel\/opentelemetry-collector-contrib:[0-9.]+@sha256:[a-f0-9]{64}$/.test(collector))
            throw Error('Pinned telemetry image required');
        this.command(['pull', collector], {
            timeout: 180000
        });
        this.images.telemetry = JSON.parse(this.command(['image', 'inspect', collector]))[0].Id;
        validateImageSet(this.images);
        save(this.root, 'images.json', this.images);
        if (JSON.stringify(sourceIdentity()) !== JSON.stringify(this.before))
            throw Error('Source changed while building capacity images');
    }
    async startCase(selected, signal, state) {
        Object.assign(state, {
            selected,
            signal,
            project: `ledgersync-capacity-${this.stamp}-${randomBytes(4).toString('hex')}`,
            targets: [],
            networks: [],
            samples: [],
            secrets: [],
            stoppedSampling: false
        });
        state.root = join(this.root, selected.id.replaceAll('/', '-'));
        mkdirSync(state.root);
        state.save = createCaseWriter(state.root);
        state.controller = new AbortController();
        state.timer = setTimeout(() => state.controller.abort(Error('Capacity case wall limit exceeded')), selected.method.wall_seconds_per_case * 1000);
        this.caseDeadline = Date.now() + selected.method.wall_seconds_per_case * 1000;
        state.caseSignal = signal ? AbortSignal.any([signal, state.controller.signal]) : state.controller.signal;
        state.save('definition.json', selected);
        const names = ['POSTGRES_PASSWORD', 'LEDGERSYNC_API_DATABASE_PASSWORD', 'LEDGERSYNC_WORKER_DATABASE_PASSWORD', 'LEDGERSYNC_SESSION_SECRET', 'LEDGERSYNC_CONSISTENCY_SIGNING_KEY', 'LEDGERSYNC_BFF_ASSERTION_SECRET', 'LEDGERSYNC_WEB_SESSION_SECRET', 'LEDGERSYNC_DEVELOPMENT_API_TOKEN'];
        const lines = names.map(name => {
            const value = randomBytes(32).toString('hex');
            state.secrets.push(value);
            return `${name}=${value}`;
        });
        state.save('runtime.env', lines.join('\n') + '\n');
        state.save('fixture.sql', capacitySeedSQL());
        state.save('collector.json', collectorConfiguration());
        const ports = [await freePort(), await freePort()];
        state.origins = ports.map(port => `http://127.0.0.1:${port}`);
        const base = JSON.parse(this.command(['compose', '--project-name', state.project, '--profile', 'collaboration', '--env-file', join(state.root, 'runtime.env'), '-f', join(this.candidate.snapshot, 'deploy/compose/docker-compose.yml'), 'config', '--format', 'json']));
        const generated = composeCapacityCase({
            base,
            selected,
            images: this.images,
            project: state.project,
            ports,
            seedPath: join(state.root, 'fixture.sql'),
            routerPath: join(this.candidate.snapshot, 'scripts/portfolio/http-capacity/router.mjs'),
            collectorPath: join(state.root, 'collector.json'),
            sourceSHA: this.candidate.manifest_sha256
        });
        state.config = generated.configuration;
        state.budgets = generated.budgets;
        state.save('compose.private.json', state.config);
        state.compose = ['compose', '--project-name', state.project, '-f', join(state.root, 'compose.private.json')];
        if (this.command(['ps', '-aq', '--filter', `label=com.docker.compose.project=${state.project}`]))
            throw Error('Capacity project already exists');
        try {
            state.save('startup.txt', this.command([...state.compose, 'up', '-d', '--no-build', '--wait', '--wait-timeout', '180'], {
                timeout: 195000
            }));
        }
        finally {
            this.discoverTargets(state);
        }
        await this.waitReady(state);
        state.workload = new CapacityWorkload(state.origins, selected.profile, this.suffix, {
            signal: state.caseSignal
        });
        await state.workload.authenticate();
        state.sampling = this.sampleLoop(state);
    }
    discoverTargets(state) {
        if (!state.config)
            return;
        const ids = this.command(['ps', '-aq', '--filter', `label=com.docker.compose.project=${state.project}`]).split(/\s+/).filter(Boolean);
        const known = new Map(state.targets.map(target => [target.id, target]));
        for (const id of ids) {
            const c = this.inspect(id), service = c.Config.Labels?.['com.docker.compose.service'], definition = state.config.services[service], budget = state.budgets[service];
            if (!definition || !budget)
                throw Error('Unplanned container appeared in capacity project');
            const tmpfs = Object.fromEntries(definition.tmpfs.map(value => {
                const colon = value.indexOf(':');
                return [value.slice(0, colon), value.slice(colon + 1)];
            }));
            const target = {
                id: c.Id,
                image: definition.image,
                project: state.project,
                owner: state.project,
                sourceSHA: this.candidate.manifest_sha256,
                service,
                ...budget,
                mounts: definition.volumes ?? [],
                tmpfs,
                networks: definition.networks.map(name => `${state.project}_${name}`),
                port: definition.ports ? Number(definition.ports[0].published) : null
            };
            this.verify(state, target, {
                allowStopped: true
            });
            known.set(target.id, target);
            state.targets = [...known.values()];
        }
        state.discovery = (state.discovery ?? 0) + 1;
        state.save(`targets-${state.discovery}.json`, state.targets);
    }
    async waitReady(state) {
        for (let attempt = 0; attempt < 60; attempt++) {
            state.caseSignal.throwIfAborted();
            let ready = true;
            for (const target of state.targets.filter(t => !['migrate', 'seed'].includes(t.service))) {
                const c = this.verify(state, target);
                if (c.State.Health && c.State.Health.Status !== 'healthy')
                    ready = false;
            }
            if (ready) {
                try {
                    const client = state.targets.find(t => t.service === 'web-0');
                    const result = this.command(['exec', client.id, 'node', '-e', "Promise.all([fetch('http://api-router:8080/readyz'),fetch('http://api:8080/readyz')]).then(r=>{if(r.some(v=>v.status!==200))process.exit(1)}).catch(()=>process.exit(1))"]);
                    if (result === '')
                        return;
                }
                catch {
                    ready = false;
                }
            }
            await sleep(500, undefined, {
                signal: state.caseSignal
            });
        }
        throw Error('Capacity service recovery deadline exceeded');
    }
    sql(state, query) {
        const pg = state.targets.find(t => t.service === 'postgres');
        this.verify(state, pg);
        const output = this.command(['exec', '-i', pg.id, 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'ledgersync', '-d', 'ledgersync', '-At'], {
            input: `SET statement_timeout='3s';\n${query};\n`,
            timeout: 10000
        });
        return JSON.parse(output.split(/\r?\n/).at(-1));
    }
    async sample(state) {
        const targets = state.targets.filter(t => !['seed', 'migrate'].includes(t.service));
        const verify = async (target) => assertCapacityTarget(JSON.parse(await this.commandAsync(['inspect', target.id]))[0], target);
        const stats = (await this.commandAsync(['stats', '--no-stream', '--format', '{{json .}}', ...targets.map(t => t.id)])).split(/\r?\n/).filter(Boolean).map(line => JSON.parse(line));
        const services = [];
        for (const target of targets) {
            await verify(target);
            const row = stats.find(s => typeof s.ID === 'string' && target.id.startsWith(s.ID)), processes = (await this.commandAsync(['top', target.id, '-eo', 'pid,rss'])).split(/\r?\n/).slice(1).filter(Boolean).map(line => line.trim().split(/\s+/).map(Number));
            if (!row || !processes.length || processes.some(p => p.length !== 2 || !p.every(Number.isFinite)))
                throw Error('Container resource evidence is absent');
            const cpu = Number(row.CPUPerc.replace('%', '')), memory = parseMemory(row.MemUsage.split('/')[0]);
            if (!Number.isFinite(cpu) || cpu < 0 || memory < 0)
                throw Error('Nonfinite resource observation');
            services.push({
                service: target.service,
                id: target.id,
                cpu_percent: cpu,
                working_set_bytes: memory,
                process_rss_bytes_sum: processes.reduce((n, p) => n + p[1] * 1024, 0)
            });
        }
        const web = targets.find(t => t.service === 'web-0');
        const metrics = await this.commandAsync(['exec', web.id, 'node', '-e', "fetch('http://telemetry:8889/metrics',{signal:AbortSignal.timeout(2000)}).then(async r=>{if(r.status!==200)process.exit(1);const b=await r.text();if(b.length>1048576)process.exit(1);console.log(b.split('\\n').filter(l=>/^ledgersync_database_(pool|policy_lock)/.test(l)).join('\\n'))}).catch(()=>process.exit(1))"]);
        const pg = targets.find(t => t.service === 'postgres');
        await verify(pg);
        const output = await this.commandAsync(['exec', pg.id, 'psql', '-v', 'ON_ERROR_STOP=1', '-U', 'ledgersync', '-d', 'ledgersync', '-At', '-c', `SET statement_timeout='3s'; ${sampleSQL}`]);
        return {
            at: new Date().toISOString(),
            status: 'sampled',
            services,
            database: JSON.parse(output.split(/\r?\n/).at(-1)),
            pool_and_policy_metrics: metrics.split('\n').filter(Boolean),
            driver: {
                cpu: process.cpuUsage(),
                rss_bytes: process.memoryUsage().rss
            },
            host: {
                free_memory_bytes: freemem(),
                load_average: platform() === 'win32' ? null : loadavg()
            }
        };
    }
    async sampleLoop(state) {
        while (!state.stoppedSampling) {
            if (state.samples.length >= 120) {
                state.controller?.abort(Error('Resource sample count exceeded'));
                return;
            }
            try {
                const sample = await this.sample(state);
                if (Buffer.byteLength(JSON.stringify(sample)) > 32768)
                    throw Error('Resource sample size exceeded');
                state.samples.push(sample);
                if (state.phase === 'steady' && (sample.database.dead > 0 || sample.database.pending > 1000 || sample.database.oldest_seconds > 5 || sample.database.database_bytes > 256 * 1024 * 1024))
                    state.loadStopConstraint = 'queue_or_database_budget';
            }
            catch {
                state.samples.push({
                    at: new Date().toISOString(),
                    status: 'unavailable'
                });
            }
            if (!state.stoppedSampling)
                await sleep(5000);
        }
    }
    async waitDrained(state) {
        for(let attempt=0;attempt<60;attempt++){
            state.caseSignal.throwIfAborted();
            const queue=this.sql(state,sampleSQL);
            if(queue.dead>0)throw Error('Dead publication event after capacity workload');
            if(queue.pending===0)return queue;
            if(attempt===59)throw Error('Capacity backlog did not drain');
            await sleep(1000,undefined,{signal:state.caseSignal});
        }
    }
    async measure(state) {
        const workload = state.workload;
        state.caseSignal.throwIfAborted();
        await workload.probeIsolation();
        await workload.stage('warmup', 5, 10);
        for (const rate of [5, 10, 20])
            await workload.stage(`calibration-${rate}`, rate, 10);
        await workload.recoverOriginals();
        state.save('pre-steady-queue.json',await this.waitDrained(state));
        const startSample = state.samples.length, attemptsBefore = workload.clients.reduce((n, c) => n + c.httpFailures + c.transportFailures, 0);
        state.phase = 'steady';
        let steady;
        try {
            steady = await workload.stage('steady', 10, 300, {
                stopRequested: () => state.loadStopConstraint
            });
        }
        finally {
            state.phase = 'recovery';
        }
        const endSample = state.samples.length;
        const technicalAttempts = workload.clients.reduce((n, c) => n + c.httpFailures + c.transportFailures, 0) - attemptsBefore;
        const lost = await workload.loseBody();
        const api = state.targets.find(t => t.service === 'api');
        this.verify(state, api);
        state.save('api-restart.txt', this.command(['restart', api.id], {
            timeout: 45000
        }));
        await this.waitReady(state);
        await workload.recoverLost(lost);
        const redis = state.targets.find(t => t.service === 'redis');
        this.verify(state, redis);
        this.command(['pause', redis.id]);
        const faultStarted = performance.now();
        const active = workload.stage('redis-fault', 10, 5);
        // Observe rejection immediately, then drain the driver before cleanup.
        const drained = Promise.allSettled([active]);
        try {
            await sleep(3000, undefined, {
                signal: state.caseSignal
            });
            this.command(['unpause', redis.id]);
            await active;
        }
        finally {
            const priorSignal = this.signal, priorDeadline = this.caseDeadline;
            this.signal = undefined;
            this.caseDeadline = undefined;
            try {
                const c = this.inspect(redis.id);
                assertCapacityTarget(c, redis, {
                    allowStopped: true
                });
                if (c.State.Paused)
                    this.command(['unpause', redis.id]);
            }
            finally {
                this.signal = priorSignal;
                this.caseDeadline = priorDeadline;
                await drained;
            }
        }
        await this.waitReady(state);
        await workload.recoverOriginals();
        await workload.probeIsolation();
        await this.waitDrained(state);
        for (const tenant of [0, 1]) {
            const client = workload.clients[tenant];
            const result = await client.request('/api/reconciliation/runs', {
                method: 'POST',
                body: {},
                intent: {
                    key: `capacity-reconcile-${this.suffix}-${tenant}`
                }
            });
            if (![200, 201].includes(result.status) || result.body.status !== 'matched' || result.body.mismatch_count !== '0')
                throw Error('BFF reconciliation did not produce a matched result');
        }
        const counts = workload.counts(), financial = validateFinancialSnapshot(this.sql(state, financialSQL), counts);
        state.save('financial.json', financial);
        const samples = state.samples.slice(startSample, endSample), assessment = assessSteadyMeasurement(steady, samples, state.targets, technicalAttempts);
        const newPosted = new Set(steady.results.filter(r => r.kind === 'new' && r.status === 'posted').map(r => r.key)).size;
        const report = {
            schema: 'ledgersync-http-capacity/v1',
            evidence_kind: 'runtime_capture',
            id: state.selected.id,
            source_manifest_sha256: this.candidate.manifest_sha256,
            machine_sha256: this.machineSHA,
            profile: state.selected.profile,
            allocation: state.selected.allocation,
            fixed_services: fixedBudgets,
            offered_per_second: 10,
            requested_steady_seconds: 300,
            observed_steady_seconds: steady.observed_seconds,
            financial_invariants: true,
            unresolved_outcomes: counts.unresolved,
            dropped: steady.dropped,
            ...assessment,
            new_posted_per_second: newPosted / steady.observed_seconds,
            counts,
            steady,
            recovery: {
                original_key_replayed: true,
                api_process_restarted: true,
                redis_pause_recovered: true,
                actual_recovery_stage_seconds: (performance.now() - faultStarted) / 1000
            },
            method_limits: ['Finite sampled CPU, container working set and summed process RSS; shared pages can be counted more than once.', 'Steady command latency includes durable intent preparation and BFF submission. Fault-stage counts are separate.', 'Local synthetic transfers without active holds; no production or maximum-throughput claim.']
        };
        report.technical_attempt_failures = technicalAttempts;
        report.background_load_identity = this.normalBefore;
        report.images = this.images;
        state.save('enforcement.json', state.targets.map(expected => {
            const value = this.verify(state, expected, {
                allowStopped: ['migrate', 'seed'].includes(expected.service)
            });
            const fields = ['NanoCpus', 'Memory', 'ReadonlyRootfs', 'Privileged', 'CapDrop', 'CapAdd', 'PublishAllPorts', 'SecurityOpt', 'PortBindings', 'Tmpfs'];
            return {
                expected,
                observed: {
                    Id: value.Id,
                    Image: value.Image,
                    Config: {
                        Labels: value.Config.Labels
                    },
                    State: value.State,
                    NetworkSettings: {
                        Networks: value.NetworkSettings.Networks
                    },
                    HostConfig: Object.fromEntries(fields.map(k => [k, value.HostConfig[k]])),
                    Mounts: value.Mounts
                }
            };
        }));
        state.save('stages.json', workload.stages);
        state.save('steady-resources.json', samples);
        state.save('report.json', report);
        return report;
    }
    async recordFailure(state, error) {
        if (state.root) {
            if (state.workload) {
                state.save('stages-partial.json', state.workload.stages);
                state.save('outcomes-partial.json', {
                    counts: state.workload.counts(),
                    receipts: [...state.workload.receipts],
                    unresolved: [...state.workload.unresolved]
                });
            }
            state.save('failure.json', {
                message: this.redact(state, error.message),
                qualification: 'failed',
                incomplete: true
            });
        }
    }
    redact(state, text) {
        for (const secret of state.secrets ?? [])
            text = text.replaceAll(secret, '[REDACTED]');
        return text;
    }
    async cleanup(state) {
        clearTimeout(state.timer);
        state.stoppedSampling = true;
        if (state.sampling)
            await state.sampling;
        if (!state.root)
            return;
        // Cleanup must still run after cancellation; it uses only saved owned IDs.
        const priorSignal = this.signal;
        this.signal = undefined;
        this.caseDeadline = Date.now() + 120000;
        try {
            const cleanup = [], failures = [];
            try {
                state.save('resource-samples.json', state.samples);
            }
            catch (error) {
                failures.push(error);
            }
            if (state.config && state.targets.length === 0)
                this.discoverTargets(state);
            for (const target of state.targets) {
                try {
                    const c = this.verify(state, target, {
                        allowStopped: true
                    });
                    if (c.State.Paused)
                        this.command(['unpause', target.id]);
                    let truncated = false;
                    try {
                        const logs = this.redact(state, this.command(['logs', '--tail', '500', target.id]));
                        truncated = logs.length > 128 * 1024;
                        state.save(`logs-${target.service}.txt`, logs.slice(-128 * 1024));
                    }
                    catch (error) {
                        failures.push(error);
                    }
                    this.command(['rm', '-f', target.id]);
                    cleanup.push({
                        id: target.id,
                        removed: true,
                        log_truncated: truncated
                    });
                }
                catch (error) {
                    failures.push(error);
                    cleanup.push({
                        id: target.id,
                        removed: false
                    });
                }
            }
            const networks = this.command(['network', 'ls', '-q', '--filter', `label=com.docker.compose.project=${state.project}`]).split(/\s+/).filter(Boolean);
            for (const id of networks) {
                const n = JSON.parse(this.command(['network', 'inspect', id]))[0];
                if (n.Labels?.['ledgersync.capacity.owner'] !== state.project || n.Labels?.['ledgersync.capacity.source'] !== this.candidate.manifest_sha256 || Object.keys(n.Containers ?? {}).length)
                    throw Error('Refuse an unowned or occupied network cleanup');
                this.command(['network', 'rm', n.Id]);
            }
            state.save('cleanup.json', cleanup);
            const files = readdirSync(state.root).filter(name => name !== 'runtime.env' && name !== 'compose.private.json').map(name => ({
                path: name,
                bytes: statSync(join(state.root, name)).size,
                sha256: hash(readFileSync(join(state.root, name)))
            }));
            state.save('artifacts.json', {
                files,
                private_files_excluded: ['runtime.env', 'compose.private.json']
            });
            if (failures.length)
                throw new AggregateError(failures, 'Owned cleanup or log capture was incomplete');
        }
        finally {
            this.signal = priorSignal;
            this.caseDeadline = undefined;
        }
    }
    async finish(result) {
        if (!this.candidate)
            return;
        const priorSignal = this.signal;
        this.signal = undefined;
        try {
            const normalAfter = this.normal(), sourceAfter = sourceIdentity();
            const preserved = JSON.stringify(normalAfter) === JSON.stringify(this.normalBefore) && JSON.stringify(sourceAfter) === JSON.stringify(this.before);
            let verificationError, captures = [];
            if (!result.failed && preserved) {
                try {
                    captures = result.reports.map(report => verifyCapacityCaseEvidence(join(this.root, report.id.replaceAll('/', '-')), {
                        sourceSHA: this.candidate.manifest_sha256,
                        machineSHA: this.machineSHA,
                        images: this.images
                    }));
                    save(this.root, 'verified-cases.json', captures.map(({ report, ...capture }) => capture));
                    if (captures.length === 27)
                        save(this.root, 'comparison-summary.json', summarizeCapacityComparisons(captures.map(c => c.report)));
                }
                catch (error) {
                    verificationError = error;
                }
            }
            save(this.root, 'completion.json', {
                ...result,
                failed: result.failed || !preserved || !!verificationError,
                preserved,
                verified_case_artifacts: captures.length,
                artifact_verification_error: verificationError?.message ?? null,
                normal_after: normalAfter,
                source_after: sourceAfter,
                completed_at: new Date().toISOString(),
                normal_data_queried: false,
                deployment: false
            });
            if (!preserved)
                throw Error('Normal target or source identity changed');
            if (verificationError)
                throw verificationError;
        }
        finally {
            this.signal = priorSignal;
        }
    }
}
