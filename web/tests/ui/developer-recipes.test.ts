import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { buildTransferRecipes } from "../../src/features/developer/developer-recipes";
import { sanitizeDeveloperMetadata, type DeveloperExample } from "../../src/lib/api/developer";

async function canonicalTransfer() {
  const raw = JSON.parse(await readFile("../contracts/developer-examples.v1.json", "utf8")) as Record<string, unknown>;
  const sanitized = sanitizeDeveloperMetadata(200, raw);
  assert.equal(sanitized.status, 200);
  const examples = sanitized.body.examples as DeveloperExample[];
  const transfer = examples.find((example) => example.id === "create_transfer");
  assert.ok(transfer);
  return transfer;
}

test("multi-language recipes are generated from the canonical exact transfer", async () => {
  const transfer = await canonicalTransfer();
  const recipes = buildTransferRecipes(transfer);

  assert.deepEqual(recipes.map((recipe) => recipe.id), ["curl", "typescript", "go", "postman"]);
  for (const recipe of recipes.slice(0, 3)) {
    assert.match(recipe.code, /create_transfer|\/transfers|createTransfer/i);
    assert.match(recipe.code, /125\.50/);
    assert.match(recipe.code, /70000000-0000-4000-8000-000000000001/);
    assert.match(recipe.code, /example-transfer-key-0001/);
    assert.doesNotMatch(recipe.code, /Bearer\s+[A-Za-z0-9._~-]{20,}/);
  }
  assert.match(recipes[3].code, /ledgersync\.postman_collection\.json/);
});

test("recipe generation does not mutate or numerically coerce canonical money", async () => {
  const transfer = await canonicalTransfer();
  const before = JSON.stringify(transfer);
  buildTransferRecipes(transfer);
  assert.equal(JSON.stringify(transfer), before);
  assert.equal(typeof transfer.body.amount, "string");
});
