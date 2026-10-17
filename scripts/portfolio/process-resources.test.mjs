import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { sampleOwnedProcess } from './process-resources.mjs';

test('native resource samples bind a live child to its executable and creation identity', async()=>{
 const child=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{windowsHide:true,stdio:'ignore'});
 try {
  await once(child,'spawn');
  const value=sampleOwnedProcess(child,process.execPath);
  assert.equal(value.status,'sampled');assert.equal(value.pid,child.pid);assert.ok(value.rss_bytes>0);assert.ok(value.cpu_seconds>=0);assert.match(value.creation_identity,/^\d+$/);
  assert.equal(sampleOwnedProcess(child,'missing-foreign-binary').status,'unavailable');
  const exited=once(child,'exit');child.kill('SIGKILL');await exited;
  assert.equal(sampleOwnedProcess(child,process.execPath).status,'unavailable');
 } finally {if(child.exitCode===null&&child.signalCode===null)child.kill('SIGKILL');}
});
