import assert from "node:assert/strict";
import test from "node:test";

import { parseIsolatedComposeProject, parseSystemWebURL } from "./real-stack-boundary";

test("real-stack web URLs are restricted to the exact loopback Compose origin", () => {
  for (const value of ["http://127.0.0.1:3000", "http://localhost:3000/", "http://[::1]:3000"]) {
    assert.match(parseSystemWebURL(value), /^http:\/\//);
  }
  for (const value of [
    "https://127.0.0.1:3000",
    "http://127.0.0.2:3000",
    "http://localhost.example:3000",
    "http://user:secret@127.0.0.1:3000",
    "http://127.0.0.1:3000/#fragment",
    "http://127.0.0.1:3000/?project=isolated",
    "http://127.0.0.1:3001",
    "http://127.0.0.1",
    " http://127.0.0.1:3000",
  ]) {
    assert.throws(() => parseSystemWebURL(value));
  }
});

test("mutating system tests reject normal and malformed Compose projects", () => {
  assert.equal(parseIsolatedComposeProject("ledgersync-system-phase3"), "ledgersync-system-phase3");
  for (const value of ["", "compose", "ledgersync", "UPPER", "../other", "-invalid"]) {
    assert.throws(() => parseIsolatedComposeProject(value));
  }
});
