import {execFileSync} from 'node:child_process';
import {readFileSync,writeFileSync} from 'node:fs';
import {join} from 'node:path';
import {fileDigest,demoPurpose} from './demo-safety.mjs';
import {candidateBackupFormat,candidateRoleNames,validateCandidateBackup} from './candidate-backup.mjs';

// Called only by the fixed recovery profile after its seed test passed. No
// caller-supplied restore destination, shell command or backup path is accepted.
export async function restoreCandidate({root,repo,project,pg,redis,targets,inspect,docker,save,env,password,signal,boundedProcess}){
 for(const target of targets)if(inspect(target)!==target.port)throw new Error('Recovery source target changed');
 const backupStarted=new Date();
 const privileged=docker('exec',pg.id,'psql','-X','-U','ledgersync','-d','ledgersync','-At','-c',"SELECT current_user='ledgersync' AND rolsuper FROM pg_roles WHERE rolname=current_user");
 if(privileged!=='t')throw new Error('Candidate backup requires the explicit isolated owner identity; filtered backups are forbidden');
 const roleSQL=`SELECT COALESCE(json_agg(rolname ORDER BY rolname),'[]') FROM pg_roles WHERE rolname IN (${candidateRoleNames.map(name=>`'${name}'`).join(',')}) AND NOT rolcanlogin AND NOT rolsuper AND NOT rolbypassrls AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication`;
 const roleNames=JSON.parse(docker('exec',pg.id,'psql','-X','-U','ledgersync','-d','ledgersync','-At','-c',roleSQL));
 const dump=execFileSync('docker',['exec',pg.id,'pg_dump','-U','ledgersync','-d','ledgersync','--format=custom','--no-owner','--no-acl'],{cwd:repo,windowsHide:true,timeout:60000,maxBuffer:128*1024*1024});
 writeFileSync(join(root,'database.dump'),dump,{flag:'wx',mode:0o600});
 const expected={source:await fileDigest(join(root,'source-manifest.json')),migrations:await fileDigest(join(root,'migration-manifest.json'))};
 save('candidate-backup.json',{format_version:candidateBackupFormat,complete:true,created_at:new Date().toISOString(),source_manifest_sha256:expected.source,migration_manifest_sha256:expected.migrations,currencies:['USD'],role_names:roleNames,database:{file_name:'database.dump',bytes:dump.length,sha256:await fileDigest(join(root,'database.dump'))}});
 const verified=await validateCandidateBackup(root,expected),backupCompleted=new Date();
 if(signal.aborted)throw new Error('Recovery cancelled before restore target creation');
 const restoreStarted=new Date();
 const id=docker('run','-d','--name',`${project}-restored-postgres`,'--label',`com.docker.compose.project=${project}`,'--label',`ledgersync.purpose=${demoPurpose}`,'--cpus','1','--memory','768m','--publish','127.0.0.1::5432','--tmpfs','/var/lib/postgresql/data:rw,size=512m','--env-file',join(root,'database.env'),pg.image);
 const restored={project,id,image:pg.image,service:'postgres'};targets.push(restored);restored.port=inspect(restored);
 save('restored-targets.json',[restored,redis]);
 let ready=false;for(let i=0;i<40;i++){if(signal.aborted)throw new Error('Recovery target startup cancelled');try{docker('exec',id,'pg_isready','-U','ledgersync','-d','ledgersync');ready=true;break;}catch{await new Promise(resolve=>setTimeout(resolve,250));}}
 if(!ready)throw new Error('Recovery target readiness deadline exceeded');
 const psql=input=>execFileSync('docker',['exec','-i',id,'psql','--single-transaction','-v','ON_ERROR_STOP=1','-X','-U','ledgersync','-d','ledgersync','-f','-'],{cwd:repo,input,encoding:'utf8',windowsHide:true,timeout:60000,maxBuffer:8*1024*1024});
 save('restore-role-identities.txt',psql(candidateRoleNames.map(name=>`CREATE ROLE ${name} NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;`).join('\n')));
 // No clients can run while owner/ACL restoration is in progress. Policy roles
 // must exist before pg_restore; executable grants follow schema restoration.
 const restoredOutput=execFileSync('docker',['exec','-i',id,'pg_restore','-U','ledgersync','-d','ledgersync','--no-owner','--no-acl','--exit-on-error'],{cwd:repo,input:readFileSync(verified.path),encoding:'utf8',windowsHide:true,timeout:60000,maxBuffer:8*1024*1024});save('pg-restore.txt',restoredOutput);
 const address=new URL(env.LEDGERSYNC_TEST_DATABASE_URL);address.port=String(restored.port);
 const restoredEnv={...env,LEDGERSYNC_DATABASE_URL:address.href,LEDGERSYNC_TEST_DATABASE_URL:address.href,LEDGERSYNC_PORTFOLIO_TARGETS_FILE:'restored-targets.json'};
 const migration=await boundedProcess('go',['run','./cmd/migrate'],restoredEnv,signal,password);save('restored-migrate.txt',migration.output+migration.errorOutput);if(migration.code!==0)throw new Error('Restored candidate migration validation failed');
 save('restored-roles.txt',psql(readFileSync(join(repo,'deploy/postgres/roles.sql'))));
 const after=inspect(restored);if(after!==restored.port)throw new Error('Restored target binding changed');
 save('candidate-restore.json',{state:'restored-awaiting-financial-validation',backup_started_at:backupStarted.toISOString(),backup_completed_at:backupCompleted.toISOString(),restore_started_at:restoreStarted.toISOString(),schema_and_grants_ready_at:new Date().toISOString(),backup_seconds:(backupCompleted-backupStarted)/1000,restore_setup_seconds:(Date.now()-restoreStarted)/1000,dump_sha256:verified.manifest.database.sha256,source_manifest_sha256:expected.source,migration_manifest_sha256:expected.migrations,source_database_id:pg.id,restored_database_id:id,source_and_target_distinct:pg.id!==id,rows_filtered:false,backup_identity:'isolated owner/superuser',cache_source:'authoritative restored PostgreSQL; Redis is disposable'});
 return restoredEnv;
}
