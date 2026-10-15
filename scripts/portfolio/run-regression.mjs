import { runDemo } from './run-demo.mjs';
if (process.argv.length !== 2) throw new Error('Usage: node scripts/portfolio/run-regression.mjs');
runDemo({ profile: 'regression' }).catch(() => { console.error('Local regression failed. Inspect the retained private bundle.'); process.exitCode = 1; });
