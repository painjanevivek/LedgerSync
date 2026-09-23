import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

async function cssFiles(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map((entry) => {
    const target = path.join(directory, entry.name);
    return entry.isDirectory() ? cssFiles(target) : entry.isFile() && entry.name.endsWith(".css") ? [target] : [];
  }));
  return nested.flat();
}

test("the UI type contract contains no fixed text below the 12px metadata floor", async () => {
  const root = path.resolve(process.cwd(), "src", "styles");
  const failures: string[] = [];
  for (const file of await cssFiles(root)) {
    const source = await readFile(file, "utf8");
    for (const match of source.matchAll(/font(?:-size)?:[^;]*?\b([0-9]+(?:\.[0-9]+)?)px(?:[;/]|\s)/gu)) {
      if (Number(match[1]) < 12) failures.push(`${path.relative(root, file)}: ${match[0].trim()}`);
    }
  }
  assert.deepEqual(failures, []);
});
