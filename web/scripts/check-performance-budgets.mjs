import { readdir, stat } from "node:fs/promises";
import { join, relative } from "node:path";

const staticRoot = join(process.cwd(), ".next", "static");
const limits = {
  largestJavaScriptChunkBytes: 350_000,
  totalJavaScriptBytes: 2_000_000,
  largestFontBytes: 160_000,
  totalFontBytes: 320_000,
};
const assets = [];

async function walk(directory) {
  for (const name of await readdir(directory)) {
    const path = join(directory, name);
    const info = await stat(path);
    if (info.isDirectory()) await walk(path);
    else assets.push({ path: relative(process.cwd(), path), bytes: info.size, extension: name.split(".").at(-1)?.toLowerCase() ?? "" });
  }
}

await walk(staticRoot);
const javascript = assets.filter((asset) => asset.extension === "js").sort((a, b) => b.bytes - a.bytes);
const fonts = assets.filter((asset) => ["woff", "woff2", "ttf", "otf"].includes(asset.extension)).sort((a, b) => b.bytes - a.bytes);
const total = (items) => items.reduce((sum, item) => sum + item.bytes, 0);
const result = {
  javascript: { chunks: javascript.length, total_bytes: total(javascript), largest: javascript[0] ?? null },
  fonts: { files: fonts.length, total_bytes: total(fonts), largest: fonts[0] ?? null },
  limits,
};

console.log(JSON.stringify(result, null, 2));
if (
  result.javascript.total_bytes > limits.totalJavaScriptBytes ||
  (result.javascript.largest?.bytes ?? 0) > limits.largestJavaScriptChunkBytes ||
  result.fonts.total_bytes > limits.totalFontBytes ||
  (result.fonts.largest?.bytes ?? 0) > limits.largestFontBytes
) {
  console.error("Frontend asset performance budget exceeded");
  process.exitCode = 1;
}
