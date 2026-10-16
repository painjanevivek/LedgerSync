export async function waitForCandidateHealth(targets, inspect, { signal, timeoutMilliseconds=180000, pollMilliseconds=500 }={}) {
  if(!Array.isArray(targets)||targets.length===0||typeof inspect!=='function'||timeoutMilliseconds<1||timeoutMilliseconds>180000||pollMilliseconds<1||pollMilliseconds>1000)throw Error('Bounded exact candidate health inputs required');
  const started=Date.now(); let samples=0,last=[];
  while(true){
    signal?.throwIfAborted();
    last=targets.map(target=>{const value=inspect(target);if(!value.State?.Running||value.State.Paused||!value.State.Health)throw Error('Candidate health target stopped, paused or has no probe');return {service:target.service,status:value.State.Health.Status};});
    samples++;
    if(last.every(item=>item.status==='healthy'))return {status:'verified_local',samples,elapsed_ms:Date.now()-started,services:last};
    if(Date.now()-started>=timeoutMilliseconds)throw Error('Candidate health recovery deadline exceeded');
    await new Promise(resolve=>setTimeout(resolve,Math.min(pollMilliseconds,timeoutMilliseconds-(Date.now()-started))));
  }
}

export function classifyCandidateShutdown(service,state) {
  const web=['web','web-peer'].includes(service),native=['api','outbox-worker'].includes(service);
  if((!web&&!native)||!state||state.Running||state.Paused||state.OOMKilled||!(state.ExitCode===0||web&&state.ExitCode===143))throw Error('Unexpected candidate shutdown: '+service+' exit='+state?.ExitCode);
  return {status:'verified_local',exit_code:state.ExitCode,oom_killed:false,termination:web&&state.ExitCode===143?'next_sigterm_cleanup':'normal_exit'};
}
