import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("administration remains a non-disclosing not-found route", async () => {
  const route = await readFile(new URL("../../src/app/admin/page.tsx", import.meta.url), "utf8");
  assert.match(route, /notFound\(\)/);
  assert.doesNotMatch(route, /ConsoleShell|fetch\(|\/api\/admin|administration:manage|tenant:admin/);
});

test("an invented browser scope cannot release administration", async () => {
  const capabilities = await readFile(new URL("../../src/features/console/capabilities.ts", import.meta.url), "utf8");
  const shell = await readFile(new URL("../../src/features/console/ConsoleShell.tsx", import.meta.url), "utf8");
  assert.match(capabilities, /administrationManage: false/);
  assert.doesNotMatch(shell, /href:\s*["']\/admin|label:\s*["']Administration/);
});

test("the administration design contract preserves external gates and four-eyes policy", async () => {
  const contract = await readFile(new URL("../../../docs/security/administration-boundary.md", import.meta.url), "utf8");
  for (const evidence of ["M11 managed identity", "M12 production infrastructure", "four-eyes", "different executor", "non-disclosing", "external security review", "must not be reported as implementation completeness"]) {
    assert.match(contract, new RegExp(evidence, "i"));
  }
  assert.match(contract, /No production persona is considered owned/);
  assert.match(contract, /no browser or private administration API is shipped/);
});
