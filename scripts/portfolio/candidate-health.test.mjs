import test from 'node:test';
import assert from 'node:assert/strict';
import {waitForCandidateHealth,classifyCandidateShutdown} from './candidate-health.mjs';
const target=[{service:'postgres',id:'exact-temporary-target'}];
test('candidate recovery waits for a fresh healthy probe after a stale unhealthy result',async()=>{
 let reads=0;const result=await waitForCandidateHealth(target,()=>({State:{Running:true,Paused:false,Health:{Status:++reads===1?'unhealthy':'healthy'}}}),{timeoutMilliseconds:200,pollMilliseconds:1});
 assert.equal(result.status,'verified_local');assert.equal(result.samples,2);assert.equal(reads,2);
});
test('candidate recovery fails on a bounded persistent failure or a stopped target',async()=>{
 await assert.rejects(waitForCandidateHealth(target,()=>({State:{Running:true,Health:{Status:'unhealthy'}}}),{timeoutMilliseconds:5,pollMilliseconds:1}),/deadline/);
 await assert.rejects(waitForCandidateHealth(target,()=>({State:{Running:false}})),/stopped/);
});
test('candidate recovery preserves identity-verifier failures and cancellation',async()=>{
 await assert.rejects(waitForCandidateHealth(target,()=>{throw Error('identity mismatch');}),/identity mismatch/);
 const controller=new AbortController();controller.abort();await assert.rejects(waitForCandidateHealth(target,()=>assert.fail(),{signal:controller.signal}),{name:'AbortError'});
});
test('candidate shutdown recognizes documented Next SIGTERM cleanup and rejects force or OOM',()=>{
 assert.equal(classifyCandidateShutdown('web',{Running:false,OOMKilled:false,ExitCode:143}).termination,'next_sigterm_cleanup');
 assert.equal(classifyCandidateShutdown('api',{Running:false,OOMKilled:false,ExitCode:0}).termination,'normal_exit');
 for(const [service,state]of [['web',{Running:false,ExitCode:137}],['api',{Running:false,ExitCode:143}],['outbox-worker',{Running:false,OOMKilled:true,ExitCode:0}],['web',{Running:true,ExitCode:0}]])assert.throws(()=>classifyCandidateShutdown(service,state));
});
