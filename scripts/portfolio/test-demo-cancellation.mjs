import { spawn } from 'node:child_process';
import { existsSync, readFileSync, readdirSync, writeFileSync, realpathSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
import { verifyDemoArtifacts } from './demo-safety.mjs';

const repo=realpathSync(resolve(dirname(fileURLToPath(import.meta.url)),'../..')),base=resolve(repo,'data/portfolio/demo-runs');
const prior=new Set(existsSync(base)?readdirSync(base):[]),child=spawn(process.execPath,[resolve(repo,'scripts/portfolio/run-demo.mjs')],{cwd:repo,windowsHide:true,stdio:['ignore','pipe','pipe']});
let output='';child.stdout.on('data',b=>output+=b);child.stderr.on('data',b=>output+=b);
const exited=new Promise((resolveResult,reject)=>{child.once('close',resolveResult);child.once('error',reject);});
let root;
for(let i=0;i<120;i++){
  for(const name of existsSync(base)?readdirSync(base):[])if(!prior.has(name)&&/^demo-\d{14}-[a-f0-9]{8}$/.test(name)&&existsSync(resolve(base,name,'isolated-targets.json')))root=resolve(base,name);
  if(root)break;await new Promise(r=>setTimeout(r,250));
}
assert.ok(root,'runner did not create a new isolated target within 30 seconds');assert.equal(realpathSync(root),root);
writeFileSync(resolve(root,'cancel.requested'),'host-only cancellation verification\n',{flag:'wx'});
const code=await Promise.race([exited,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('Cancellation deadline exceeded')),30000);timer.unref();})]);
assert.notEqual(code,0);const manifest=JSON.parse(readFileSync(resolve(root,'run-manifest.json'),'utf8'));
assert.ok(manifest.scenarios.some(s=>s.status==='incomplete'));assert.ok(manifest.scenarios.some(s=>s.id==='runner-completion'&&s.status==='failed'));
assert.ok(JSON.parse(readFileSync(resolve(root,'failure.json'),'utf8')).cancelled);
assert.ok(JSON.parse(readFileSync(resolve(root,'cleanup.json'),'utf8')).every(s=>s.removed));
await verifyDemoArtifacts(manifest,root);
console.log(JSON.stringify({cancelled:true,run_id:manifest.run_id,cleanup:'verified',runtime_pass_claimed:false,child_exit:code,output_bytes:Buffer.byteLength(output)}));
