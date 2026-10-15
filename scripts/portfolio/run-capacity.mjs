import { parseCapacityOptions } from './capacity-options.mjs';
import { runDemo } from './run-demo.mjs';
try{await runDemo({profile:'capacity',...parseCapacityOptions(process.argv.slice(2))});}
catch{console.error('Local capacity run failed. Inspect retained private evidence; no capacity or overall pass is claimed.');process.exitCode=1;}
