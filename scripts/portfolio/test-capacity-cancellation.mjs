import {spawn} from 'node:child_process';
import {existsSync,readFileSync,readdirSync,writeFileSync,realpathSync} from 'node:fs';
import {dirname,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
import {verifyDemoArtifacts} from './demo-safety.mjs';

if(process.argv.length!==2)throw new Error('This check accepts no target or command input');
const repo=realpathSync(resolve(dirname(fileURLToPath(import.meta.url)),'../..')),base=resolve(repo,'data/portfolio/demo-runs');
const prior=new Set(existsSync(base)?readdirSync(base):[]),child=spawn(process.execPath,[resolve(repo,'scripts/portfolio/run-capacity.mjs'),'--batch=10','--rate=10','--steady=300'],{cwd:repo,windowsHide:true,stdio:['ignore','pipe','pipe']});
let output='';const collect=chunk=>{output+=chunk;if(Buffer.byteLength(output)>1024*1024)child.kill('SIGTERM');};child.stdout.on('data',collect);child.stderr.on('data',collect);
const exited=new Promise((resolveResult,reject)=>{child.once('close',resolveResult);child.once('error',reject);});
let root,workerPID;
try{
 for(let attempt=0;attempt<600;attempt++){
  for(const name of existsSync(base)?readdirSync(base):[]){
   if(prior.has(name)||!/^demo-\d{14}-[a-f0-9]{8}$/.test(name))continue;
   const candidate=resolve(base,name),profilePath=resolve(candidate,'run-profile.json');if(!existsSync(profilePath))continue;
   const profile=JSON.parse(readFileSync(profilePath,'utf8'));if(profile.profile==='capacity'&&profile.host_pid===child.pid){assert.ok(!root||root===candidate,'one owned run is required');root=candidate;}
  }
  if(root&&existsSync(resolve(root,'worker-started.json'))){workerPID=JSON.parse(readFileSync(resolve(root,'worker-started.json'),'utf8')).pid;break;}
  if(child.exitCode!==null||child.signalCode!==null)break;
  await new Promise(resolve=>setTimeout(resolve,250));
 }
 assert.ok(root&&Number.isSafeInteger(workerPID)&&workerPID>0,'owned capacity worker did not become ready within 150 seconds');assert.equal(realpathSync(root),root);
 process.kill(workerPID,0);writeFileSync(resolve(root,'cancel.requested'),'host-only capacity cancellation verification\n',{flag:'wx'});
 const code=await Promise.race([exited,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('Capacity cancellation deadline exceeded')),60000);timer.unref();})]);
 assert.notEqual(code,0);const manifest=JSON.parse(readFileSync(resolve(root,'run-manifest.json'),'utf8'));
 assert.ok(manifest.scenarios.some(s=>s.id==='bounded-capacity'&&s.status!=='passed'));assert.ok(manifest.scenarios.some(s=>s.id==='runner-completion'&&s.status==='failed'));
 assert.equal(JSON.parse(readFileSync(resolve(root,'failure.json'),'utf8')).cancelled,true);assert.ok(JSON.parse(readFileSync(resolve(root,'cleanup.json'),'utf8')).every(s=>s.removed));
 assert.throws(()=>process.kill(workerPID,0),error=>error.code==='ESRCH','owned native worker must be gone');await verifyDemoArtifacts(manifest,root);
 console.log(JSON.stringify({cancelled:true,run_id:manifest.run_id,worker_started:true,worker_stopped:true,cleanup:'verified',capacity_qualified:false,child_exit:code,output_bytes:Buffer.byteLength(output)}));
}finally{
 if(child.exitCode===null&&child.signalCode===null){if(root&&!existsSync(resolve(root,'cancel.requested')))writeFileSync(resolve(root,'cancel.requested'),'failed check requests owned cleanup\n',{flag:'wx'});child.kill('SIGTERM');}
}
