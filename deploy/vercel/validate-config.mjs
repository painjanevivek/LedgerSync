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

assertEqual(rootConfig.framework, null, "repository-root framework");
assertEqual(rootConfig.installCommand, "node --version", "repository-root install guard");
assertEqual(
  rootConfig.buildCommand,
  "node deploy/vercel/reject-root-deployment.mjs",
  "repository-root build guard",
);

assertEqual(webConfig.framework, "nextjs", "frontend framework");
assertEqual(webConfig.installCommand, "npm ci", "frontend install command");
assertEqual(
  webConfig.buildCommand,
  "node scripts/verify-vercel-root.mjs && npm run build",
  "frontend build command",
);
assertEqual(webPackage.name, "ledgersync-web", "frontend package name");
assertEqual(webPackage.scripts?.build, "next build", "frontend package build command");

for (const unsupportedKey of ["builds", "functions", "outputDirectory"]) {
  if (unsupportedKey in rootConfig || unsupportedKey in webConfig) {
    throw new Error(`unexpected ${unsupportedKey} override in Vercel configuration`);
  }
}

console.log("Vercel configuration is scoped to the web Next.js application.");
