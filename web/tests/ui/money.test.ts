import assert from "node:assert/strict";
import test from "node:test";

import { formatMinorUnits, minorUnitsFromDecimal } from "../../src/lib/money";

test("decimal input becomes exact minor units without floating point", () => {
  assert.equal(minorUnitsFromDecimal("USD", "12.50"), "1250");
  assert.equal(minorUnitsFromDecimal("JPY", "12"), "12");
  assert.throws(() => minorUnitsFromDecimal("USD", "0.001"));
  assert.equal(formatMinorUnits("USD", "1250"), "USD 12.50");
});
