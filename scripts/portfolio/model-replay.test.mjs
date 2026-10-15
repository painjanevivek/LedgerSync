import test from 'node:test';
import assert from 'node:assert/strict';
import { parseModelReplay } from './model-replay.mjs';
import { runDemo } from './run-demo.mjs';

const operation={kind:'transfer',minor:100,reference:0,reverse:false};
test('model replay selects the exact recorded seed and minimal operation vector',()=>{
  const report={seed:17,operations:[operation,operation],minimal_operations:[operation],error:'historical harness mismatch'};
  assert.deepEqual(JSON.parse(parseModelReplay(report)),{seed:17,operations:[operation]});
});
test('model replay rejects ambiguous or unbounded operation data',()=>{
  for(const report of [{seed:1.5,operations:[operation]},{seed:17,operations:[]},{seed:17,operations:Array(257).fill(operation)},{seed:17,operations:[{...operation,minor:0}]},{seed:17,operations:[{...operation,minor:1000001}]},{seed:17,operations:[{...operation,reference:-1}]},{seed:17,operations:[{...operation,reverse:'false'}]},{seed:17,operations:[{...operation,kind:'shell'}]},{seed:17,operations:[{...operation,command:'unexpected'}]}])assert.throws(()=>parseModelReplay(report));
});
test('foreign host profiles fail before any container or output mutation',async()=>{
  await assert.rejects(runDemo({profile:'production'}),/Unsupported bounded/);
  await assert.rejects(runDemo({profile:'failures',modelReplayJSON:'{}'}),/Unsupported bounded/);
});
