import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { experiencePreferenceKey, readExperienceMode, writeExperienceMode } from "../../src/features/console/experience-mode";
import { reconciliationPresentation, transferStatusPresentation } from "../../src/features/console/presentation";
import { sanitizeUIPreference } from "../../src/lib/api/ui-preferences";
import { disclosurePreferenceKey, readDisclosurePreference, writeDisclosurePreference } from "../../src/ui/disclosure/disclosure-preference";

test("experience mode is scoped to tenant and operator and fails safely to simple", () => {
  const values = new Map<string, string>();
  const storage = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); } };
  assert.equal(readExperienceMode(storage, "tenant-a", "operator-a"), "simple");
  assert.equal(writeExperienceMode(storage, "tenant-a", "operator-a", "expert"), true);
  assert.equal(readExperienceMode(storage, "tenant-a", "operator-a"), "expert");
  assert.equal(readExperienceMode(storage, "tenant-a", "operator-b"), "simple");
  assert.notEqual(experiencePreferenceKey("tenant-a", "operator-a"), experiencePreferenceKey("tenant-b", "operator-a"));
  assert.equal(readExperienceMode({ getItem: () => { throw new Error("blocked"); }, setItem: () => undefined }, "tenant-a", "operator-a"), "simple");
});

test("plain-language presentation never invites an unsafe duplicate transfer", () => {
  const pending = transferStatusPresentation("pending");
  assert.equal(pending.attention, true);
  assert.match(pending.explanation, /Do not create another transfer/i);
  assert.doesNotMatch(pending.title, /idempotency|consistency|authoritative/i);

  const mismatch = reconciliationPresentation("mismatch", "2");
  assert.equal(mismatch.attention, true);
  assert.equal(mismatch.title, "A balance needs review");
});

test("disclosure memory is tenant and operator scoped and fails safely", () => {
  const values = new Map<string, string>();
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value); },
  };
  assert.equal(readDisclosurePreference(storage, "tenant-a", "operator-a", "details"), undefined);
  assert.equal(writeDisclosurePreference(storage, "tenant-a", "operator-a", "details", true), true);
  assert.equal(readDisclosurePreference(storage, "tenant-a", "operator-a", "details"), true);
  assert.equal(readDisclosurePreference(storage, "tenant-a", "operator-b", "details"), undefined);
  assert.notEqual(
    disclosurePreferenceKey("tenant-a", "operator-a", "details"),
    disclosurePreferenceKey("tenant-b", "operator-a", "details"),
  );
  assert.equal(readDisclosurePreference(undefined, "tenant-a", "operator-a", "details"), undefined);
});

test("simple navigation and expert evidence share one shell", () => {
  const shell = readFileSync(new URL("../../src/features/console/ConsoleShell.tsx", import.meta.url), "utf8");
  const navigation = readFileSync(new URL("../../src/features/console/ConsoleNavigation.tsx", import.meta.url), "utf8");
  assert.match(navigation, /label: "Home"/);
  assert.match(navigation, /label: "Add money"/);
  assert.match(navigation, /label: "Tasks"/);
  assert.match(navigation, /experience: "expert"/);
  assert.match(shell, /mode === "expert"/);
});

test("server-backed experience preferences accept only the exact presentation contract", () => {
  assert.deepEqual(sanitizeUIPreference({ experience_mode: "expert", version: "2", updated_at: "2026-09-05T10:00:00Z" }), { experience_mode: "expert", version: "2", updated_at: "2026-09-05T10:00:00Z" });
  assert.equal(sanitizeUIPreference({ experience_mode: "expert", version: 2 }), null);
  assert.equal(sanitizeUIPreference({ experience_mode: "simple", version: "1", tenant_id: "forged" }), null);
});
