import test from 'node:test';
import assert from 'node:assert/strict';
import { parseCapacityOptions } from './capacity-options.mjs';
test('capacity options keep measured variants and intervals bounded',()=>{
 assert.deepEqual(parseCapacityOptions([]),{outboxBatchSize:10,capacitySteadySeconds:300,capacityRate:30,capacityProfile:'mixed',capacityArrivalMode:'open-loop'});
 assert.deepEqual(parseCapacityOptions(['--batch=50','--steady=10']),{outboxBatchSize:50,capacitySteadySeconds:10,capacityRate:30,capacityProfile:'mixed',capacityArrivalMode:'open-loop'});
 for(const args of [['--batch=1000'],['--steady=0'],['--rate=1000'],['--steady=900'],['--batch=10','--batch=50'],['--target=compose'],['--command=anything'],['--batch=50;echo']])assert.throws(()=>parseCapacityOptions(args));
});
test('profile and arrival selection cannot invent tenants or unbounded traffic',()=>{
 const selected=parseCapacityOptions(['--profile=hot-pair','--arrival=closed-clients']);
 assert.equal(selected.capacityProfile,'hot-pair');assert.equal(selected.capacityArrivalMode,'closed-clients');
 for(const args of [['--profile=tenant-a'],['--profile=mixed','--profile=hot-pair'],['--arrival=unbounded'],['--arrival=open-loop;echo']])assert.throws(()=>parseCapacityOptions(args));
});
