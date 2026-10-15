import { createHash } from "node:crypto";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Install the pinned renderer into ignored tooling state:
// npm install --prefix .tmp/diagram-renderer --save-exact mermaid@12.1.0
// The browser uses the project's installed Playwright and Chromium.
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const requireWeb = createRequire(path.join(root, "web/package.json"));
const { chromium } = requireWeb("@playwright/test");
const input = path.join(root, "docs/architecture/ledgersync-system-design.md");
const output = path.join(root, "docs/architecture/diagrams");
const tooling = path.join(root, ".tmp/diagram-renderer/node_modules/mermaid");
const version = JSON.parse(await readFile(path.join(tooling, "package.json"), "utf8")).version;
const markdown = await readFile(input, "utf8");
const escapeHTML = (value) => value.replace(/[&<>"']/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[character]);
const diagrams = [];
let heading = "System diagram";
const lines = markdown.split(/\r?\n/);
for (let line = 0; line < lines.length; line++) {
  if (/^#{1,6} /.test(lines[line])) heading = lines[line].replace(/^#{1,6} /, "");
  if (lines[line].trim() !== "```mermaid") continue;
  const start = line + 1;
  const source = [];
  while (++line < lines.length && lines[line].trim() !== "```") source.push(lines[line]);
  if (line === lines.length) throw new Error(`Unclosed Mermaid fence at line ${start}`);
  const number = String(diagrams.length + 1).padStart(2, "0");
  const slug = heading.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
  const text = source.join("\n");
  diagrams.push({ title: heading, name: `${number}-${slug}`, source: text, sourceLine: start + 1, sha256: createHash("sha256").update(text).digest("hex") });
}
if (!diagrams.length) throw new Error("No Mermaid diagrams found");
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1600, height: 1000 }, deviceScaleFactor: 1 });
  await page.setContent('<!doctype html><html><head><style>body{margin:0;background:white}#diagram{display:inline-block;padding:24px}svg{display:block;max-width:none!important}</style></head><body><div id="diagram"></div></body></html>');
  await page.addScriptTag({ path: path.join(tooling, "dist/mermaid.min.js") });
  await page.evaluate(() => window.mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "neutral", fontFamily: "Segoe UI, Arial, sans-serif", flowchart: { useMaxWidth: false }, sequence: { useMaxWidth: false } }));
  for (const [index, diagram] of diagrams.entries()) {
    const rendered = await page.evaluate(async ({ source, id }) => {
      await window.mermaid.parse(source);
      const { svg } = await window.mermaid.render(id, source);
      document.getElementById("diagram").innerHTML = svg;
      await document.fonts.ready;
      const element = document.querySelector("#diagram svg");
      const viewBox = element.viewBox.baseVal;
      if (viewBox.width && viewBox.height) {
        element.setAttribute("width", String(Math.ceil(viewBox.width)));
        element.setAttribute("height", String(Math.ceil(viewBox.height)));
      }
      const box = element.getBoundingClientRect();
      if (!box.width || !box.height || !element.querySelector("text,foreignObject")) throw new Error("Empty rendered diagram");
      const clippedLabels = Array.from(element.querySelectorAll("text,foreignObject")).filter((label) => {
        const rect = label.getBoundingClientRect();
        return rect.width && rect.height && (rect.left < box.left - 2 || rect.top < box.top - 2 || rect.right > box.right + 2 || rect.bottom > box.bottom + 2);
      });
      if (clippedLabels.length) throw new Error(`Diagram clips ${clippedLabels.length} labels`);
      return { svg: element.outerHTML, width: Math.ceil(box.width), height: Math.ceil(box.height) };
    }, { source: diagram.source, id: `system-diagram-${index + 1}` });
    await writeFile(path.join(output, `${diagram.name}.svg`), rendered.svg);
    await writeFile(path.join(output, `${diagram.name}.mmd`), `${diagram.source}\n`);
    await page.locator("#diagram").screenshot({ path: path.join(output, `${diagram.name}.png`), animations: "disabled" });
    diagram.width = rendered.width;
    diagram.height = rendered.height;
    console.log(`Rendered ${diagram.name}: ${rendered.width} x ${rendered.height}`);
  }
} finally {
  await browser.close();
}
const manifest = { source: "../ledgersync-system-design.md", renderer: `mermaid@${version}`, diagramCount: diagrams.length, diagrams: diagrams.map(({ source, ...metadata }) => metadata) };
await writeFile(path.join(output, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
const cards = diagrams.map((diagram) => `<section id="${diagram.name}"><h2>${escapeHTML(diagram.title)}</h2><div class="links"><a href="${diagram.name}.svg">SVG</a><a href="${diagram.name}.png">PNG</a><a href="${diagram.name}.mmd">Source</a></div><div class="canvas"><img loading="lazy" src="${diagram.name}.svg" alt="${escapeHTML(diagram.title)}" width="${diagram.width}" height="${diagram.height}"></div></section>`).join("\n");
await writeFile(path.join(output, "index.html"), `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>LedgerSync system diagrams</title><style>*{box-sizing:border-box}body{margin:0;color:#17221e;background:#f5f6f3;font:16px/1.5 "Segoe UI",Arial,sans-serif}main{max-width:1440px;margin:auto;padding:32px}h1{font-size:32px}h2{font-size:22px}p{max-width:75ch}nav{display:flex;flex-wrap:wrap;gap:12px;margin:24px 0}a{color:#145b43}a:focus-visible{outline:3px solid #145b43;outline-offset:4px}section{background:white;border:1px solid #ccd4ce;padding:24px;margin:24px 0;scroll-margin-top:16px}.links{display:flex;gap:20px;margin:12px 0}.canvas{overflow:auto;background:white;border:1px solid #e0e5e1}.canvas img{display:block;max-width:none}button{padding:10px 16px;font:inherit;color:#17221e;background:white;border:1px solid #758d7e;cursor:pointer;min-height:44px}body.fit .canvas img{max-width:100%;height:auto}body.fit .canvas{overflow:hidden}@media(max-width:600px){main{padding:16px}section{padding:16px}}@media(prefers-reduced-motion:no-preference){html{scroll-behavior:smooth}}</style><body class="fit"><main><h1>LedgerSync system diagrams</h1><p>${diagrams.length} diagrams rendered and parsed with Mermaid ${escapeHTML(version)}. Follow the user sequence, then inspect the architecture and deployment boundaries. Proposed and deferred components remain labelled in the source.</p><button id="size" type="button" aria-pressed="false">Use full size</button><nav aria-label="Diagrams">${diagrams.map((diagram) => `<a href="#${diagram.name}">${escapeHTML(diagram.title)}</a>`).join(" ")}</nav>${cards}<p><a href="../ledgersync-system-design.md">System design and evidence</a></p></main><script>document.getElementById('size').addEventListener('click',function(){const full=document.body.classList.toggle('fit')===false;this.setAttribute('aria-pressed',String(full));this.textContent=full?'Fit diagrams':'Use full size'})</script></body></html>\n`);
await writeFile(path.join(output, "README.md"), `# Rendered system diagrams\n\nAll ${diagrams.length} Mermaid blocks in [the system design](../ledgersync-system-design.md) were parsed and rendered with Mermaid ${version}. [Open the gallery](index.html). Each diagram has an SVG, PNG, and Mermaid source file. The [manifest](manifest.json) records source hashes, source lines, and rendered dimensions.\n\n## Rebuild\n\nFrom the repository root, with the frontend dependencies and Playwright Chromium installed:\n\n\x60\x60\x60powershell\nnpm install --prefix .tmp/diagram-renderer --save-exact mermaid@${version}\nnode scripts/render-system-diagrams.mjs\n\x60\x60\x60\n\nThis is local rendering. It does not publish files or certify the proposed cloud deployment.\n\n${diagrams.map((diagram) => `- [${diagram.title}](${diagram.name}.svg) · [PNG](${diagram.name}.png) · [source](${diagram.name}.mmd)`).join("\n")}\n`);
console.log(`Saved ${diagrams.length} diagrams and gallery to ${output}`);
