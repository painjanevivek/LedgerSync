console.error("Refusing to deploy LedgerSync from the repository root.");
console.error('This directory contains the long-running Go API and is not the Vercel frontend.');
console.error('Set the frontend Vercel project Root Directory to "web" and redeploy.');
process.exit(1);
