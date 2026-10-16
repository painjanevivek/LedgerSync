import { inspectProductionPackage } from './production-package.mjs';
if(process.argv.length>3||process.argv[2]&&process.argv[2]!=='--require-ready')throw Error('Usage: node scripts/portfolio/verify-production-package.mjs [--require-ready]');
const result=inspectProductionPackage();console.log(JSON.stringify(result,null,2));
if(process.argv[2]==='--require-ready'&&!result.deployment_ready){console.error('Managed deployment inputs and acceptance remain pending. No deployment was attempted.');process.exitCode=1;}
