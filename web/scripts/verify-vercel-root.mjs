import { access, readFile } from "node:fs/promises";
import path from "node:path";

const projectRoot = process.cwd();
const packagePath = path.join(projectRoot, "package.json");
const nextConfigPath = path.join(projectRoot, "next.config.ts");
const goModulePath = path.join(projectRoot, "go.mod");

const fail = (message) => {
  console.error(`Invalid Vercel frontend root: ${message}`);
  console.error('Set the Vercel project Root Directory to "web" and redeploy.');
  process.exit(1);
};

let packageJSON;
try {
  packageJSON = JSON.parse(await readFile(packagePath, "utf8"));
  await access(nextConfigPath);
} catch {
  fail("the working directory is not the LedgerSync Next.js application");
}

if (packageJSON.name !== "ledgersync-web" || packageJSON.scripts?.build !== "next build") {
  fail("package.json does not identify the expected web application");
}

try {
  await access(goModulePath);
  fail("the repository-root Go module is visible as the project root");
} catch (error) {
  if (error?.code !== "ENOENT") throw error;
}

console.log("Verified Vercel project root: web");
