import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(
  new URL("../../../scripts/test-local-fault-recovery.ps1", import.meta.url),
  "utf8",
);

test("targeted fault recovery cannot traverse healthy Compose dependencies", () => {
  const targetedRecoveryCommands = [
    '@("up", "-d", "--wait", "--no-deps", "outbox-worker")',
    '@("up", "-d", "--wait", "--no-deps", "redis", "outbox-worker", "api", "web")',
    '@("up", "-d", "--wait", "--no-deps", "api", "web")',
  ];

  for (const command of targetedRecoveryCommands) {
    assert.ok(source.includes(command), `missing isolated Compose recovery command: ${command}`);
  }

  for (const unsafeCommand of [
    '@("up", "-d", "--wait", "outbox-worker")',
    '@("up", "-d", "--wait", "redis", "outbox-worker", "api", "web")',
    '@("up", "-d", "--wait", "api", "web")',
  ]) {
    assert.ok(!source.includes(unsafeCommand), `targeted recovery may traverse dependencies: ${unsafeCommand}`);
  }
});

test("authority and dependency-order drills retain explicit full-stack recovery", () => {
  const fullRecovery = '@("up", "-d", "--wait")';
  assert.equal(
    source.split(fullRecovery).length - 1,
    3,
    "PostgreSQL recovery, dependency-order recovery, and failure cleanup must remain full-stack",
  );
});
