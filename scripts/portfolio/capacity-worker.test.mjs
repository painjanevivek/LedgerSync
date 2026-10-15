import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { hasExited } from './capacity-worker.mjs';

test('a terminated owned child is recognized after signal exit',async()=>{
 const child=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{windowsHide:true,stdio:'ignore'});
 try{await once(child,'spawn');assert.equal(hasExited(child),false);const exited=once(child,'exit');child.kill('SIGKILL');await exited;assert.equal(hasExited(child),true);}
 finally{if(!hasExited(child))child.kill('SIGKILL');}
});
