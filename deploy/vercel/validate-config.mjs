import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const repositoryRoot = path.resolve(scriptDirectory, "..", "..");

const readJSON = async (relativePath) =>
  JSON.parse(await readFile(path.join(repositoryRoot, relativePath), "utf8"));

const assertEqual = (actual, expected, label) => {
  if (actual !== expected) {
    throw new Error(`${label}: expected ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}`);
  }
};

const rootConfig = await readJSON("vercel.json");
const webConfig = await readJSON("web/vercel.json");
const webPackage = await readJSON("web/package.json");

assertEqual(rootConfig.framework, "go", "repository-root framework");
assertEqual(
  rootConfig.buildCommand,
  "go build -o server ./cmd/api",
  "repository-root backend build command",
);
assertEqual(rootConfig.crons?.length, 1, "backend cron count");
assertEqual(rootConfig.crons?.[0]?.path, "/internal/cron/drain", "backend cron route");
assertEqual(rootConfig.crons?.[0]?.schedule, "* * * * *", "backend cron schedule");

assertEqual(webConfig.framework, "nextjs", "frontend framework");
assertEqual(webConfig.installCommand, "npm ci", "frontend install command");
assertEqual(
  webConfig.buildCommand,
  "node scripts/verify-vercel-root.mjs && npm run build",
  "frontend build command",
);
assertEqual(webPackage.name, "ledgersync-web", "frontend package name");
assertEqual(webPackage.scripts?.build, "next build", "frontend package build command");

for (const unsupportedKey of ["builds", "outputDirectory"]) {
  if (unsupportedKey in rootConfig || unsupportedKey in webConfig) {
    throw new Error(`unexpected ${unsupportedKey} override in Vercel configuration`);
  }
}

console.log("Vercel configuration defines separate Go backend and Next.js frontend project roots.");
