import { readFileSync,readdirSync } from 'node:fs';
import { resolve,dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..'),require=createRequire(import.meta.url),YAML=require('../../web/node_modules/yaml');
export function inspectProductionPackage(directory=resolve(root,'deploy/production')){
 const inputs=JSON.parse(readFileSync(resolve(directory,'inputs.example.json'),'utf8')),collector=JSON.parse(readFileSync(resolve(directory,'collector-image.json'),'utf8')),compose=YAML.parse(readFileSync(resolve(directory,'compose.yaml'),'utf8')),telemetry=YAML.parse(readFileSync(resolve(directory,'otel-collector.yaml'),'utf8'));
 if(inputs.schema_version!==1||inputs.status!=='prepared_not_deployed'||inputs.execution_scope!=='local_only')throw Error('Explicit local-only package status required');
 const pending=['provider','account_or_project','region','public_domain','monthly_budget','currency','operating_owner','incident_owner','identity_issuer','backup_owner','recovery_point_target','recovery_time_target'];
 if(pending.some(key=>!(key in inputs)))throw Error('Required owner/deployment input absent');
 if(!/^otel\/opentelemetry-collector-contrib:0\.161\.0@sha256:[a-f0-9]{64}$/.test(collector.image))throw Error('Reviewed immutable collector identity required');
 const names=Object.keys(compose.services??{}).sort();if(JSON.stringify(names)!==JSON.stringify(['api','migrate','web','worker']))throw Error('Unexpected production workload set');
 for(const [name,service]of Object.entries(compose.services)){
  if(service.build||!/^\$\{LEDGERSYNC_RELEASE_[A-Z_]+_IMAGE:\?/.test(service.image??'')||!/^\$\{LEDGERSYNC_PRODUCTION_[A-Z_]+_ENV:\?/.test(service.env_file??''))throw Error(`${name}: verified image and private environment inputs required`);
  if(!service.read_only||!service.cap_drop?.includes('ALL')||!service.security_opt?.includes('no-new-privileges:true')||!service.user||['root','0'].includes(service.user)||!service.mem_limit||service.cpus!==1||service.pids_limit!==128||!service.tmpfs?.length||service.stop_grace_period!=='30s')throw Error(`${name}: bounded restricted workload required`);
  if(service.volumes?.length||service.privileged||service.network_mode==='host')throw Error(`${name}: unexpected host authority`);
  if(service.ports?.some(port=>typeof port!=='string'||!port.startsWith('127.0.0.1:')))throw Error(`${name}: public port must not be supplied by this local package`);
  if(name!=='migrate'&&!service.healthcheck)throw Error(`${name}: health probe required`);
  if(name!=='web'&&service.environment.LEDGERSYNC_ENV!=='production')throw Error(`${name}: production validation cannot be disabled`);
  if(name!=='migrate'&&(service.environment.LEDGERSYNC_STUDY_ENABLED!=='false'||service.environment.LEDGERSYNC_LIVE_INVESTIGATION_ENABLED!=='false'))throw Error(`${name}: core feature set required`);
 }
 if(compose.services.migrate.profiles?.[0]!=='migration'||compose.services.migrate.restart!=='no'||/seed/i.test(JSON.stringify(compose)))throw Error('Migration must be explicit, one-shot and seed-free');
 if(telemetry.exporters?.['otlphttp/traces']?.tls?.insecure!==false||!telemetry.receivers?.otlp?.protocols?.http?.tls?.cert_file||!telemetry.processors?.['transform/privacy']||!telemetry.service?.pipelines?.traces?.processors?.includes('transform/privacy')||telemetry.processors?.['filter/private_events']?.traces?.spanevent?.[0]!=='true'||!telemetry.service?.pipelines?.traces?.processors?.includes('filter/private_events'))throw Error('Private TLS telemetry, privacy processing and event removal required');
 const migrations=readdirSync(resolve(root,'migrations')).filter(name=>/^\d{6}.*\.up\.sql$/.test(name)).sort();
 const version=/const Version = "([^"]+)"/.exec(readFileSync(resolve(root,'contracts/assets.go'),'utf8'))?.[1];
 return {status:'prepared_not_deployed',execution_scope:'local_only',deployment_ready:false,pending_inputs:pending.filter(key=>inputs[key]===null),workloads:names,collector_image:collector.image,latest_migration:migrations.at(-1),source_migration_count:migrations.length,contract_version:version,limits:['Static package validation does not establish managed identity, network, backup, failover or operator acceptance.','The normal local database is preserved; source migration count is not its history count.']};
}
