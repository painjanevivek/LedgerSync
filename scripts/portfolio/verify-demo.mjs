import { readFileSync, realpathSync, existsSync, mkdirSync, writeFileSync, lstatSync } from 'node:fs';
import { dirname, resolve, relative, isAbsolute } from 'node:path';
import { fileURLToPath } from 'node:url';
import { readDemoManifest } from './demo-contract.mjs';
import { publicDemoSummary, verifyDemoArtifacts, canSaveFailureSummary } from './demo-safety.mjs';

const repo=realpathSync(resolve(dirname(fileURLToPath(import.meta.url)),'../..'));
const root=resolve(repo,process.argv[2]??'');
const offset=relative(resolve(repo,'data/portfolio/demo-runs'),root).replaceAll('\\','/');
if(process.argv.length<3||process.argv.length>4||process.argv[3]&&process.argv[3]!=='--save-summary'||isAbsolute(offset)||!/^demo-\d{14}-[a-f0-9]{8}$/.test(offset)||realpathSync(root)!==root)throw new Error('Select one exact local demonstration bundle');
const manifest=readDemoManifest(resolve(root,'run-manifest.json')),verification=await verifyDemoArtifacts(manifest,root);
if(manifest.run_id!==offset)throw new Error('Run identity does not match selected bundle');
const summary=publicDemoSummary(manifest,verification);
if(process.argv[3]==='--save-summary') {
  const profile=JSON.parse(readFileSync(resolve(root,'run-profile.json'),'utf8'));
  if(!canSaveFailureSummary(profile,manifest))throw new Error('Only the fixed failure profile can update the Systems Lab summary');
  const destination=resolve(repo,'docs/release-evidence/portfolio/r07');mkdirSync(destination,{recursive:true});
  if(lstatSync(destination).isSymbolicLink()||realpathSync(destination)!==destination)throw new Error('Unsafe summary destination');
  const content=JSON.stringify(summary,null,2)+'\n',archive=resolve(destination,`${manifest.run_id}.json`),latest=resolve(destination,'latest-demo.json');
  if(existsSync(archive)&&readFileSync(archive,'utf8')!==content)throw new Error('Immutable summary already exists with different content');
  if(!existsSync(archive))writeFileSync(archive,content,{flag:'wx'});
  if(existsSync(latest)){const prior=JSON.parse(readFileSync(latest,'utf8'));if(!/^demo-\d{14}-[a-f0-9]{8}$/.test(prior.run_id)||!existsSync(resolve(destination,`${prior.run_id}.json`)))throw new Error('Previous summary must remain archived');}
  writeFileSync(latest,content);
  writeFileSync(resolve(repo,'web/src/lib/portfolio-demo.generated.json'),content);
}
console.log(JSON.stringify({run_id:manifest.run_id,...verification,passed:summary.passed}));
