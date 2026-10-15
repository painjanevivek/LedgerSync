import { runDemo } from './run-demo.mjs';
if (process.argv.length !== 2) { console.error('Usage: node scripts/portfolio/run-optional.mjs'); process.exitCode = 1; }
else await runDemo({ profile: 'optional-features' }).catch(() => { console.error('Optional feature qualification failed. Retained private evidence records the failed check.'); process.exitCode = 1; });
