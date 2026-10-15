import { runDemo } from './run-demo.mjs';
if (process.argv.length !== 2) { console.error('Usage: node scripts/portfolio/run-developer.mjs'); process.exitCode = 1; }
else await runDemo({ profile: 'developer-integration' }).catch(() => { console.error('Independent local integration failed. Inspect the retained private bundle. No integration pass is claimed.'); process.exitCode = 1; });
