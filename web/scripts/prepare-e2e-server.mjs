import { cp } from "node:fs/promises";

// Next.js intentionally leaves static assets outside standalone output. The
// production-like E2E server needs the same copy performed by deployment.
await cp(".next/static", ".next/standalone/.next/static", { recursive: true });
