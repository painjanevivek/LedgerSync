import { readFileSync,statSync,openSync,readSync,closeSync } from 'node:fs';
import { bundlePath,fileDigest } from './demo-safety.mjs';

export const candidateBackupFormat='ledgersync-isolated-candidate-backup/v1';
export const candidateRoleNames=['ledgersync_api','ledgersync_break_glass','ledgersync_maintenance','ledgersync_maintenance_owner','ledgersync_migration_owner','ledgersync_preflight_owner','ledgersync_provisioning','ledgersync_queue_owner','ledgersync_reconciliation','ledgersync_scheduler','ledgersync_support_readonly','ledgersync_worker'];
const keys=(value,expected)=>value&&typeof value==='object'&&!Array.isArray(value)&&JSON.stringify(Object.keys(value).sort())===JSON.stringify([...expected].sort());
const digest=value=>typeof value==='string'&&/^[a-f0-9]{64}$/.test(value);

// This candidate format is distinct from historical normal-stack backup v1.
// Exact source and migration manifests are required before a target is created.
export async function validateCandidateBackup(root,expected){
 const manifestPath=bundlePath(root,'candidate-backup.json');if(statSync(manifestPath).size>16384)throw new Error('Candidate backup metadata exceeds bound');
 const manifest=JSON.parse(readFileSync(manifestPath,'utf8'));
 if(!keys(manifest,['format_version','complete','created_at','source_manifest_sha256','migration_manifest_sha256','currencies','role_names','database'])||manifest.format_version!==candidateBackupFormat||manifest.complete!==true||typeof manifest.created_at!=='string'||!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/.test(manifest.created_at)||!Number.isFinite(Date.parse(manifest.created_at)))throw new Error('Incomplete or incompatible candidate backup metadata');
 if(!digest(expected.source)||!digest(expected.migrations)||manifest.source_manifest_sha256!==expected.source||manifest.migration_manifest_sha256!==expected.migrations)throw new Error('Candidate source or schema identity differs');
 if(JSON.stringify(manifest.currencies)!=='["USD"]'||!Array.isArray(manifest.role_names)||new Set(manifest.role_names).size!==manifest.role_names.length||JSON.stringify([...manifest.role_names].sort())!==JSON.stringify(candidateRoleNames))throw new Error('Unsupported candidate backup scope');
 const database=manifest.database;if(!keys(database,['file_name','bytes','sha256'])||database.file_name!=='database.dump'||!Number.isSafeInteger(database.bytes)||database.bytes<5||database.bytes>128*1024*1024||!digest(database.sha256))throw new Error('Invalid bounded candidate dump identity');
 const path=bundlePath(root,database.file_name);if(statSync(path).size!==database.bytes||await fileDigest(path)!==database.sha256)throw new Error('Candidate dump length or checksum differs');
 const file=openSync(path,'r');try{const magic=Buffer.alloc(5);if(readSync(file,magic,0,5,0)!==5||magic.toString('ascii')!=='PGDMP')throw new Error('Candidate dump is not PostgreSQL custom format');}finally{closeSync(file);}
 return {manifest,path};
}
