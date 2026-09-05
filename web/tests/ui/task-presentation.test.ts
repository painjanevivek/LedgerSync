import assert from "node:assert/strict";
import test from "node:test";
import { orderTasks, recoveryTasks, type WorkspaceTask } from "../../src/features/tasks/taskPresentation";

const task = (id: string, priority: number, actionable: boolean, occurredAt: string): WorkspaceTask => ({ id, priority, actionable, occurredAt, title: id, explanation: "Test record", tone: "warning", group: "attention", action: { label: "Review", href: "/tasks" } });
test("tasks order financial risk, actionability, oldest occurrence then stable identity", () => {
  const items = [task("delivery", 3, true, "2026-01-01"), task("review-b", 2, true, "2026-08-02"), task("unknown", 0, true, "2026-09-05"), task("review-a", 2, true, "2026-08-01"), task("waiting", 2, false, "2026-01-01")];
  assert.deepEqual(orderTasks(items).map(item => item.id), ["unknown", "review-a", "review-b", "waiting", "delivery"]);
});
test("the same funding or correction record appears only once with its actionable adapter", () => {
  const read = task("funding:one", 2, false, "2026-08-01");
  const approval = { ...read, actionable: true, title: "Make a decision" };
  assert.deepEqual(orderTasks([read, approval]), [approval]);
});
test("missing recovery evidence is a setup gap, not an urgent financial incident", () => {
  const result = recoveryTasks({ format_version: "ledgersync-recovery-evidence-index/v1", generated_at_utc: "2026-09-05T00:00:00Z", latest_backup: null, latest_restore: null, retention: { valid_backup_count: 0, ignored_entry_count: 0, configured_keep_count: 5 } });
  assert.equal(result[0].group, "setup");
  assert.equal(result[0].action.href, "/recovery");
});
