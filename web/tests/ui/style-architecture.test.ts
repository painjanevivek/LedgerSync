import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

import { COMPACT_NAVIGATION_MAX_WIDTH_PX, COMPACT_NAVIGATION_MEDIA_QUERY } from "../../src/lib/responsive";

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const stylesRoot = resolve(webRoot, "src/styles");

function read(relativePath: string) {
  return readFileSync(resolve(webRoot, relativePath), "utf8");
}

function cssFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name);
    return entry.isDirectory() ? cssFiles(path) : entry.name.endsWith(".css") ? [path] : [];
  });
}

test("global styles remain an import-only ownership manifest", () => {
  const globalsPath = resolve(webRoot, "src/app/globals.css");
  const globals = readFileSync(globalsPath, "utf8");
  const meaningfulLines = globals.split(/\r?\n/u).map((line) => line.trim()).filter(Boolean);

  assert.ok(meaningfulLines.length > 0);
  for (const line of meaningfulLines) {
    assert.match(line, /^@import\s+["'][^"']+["'];$/u);
    const [, importPath] = line.match(/^@import\s+["']([^"']+)["'];$/u) ?? [];
    assert.ok(importPath && existsSync(resolve(dirname(globalsPath), importPath)), `Missing stylesheet: ${importPath}`);
  }
});

test("root stylesheet imports preserve the intentional cascade order", () => {
  const layout = read("src/app/layout.tsx");
  const expectedImports = [
    'import "../styles/tokens.css";',
    'import "../styles/foundations/reset.css";',
    'import "./globals.css";',
    'import "../styles/features/approvals.css";',
    'import "../styles/layout/responsive-shell.css";',
  ];
  let previousIndex = -1;

  for (const expectedImport of expectedImports) {
    const index = layout.indexOf(expectedImport);
    assert.ok(index > previousIndex, `${expectedImport} must follow the preceding stylesheet import`);
    previousIndex = index;
  }
});

test("non-token styles use semantic color variables", () => {
  const colorLiteral = /(?:#[0-9a-f]{3,8}\b|\b(?:rgb|rgba|hsl|hsla)\s*\()/iu;

  for (const path of cssFiles(stylesRoot)) {
    if (path === resolve(stylesRoot, "tokens.css")) continue;
    assert.doesNotMatch(readFileSync(path, "utf8"), colorLiteral, path);
  }
});

test("responsive shell owns canonical compact behavior and accessibility fallbacks", () => {
  const responsive = read("src/styles/layout/responsive-shell.css");

  assert.equal(COMPACT_NAVIGATION_MAX_WIDTH_PX, 760);
  assert.equal(COMPACT_NAVIGATION_MEDIA_QUERY, "(max-width: 760px)");
  assert.equal((responsive.match(/@media \(max-width: 760px\)/gu) ?? []).length, 1);
  assert.equal((responsive.match(/@media \(max-width: 520px\)/gu) ?? []).length, 1);
  assert.match(responsive, /@media \(prefers-reduced-motion: reduce\)/u);
  assert.match(responsive, /@media \(forced-colors: active\)/u);

  const shell = read("src/features/console/ConsoleShell.tsx");
  assert.match(shell, /window\.matchMedia\(COMPACT_NAVIGATION_MEDIA_QUERY\)/u);
  assert.doesNotMatch(shell, /matchMedia\(["']\(max-width:\s*760px\)["']\)/u);
});

test("the style system does not depend on a competing CSS runtime", () => {
  const packageJson = JSON.parse(read("package.json")) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  const packages = { ...packageJson.dependencies, ...packageJson.devDependencies };

  for (const dependency of ["styled-components", "@emotion/react", "@emotion/styled", "tailwindcss"]) {
    assert.equal(packages[dependency], undefined, `${dependency} would introduce a second styling architecture`);
  }
});
