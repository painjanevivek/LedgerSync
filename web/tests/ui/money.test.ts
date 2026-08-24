import assert from "node:assert/strict";
import test from "node:test";

import { formatMinorUnits, minorUnitsFromDecimal } from "../../src/lib/money";

test("decimal input becomes exact minor units without floating point", () => {
  assert.equal(minorUnitsFromDecimal("USD", "12.50"), "1250");
  assert.equal(minorUnitsFromDecimal("JPY", "12"), "12");
  assert.throws(() => minorUnitsFromDecimal("USD", "0.001"));
  assert.equal(formatMinorUnits("USD", "1250"), "USD 12.50");
});

test("exact-money input enforces canonical signed-64-bit boundaries", () => {
  assert.equal(minorUnitsFromDecimal("INR", "92233720368547758.07"), "9223372036854775807");
  assert.equal(formatMinorUnits("INR", "9223372036854775807"), "INR 92233720368547758.07");
  assert.equal(minorUnitsFromDecimal("INR", " 1.00 "), "100");

  for (const input of ["0", "0.00", "-1.00", "+1.00", "1e3", "1 .00", "1.001", "92233720368547758.08"]) {
    assert.throws(() => minorUnitsFromDecimal("INR", input), input);
  }
  assert.throws(() => minorUnitsFromDecimal("BTC", "1.00"));
  assert.equal(formatMinorUnits("INR", "9223372036854775808"), "Unavailable");
  assert.equal(formatMinorUnits("BTC", "100"), "Unavailable");
});
