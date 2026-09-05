import assert from "node:assert/strict";
import test from "node:test";
import { expectedTransferBalances, reviewAccountChanged } from "../../src/features/transfers/transferReviewModel";
import { formatCurrencyMinorUnits } from "../../src/lib/money";

const source = { account_id: "source", currency: "INR", status: "active" as const, available_minor: "9007199254740993", ledger_minor: "9007199254740993", account_version: "2", version: "1", as_of: "2026-09-05T00:00:00Z" };
const destination = { ...source, account_id: "destination", available_minor: "10", ledger_minor: "10" };

test("localized money uses grouping, exact currency precision and safe large integers", () => {
  assert.equal(formatCurrencyMinorUnits("INR", "125000"), "₹1,250.00");
  assert.equal(formatCurrencyMinorUnits("INR", "9007199254740993"), "₹9,00,71,99,25,47,409.93");
  assert.equal(formatCurrencyMinorUnits("JPY", "1250"), "¥1,250");
  assert.equal(formatCurrencyMinorUnits("KWD", "1251").endsWith("1.251"), true);
  assert.equal(formatCurrencyMinorUnits("INR", "0"), "₹0.00");
  assert.equal(formatCurrencyMinorUnits("INR", "1.25"), "Unavailable");
});

test("expected effects retain exact integers beyond JavaScript safe-number precision", () => {
  assert.deepEqual(expectedTransferBalances({ source, destination, amountMinor: "3" }), { source: "9007199254740990", destination: "13" });
});

test("review rejects overdraws, inactive or same accounts, mixed currencies and invalid amounts", () => {
  const base = { source, destination, amountMinor: "1" };
  for (const amountMinor of ["0", "-1", "1.5", "9007199254740994"]) assert.throws(() => expectedTransferBalances({ ...base, amountMinor }));
  assert.throws(() => expectedTransferBalances({ ...base, destination: source }));
  assert.throws(() => expectedTransferBalances({ ...base, destination: { ...destination, currency: "USD" } }));
  assert.throws(() => expectedTransferBalances({ ...base, source: { ...source, status: "frozen" } }));
});

test("changed financial review data requires renewed confirmation but a new read timestamp alone does not", () => {
  assert.equal(reviewAccountChanged(source, { ...source, as_of: "2026-09-05T00:01:00Z" }), false);
  for (const change of [{ available_minor: "9007199254740992" }, { account_version: "3" }, { version: "2" }, { display_name: "New label" }]) assert.equal(reviewAccountChanged(source, { ...source, ...change }), true);
});
