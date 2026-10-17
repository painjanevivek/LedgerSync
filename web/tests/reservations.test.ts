import assert from "node:assert/strict";
import test from "node:test";
import { isReservationReceipt, parseReservationBody, reservationRoute } from "../src/lib/api/reservations";
import { clearReservationIntent, createReservationIntent, parseReservationIntent, saveReservationIntent } from "../src/features/reservations/reservationIntent";
const source = "11111111-1111-4111-8111-111111111111", destination = "22222222-2222-4222-8222-222222222222";
const body = { debit_account_id: source, credit_account_id: destination, amount_minor: "9007199254740993", currency: "INR", ttl_seconds: 3600 };
test("reservation authority, exact integers and bounded full decisions", () => {
    assert.deepEqual(parseReservationBody(body, false), body);
    for (const invalid of [{ ...body, tenant_id: source }, { ...body, amount_minor: 9007199254740993 }, { ...body, amount_minor: "01" }, { ...body, amount_minor: "9223372036854775808" }, { ...body, ttl_seconds: 86401 }, { ...body, ttl_seconds: 1.5 }, { ...body, credit_account_id: source }])
        assert.throws(() => parseReservationBody(invalid, false));
    assert.deepEqual(parseReservationBody({ decision: "capture" }, true), { decision: "capture" });
    assert.throws(() => parseReservationBody({ decision: "capture", amount_minor: "1" }, true));
    assert.throws(() => parseReservationBody({ decision: "expire" }, true));
    assert.equal(reservationRoute("POST", [source, "decisions"])?.path, `/api/reservations/${source}/decisions`);
    assert.equal(reservationRoute("GET", ["export.csv"])?.result, "csv");
    assert.equal(reservationRoute("POST", ["export.csv"]), null);
    for (const parts of [[".."], ["%2f"], [source, "expire"], [source, "decisions", "extra"]])
        assert.equal(reservationRoute("POST", parts), null);
});
test("reservation full intent survives reload, actor changes and future formats fail closed", async () => {
    const saved = await createReservationIntent("tenant-a", "actor-a", "/api/reservations", body), raw = JSON.stringify(saved);
    assert.deepEqual(await parseReservationIntent(raw, "tenant-a", "actor-a"), saved);
    for (const invalid of [raw.replace('9007199254740993', '9007199254740994'), JSON.stringify({ ...saved, version: 2 }), JSON.stringify({ ...saved, path: "https://other.invalid/" }), JSON.stringify({ ...saved, extra: true })])
        await assert.rejects(() => parseReservationIntent(invalid, "tenant-a", "actor-a"));
    await assert.rejects(() => parseReservationIntent(raw, "tenant-a", "actor-b"));
});
test("reservation storage fails before send and preserves a newer command", async () => {
    const first = await createReservationIntent("t", "a", "/api/reservations", body), next = await createReservationIntent("t", "a", `/api/reservations/${source}/decisions`, { decision: "release" });
    const map = new Map<string, string>(), storage = { getItem: (key: string) => map.get(key) ?? null, setItem: (key: string, value: string) => { map.set(key, value); }, removeItem: (key: string) => { map.delete(key); } } as Storage;
    saveReservationIntent(storage, "key", first);
    assert.throws(() => saveReservationIntent(storage, "key", next));
    map.set("key", JSON.stringify(next));
    assert.throws(() => clearReservationIntent(storage, "key", first));
    assert.equal(map.get("key"), JSON.stringify(next));
    const unavailable = { getItem: () => { throw Error("storage blocked"); } } as unknown as Storage;
    assert.throws(() => saveReservationIntent(unavailable, "key", first));
});
test("reservation receipt distinguishes durable denial and terminal transfer", () => {
    assert.equal(isReservationReceipt({ id: source, state: "active", replayed: false, rejection_code: "capture_policy_denied" }), true);
    assert.equal(isReservationReceipt({ id: source, state: "captured", replayed: true, source_balance_version: "9007199254740993", transfer_id: destination }), true);
    assert.equal(isReservationReceipt({ id: source, state: "captured", replayed: false, source_balance_version: "2" }), false);
    assert.equal(isReservationReceipt({ id: source, state: "active", replayed: false, rejection_code: "capture_policy_denied", transfer_id: destination }), false);
});
import { reservedMinorUnits } from "../src/lib/money";
import {hasReservedTransferBalance} from "../src/lib/api/transfers";
test("transfer receipts confirm reserved fields only when their exact equation agrees",()=>{
    const balance={account_id:source,currency:"INR",posted_minor:"9007199254740993",version:"5",as_of:"2026-10-16T00:00:00Z"};
    assert.equal(hasReservedTransferBalance(balance),false);
    assert.equal(hasReservedTransferBalance({...balance,available_minor:"9007199254740990",reserved_minor:"3"}),true);
    assert.equal(hasReservedTransferBalance({...balance,available_minor:"9007199254740990",reserved_minor:"2"}),false);
    assert.equal(hasReservedTransferBalance({...balance,available_minor:"invalid",reserved_minor:"0"}),false);
});
test("reserved funds use exact arithmetic and invalid evidence stays unavailable", () => {
    assert.equal(reservedMinorUnits("9007199254740993", "9007199254740990"), "3");
    for (const pair of [["100", "101"], ["100", "invalid"], ["9223372036854775808", "0"], ["-1", "0"]])
        assert.equal(reservedMinorUnits(pair[0], pair[1]), null);
});
