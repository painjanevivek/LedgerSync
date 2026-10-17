import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { checkHumanAcceptance, humanAcceptanceTasks } from './human-acceptance.mjs';

const fixture=()=>({schema:'ledgersync-human-acceptance/v1',purpose:'human_observations',source_manifest_sha256:'1'.repeat(64),reviewed_at:'2026-10-17T12:00:00Z',reviewer:{reference:'reviewer-test-fixture',unfamiliar_with_project:true,is_author:false,human_attestation:true},reviewer_explanation:'Synthetic unit fixture only. This is not real human acceptance.',tasks:humanAcceptanceTasks.map(id=>({id,result:'passed',help_required:false,elapsed_seconds:10,observation:'Synthetic unit fixture observation.'}))});
test('a complete record never authenticates its author or grants acceptance',()=>{
 const result=checkHumanAcceptance(fixture());assert.equal(result.tasks_complete,true);assert.equal(result.acceptance,'not_automatically_granted');
 const partial=fixture();partial.tasks[2].result='not_run';assert.equal(checkHumanAcceptance(partial).tasks_complete,false);
 partial.tasks[2].result='passed';partial.tasks[2].help_required=true;assert.equal(checkHumanAcceptance(partial).tasks_complete,false);
});
test('templates, author review and missing/duplicate tasks cannot substitute for a reviewer',()=>{
 assert.throws(()=>checkHumanAcceptance(JSON.parse(readFileSync(new URL('../../contracts/portfolio/human-acceptance.template.json',import.meta.url),'utf8'))));
 for(const edit of [r=>{r.reviewer.is_author=true;},r=>{r.reviewer.human_attestation=false;},r=>r.tasks.pop(),r=>{r.tasks[1]=r.tasks[0];},r=>{r.reviewer_explanation='';}]){const r=fixture();edit(r);assert.throws(()=>checkHumanAcceptance(r));}
});
