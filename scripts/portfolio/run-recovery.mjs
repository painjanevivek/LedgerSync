import { runDemo } from './run-demo.mjs';
if(process.argv.length!==2){console.error('Usage: node scripts/portfolio/run-recovery.mjs');process.exitCode=1;}
else await runDemo({profile:'recovery'}).catch(()=>{console.error('Candidate recovery failed. Inspect retained private evidence; no restore success is claimed.');process.exitCode=1;});
