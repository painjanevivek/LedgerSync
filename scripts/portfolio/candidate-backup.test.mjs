import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,dirname,basename} from 'node:path';
import {createHash} from 'node:crypto';
import {candidateBackupFormat,candidateRoleNames,validateCandidateBackup} from './candidate-backup.mjs';

test('candidate backup rejects incomplete, stale, truncated and corrupt evidence before restore',async()=>{
 const root=mkdtempSync(join(tmpdir(),'ledger-candidate-backup-'));
 try{
  const dump=Buffer.from('PGDMPsynthetic-unit-fixture'),expected={source:'a'.repeat(64),migrations:'b'.repeat(64)};
  const manifest={format_version:candidateBackupFormat,complete:true,created_at:new Date().toISOString(),source_manifest_sha256:expected.source,migration_manifest_sha256:expected.migrations,currencies:['USD'],role_names:candidateRoleNames,database:{file_name:'database.dump',bytes:dump.length,sha256:createHash('sha256').update(dump).digest('hex')}};
  const save=value=>writeFileSync(join(root,'candidate-backup.json'),JSON.stringify(value));writeFileSync(join(root,'database.dump'),dump);save(manifest);
  assert.equal((await validateCandidateBackup(root,expected)).path,join(root,'database.dump'));
  for(const alter of [v=>v.complete=false,v=>v.format_version='unknown',v=>v.extra=true,v=>delete v.source_manifest_sha256,v=>v.source_manifest_sha256='c'.repeat(64),v=>v.migration_manifest_sha256='d'.repeat(64),v=>v.role_names=[],v=>v.role_names.push('arbitrary_owner'),v=>v.currencies=['INR'],v=>v.database.file_name='../outside',v=>v.database.bytes++,v=>v.database.bytes=129*1024*1024,v=>v.database.sha256='e'.repeat(64)]){const value=structuredClone(manifest);alter(value);save(value);await assert.rejects(validateCandidateBackup(root,expected));}
  const legacy=structuredClone(manifest);legacy.format_version='ledgersync-isolated-candidate-backup/v1';legacy.role_names=legacy.role_names.filter(name=>name!=='ledgersync_statement_owner');save(legacy);assert.equal((await validateCandidateBackup(root,expected)).manifest.complete,true);
 const inr=structuredClone(manifest);inr.currencies=['INR'];save(inr);assert.equal((await validateCandidateBackup(root,{...expected,currency:'INR'})).manifest.complete,true);await assert.rejects(validateCandidateBackup(root,expected));
 legacy.currencies=['INR'];save(legacy);await assert.rejects(validateCandidateBackup(root,{...expected,currency:'INR'}));
 save(manifest);writeFileSync(join(root,'database.dump'),dump.subarray(0,10));await assert.rejects(validateCandidateBackup(root,expected),/length or checksum/);
  const corrupted=Buffer.from(dump);corrupted[10]^=1;writeFileSync(join(root,'database.dump'),corrupted);await assert.rejects(validateCandidateBackup(root,expected),/length or checksum/);
  writeFileSync(join(root,'database.dump'),dump);assert.deepEqual(readFileSync(join(root,'database.dump')),dump);assert.equal((await validateCandidateBackup(root,expected)).manifest.complete,true);
 }finally{assert.equal(dirname(root),tmpdir());assert.match(basename(root),/^ledger-candidate-backup-/);rmSync(root,{recursive:true,force:true});}
});
