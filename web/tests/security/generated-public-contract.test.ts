import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import YAML from "yaml";

test("public SDK catalogues contain every public operation and no internal session or preference operations", async () => {
  const contract = YAML.parse(await readFile("../contracts/openapi.yaml", "utf8"));
  const manifest = JSON.parse(await readFile("../contracts/generated/sdk-manifest.json", "utf8")) as { operations: { id: string; path: string }[] };
  const expected: string[] = [];
  const internal: string[] = [];
  for (const [, item] of Object.entries(contract.paths as Record<string, Record<string, unknown>>)) {
    for (const method of ["get", "post", "put", "patch", "delete"]) {
      const operation = item[method] as { operationId?: string; "x-internal"?: boolean } | undefined;
      if (!operation?.operationId) continue;
      (item["x-internal"] === true || operation["x-internal"] === true ? internal : expected).push(operation.operationId);
    }
  }
  assert.ok(internal.length >= 6, "internal sessions and scoped preferences must remain marked");
  assert.deepEqual(manifest.operations.map(operation => operation.id).sort(), expected.sort());
  for (const path of ["typescript/ledgersync.ts", "go/ledgersync.go", "ledgersync.postman_collection.json"]) {
    const source = await readFile(`../contracts/generated/${path}`, "utf8");
    for (const name of internal) assert.ok(!source.includes(name), `${path} exposes ${name}`);
  }
  assert.ok(manifest.operations.every(operation => !operation.path.startsWith("/internal/")));
});
