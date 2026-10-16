import { runDemo } from './run-demo.mjs';
if(process.argv.length!==2)throw new Error('Usage: node scripts/portfolio/run-statement.mjs');
runDemo({profile:'statement-reconciliation'}).catch(()=>{console.error('Local statement qualification failed. Keep the private bundle; no overall pass is claimed.');process.exitCode=1;});
