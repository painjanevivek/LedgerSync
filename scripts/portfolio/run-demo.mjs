import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { existsSync, lstatSync, mkdirSync, readFileSync, readdirSync, realpathSync, statSync, writeFileSync } from 'node:fs';
import { arch, cpus, platform, totalmem } from 'node:os';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { assertDisposableTarget, demoPurpose, fileDigest, scenarioResult, verifyDemoArtifacts } from './demo-safety.mjs';
import { withCapacityWorker } from './capacity-worker.mjs';
import { restoreCandidate } from './restore-candidate.mjs';
import { regressionExcluded, regressionResult } from './regression-result.mjs';
import { failureScenarios, demoPreview } from './demo-scenarios.mjs';

const repo = realpathSync(resolve(dirname(fileURLToPath(import.meta.url)), '../..'));
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const stateRoot = join(repo, 'data/portfolio/demo-runs');

function command(executable, args, options = {}) {
  return execFileSync(executable, args, { cwd: repo, encoding: 'utf8', windowsHide: true, timeout: 30000, maxBuffer: 16 * 1024 * 1024, ...options });
}
const docker = (...args) => command('docker', args).trim();
function safeContainer(value) {
  return { id: value.Id, image: value.Image, running: value.State.Running, mounts: value.Mounts.map(m => ({ type: m.Type, destination: m.Destination, source: m.Source })), project: value.Config.Labels?.['com.docker.compose.project'], service: value.Config.Labels?.['com.docker.compose.service'] };
}
function normalIdentity() {
  const ids = docker('ps', '-aq', '--filter', 'label=com.docker.compose.project=compose').split(/\r?\n/).filter(Boolean).sort();
  return ids.map(id => safeContainer(JSON.parse(docker('inspect', id))[0])).sort((a,b) => a.id.localeCompare(b.id));
}
function sourceIdentity() {
  const names = command('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z']).split('\0').filter(path => path && existsSync(join(repo, path))).sort();
  const files = names.map(path => {
    if (!lstatSync(join(repo,path)).isFile()) throw new Error('Source must be regular local files');
    return { path, sha256: hash(readFileSync(join(repo,path))) };
  });
  return { head: command('git', ['rev-parse','HEAD']).trim(), index_sha256: hash(command('git',['ls-files','--stage','-z'])), dirty: command('git',['status','--porcelain']).trim().length > 0, files };
}
function inspectTarget(expected,options) { return assertDisposableTarget(JSON.parse(docker('inspect', expected.id))[0], expected,options); }
function boundedProcess(executable, args, env, signal, secret, timeout = 180000) {
  return new Promise((resolveResult, reject) => {
    const child = spawn(executable, args, { cwd: repo, env, windowsHide: true, detached: process.platform !== 'win32', stdio: ['ignore','pipe','pipe'] });
    let output = '', errorOutput = '', terminated = false;
    const kill = () => {
      if (child.exitCode !== null || terminated) return; terminated = true;
      try { if (process.platform === 'win32') command('taskkill', ['/PID', String(child.pid), '/T', '/F']); else process.kill(-child.pid, 'SIGKILL'); } catch { /* exit raced cancellation */ }
    };
    const timer = setTimeout(kill, timeout);
    signal.addEventListener('abort', kill, { once: true });
    const collect = field => chunk => { if (Buffer.byteLength(output + errorOutput) + chunk.length > 8 * 1024 * 1024) { kill(); return; } if (field === 'out') output += chunk; else errorOutput += chunk; };
    child.stdout.on('data', collect('out')); child.stderr.on('data', collect('err'));
    child.once('error', error => { clearTimeout(timer); signal.removeEventListener('abort', kill); reject(error); });
    child.once('close', code => {
      clearTimeout(timer); signal.removeEventListener('abort', kill);
      resolveResult({ code, output: output.replaceAll(secret,'[REDACTED]'), errorOutput: errorOutput.replaceAll(secret,'[REDACTED]'), terminated });
    });
    if (signal.aborted) kill();
  });
}

export async function runDemo({profile='failures',modelReplayJSON,outboxBatchSize=10,capacitySteadySeconds=300,capacityRate=30,capacityProfile='mixed',capacityArrivalMode='open-loop'}={}) {
  if(!['failures','financial-model','capacity','recovery','optional-features','developer-integration','regression','statement-reconciliation','reservations','extension-recovery'].includes(profile)||modelReplayJSON!==undefined&&(profile!=='financial-model'||typeof modelReplayJSON!=='string'||Buffer.byteLength(modelReplayJSON)>24000)||![10,25,50].includes(outboxBatchSize)||![10,300].includes(capacitySteadySeconds)||![10,20,30].includes(capacityRate)||!['mixed','independent-pairs','hot-pair','multi-tenant'].includes(capacityProfile)||!['open-loop','closed-clients'].includes(capacityArrivalMode))throw new Error('Unsupported bounded host profile');
  let scenarios=profile==='extension-recovery'?[['extension-recovery-fixture','integration','TestPortfolioExtensionRecoverySeed'],['extension-recovery-restored','integration','TestPortfolioExtensionRecoveryRestored']]:profile==='reservations'?[['internal-reservations','integration','TestInternalReservations'],['reservation-model','integration','TestReservationGeneratedModel'],['reservation-int64','integration','TestReservationExactIntegerBoundary'],['reservation-financial-boundary','integration','TestReservationsPreserveFundingAndCorrectionBoundary']]:profile==='statement-reconciliation'?[['statement-reconciliation','integration','TestStatementReconciliation'],['statement-snapshot','integration','TestStatementSnapshotIsolationAndConsumption'],['statement-bounds','integration','TestStatementRowBoundsAndPaging']]:profile==='developer-integration'?[['independent-developer-integration','integration','TestPortfolioDeveloperIntegration']]:profile==='optional-features'?[['optional-features','integration','TestPortfolioOptionalFeatures']]:profile==='financial-model'?[['generated-financial-model','integration','TestGeneratedFinancialModel'],['workflow-financial-model','integration','TestGeneratedWorkflowModel'],['concurrent-financial-histories','integration','TestFinancialConcurrentHistoriesMatchSerialOrders']]:profile==='capacity'?[['bounded-capacity','integration','TestPortfolioCapacity']]:profile==='recovery'?[['candidate-recovery-fixture','integration','TestPortfolioRecoverySeed'],['candidate-recovery-restored','integration','TestPortfolioRecoveryRestored'],['candidate-recovery-migrations','integration','TestPortfolioRecoveryMigrations']]:failureScenarios.map(row=>[...row]);
  if(profile==='regression')scenarios=[['complete-integration','integration','Test'],['complete-fault','fault','Test']];
  const started = new Date(), stamp = started.toISOString().replace(/\D/g,'').slice(0,14), suffix = randomBytes(4).toString('hex');
  const project = `ledgersync-acceptance-${stamp}-${suffix}`, runId = `demo-${stamp}-${suffix}`, root = join(stateRoot,runId);
  mkdirSync(stateRoot,{recursive:true});
  if (lstatSync(stateRoot).isSymbolicLink() || realpathSync(stateRoot) !== stateRoot || existsSync(root)) throw new Error('Unsafe or existing demonstration output');
  mkdirSync(root); const save = (name,value) => writeFileSync(join(root,name),typeof value === 'string' ? value : JSON.stringify(value,null,2)+'\n',{flag:'wx',mode:0o600});
  const before = sourceIdentity(), normalBefore = normalIdentity(); save('source-manifest.json',before); save('normal-containers-before.json',normalBefore);
  save('run-profile.json',{profile,...(profile==='failures'?{failure_matrix_version:'r07/v2'}:{}),host_pid:process.pid,...(modelReplayJSON?{model_replay:JSON.parse(modelReplayJSON)}:{}),...(profile==='capacity'?{outbox_batch_size:outboxBatchSize,steady_seconds:capacitySteadySeconds,target_commands_per_second:capacityRate,workload_profile:capacityProfile,steady_arrival_model:capacityArrivalMode}:{})});
  const images = ['postgres','redis'].map(service => {
    const matches = normalBefore.filter(c => c.service === service); if (matches.length !== 1) throw new Error('One preserved dependency image is required');
    return {service,digest:matches[0].image};
  });
  const sourceMigrations = readdirSync(join(repo,'migrations')).filter(p => p.endsWith('.up.sql')).sort().map(path => ({path,sha256:hash(readFileSync(join(repo,'migrations',path)))}));
  const password = randomBytes(32).toString('hex');
  save('database.env',`POSTGRES_USER=ledgersync\nPOSTGRES_DB=ledgersync\nPOSTGRES_PASSWORD=${password}\n`);
  const controller = new AbortController(), stop = () => controller.abort();
  process.once('SIGINT',stop); process.once('SIGTERM',stop);
  const deadline = setTimeout(stop,600000), cancellation = setInterval(()=>{if(existsSync(join(root,'cancel.requested')))stop();},250), targets = [], results = scenarios.map(([id]) => ({id,status:'incomplete',reason:'Scenario did not start.',checks:[]}));
  let failure = null, migration = {latest:'unavailable',count:0,manifest_sha256:''};
  try {
    if (docker('ps','-aq','--filter',`label=com.docker.compose.project=${project}`)) throw new Error('Disposable project already exists');
    for (const image of images) {
      const pg = image.service === 'postgres';
      const args = ['run','-d','--name',`${project}-${image.service}`,'--label',`com.docker.compose.project=${project}`,'--label',`ledgersync.purpose=${demoPurpose}`,'--cpus','1','--memory',pg?'768m':'256m','--publish',pg?'127.0.0.1::5432':'127.0.0.1::6379','--tmpfs',pg?'/var/lib/postgresql/data:rw,size=512m':'/data:rw,size=128m'];
      if (pg) args.push('--env-file',join(root,'database.env'));
      args.push(image.digest); if (!pg) args.push('redis-server','--save','','--appendonly','no');
      const id = docker(...args), expected = {project,id,image:image.digest,service:image.service}; targets.push(expected);
      expected.port = inspectTarget(expected);
    }
    save('isolated-targets.json',targets);
    const pg = targets.find(t=>t.service==='postgres'), redis = targets.find(t=>t.service==='redis');
    let ready = false;
    for (let attempt=0;attempt<30;attempt++) {
      if (controller.signal.aborted) throw new Error('Demonstration cancelled');
      try { docker('exec',pg.id,'pg_isready','-U','ledgersync','-d','ledgersync'); ready=true;break; } catch { await new Promise(r=>setTimeout(r,250)); }
    }
    if (!ready) throw new Error('Disposable database startup deadline exceeded');
    const env = Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('LEDGERSYNC_')&&!key.startsWith('PG')));
    Object.assign(env,{GOMAXPROCS:'2',LEDGERSYNC_ENV:'development',LEDGERSYNC_DATABASE_URL:`postgres://ledgersync:${password}@127.0.0.1:${pg.port}/ledgersync?sslmode=disable`,LEDGERSYNC_TEST_REDIS_ADDR:`127.0.0.1:${redis.port}`,LEDGERSYNC_TEST_REPOSITORY_ROOT:repo,LEDGERSYNC_PORTFOLIO_CONTROL_DIRECTORY:root}); env.LEDGERSYNC_TEST_DATABASE_URL=env.LEDGERSYNC_DATABASE_URL;
    if(profile==='recovery'||profile==='extension-recovery')env.LEDGERSYNC_RUN_RECOVERY='true';
    if(profile==='optional-features')env.LEDGERSYNC_RUN_OPTIONAL='true';
    if(profile==='developer-integration')env.LEDGERSYNC_RUN_DEVELOPER='true';
    if(modelReplayJSON)env.LEDGERSYNC_MODEL_REPLAY_JSON=modelReplayJSON;
    if(profile==='capacity')Object.assign(env,{LEDGERSYNC_RUN_CAPACITY:'true',LEDGERSYNC_CAPACITY_CONTROL_DIRECTORY:root,LEDGERSYNC_CAPACITY_STEADY_SECONDS:String(capacitySteadySeconds),LEDGERSYNC_CAPACITY_RATE:String(capacityRate),LEDGERSYNC_CAPACITY_PROFILE:capacityProfile,LEDGERSYNC_CAPACITY_ARRIVAL_MODE:capacityArrivalMode,LEDGERSYNC_CAPACITY_OUTBOX_BATCH_SIZE:String(outboxBatchSize)});
    const migrate = await boundedProcess('go',['run','./cmd/migrate'],env,controller.signal,password); save('migrate.txt',migrate.output+migrate.errorOutput); if(migrate.code!==0) throw new Error('Disposable migration failed');
    const roleOutput = command('docker',['exec','-i',pg.id,'psql','--single-transaction','-v','ON_ERROR_STOP=1','-U','ledgersync','-d','ledgersync','-f','-'],{input:readFileSync(join(repo,'deploy/postgres/roles.sql'))}); save('roles.txt',roleOutput);
    const applied = docker('exec',pg.id,'psql','-U','ledgersync','-d','ledgersync','-At','-c','SELECT version FROM schema_migrations ORDER BY version').split(/\r?\n/).filter(Boolean);
    save('migration-manifest.json',{applied,source_migrations:sourceMigrations}); migration={latest:applied.at(-1)??'empty',count:applied.length,manifest_sha256:await fileDigest(join(root,'migration-manifest.json'))};
    for(const group of new Set(scenarios.map(s=>s[1]))) {
      const binary=`${group}.test${process.platform==='win32'?'.exe':''}`;
      const build=await boundedProcess('go',['test','-c','-o',join(root,binary),`./tests/${group}`],env,controller.signal,password);save(`${group}-build.txt`,build.output+build.errorOutput);if(build.code!==0)throw new Error('Demonstration binary build failed');
    }
    let workerBinary,workerEnv,workerPassword;
    if(profile==='capacity'){
      workerBinary=join(root,`outbox-worker${process.platform==='win32'?'.exe':''}`);
      const build=await boundedProcess('go',['build','-o',workerBinary,'./cmd/outbox-worker'],env,controller.signal,password);save('worker-build.txt',build.output+build.errorOutput);if(build.code!==0)throw new Error('Native worker build failed');
      workerPassword=randomBytes(32).toString('hex');const login='portfolio_capacity_worker_'+suffix;
      command('docker',['exec','-i',pg.id,'psql','-v','ON_ERROR_STOP=1','-U','ledgersync','-d','ledgersync','-f','-'],{input:`CREATE ROLE ${login} LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS PASSWORD '${workerPassword}' IN ROLE ledgersync_worker;`});
      workerEnv={...Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('LEDGERSYNC_')&&!key.startsWith('PG'))),GOMAXPROCS:'2',LEDGERSYNC_ENV:'development',LEDGERSYNC_DATABASE_URL:`postgres://${login}:${workerPassword}@127.0.0.1:${pg.port}/ledgersync?sslmode=disable`,LEDGERSYNC_REDIS_ADDR:`127.0.0.1:${redis.port}`,LEDGERSYNC_DATABASE_MAX_OPEN_CONNS:'2',LEDGERSYNC_NODE_ID:`capacity-${suffix}`,LEDGERSYNC_OUTBOX_BATCH_SIZE:String(outboxBatchSize),LEDGERSYNC_WORKER_HEALTH_PATH:join(root,'worker-health.json'),LEDGERSYNC_TELEMETRY_ENABLED:'false'};
      save('worker.env',Object.entries(workerEnv).filter(([key])=>key.startsWith('LEDGERSYNC_')).map(([key,value])=>`${key}=${value}`).join('\n')+'\n');
    }
    let runtimeEnv=env;
    for(let i=0;i<scenarios.length;i++) {
      if(controller.signal.aborted)throw new Error('Demonstration cancelled');
      for(const target of targets) if(inspectTarget(target)!==target.port)throw new Error('Disposable bind changed');
      if((profile==='recovery'||profile==='extension-recovery')&&i===1)runtimeEnv=await restoreCandidate({root,repo,project,pg,redis,targets,inspect:inspectTarget,docker,save,env,password,signal:controller.signal,boundedProcess,currency:profile==='extension-recovery'?'INR':'USD'});
      const [id,group,test]=scenarios[i],binary=join(root,`${group}.test${process.platform==='win32'?'.exe':''}`);
      let expected=[];
      if(profile==='regression'){
        expected=command(binary,['-test.list','^Test']).split(/\r?\n/).filter(name=>/^Test[A-Za-z0-9_]+$/.test(name)&&!new RegExp(regressionExcluded).test(name));
        save(`${id}-inventory.json`,{expected,excluded:regressionExcluded});
      }
      const execute=async()=>{const run=await boundedProcess('go',['tool','test2json','-t','-p',`ledgersync/${group}`,binary,'-test.v',profile==='capacity'?'-test.timeout=9m':'-test.timeout=150s',profile==='regression'?'-test.run=^Test':`-test.run=^${test}$`,...(profile==='regression'?[`-test.skip=${regressionExcluded}`]:[])],runtimeEnv,controller.signal,password,profile==='capacity'?570000:180000);save(`${id}.jsonl`,run.output);save(`${id}.stderr.txt`,run.errorOutput);return run;};
      const run=profile==='capacity'?await withCapacityWorker(execute,{root,workerBinary,workerEnv,redis,targets,inspect:inspectTarget,docker,signal:controller.signal,abort:stop,secrets:[password,workerPassword]}):await execute();
      results[i]={id,...(profile==='regression'?regressionResult(run.output,expected,run.code):scenarioResult(run.output,test,run.code))};
      if(profile==='capacity'&&results[i].status==='passed'){
        const output=run.output.split(/\r?\n/).filter(Boolean).map(line=>JSON.parse(line)).map(event=>event.Output??'').join('');
        const reports=output.split('\n').filter(line=>line.includes('CAPACITY_REPORT ')).map(line=>JSON.parse(line.slice(line.indexOf('CAPACITY_REPORT ')+16)));
        if(reports.length!==1||reports[0].financial_invariants!==true)throw new Error('One complete capacity correctness report is required');
        const report=reports[0];const samples=JSON.parse(readFileSync(join(root,'resource-samples.json'),'utf8'));report.native_worker_resource_samples=samples.filter(sample=>sample.worker?.status==='sampled').length;if(report.qualification_requested&&report.native_worker_resource_samples<2){report.qualified=false;report.stop_constraint='native_worker_resource_evidence';}save('capacity-report.json',report);
        if(report.qualification_requested&&!report.qualified){results[i].status='incomplete';results[i].reason='The local performance budget was exceeded. Financial checks passed.';results[i].checks.push({id:'five-minute-capacity-budget',status:'failed',reason:report.stop_constraint||'Qualification did not complete'});failure=new Error('Local capacity budget was exceeded');}
        else if(report.calibration_only)results[i].checks.push({id:'five-minute-capacity-budget',status:'not-applicable',reason:'Short calibration only; no capacity qualification is claimed.'});
        else results[i].checks.push({id:'five-minute-capacity-budget',status:'passed'});
      }
      console.log(`${id}: ${results[i].status}`);
      if(results[i].status!=='passed'&&!(profile==='capacity'&&failure))throw new Error('A named demonstration scenario failed');
    }
    if(profile==='recovery'){const record=JSON.parse(readFileSync(join(root,'candidate-restore.json'),'utf8'));save('candidate-recovery-result.json',{status:'verified_local',financial_validation:'named real restricted-role test passed',validated_at:new Date().toISOString(),backup_and_validated_restore_seconds:(Date.now()-Date.parse(record.backup_started_at))/1000,source_and_target_distinct:record.source_and_target_distinct,managed_recovery_claim:false});}
    const sourceAfter=sourceIdentity(),normalAfter=normalIdentity();save('source-after.json',sourceAfter);save('normal-containers-after.json',normalAfter);
    if(JSON.stringify(sourceAfter)!==JSON.stringify(before))throw new Error('Source or index changed during demonstration');
    if(JSON.stringify(normalAfter)!==JSON.stringify(normalBefore))throw new Error('Preserved normal container identity changed');
    results.push({id:'source-and-target-preservation',status:'passed',checks:[{id:'unchanged-source-index-and-normal-containers',status:'passed'}]});
  } catch(error) {
    failure=error;results.push({id:'runner-completion',status:'failed',reason:'The runner stopped before complete qualification. See its private failure record.',checks:[{id:'runner-completed',status:'failed',reason:'No overall pass is claimed.'}]});
    save('failure.json',{message:String(error.message).replaceAll(password,'[REDACTED]'),cancelled:controller.signal.aborted});
  } finally {
    clearTimeout(deadline);clearInterval(cancellation);process.removeListener('SIGINT',stop);process.removeListener('SIGTERM',stop);
    const cleanup=[];
    for(const target of targets) {
      try { inspectTarget(target,{allowStopped:true,allowPaused:true});docker('rm','-f',target.id);cleanup.push({id:target.id,removed:true}); }
      catch { cleanup.push({id:target.id,removed:false});failure??=new Error('Disposable cleanup could not be verified'); }
    }
    save('cleanup.json',cleanup);
    results.push(cleanup.every(c=>c.removed) ? {id:'disposable-cleanup',status:'passed',checks:[{id:'exact-temporary-containers-removed',status:'passed'}]} : {id:'disposable-cleanup',status:'failed',reason:'At least one temporary target could not be safely removed.',checks:[{id:'exact-temporary-containers-removed',status:'failed',reason:'Inspect cleanup evidence before manual repair.'}]});
  }
  if(!existsSync(join(root,'migration-manifest.json'))) {save('migration-manifest.json',{applied:[],source_migrations:sourceMigrations,complete:false});migration.manifest_sha256=await fileDigest(join(root,'migration-manifest.json'));}
  save('host-limits.json',{platform:platform(),arch:arch(),cpu_model:cpus()[0]?.model,cpu_count:cpus().length,memory_bytes:totalmem(),go:command('go',['version']).trim(),container_limits:{postgres_cpu:1,postgres_memory_mib:768,redis_cpu:1,redis_memory_mib:256},maximum_wall_seconds:600,scenario_timeout_seconds:profile==='capacity'?540:150});
  const artifacts=[];
  for(const name of readdirSync(root).sort()) {if(name.endsWith('.env'))continue;const path=join(root,name);artifacts.push({path:name,sha256:await fileDigest(path),bytes:statSync(path).size});}
  const version=readFileSync(join(repo,'contracts/openapi.yaml'),'utf8').match(/^  version: ([0-9.]+)$/m)?.[1];
  const manifest={schema_version:'ledgersync-demo-run/v1',purpose:'run-evidence',run_id:runId,started_at:started.toISOString(),completed_at:new Date().toISOString(),source:{commit:before.head,manifest_sha256:await fileDigest(join(root,'source-manifest.json')),dirty:before.dirty},images,migration,api_contract_version:version,isolated_project:project,limits:{duration_seconds:600,concurrency:profile==='recovery'?4:profile==='financial-model'?4:profile==='capacity'?8:64,operations:profile==='financial-model'?2048:profile==='capacity'?12000:512},scenarios:results,artifacts,limitations:['One local Docker host with synthetic test fixtures. No cloud, external funds or production SLA claim.','Go test binaries were compiled from the recorded dirty source. PostgreSQL/Redis image IDs are dependency images; no candidate API image claim.',`Selected fixed host profile: ${profile}. Read its named test output and run-profile artifact for the executed scope.`,'Failure-profile lost response is a service-boundary case. Its process-crash case terminates an actual worker child; cache loss uses an isolated Redis flush.','All normal containers and source/index identities were checked. The calling phase separately compares normal financial fingerprints.','Raw artifacts stay in ignored local storage. The browser has no fault-execution route.']};
  save('run-manifest.json',manifest);const verification=await verifyDemoArtifacts(manifest,root);const capacity=profile==='capacity'&&existsSync(join(root,'capacity-report.json'))?JSON.parse(readFileSync(join(root,'capacity-report.json'),'utf8')):null;console.log(JSON.stringify({run_id:runId,bundle:relative(repo,root).replaceAll('\\','/'),verified:verification.verified,passed:!failure,...(capacity?{capacity_qualified:capacity.qualified,calibration_only:capacity.calibration_only}:{})}));
  if(failure)throw failure;return {root,manifest};
}

if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)) {
  if(process.argv.length===3&&process.argv[2]==='--preview') console.log(JSON.stringify(demoPreview(),null,2));
  else if(process.argv.length!==2) {console.error('Usage: node scripts/portfolio/run-demo.mjs [--preview]');process.exitCode=1;}
  else runDemo().catch(()=>{console.error('Local demonstration failed. Inspect the retained private bundle; no overall pass is claimed.');process.exitCode=1;});
}
