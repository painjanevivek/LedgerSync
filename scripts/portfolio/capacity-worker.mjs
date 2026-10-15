import { spawn, execFileSync } from 'node:child_process';
import { existsSync, readFileSync, writeFileSync, unlinkSync } from 'node:fs';
import { join } from 'node:path';

const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
export const hasExited=child=>child.exitCode!==null||child.signalCode!==null;

// This coordinator owns only its spawned worker and the exact guarded Redis
// target. A file can request one fixed fault; no command or target is input.
export async function withCapacityWorker(runTest,{root,workerBinary,workerEnv,redis,targets,inspect,docker,signal,abort,secrets}) {
  let child,finished=false,paused=false,intentional=false,workerOutput='',controllerError;
  const lifecycle=[],resourceSamples=[];
  const save=(name,value)=>writeFileSync(join(root,name),JSON.stringify(value,null,2)+'\n',{flag:'wx',mode:0o600});
  const log=chunk=>{if(Buffer.byteLength(workerOutput)+chunk.length>8*1024*1024){controllerError=new Error('Worker output exceeded bound');abort();return};workerOutput+=chunk.toString();};
  const stopWorker=async(force=false)=>{
    if(!child||hasExited(child))return;
    intentional=true;child.kill(force?'SIGKILL':'SIGTERM');
    for(let i=0;i<30&&!hasExited(child);i++)await delay(100);
    if(!hasExited(child))child.kill('SIGKILL');
    for(let i=0;i<20&&!hasExited(child);i++)await delay(100);
    if(!hasExited(child))throw new Error('Owned capacity worker did not stop');
  };
  const startWorker=async()=>{
    const health=workerEnv.LEDGERSYNC_WORKER_HEALTH_PATH;
    if(existsSync(health))unlinkSync(health);
    intentional=false;child=spawn(workerBinary,[],{env:workerEnv,windowsHide:true,stdio:['ignore','pipe','pipe']});
    child.stdout.on('data',log);child.stderr.on('data',log);
    child.on('error',()=>{controllerError=new Error('Owned capacity worker could not start');abort();});
    child.on('exit',()=>{if(!intentional&&!finished&&!signal.aborted){controllerError=new Error('Owned capacity worker exited unexpectedly');abort();}});
    const started=new Date().toISOString();
    for(let i=0;i<80;i++){
      if(signal.aborted||finished)throw new Error('Capacity worker startup cancelled');
      if(hasExited(child))throw new Error('Capacity worker startup failed');
      try{execFileSync(workerBinary,['--healthcheck'],{env:workerEnv,windowsHide:true,timeout:2000,stdio:'pipe'});const ready={pid:child.pid,started_at:started,ready_at:new Date().toISOString()};lifecycle.push({action:'worker_ready',...ready});return ready;}catch{await delay(500);}
    }
    throw new Error('Capacity worker readiness deadline exceeded');
  };
  const sample=()=>{
    try{resourceSamples.push({at:new Date().toISOString(),containers:docker('stats','--no-stream','--format','{{json .}}',...targets.map(t=>t.id)).split(/\r?\n/).filter(Boolean).map(line=>JSON.parse(line))});}catch{resourceSamples.push({at:new Date().toISOString(),state:'unavailable'});}
  };
  let timer;
  const controller=async()=>{
    while(!existsSync(join(root,'capacity-ready.json'))){if(finished||signal.aborted)return;await delay(100);}
    for(const target of targets)inspect(target);const ready=await startWorker();save('worker-started.json',ready);
    let pausedOnce=false,restartedOnce=false;
    timer=setInterval(sample,5000);
    while(!finished&&!signal.aborted){
      if(!pausedOnce&&existsSync(join(root,'pause-redis.requested'))){
        pausedOnce=true;for(const target of targets)inspect(target);const started=new Date().toISOString();docker('pause',redis.id);paused=true;lifecycle.push({action:'redis_paused',at:started});
        await delay(3000);inspect(redis,{allowPaused:true});docker('unpause',redis.id);paused=false;inspect(redis);const result={started_at:started,completed_at:new Date().toISOString(),requested_pause_ms:3000};save('redis-pause-result.json',result);lifecycle.push({action:'redis_resumed',at:result.completed_at});
      }
      if(!restartedOnce&&existsSync(join(root,'restart-worker.requested'))){
        restartedOnce=true;for(const target of targets)inspect(target);const started=new Date().toISOString();lifecycle.push({action:'worker_kill',at:started,pid:child.pid});await stopWorker(true);await delay(5000);const ready=await startWorker();save('worker-restart-result.json',{requested_at:started,...ready,requested_down_ms:5000});
      }
      await delay(100);
    }
  };
  const controllerPromise=controller().catch(error=>{controllerError=error;abort();});
  try{return await runTest();}
  finally{
    finished=true;if(timer)clearInterval(timer);await controllerPromise;
    try{await stopWorker();}catch(error){controllerError??=error;}
    if(paused){try{inspect(redis,{allowPaused:true});docker('unpause',redis.id);paused=false;}catch(error){controllerError??=error;}}
    for(const secret of secrets)workerOutput=workerOutput.replaceAll(secret,'[REDACTED]');
    writeFileSync(join(root,'native-worker.txt'),workerOutput,{flag:'wx',mode:0o600});save('worker-lifecycle.json',lifecycle);save('resource-samples.json',resourceSamples);
    if(controllerError)throw controllerError;
  }
}
