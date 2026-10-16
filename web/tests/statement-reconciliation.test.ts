import assert from "node:assert/strict";
import test from "node:test";
import { parseStatementBody, statementRoute, isStatementReceipt, isStatementPreview } from "../src/lib/api/statement-reconciliation";
import { createStatementIntent, parseStatementIntent, persistStatementIntent, clearStatementIntent, statementDigest, statementRequestBody, statementStorageKey } from "../src/features/statement-reconciliation/statementIntent";
const tenant = "00000000-0000-4000-8000-000000000001", mapping = "00000000-0000-4000-8000-000000000002";
function memory(): Storage { const values = new Map<string, string>(); return { get length() { return values.size; }, getItem: k => values.get(k) ?? null, setItem: (k, v) => { values.set(k, v); }, removeItem: k => { values.delete(k); }, clear: () => values.clear(), key: i => Array.from(values.keys())[i] ?? null }; }
test("statement input rejects browser authority, number versions and escaped routes", () => {
    assert.throws(() => parseStatementBody({ account_id: mapping, source_account_ref: "synthetic", tenant_id: tenant }, "mapping"));
    assert.throws(() => parseStatementBody({ import_id: "../other" }, "run"));
    assert.throws(() => parseStatementBody({ source_base64: "x" }, "preview"));
    assert.throws(() => parseStatementBody({ source_base64: "AAAA".repeat(349527) }, "preview"));
    assert.equal(statementRoute("POST", ["runs", mapping, "reviews"])?.body, "review");
    for (const parts of [["..", "private"], ["runs", "%2fprivate"], ["runs", mapping, "reviews", "extra"], ["imports", mapping, "other.csv"]])
        assert.equal(statementRoute("GET", parts), null);
    assert.equal(isStatementReceipt({ id: mapping, replayed: true }), true);
    assert.equal(isStatementReceipt({ id: mapping, replayed: "true" }), false);
    assert.equal(isStatementPreview({ sha256: "0".repeat(64), source_account_ref: "synthetic", period_start: "2026-10-01T00:00:00Z", period_end: "2026-11-01T00:00:00Z", opening_minor: 100, closing_minor: "0", row_count: 1 }), false);
});
test("import recovery stores a digest, binds exact bytes and retains original key", async () => {
    const source = new TextEncoder().encode("synthetic original CSV\r\n"), digest = await statementDigest(source);
    const intent = await createStatementIntent(tenant, "actor", "/api/statement-reconciliation/imports", "import", { mapping_id: mapping }, digest);
    assert.ok(!intent.payload.includes("synthetic original"));
    const restored = await parseStatementIntent(JSON.stringify(intent), tenant, "actor");
    assert.deepEqual(restored, intent);
    const body = JSON.parse(await statementRequestBody(intent, source)) as {
        source_base64: string;
    };
    assert.equal(atob(body.source_base64), "synthetic original CSV\r\n");
    await assert.rejects(statementRequestBody(intent, new TextEncoder().encode("synthetic original CSV\n")));
    await assert.rejects(parseStatementIntent(JSON.stringify(intent), tenant, "other"));
    await assert.rejects(parseStatementIntent(JSON.stringify({ ...intent, key: crypto.randomUUID() }), tenant, "actor"));
});
test("recovery fails closed and preserves unknown formats or newer commands", async () => {
    const intent = await createStatementIntent(tenant, "actor", "/api/statement-reconciliation/runs", "run", { import_id: mapping });
    const storage = memory(), key = statementStorageKey(tenant, "actor");
    persistStatementIntent(storage, key, intent);
    persistStatementIntent(storage, key, intent);
    const changed = await createStatementIntent(tenant, "actor", "/api/statement-reconciliation/runs", "run", { import_id: mapping });
    assert.throws(() => persistStatementIntent(storage, key, changed));
    assert.throws(() => clearStatementIntent(storage, key, changed));
    assert.equal(storage.getItem(key), JSON.stringify(intent));
    clearStatementIntent(storage, key, intent);
    assert.equal(storage.getItem(key), null);
    const unknown = JSON.stringify({ ...intent, version: 2 });
    storage.setItem(key, unknown);
    await assert.rejects(parseStatementIntent(storage.getItem(key), tenant, "actor"));
    assert.equal(storage.getItem(key), unknown);
    const unavailable = memory();
    unavailable.setItem = () => { throw new Error("quota"); };
    assert.throws(() => persistStatementIntent(unavailable, key, intent));
    assert.equal(unavailable.getItem(key), null);
    const noReadback = memory();
    noReadback.setItem = () => { };
    assert.throws(() => persistStatementIntent(noReadback, key, intent));
});
test("review intent retains exact version, comment and destination across restart", async () => {
    const body = { kind: "statement", item_id: "row-1", expected_version: "9007199254740993", status: "reviewed_exception", assignee: "actor", reason: "investigation", comment: "=retain original text", evidence: "" };
    const path = `/api/statement-reconciliation/runs/${mapping}/reviews`, intent = await createStatementIntent(tenant, "actor", path, "review", body);
    assert.equal(await statementRequestBody((await parseStatementIntent(JSON.stringify(intent), tenant, "actor"))!), JSON.stringify(body));
    assert.throws(() => parseStatementBody({ ...body, expected_version: 9007199254740993 }, "review"));
    assert.throws(() => parseStatementBody({ ...body, status: "matched", posting_id: mapping, kind: "ledger" }, "review"));
    assert.throws(() => parseStatementBody({ ...body, evidence: "javascript:alert(1)" }, "review"));
    assert.throws(() => parseStatementBody({ ...body, comment: "🙂".repeat(129) }, "review"));
});
import { readStatementBytes } from "../src/lib/statement-stream";
test("bounded statement stream cancels an oversized body without trusting length metadata", async () => {
    let cancelled = false;
    const stream = new ReadableStream<Uint8Array>({ start(controller) { controller.enqueue(new Uint8Array(8)); controller.enqueue(new Uint8Array(8)); }, cancel() { cancelled = true; } });
    await assert.rejects(readStatementBytes(stream, 10));
    assert.equal(cancelled, true);
    const valid = new ReadableStream<Uint8Array>({ start(controller) { controller.enqueue(new Uint8Array([1, 2])); controller.enqueue(new Uint8Array([3])); controller.close(); } });
    assert.deepEqual(Array.from(await readStatementBytes(valid, 3)), [1, 2, 3]);
});
