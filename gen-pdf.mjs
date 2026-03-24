#!/usr/bin/env node
/**
 * Goqu Quantum Computing Textbook - PDF Generator
 *
 * Assembles all 42 chapters into a single print-edition HTML document,
 * renders it with headless Chromium (Playwright) so KaTeX math renders
 * natively, then post-processes with pdf-lib for page numbers and metadata.
 *
 * Usage: node gen-pdf.mjs [output.pdf]
 */

import { chromium } from "playwright";
import { PDFDocument, rgb, StandardFonts } from "pdf-lib";
import { createServer } from "http";
import { execSync } from "child_process";
import {
  readFileSync,
  writeFileSync,
  unlinkSync,
  existsSync,
} from "fs";
import { join, dirname, extname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const OUTPUT = process.argv[2] || join(__dirname, "goqu-textbook.pdf");

// Preflight checks

if (!existsSync(join(__dirname, "chapters"))) {
  console.error(
    "Error: textbook/chapters/ not found.\n" +
      'Run "go run ./textbook/gen" from the repo root first to generate the HTML chapters.',
  );
  process.exit(1);
}

// Build WASM so interactive components can execute
const repoRoot = join(__dirname, "..");
const goroot = execSync("go env GOROOT", { encoding: "utf-8" }).trim();

if (!existsSync(join(__dirname, "main.wasm"))) {
  console.log("Building WASM...");
  execSync("GOOS=js GOARCH=wasm go build -o textbook/main.wasm ./textbook/wasm/", {
    cwd: repoRoot,
    stdio: "inherit",
  });
}

if (!existsSync(join(__dirname, "wasm_exec.js"))) {
  execSync(`cp "${goroot}/lib/wasm/wasm_exec.js" "${join(__dirname, "wasm_exec.js")}"`);
}

// Chapter metadata

const chapters = JSON.parse(
  readFileSync(join(__dirname, "gen", "chapters.json"), "utf-8"),
);

const parts = [];
let cur = null;
for (const ch of chapters) {
  if (!cur || cur.number !== ch.part) {
    cur = {
      number: ch.part,
      title: ch.partTitle,
      description: ch.partDescription || "",
      chapters: [],
    };
    parts.push(cur);
  }
  cur.chapters.push(ch);
}

// Helpers

function esc(s) {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** Extract raw content between <article> and </article>. */
function extractArticle(slug, chapterNum) {
  const html = readFileSync(
    join(__dirname, "chapters", `${slug}.html`),
    "utf-8",
  );
  const open = html.indexOf("<article>");
  const close = html.lastIndexOf("</article>");
  if (open === -1 || close === -1) return "";
  let content = html.slice(open + "<article>".length, close);

  // Prefix element IDs and their JS references with chapter number to prevent
  // collisions when all 42 chapters share a single document.
  const p = `c${chapterNum}-`;
  content = content.replace(/\bid="([^"]+)"/g, `id="${p}$1"`);
  content = content.replace(/\bfor="([^"]+)"/g, `for="${p}$1"`);
  content = content.replace(
    /getElementById\(["']([^"']+)["']\)/g,
    (_m, id) => `getElementById("${p}${id}")`,
  );
  content = content.replace(
    /querySelector\(["']#([^"']+)["']\)/g,
    (_m, id) => `querySelector("#${p}${id}")`,
  );
  return content;
}

/** Shift heading levels down by one so parts can own h1. */
function shiftHeadings(html) {
  html = html.replace(/<(\/?)h3(?=[\s>])/gi, "<$1h4");
  html = html.replace(/<(\/?)h2(?=[\s>])/gi, "<$1h3");
  html = html.replace(/<(\/?)h1(?=[\s>])/gi, "<$1h2");
  return html;
}

// Inter-chapter link rewriting: 06-entanglement.html -> #chapter-6
const slugToChapter = new Map(chapters.map((ch) => [ch.slug, ch.chapter]));
const slugRe = chapters
  .map((ch) => ch.slug.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
  .join("|");
const linkRe = new RegExp(
  `href=(["'])(${slugRe})\\.html(?:#[^"']*)?\\1`,
  "g",
);

function rewriteLinks(html) {
  return html.replace(linkRe, (_m, q, slug) => {
    const n = slugToChapter.get(slug);
    return n != null ? `href=${q}#chapter-${n}${q}` : _m;
  });
}

// CSS

const CSS = /* css */ `
/* === Reset & base === */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --accent:    #4C72B0;
  --text:      #333333;
  --muted:     #666666;
  --light:     #999999;
  --border:    #e0e0e0;
  --code-bg:   #f5f5f4;
  --success:   #22c55e;
  --warning:   #f59e0b;
  --error:     #ef4444;
  --font-body: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  --font-mono: ui-monospace, "Cascadia Code", "Fira Code", "JetBrains Mono", monospace;
  --page-h:    calc(297mm - 25mm - 30mm);
  /* Aliases for inline SVG diagrams (match web textbook variable names) */
  --color-text:       #333333;
  --color-text-muted: #666666;
  --color-gate-1q:    #BDD7FF;
  --color-gate-2q:    #D4BBFF;
  --color-measure:    #FFDDAA;
  --color-border:     #e0e0e0;
  --color-code-bg:    #f5f5f4;
}

body {
  font-family: var(--font-body);
  font-size: 10.5pt;
  line-height: 1.65;
  color: var(--text);
  background: #fff;
  orphans: 3;
  widows: 3;
}

a { color: var(--accent); text-decoration: none; }

img, svg { max-width: 100%; height: auto; }

/* === Cover page === */
.pdf-cover {
  min-height: var(--page-h);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  page-break-after: always;
  break-after: page;
}
.cover-spacer { flex: 1; }
.cover-brand {
  font-size: 56pt;
  font-weight: 800;
  letter-spacing: 0.28em;
  color: var(--accent);
  margin-bottom: 10pt;
}
.cover-title {
  font-size: 20pt;
  font-weight: 300;
  color: var(--text);
  line-height: 1.4;
  margin-bottom: 30pt;
}
.cover-rule {
  width: 45%;
  max-width: 170pt;
  height: 0.75pt;
  background: var(--accent);
  margin: 0 auto 30pt;
}
.cover-sub {
  font-size: 10pt;
  color: var(--muted);
  line-height: 1.6;
  margin-bottom: 44pt;
}
.cover-circuit pre {
  font-family: var(--font-mono);
  font-size: 11pt;
  color: var(--light);
  line-height: 1.35;
  text-align: left;
  display: inline-block;
}
.cover-url {
  font-size: 8.5pt;
  color: var(--light);
  letter-spacing: 0.08em;
  margin-top: auto;
  padding-top: 60pt;
}

/* === Table of contents === */
.pdf-toc {
  page-break-after: always;
  break-after: page;
}
.toc-heading {
  font-size: 26pt;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 24pt;
  padding-bottom: 6pt;
  border-bottom: 2pt solid var(--accent);
}
.toc-part { margin-bottom: 12pt; }
.toc-part-label {
  font-size: 7.5pt;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--light);
  margin-bottom: 1pt;
}
.toc-part-title {
  font-size: 12pt;
  font-weight: 600;
  color: var(--accent);
  margin-bottom: 3pt;
  padding-bottom: 3pt;
  border-bottom: 0.5pt solid var(--border);
}
.toc-entries { padding-left: 6pt; }
.toc-entry {
  display: flex;
  padding: 2pt 0;
  color: var(--text);
  text-decoration: none;
  font-size: 10pt;
}
.toc-num {
  width: 22pt;
  font-weight: 600;
  color: var(--light);
  flex-shrink: 0;
  text-align: right;
  padding-right: 8pt;
}
.toc-name { flex: 1; }

/* === Part divider === */
.pdf-part-divider {
  min-height: var(--page-h);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  page-break-before: always;
  page-break-after: always;
  break-before: page;
  break-after: page;
}
.part-spacer { flex: 1; }
.part-label {
  font-size: 9pt;
  text-transform: uppercase;
  letter-spacing: 0.2em;
  color: var(--light);
  margin-bottom: 12pt;
}
.pdf-part-divider > h1 {
  font-size: 26pt;
  font-weight: 700;
  color: var(--accent);
  margin-bottom: 16pt;
  max-width: 85%;
  line-height: 1.3;
}
.part-desc {
  font-size: 10.5pt;
  color: var(--muted);
  font-style: italic;
  max-width: 75%;
  line-height: 1.6;
}

/* === Chapter article === */
article.chapter {
  page-break-before: always;
  break-before: page;
}

/* Chapter title (was h1, now h2 after shift) */
article.chapter > h2:first-of-type {
  font-size: 22pt;
  font-weight: 700;
  margin: 0 0 20pt;
  padding-bottom: 6pt;
  border-bottom: 2pt solid var(--accent);
  color: var(--text);
  line-height: 1.25;
}

/* Section headings (was h2, now h3) */
article.chapter h3 {
  font-size: 13.5pt;
  font-weight: 600;
  margin: 20pt 0 8pt;
  padding-bottom: 3pt;
  border-bottom: 0.5pt solid var(--border);
  color: var(--text);
  page-break-after: avoid;
  break-after: avoid;
}

/* Subsection headings (was h3, now h4) */
article.chapter h4 {
  font-size: 11.5pt;
  font-weight: 600;
  margin: 14pt 0 6pt;
  color: var(--text);
  page-break-after: avoid;
  break-after: avoid;
}

/* === Typography === */
article.chapter p { margin: 0 0 7pt; }
article.chapter ul, article.chapter ol { margin: 0 0 7pt; padding-left: 18pt; }
article.chapter li { margin-bottom: 2pt; }
article.chapter strong { font-weight: 600; }
article.chapter hr {
  border: none;
  border-top: 0.5pt solid var(--border);
  margin: 14pt 0;
}

/* === Inline code === */
article.chapter code {
  font-family: var(--font-mono);
  font-size: 0.88em;
  background: var(--code-bg);
  padding: 0.08em 0.3em;
  border-radius: 2pt;
}

/* === Code blocks === */
article.chapter pre,
.print-code {
  background: var(--code-bg);
  border: 0.5pt solid var(--border);
  border-radius: 3pt;
  padding: 8pt 10pt;
  font-family: var(--font-mono);
  font-size: 8.5pt;
  line-height: 1.45;
  margin: 0 0 7pt;
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow: hidden;
  page-break-inside: avoid;
  break-inside: avoid;
}
article.chapter pre code,
.print-code code {
  background: none;
  padding: 0;
  font-size: inherit;
}

/* === Blockquotes === */
article.chapter blockquote {
  margin: 0 0 7pt;
  padding: 6pt 10pt;
  border-left: 3pt solid var(--accent);
  background: var(--code-bg);
  border-radius: 0 3pt 3pt 0;
}
article.chapter blockquote p:last-child { margin-bottom: 0; }

/* === Tables === */
article.chapter table,
.state-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 7pt;
  font-size: 9pt;
  page-break-inside: avoid;
  break-inside: avoid;
}
article.chapter th, article.chapter td,
.state-table th, .state-table td {
  padding: 4pt 6pt;
  text-align: left;
  border-bottom: 0.5pt solid var(--border);
}
article.chapter th, .state-table th {
  font-weight: 600;
  background: var(--code-bg);
}

/* === Callout boxes === */
.callout {
  margin: 8pt 0;
  padding: 8pt 10pt;
  border-radius: 3pt;
  border-left: 3pt solid;
  page-break-inside: avoid;
  break-inside: avoid;
}
.callout-info    { background: rgba(76,114,176,0.06); border-color: var(--accent); }
.callout-warning { background: rgba(245,158,11,0.06); border-color: var(--warning); }
.callout-key     { background: rgba(34,197,94,0.06);  border-color: var(--success); }
.callout p:last-child { margin-bottom: 0; }
.callout strong:first-child { display: block; margin-bottom: 2pt; }

/* === KaTeX === */
.katex-display {
  overflow-x: visible;
  padding: 4pt 0;
  page-break-inside: avoid;
  break-inside: avoid;
}

/* === Interactive elements: print-friendly treatment === */

/* Hide controls that make no sense on paper */
.sandbox-controls,
.exercise-controls,
.flashcard-controls,
.step-controls,
.walkthrough-controls,
.comparison-controls,
.poe-progress,
.walkthrough-progress,
.poe-step button,
.poe-step textarea,
.widget button,
.sim-controls input[type="range"],
.sim-controls select,
.exercise-feedback,
.exercise-expected,
.exercise-hint,
.sim-qasm,
.step-qasm,
.walkthrough-qasm,
.comparison-qasm { display: none !important; }

/* Sandbox / Simulation / Exercise containers */
.sandbox, .simulation, .exercise, .step-through, .widget, .poe,
.animated-walkthrough, .comparison {
  margin: 8pt 0;
  border: 0.5pt solid var(--border);
  border-radius: 3pt;
  overflow: hidden;
  page-break-inside: avoid;
  break-inside: avoid;
}
.exercise {
  border-width: 1.5pt;
  border-color: var(--accent);
}
.exercise-header {
  padding: 4pt 10pt;
  background: var(--accent);
  color: #fff;
  font-weight: 600;
  font-size: 9pt;
}
.exercise-prompt {
  padding: 6pt 10pt;
  border-bottom: 0.5pt solid var(--border);
  font-size: 9.5pt;
}
.exercise-prompt p:last-child { margin-bottom: 0; }

/* Sim controls: show param labels, hide sliders */
.sim-controls {
  padding: 5pt 10pt;
  background: var(--code-bg);
  border-bottom: 0.5pt solid var(--border);
  font-family: var(--font-mono);
  font-size: 8pt;
  color: var(--muted);
}
.sim-controls label {
  display: inline-flex;
  align-items: center;
  gap: 3pt;
}

/* Output areas: hide when empty */
.sandbox .output,
.sim-output,
.step-output,
.widget-output {
  padding: 6pt;
  min-height: 0;
}
.sandbox .output:empty,
.sim-output:empty,
.step-output:empty,
.widget-output:empty { display: none; }
.sandbox .output svg { max-width: 100%; height: auto; }

/* Flashcards: reveal all answers */
.flashcard-deck {
  margin: 8pt 0;
  border: 0.5pt solid var(--border);
  border-radius: 3pt;
  overflow: hidden;
  page-break-inside: avoid;
  break-inside: avoid;
}
.flashcard-deck-header {
  padding: 4pt 10pt;
  background: var(--code-bg);
  border-bottom: 0.5pt solid var(--border);
  font-weight: 600;
  font-size: 9pt;
}
.flashcard {
  padding: 8pt 10pt;
  border-bottom: 0.5pt solid var(--border);
}
.flashcard:last-child { border-bottom: none; }
.flashcard .card-front { font-weight: 500; font-size: 10pt; }
.flashcard .card-back {
  display: block !important;
  margin-top: 4pt;
  padding-top: 4pt;
  border-top: 0.5pt dashed var(--border);
  color: var(--muted);
  font-size: 9.5pt;
}

/* POE: show all steps */
.poe-step {
  display: block !important;
  padding: 6pt 10pt;
  border-bottom: 0.5pt solid var(--border);
}
.poe-step:last-child { border-bottom: none; }
.poe-step h3 {
  margin: 0 0 3pt;
  font-size: 10pt;
  color: var(--accent);
  border-bottom: none;
  padding-bottom: 0;
}

/* Stabilizer table */
.stabilizer-table { width: 100%; margin-top: 4pt; }
.stabilizer-table h4 {
  margin: 0 0 3pt;
  font-size: 8pt;
  color: var(--light);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.stabilizer-table table {
  border-collapse: collapse;
  margin: 0 auto;
  font-family: var(--font-mono);
  font-size: 10pt;
  letter-spacing: 0.1em;
}
.stabilizer-table td { padding: 2pt 6pt; border-bottom: 0.5pt solid var(--border); }
.stab-sign { color: var(--light); }
.stab-i { color: var(--light); opacity: 0.5; }
.stab-x { color: #e74c3c; font-weight: 600; }
.stab-y { color: #27ae60; font-weight: 600; }
.stab-z { color: #2980b9; font-weight: 600; }

/* Animated walkthrough: show all stages */
.walkthrough-stage {
  display: block !important;
  padding: 6pt 10pt;
  border-bottom: 0.5pt solid var(--border);
  page-break-inside: avoid;
  break-inside: avoid;
}
.walkthrough-stage:last-of-type { border-bottom: none; }
.walkthrough-narration h4 {
  margin: 0 0 3pt;
  font-size: 10pt;
  color: var(--accent);
  border-bottom: none;
  padding-bottom: 0;
}
.walkthrough-narration p { font-size: 9.5pt; }
.walkthrough-output { padding: 4pt 0; }
.walkthrough-output svg { max-width: 100%; height: auto; }

/* Comparison: show all panels side by side */
.comparison-panels {
  display: flex;
  gap: 8pt;
  width: 100%;
  justify-content: center;
}
.comparison-panel {
  flex: 1;
  min-width: 0;
  text-align: center;
  page-break-inside: avoid;
  break-inside: avoid;
}
.comparison-panel[data-label]::before {
  content: attr(data-label);
  display: block;
  font-size: 8pt;
  font-weight: 600;
  color: var(--light);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 4pt;
}
.comparison-output { padding: 4pt 0; }
.comparison-output svg { max-width: 100%; height: auto; }

/* Comparison panels */
.comparison-panels {
  display: flex;
  gap: 8pt;
  width: 100%;
  justify-content: center;
}
.comparison-panel { flex: 1; min-width: 0; text-align: center; }
.comparison-panel h4 {
  margin: 0 0 4pt;
  font-size: 8pt;
  color: var(--light);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.comparison-panel svg { max-width: 100%; height: auto; }

/* Amplitude bars */
.amplitude-bars {
  display: flex;
  gap: 2pt;
  align-items: flex-end;
  height: 100pt;
  padding: 4pt;
  border: 0.5pt solid var(--border);
  border-radius: 3pt;
  background: var(--code-bg);
}
.amp-bar {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  height: 100%;
}
.amp-fill {
  width: 80%;
  background: var(--accent);
  border-radius: 1pt 1pt 0 0;
}
.amp-bar.target .amp-fill { background: var(--error); }
.amp-label { font-family: var(--font-mono); font-size: 6pt; color: var(--light); margin-top: 2pt; }
.amp-value { font-family: var(--font-mono); font-size: 5.5pt; color: var(--light); }

/* Sim stats */
.sim-stats {
  display: flex;
  gap: 10pt;
  justify-content: center;
  padding: 3pt 0;
  font-family: var(--font-mono);
  font-size: 8.5pt;
  color: var(--light);
}

/* Utility */
.visually-hidden { display: none; }
section { page-break-before: auto; }

/* Sections with data-component that are fully empty should collapse */
[data-component]:empty { display: none; }
`;

// HTML builder

function buildHTML() {
  // Cover
  const cover = `
  <div class="pdf-cover">
    <div class="cover-spacer"></div>
    <div class="cover-brand">GOQU</div>
    <div class="cover-title">Quantum Computing<br>Textbook</div>
    <div class="cover-rule"></div>
    <div class="cover-sub">From Classical Bits to Quantum Frontiers<br>in 42 Chapters</div>
    <div class="cover-circuit"><pre>q0: \u2500H\u2500\u2500\u2500@\u2500\u2500
         \u2502
q1: \u2500\u2500\u2500\u2500\u2500X\u2500\u2500</pre></div>
    <div class="cover-spacer"></div>
    <div class="cover-url">splch.github.io/goqu</div>
  </div>`;

  // Table of contents
  const toc = `
  <div class="pdf-toc">
    <h1 class="toc-heading">Contents</h1>
    ${parts
      .map(
        (p) => `
    <div class="toc-part">
      <div class="toc-part-label">Part ${p.number}</div>
      <div class="toc-part-title">${esc(p.title)}</div>
      <div class="toc-entries">
        ${p.chapters
          .map(
            (ch) => `
        <a href="#chapter-${ch.chapter}" class="toc-entry">
          <span class="toc-num">${ch.chapter}</span>
          <span class="toc-name">${esc(ch.title)}</span>
        </a>`,
          )
          .join("")}
      </div>
    </div>`,
      )
      .join("")}
  </div>`;

  // Part dividers + chapters
  let content = "";
  for (const part of parts) {
    content += `
  <div class="pdf-part-divider" id="part-${part.number}">
    <div class="part-spacer"></div>
    <div class="part-label">Part ${part.number}</div>
    <h1>${esc(part.title)}</h1>
    ${part.description ? `<div class="part-desc">${esc(part.description)}</div>` : ""}
    <div class="part-spacer"></div>
  </div>`;

    for (const ch of part.chapters) {
      let article = extractArticle(ch.slug, ch.chapter);
      article = shiftHeadings(article);
      article = rewriteLinks(article);
      content += `
  <article class="chapter" id="chapter-${ch.chapter}">
    ${article}
  </article>`;
    }
  }

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Goqu Quantum Computing Textbook</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.min.css">
  <script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.min.js"><\/script>
  <script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/contrib/auto-render.min.js"><\/script>
  <script src="wasm_exec.js"><\/script>
  <style>${CSS}</style>
</head>
<body>
  ${cover}
  ${toc}
  ${content}

  <script>
  "use strict";
  // ── Utilities ──────────────────────────────────────────────────────────
  function esc(s) { return s.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;"); }
  function isDark() { return false; } // always light mode for print
  function renderMath(el) {
    if (typeof renderMathInElement === "function")
      renderMathInElement(el, { delimiters: [{left:"$$",right:"$$",display:true},{left:"$",right:"$",display:false}], throwOnError:false });
  }

  // ── WASM loader ────────────────────────────────────────────────────────
  var wasmReady = null;
  function ensureWasm() {
    if (!wasmReady) {
      wasmReady = (async function() {
        var go = new Go();
        var result = await WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject);
        go.run(result.instance);
      })();
    }
    return wasmReady;
  }

  // ── Sandbox ────────────────────────────────────────────────────────────
  function initSandbox(el) {
    var code = el.querySelector(".code"), output = el.querySelector(".output");
    if (!code || !output) return;
    var shotsEl = el.querySelector("[data-shots]");
    function shots() { return parseInt(shotsEl ? shotsEl.value : el.dataset.shots || "1024", 10); }
    async function run() {
      await ensureWasm();
      await new Promise(function(r) { setTimeout(r, 0); });
      try {
        var r = JSON.parse(window.runQASM(code.value, shots(), false));
        if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; return; }
        output.innerHTML = (r.circuit||"") + (r.histogram||"") + (r.bloch||"");
        if (el.hasAttribute("data-bloch") && window.getStateVector && window.renderBloch) {
          try {
            var sv = JSON.parse(window.getStateVector(code.value));
            if (sv.amplitudes && sv.amplitudes.length === 2) {
              var a = sv.amplitudes;
              var blochSVG = window.renderBloch(a[0].re, a[0].im, a[1].re, a[1].im, false);
              if (blochSVG) output.innerHTML += blochSVG;
            }
          } catch(e) { /* ignore */ }
        }
      } catch(e) { output.innerHTML = '<pre class="error">' + esc(e.message) + '</pre>'; }
    }
    // Always run for PDF (not just data-autorun)
    run();
  }

  // ── Simulation ─────────────────────────────────────────────────────────
  function initSimulation(el) {
    var tmplEl = el.querySelector(".sim-qasm"), output = el.querySelector(".sim-output");
    var sliders = el.querySelectorAll('input[type="range"]');
    var selects = el.querySelectorAll("select[data-param]");
    if (!tmplEl || !output || (sliders.length === 0 && selects.length === 0)) return;
    var tmpl = tmplEl.textContent, pending = false;
    var useNoisy = el.hasAttribute("data-noisy");
    var useComparison = el.hasAttribute("data-comparison");
    var useExpectation = el.hasAttribute("data-expectation");
    var useClifford = el.hasAttribute("data-clifford");
    async function update() {
      if (pending) return; pending = true;
      await ensureWasm();
      await new Promise(function(r) { setTimeout(r, 0); });
      try {
        var params = {};
        sliders.forEach(function(s) { params[s.dataset.param] = parseFloat(s.value); var o = s.parentElement.querySelector("output"); if (o) o.textContent = parseFloat(s.value).toFixed(2); });
        selects.forEach(function(s) { params[s.dataset.param] = s.value; });
        var qasm = tmpl;
        for (var k in params) qasm = qasm.replaceAll("{"+k+"}", params[k].toString());
        var html = "";
        if (useComparison && window.compareIdealNoisy) {
          var noiseModel = params.noiseModel || "depolarizing";
          var noiseParam = parseFloat(params.noiseParam || "0.1");
          var r = JSON.parse(window.compareIdealNoisy(qasm, 1024, noiseModel, noiseParam, false));
          if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; pending = false; return; }
          html = (r.circuitSVG||"") + '<div class="comparison-panels"><div class="comparison-panel"><h4>Ideal</h4>' + (r.idealHistogramSVG||"") + '</div><div class="comparison-panel"><h4>Noisy</h4>' + (r.noisyHistogramSVG||"") + '</div></div>';
          if (r.stateCitySVG) html += r.stateCitySVG;
          if (r.purity !== undefined) html += '<div class="sim-stats"><span>Purity: ' + r.purity.toFixed(4) + '</span><span>Fidelity: ' + r.fidelity.toFixed(4) + '</span></div>';
        } else if (useNoisy && window.runNoisyQASM) {
          var noiseModel = params.noiseModel || "depolarizing";
          var noiseParam = parseFloat(params.noiseParam || "0.1");
          var r = JSON.parse(window.runNoisyQASM(qasm, 1024, noiseModel, noiseParam, false));
          if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; pending = false; return; }
          html = (r.circuitSVG||"") + (r.histogramSVG||"") + (r.stateCitySVG||"");
          if (r.purity !== undefined) html += '<div class="sim-stats"><span>Purity: ' + r.purity.toFixed(4) + '</span><span>Fidelity: ' + r.fidelity.toFixed(4) + '</span></div>';
        } else if (useClifford && window.runClifford) {
          var r = JSON.parse(window.runClifford(qasm, 1024, false));
          if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; pending = false; return; }
          html = (r.circuitSVG||"") + (r.histogramSVG||"");
          if (r.stabilizers) { html += '<div class="stabilizer-table"><h4>Stabilizers</h4><table>'; r.stabilizers.forEach(function(s) { html += '<tr><td>' + formatStabilizer(s) + '</td></tr>'; }); html += '</table></div>'; }
        } else if (useExpectation && window.computeExpectation) {
          var pauliStr = params.pauliStr || el.dataset.pauli || "Z";
          var r = JSON.parse(window.computeExpectation(qasm, pauliStr, false));
          if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; pending = false; return; }
          html = (r.circuitSVG||"") + '<div class="sim-stats"><span>&lt;' + esc(pauliStr) + '&gt; = ' + r.expectation.toFixed(6) + '</span></div>';
        } else {
          var r = JSON.parse(window.runQASM(qasm, 1024, false));
          if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; pending = false; return; }
          html = (r.circuit||"") + (r.histogram||"") + (r.bloch||"");
          if (el.hasAttribute("data-bloch") && window.getStateVector && window.renderBloch) {
            try {
              var sv = JSON.parse(window.getStateVector(qasm));
              if (sv.amplitudes && sv.amplitudes.length === 2) {
                var a = sv.amplitudes;
                var blochSVG = window.renderBloch(a[0].re, a[0].im, a[1].re, a[1].im, false);
                if (blochSVG) html += blochSVG;
              }
            } catch(e) { /* ignore */ }
          }
        }
        output.innerHTML = html;
      } catch(e) { output.innerHTML = '<pre class="error">' + esc(e.message) + '</pre>'; }
      pending = false;
    }
    update(); // auto-run with default slider values
  }

  // ── Stabilizer formatting ──────────────────────────────────────────────
  function formatStabilizer(s) {
    if (!s) return "";
    var sign = s[0], ops = s.slice(1), html = '<span class="stab-sign">' + esc(sign) + '</span>';
    for (var i = 0; i < ops.length; i++) {
      var c = ops[i], cls = "stab-" + c.toLowerCase();
      html += '<span class="' + cls + '">' + c + '</span>';
    }
    return html;
  }

  // ── Exercise (show circuit from starter code) ──────────────────────────
  function initExercise(el) {
    var code = el.querySelector(".code");
    if (!code) return;
    // Auto-run to show the circuit diagram for the starter code
    (async function() {
      await ensureWasm();
      await new Promise(function(r) { setTimeout(r, 0); });
      try {
        var r = JSON.parse(window.runQASM(code.value, 1024, false));
        if (!r.error) {
          var out = el.querySelector(".output");
          if (!out) { out = document.createElement("div"); out.className = "output"; el.appendChild(out); }
          out.innerHTML = (r.circuit||"") + (r.histogram||"");
        }
      } catch(e) { /* ignore - starter code may be intentionally incomplete */ }
    })();
  }

  // ── Flashcard deck ─────────────────────────────────────────────────────
  function initFlashcardDeck(el) {
    el.querySelectorAll(".flashcard").forEach(function(card) {
      card.classList.add("flipped"); // reveal all answers for PDF
      var back = card.querySelector(".card-back");
      if (back) renderMath(back);
    });
  }

  // ── POE (Predict-Observe-Explain) - show all steps, run all sandboxes ─
  function initPOE(el) {
    el.querySelectorAll(".poe-step").forEach(function(s) { s.classList.add("active"); });
    el.querySelectorAll(".sandbox").forEach(function(sb) {
      sb.setAttribute("data-initialized", "true");
      initSandbox(sb);
    });
  }

  // ── Animated Walkthrough - show all stages, run all QASM
  function initAnimatedWalkthrough(el) {
    el.querySelectorAll(".walkthrough-stage").forEach(function(stage) {
      stage.classList.add("active");
      stage.style.display = "block";
      var qasmEl = stage.querySelector(".walkthrough-qasm");
      var output = stage.querySelector(".walkthrough-output");
      if (!qasmEl || !output) return;
      var qasm = qasmEl.textContent;
      (async function() {
        await ensureWasm();
        await new Promise(function(r) { setTimeout(r, 0); });
        try {
          // runQASM returns trusted SVG from our own WASM module
          var r = JSON.parse(window.runQASM(qasm, 1024, false));
          if (!r.error) output.innerHTML = (r.circuit||"") + (r.histogram||"");
        } catch(e) { /* ignore */ }
      })();
    });
  }

  // ── Comparison - run QASM in all panels
  function initComparison(el) {
    el.querySelectorAll(".comparison-panel").forEach(function(panel) {
      var qasmEl = panel.querySelector(".comparison-qasm");
      var output = panel.querySelector(".comparison-output");
      if (!qasmEl || !output) return;
      var qasm = qasmEl.textContent;
      (async function() {
        await ensureWasm();
        await new Promise(function(r) { setTimeout(r, 0); });
        try {
          // runQASM returns trusted SVG from our own WASM module
          var r = JSON.parse(window.runQASM(qasm, 1024, false));
          if (!r.error) output.innerHTML = (r.circuit||"") + (r.histogram||"");
        } catch(e) { /* ignore */ }
      })();
    });
  }

  // ── Step-Through - show final step ─────────────────────────────────────
  function initStepThrough(el) {
    var qasmEl = el.querySelector(".step-qasm");
    var output = el.querySelector(".step-output");
    if (!qasmEl || !output) return;
    var qasm = qasmEl.textContent;
    var mode = el.dataset.mode || "clifford";
    async function load() {
      await ensureWasm();
      try {
        var steps = null;
        if (mode === "clifford" && window.cliffordStepThrough) {
          var r = JSON.parse(window.cliffordStepThrough(qasm));
          if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; return; }
          steps = r.steps || [];
          steps.unshift({gate:"init", qubits:[], stabilizers: steps.length > 0 ? null : []});
        } else if (mode === "grover" && window.groverStepThrough) {
          var nq = parseInt(el.dataset.qubits || "2", 10);
          var targets = JSON.parse(el.dataset.targets || "[]");
          var r = JSON.parse(window.groverStepThrough(nq, targets.join(",")));
          if (r.error) { output.innerHTML = '<pre class="error">' + esc(r.error) + '</pre>'; return; }
          steps = r.steps || [];
        }
        if (!steps || steps.length === 0) return;
        // Show final step for PDF
        var cur = steps.length - 1, s = steps[cur], html = "";
        if (mode === "clifford") {
          if (s.gate === "init") {
            html = '<p class="step-label">Initial state |0...0&gt;</p>';
          } else {
            html = '<p class="step-label">After <strong>' + esc(s.gate) + '</strong> on qubit' + (s.qubits.length > 1 ? 's ' : ' ') + s.qubits.join(', ') + '</p>';
            if (s.stabilizers) {
              html += '<div class="stabilizer-table"><table>';
              s.stabilizers.forEach(function(st) { html += '<tr><td>' + formatStabilizer(st) + '</td></tr>'; });
              html += '</table></div>';
            }
          }
        } else if (mode === "grover" && s.probabilities) {
          html = '<p class="step-label">Final iteration (' + s.iteration + ')</p>';
          html += '<div class="amplitude-bars">';
          for (var i = 0; i < s.probabilities.length; i++) {
            var label = i.toString(2).padStart(Math.ceil(Math.log2(s.probabilities.length)), '0');
            var pct = (s.probabilities[i] * 100).toFixed(1);
            var isTarget = el.dataset.targets && JSON.parse(el.dataset.targets).indexOf(i) >= 0;
            html += '<div class="amp-bar' + (isTarget ? ' target' : '') + '"><div class="amp-fill" style="height:' + pct + '%"></div><span class="amp-label">|' + label + '&gt;</span><span class="amp-value">' + pct + '%</span></div>';
          }
          html += '</div>';
        }
        output.innerHTML = html;
      } catch(e) { output.innerHTML = '<pre class="error">' + esc(e.message) + '</pre>'; }
    }
    load();
  }

  // ── Wait until page content stabilizes (all async WASM ops done) ───────
  function waitForStable(maxMs) {
    maxMs = maxMs || 120000;
    return new Promise(function(resolve) {
      var lastLen = 0, stable = 0, elapsed = 0;
      var timer = setInterval(function() {
        elapsed += 1000;
        var curLen = document.body.innerHTML.length;
        if (curLen === lastLen) { stable++; if (stable >= 3) { clearInterval(timer); resolve(); } }
        else { stable = 0; lastLen = curLen; }
        if (elapsed >= maxMs) { clearInterval(timer); resolve(); }
      }, 1000);
    });
  }

  // ── Main init ──────────────────────────────────────────────────────────
  document.addEventListener("DOMContentLoaded", async function () {
    // 1. Render KaTeX math
    if (typeof renderMathInElement !== "undefined") {
      renderMathInElement(document.body, {
        delimiters: [
          { left: "$$", right: "$$", display: true },
          { left: "$",  right: "$",  display: false },
        ],
        throwOnError: false,
      });
    }

    // 2. Initialize all interactive components (WASM loads on first use)
    var reg = {
      sandbox: initSandbox,
      simulation: initSimulation,
      exercise: initExercise,
      "flashcard-deck": initFlashcardDeck,
      poe: initPOE,
      "step-through": initStepThrough,
      "animated-walkthrough": initAnimatedWalkthrough,
      comparison: initComparison,
    };
    var count = 0;
    document.querySelectorAll("[data-component]").forEach(function (el) {
      var fn = reg[el.dataset.component];
      if (fn) { fn(el); count++; }
    });
    console.log("Initialized " + count + " interactive components");

    // 3. Trigger all inline widget buttons (onclick handlers for WASM-powered
    //    widgets like Deutsch-Jozsa, Shor, VQE, QAOA, hardware topology, etc.)
    var widgetBtns = document.querySelectorAll(".widget button[onclick]");
    widgetBtns.forEach(function (btn) {
      try { btn.click(); } catch (e) { console.error("Widget click error:", e.message); }
    });
    console.log("Triggered " + widgetBtns.length + " inline widget buttons");

    // 4. Wait for all async WASM operations to finish
    await waitForStable();
    console.log("All components stabilized");

    // 5. Convert textareas to styled pre blocks for clean print rendering
    document.querySelectorAll("textarea.code").forEach(function (ta) {
      var pre = document.createElement("pre");
      pre.className = "print-code";
      var code = document.createElement("code");
      code.textContent = ta.value || ta.textContent || "";
      pre.appendChild(code);
      ta.parentNode.replaceChild(pre, ta);
    });

    // 6. Re-render KaTeX on any newly-revealed content (flashcard backs, etc.)
    if (typeof renderMathInElement !== "undefined") {
      renderMathInElement(document.body, {
        delimiters: [
          { left: "$$", right: "$$", display: true },
          { left: "$",  right: "$",  display: false },
        ],
        throwOnError: false,
      });
    }

    // 7. Signal to Playwright
    document.body.setAttribute("data-rendered", "true");
  });
  <\/script>
</body>
</html>`;
}

// Local static server (avoids file:// cross-origin issues with CDN resources)

const MIME = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css",
  ".js": "application/javascript",
  ".wasm": "application/wasm",
  ".svg": "image/svg+xml",
  ".json": "application/json",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".woff2": "font/woff2",
  ".woff": "font/woff",
  ".ttf": "font/ttf",
};

function startServer() {
  return new Promise((resolve) => {
    const srv = createServer((req, res) => {
      const safe = decodeURIComponent(req.url.split("?")[0]).replace(
        /\.\./g,
        "",
      );
      const file = join(__dirname, safe === "/" ? "_print.html" : safe);
      try {
        const data = readFileSync(file);
        res.writeHead(200, {
          "Content-Type": MIME[extname(file)] || "application/octet-stream",
        });
        res.end(data);
      } catch {
        res.writeHead(404);
        res.end("Not found");
      }
    });
    srv.listen(0, "127.0.0.1", () => resolve(srv));
  });
}

// Main

async function main() {
  const t0 = Date.now();

  // 1. Build the single print-edition HTML
  console.log("Assembling 42 chapters into print HTML...");
  const html = buildHTML();
  writeFileSync(join(__dirname, "_print.html"), html);

  // 2. Serve it locally
  const server = await startServer();
  const port = server.address().port;

  // 3. Render with headless Chromium
  console.log("Launching headless Chromium...");
  const browser = await chromium.launch();
  const page = await browser.newPage();

  // Listen for console messages from the page (progress reporting)
  page.on("console", (msg) => {
    if (msg.type() === "log") console.log("  [page]", msg.text());
    if (msg.type() === "error") console.log("  [page error]", msg.text());
  });

  console.log("Loading page, rendering KaTeX & executing WASM components...");
  await page.goto(`http://127.0.0.1:${port}/`, {
    waitUntil: "networkidle",
    timeout: 180_000,
  });
  await page.waitForFunction(
    () => document.body.getAttribute("data-rendered") === "true",
    { timeout: 300_000 },
  );
  // Let fonts settle after all rendering completes
  await page.waitForTimeout(2000);

  console.log("Generating PDF (this may take a minute)...");
  const pdfBytes = await page.pdf({
    format: "A4",
    margin: { top: "25mm", bottom: "30mm", left: "25mm", right: "20mm" },
    printBackground: true,
    outline: true,
    tagged: true,
  });

  await browser.close();
  server.close();

  // 4. Post-process: add page numbers and metadata
  console.log("Adding page numbers & metadata...");
  const doc = await PDFDocument.load(pdfBytes);
  const font = await doc.embedFont(StandardFonts.Helvetica);
  const pages = doc.getPages();

  // Skip page 0 (cover) when numbering
  for (let i = 1; i < pages.length; i++) {
    const pg = pages[i];
    const { width } = pg.getSize();
    const label = String(i);
    const tw = font.widthOfTextAtSize(label, 9);
    pg.drawText(label, {
      x: (width - tw) / 2,
      y: 35, // within the 30 mm bottom margin
      size: 9,
      font,
      color: rgb(0.5, 0.5, 0.5),
    });
  }

  doc.setTitle("Goqu Quantum Computing Textbook");
  doc.setSubject("Quantum Computing");
  doc.setCreator("Goqu gen-pdf");

  const final = await doc.save();
  writeFileSync(OUTPUT, final);

  // 5. Clean up temp file
  try {
    unlinkSync(join(__dirname, "_print.html"));
  } catch {
    /* ignore */
  }

  const elapsed = ((Date.now() - t0) / 1000).toFixed(1);
  console.log(
    `\nDone! ${pages.length} pages written to ${OUTPUT} (${elapsed}s)`,
  );
}

main().catch((err) => {
  console.error("Error:", err);
  process.exit(1);
});
