import assert from "node:assert/strict";
import test from "node:test";

import { NextRequest } from "next/server";

import { readBoundedJSON } from "../../src/lib/security";

test("BFF financial JSON accepts only bounded declared bodies", async () => {
  const validBody = JSON.stringify({ amount: "1.00" });
  const valid = new NextRequest("http://localhost:3000/api/transfers", {
    method: "POST",
    headers: { "content-length": String(Buffer.byteLength(validBody)) },
    body: validBody,
  });
  assert.deepEqual(await readBoundedJSON(valid), { amount: "1.00" });

  const oversized = new NextRequest("http://localhost:3000/api/transfers", {
    method: "POST",
    headers: { "content-length": "20000" },
    body: "{}",
  });
  await assert.rejects(() => readBoundedJSON(oversized), /outside the permitted size/);

  const undeclared = new NextRequest("http://localhost:3000/api/transfers", { method: "POST", body: "{}" });
  await assert.rejects(() => readBoundedJSON(undeclared), /outside the permitted size/);
});
