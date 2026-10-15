import { readFileSync, realpathSync, statSync, lstatSync } from 'node:fs';
import { dirname, resolve, relative, isAbsolute } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseModelReplay } from './model-replay.mjs';
import { runDemo } from './run-demo.mjs';

try {
  if(process.argv.length>3)throw new Error('Select at most one recorded model report');
  let modelReplayJSON;
  if(process.argv[2]) {
    const repo=realpathSync(resolve(dirname(fileURLToPath(import.meta.url)),'../..')),path=resolve(repo,process.argv[2]),offset=relative(resolve(repo,'data/portfolio'),path);
    if(isAbsolute(offset)||offset.startsWith('..')||!path.endsWith('.json')||realpathSync(path)!==path||!lstatSync(path).isFile()||statSync(path).size>1024*1024)throw new Error('Select a bounded local portfolio report');
    modelReplayJSON=parseModelReplay(JSON.parse(readFileSync(path,'utf8')));
  }
  await runDemo({profile:'financial-model',modelReplayJSON});
} catch {console.error('Local model run failed. Inspect its retained private evidence; no overall pass is claimed.');process.exitCode=1;}
