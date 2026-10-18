// Run: cd docs/tools && bun render-user.ts
// Builds the one docs site in docs/user/site: the user pages in docs/user/src
// and, under spec/, one page per specification chapter in docs/design/src.
// docs/user/inventory.md orders the sidebar. Each page also gets a Markdown copy
// beside its HTML, and llms.txt lists the copies. Uses the package.json pin: marked 18.0.12.
import { Marked, type Tokens } from "marked"
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { dirname, extname, join, relative, resolve } from "node:path"
import { hoverCss, useHoverRenderer } from "./render-hovers"
import { renderLanding, sourceLinks } from "./render-landing"
const repo = resolve(import.meta.dir, "../..")
const root = join(repo, "docs/user")
const src = join(root, "src")
const design = join(repo, "docs/design/src")
const site = join(root, "site")
// Page keys are site paths with a .md extension. Specification chapters live under spec/.
const fileOf = (key: string) => key.startsWith("spec/") ? join(design, key.slice(5)) : join(src, key)
const keyOf = (file: string) =>
  file.startsWith(design + "/") ? "spec/" + relative(design, file) : file.startsWith(src + "/") ? relative(src, file) : undefined
const where = (key: string) => relative(repo, fileOf(key))
if (existsSync(join(src, "spec"))) throw new Error("docs/user/src/spec would collide with the specification's pages")

// The facts the specification states are derived here, not typed by hand. The
// build refuses to run if the kernel's law suite fails: no [Kernel] claim
// outlives a broken law. The 100,000-element stress cases also run while build
// lanes share the host. The suite needs bun install in experiments and packages/runtime.
const experiments = join(repo, "experiments")
const suite = Bun.spawnSync(["bun", "test", "--timeout=30000"], { cwd: experiments, stdout: "pipe", stderr: "pipe" })
const suiteOutput = suite.stdout.toString() + suite.stderr.toString()
const ran = /Ran (\d+) tests? across (\d+) files?/.exec(suiteOutput)
const failed = /(\d+) fail/.exec(suiteOutput)
if (suite.exitCode !== 0 || ran === null || failed === null || failed[1] !== "0") {
  console.error(suiteOutput.split("\n").slice(-30).join("\n"))
  throw new Error("the kernel law suite in experiments/ failed; refusing to build the docs site")
}
const lineCount = (path: string) => readFileSync(join(repo, path), "utf8").split("\n").length - 1
const facts: Record<string, string> = {
  testCount: ran[1]!,
  testFiles: ran[2]!,
  kernelLines: String(lineCount("packages/runtime/src/kernel/kernel.ts") + lineCount("packages/runtime/src/kernel/internals.ts"))
}
const escape = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;")
// The inverse for text marked has already escaped, such as a heading's plain text.
const unescape = (s: string) => s.replace(/&(amp|lt|gt|quot|#39);/g, (_, e: string) => ({ amp: "&", lt: "<", gt: ">", quot: '"', "#39": "'" })[e]!)
const slug = (s: string) => s.toLowerCase().replace(/<[^>]+>/g, "").replace(/[^\p{L}\p{N}\s-]/gu, "").trim().replace(/\s/g, "-")
const output = (s: string) => s.replace(/\.md$/i, ".html")
// Code block roles, written after the language in the fence: ```ts effect
const CAPTIONS: Record<string, string> = {
  effect: "Effect today (TypeScript)",
  long: "VibeLang long form",
  es: "VibeLang",
  gen: "Generated TypeScript",
  dts: "Generated declaration",
  ts: "TypeScript",
  sh: "Shell",
  out: "Output"
}

// A small build-time TypeScript highlighter: comments, strings, numbers,
// keywords, literals, capitalized names, and called functions. VibeLang code,
// fenced ```vibe, shares it.
const KEYWORDS = new Set([
  "abstract", "as", "async", "await", "break", "case", "catch", "class", "const", "continue", "control",
  "declare", "default", "do", "else", "enum", "export", "extends", "finally", "for", "from", "function",
  "get", "if", "implements", "import", "in", "instanceof", "interface", "keyof", "let", "namespace", "new",
  "of", "private", "protected", "public", "readonly", "return", "satisfies", "service", "set", "static",
  "switch", "this", "throw", "try", "type", "typeof", "using", "var", "while", "yield"
])
// Like the design mock: primitive type names are types and true, false, null
// and undefined stay plain. The landing adds VibeLang's form words as keywords
// and !, % and |> as operators for its hero.
const LANDING_KEYWORDS = new Set(["error", "provide", "fork", "join", "match", "when", "schema", "sync", "callback"])
const LANDING_TYPES = new Set(["string", "number", "boolean", "never", "unknown", "void", "any", "object", "symbol", "bigint"])
const TOKEN =
  /(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|("(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|`(?:[^`\\]|\\.)*`)|(\b\d[\d_]*(?:\.\d+)?\b)|([A-Za-z_$][\w$]*)|(\s+)|(\|>|[\s\S])/g
const highlight = (code: string, lang: string, landing = false): string => {
  if (!["vibe", "ts", "tsx", "js", "typescript"].includes(lang)) return escape(code)
  let out = ""
  for (const m of code.matchAll(TOKEN)) {
    const [text, comment, str, num, ident] = m
    if (comment !== undefined) out += `<span class="c">${escape(text)}</span>`
    else if (str !== undefined) out += `<span class="s">${escape(text)}</span>`
    else if (num !== undefined) out += `<span class="n">${escape(text)}</span>`
    else if (ident !== undefined) {
      if (LANDING_TYPES.has(ident)) out += `<span class="t">${ident}</span>`
      else if (KEYWORDS.has(ident) || landing && LANDING_KEYWORDS.has(ident)) out += `<span class="k">${ident}</span>`
      else if (/^[A-Z]/.test(ident)) out += `<span class="t">${ident}</span>`
      else if (/^\s*\(/.test(code.slice(m.index! + text.length))) out += `<span class="f">${ident}</span>`
      else out += escape(ident)
    } else if (/[!%]|\|>/.test(text)) out += `<span class="k">${escape(text)}</span>`
    else out += escape(text)
  }
  return out
}


// The docs skin is the design mock (.specs/design/mock/src/sites/front): its
// palette, Inter and IBM Plex Mono, the shared header and footer, the docs
// grid, and code blocks as cards whose header row carries a label and a Copy
// button. __FONTS__ becomes each page's relative path to the site's fonts/.
// The mock's --code-* and syntax colors; the --fg/--muted/--rule/--panel/
// --accent/--link aliases are the names render-hovers.ts styles reference.
const CODE_LIGHT = `--code-keyword: var(--water-600); --code-string: var(--sediment-600); --code-type: var(--slate-600); --code-number: var(--ember-600); --code-comment: var(--ink-500); --code-fn: var(--water-700);`
const CODE_DARK = `--code-keyword: var(--water-300); --code-string: var(--sediment-300); --code-type: var(--slate-400); --code-number: var(--ember-400); --code-comment: var(--ink-400); --code-fn: var(--water-400);`
const DARK = `color-scheme: dark;
  --bg: #0d1514; --text: #ece7db; --text-muted: #a8a193; --text-faint: #969083;
  --surface: #131b1a; --surface-2: #1a2422;
  --border: rgb(236 231 219 / 10%); --border-strong: rgb(236 231 219 / 18%);
  --brand: #45c4b2; --warning: var(--sediment-300);
  --inline-code-bg: rgb(236 231 219 / 8%);
  --shadow-2: 0 1px 2px rgb(0 0 0 / 24%), 0 12px 32px rgb(0 0 0 / 40%);
  ${CODE_DARK}`
const css = `@font-face { font-family: "Inter"; font-style: normal; font-weight: 400; font-display: swap; src: url("__FONTS__/inter-latin-400-normal.woff2") format("woff2"); }
@font-face { font-family: "Inter"; font-style: normal; font-weight: 500; font-display: swap; src: url("__FONTS__/inter-latin-500-normal.woff2") format("woff2"); }
@font-face { font-family: "Inter"; font-style: normal; font-weight: 600; font-display: swap; src: url("__FONTS__/inter-latin-600-normal.woff2") format("woff2"); }
@font-face { font-family: "IBM Plex Mono"; font-style: normal; font-weight: 400; font-display: swap; src: url("__FONTS__/ibm-plex-mono-latin-400-normal.woff2") format("woff2"); }
:root {
  color-scheme: light;
  --paper-0: #fffefa; --paper-1: #f7f4ee; --paper-2: #efeae0;
  --ink-900: #211d18; --ink-500: #6e675c; --ink-400: #9a9284;
  --water-300: #7fbfb3; --water-400: #3e9e90; --water-500: #0f766e; --water-600: #0b5b57; --water-700: #134e4a;
  --sediment-300: #f0c169; --sediment-600: #8c5a08;
  --ember-400: #d97757; --ember-600: #a4442a;
  --slate-400: #82a8b8; --slate-600: #3c6879;
  --font-ui: "Inter", ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
  --font-mono: "IBM Plex Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  --bg: var(--paper-1); --text: var(--ink-900); --text-muted: #665f54; --text-faint: #6c655a;
  --surface: var(--paper-0); --surface-2: var(--paper-2);
  --border: rgb(33 29 24 / 8%); --border-strong: rgb(33 29 24 / 14%);
  --brand: var(--water-500); --warning: var(--sediment-600);
  --inline-code-bg: rgb(33 29 24 / 6%);
  --brand-soft: var(--brand);
  --warning-soft: var(--warning);
  --warning-border: var(--warning);
  @supports (color: color-mix(in lab, red, red)) {
    --brand-soft: color-mix(in srgb, var(--brand) 10%, var(--surface));
    --warning-soft: color-mix(in srgb, var(--warning) 12%, var(--surface));
    --warning-border: color-mix(in srgb, var(--warning) 40%, transparent);
  }
  --shadow-2: 0 1px 2px rgb(33 29 24 / 4%), 0 12px 32px rgb(33 29 24 / 10%);
  ${CODE_LIGHT}
  --fg: var(--text); --muted: var(--text-muted); --rule: var(--border); --panel: var(--surface);
  --accent: var(--warning); --link: var(--brand); --mark: var(--warning-soft);
  font-synthesis: none;
  text-rendering: optimizeLegibility;
  -webkit-font-smoothing: antialiased;
}
@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { ${DARK} } }
:root[data-theme="dark"] { ${DARK} }
* { box-sizing: border-box; }
html { scroll-padding-top: 16px; }
body { margin: 0; background: var(--bg); }
:focus-visible { outline: 2px solid var(--brand); outline-offset: 3px; }
.front-page { min-height: 100vh; min-height: 100dvh; display: flex; flex-direction: column; background: var(--bg); color: var(--text); font: 14px/1.5 var(--font-ui); }
.skip { position: absolute; left: 16px; top: -100px; background: var(--surface); color: var(--brand); padding: 8px; z-index: 20; }
.skip:focus { top: 8px; }

/* ---------- header and footer (the landing's chrome) ---------- */

.front-eh { display: flex; align-items: center; gap: 20px; height: 56px; padding: 0 24px; border-bottom: 1px solid var(--border); flex: 0 0 auto; }
.front-eh .brand { font: 600 18px var(--font-ui); color: var(--text); text-decoration: none; }
.front-eh-nav { display: flex; gap: 16px; }
.front-eh-link { color: var(--text-muted); text-decoration: none; font-size: 14px; }
.front-eh-link:hover { color: var(--text); }
.front-efoot { margin-top: auto; display: flex; justify-content: center; gap: 8px; padding: 48px 16px 24px; font-size: 12px; color: var(--text-faint); }
.front-efoot a { color: var(--text-muted); text-decoration: none; }
.front-efoot a:hover { color: var(--text); text-decoration: underline; }

/* ---------- buttons ---------- */

.front-btn { display: inline-flex; align-items: center; justify-content: center; gap: 6px; height: 36px; padding: 0 14px; border-radius: 8px; border: 1px solid transparent; font: 500 14px var(--font-ui); cursor: pointer; text-decoration: none; white-space: nowrap; }
.front-btn-secondary { background: var(--surface); border-color: var(--border-strong); color: var(--text); }
.front-btn-secondary:hover { background: var(--surface-2); }
.front-btn-xs { height: 28px; padding: 0 10px; font-size: 12px; }
button.copy .i-check { display: none; }
button.copy[data-copied] .i-check { display: block; }
button.copy[data-copied] .i-copy { display: none; }
@media (max-width: 640px), (pointer: coarse) { .front-btn { height: 40px; } }

/* ---------- docs grid, sidebar, on-this-page ---------- */

.front-docs-body { flex: 1; width: 100%; max-width: 1200px; margin: 0 auto; padding: 24px 24px 64px; display: grid; grid-template-columns: 220px minmax(0, 1fr) 180px; gap: 32px; align-items: start; }
@media (min-width: 1280px) { .front-docs-body { max-width: 1280px; grid-template-columns: 180px minmax(0, 760px) 160px; gap: 24px; } }
.front-docs-side { position: sticky; top: 24px; max-height: calc(100dvh - 48px); overflow-y: auto; }
.front-docs-menu { display: none; align-items: center; gap: 6px; height: 40px; padding: 0 14px; border: 1px solid var(--border-strong); border-radius: 8px; background: var(--surface); font: 500 14px var(--font-ui); color: var(--text); cursor: pointer; }
.front-docs-nav { display: grid; gap: 2px; align-content: start; }
.front-docs-section { margin: 16px 0 4px; font-size: 12px; font-weight: 600; color: var(--text-faint); }
.front-docs-section:first-child { margin-top: 0; }
.front-docs-view { margin-top: 28px; }
.front-docs-item { display: block; padding: 5px 10px; border-radius: 6px; font-size: 14px; color: var(--text-muted); text-decoration: none; }
.front-docs-item:hover { background: var(--surface-2); color: var(--text); }
.front-docs-item[data-current="true"] { background: var(--brand-soft); color: var(--brand); font-weight: 500; }
.front-docs-toc { position: sticky; top: 24px; display: grid; gap: 2px; align-content: start; }
.front-docs-toc-h { margin: 0 0 4px; font-size: 12px; font-weight: 600; color: var(--text-faint); }
.front-docs-toc-item { display: block; padding: 4px 0; font-size: 13px; color: var(--text-muted); text-decoration: none; }
.front-docs-toc-item:hover { color: var(--text); }

/* ---------- article ---------- */

.front-docs-main { min-width: 0; }
article { min-width: 0; }
.front-docs-main p, .front-docs-main li { overflow-wrap: anywhere; }
article h1 { font-size: 28px; font-weight: 600; letter-spacing: -0.01em; line-height: 1.25; margin: 0 0 16px; scroll-margin-top: 16px; }
article h2 { font-size: 20px; font-weight: 600; line-height: 1.35; margin: 32px 0 8px; scroll-margin-top: 16px; }
article h3 { font-size: 16px; font-weight: 600; line-height: 1.4; margin: 24px 0 8px; scroll-margin-top: 16px; }
article h4 { font-size: 14px; font-weight: 600; margin: 20px 0 6px; scroll-margin-top: 16px; }
h1 a.anchor, h2 a.anchor, h3 a.anchor, h4 a.anchor { color: inherit; text-decoration: none; }
article p { font-size: 16px; line-height: 1.65; margin: 0 0 12px; }
article ul, article ol { margin: 0 0 12px; padding-left: 24px; font-size: 16px; line-height: 1.65; }
article ul { list-style: disc outside; }
article ol { list-style: decimal outside; }
article li + li { margin-top: 4px; }
article a { color: var(--brand); text-decoration: none; }
article a:hover { text-decoration: underline; }
article strong { font-weight: 600; }
article hr { border: none; border-top: 1px solid var(--border); margin: 32px 0; }
article img { max-width: 100%; height: auto; }
article > .part { margin: 8px 0 16px; color: var(--text-faint); font-size: 12px; font-weight: 600; letter-spacing: .06em; text-transform: uppercase; }
.part a.anchor { color: inherit; text-decoration: none; }
article code { font: 0.85em var(--font-mono); background: var(--inline-code-bg); padding: 1px 5px; border-radius: 4px; white-space: normal; overflow-wrap: anywhere; }

/* ---------- callouts ---------- */

.front-callout, article blockquote { margin: 0 0 16px; padding: 10px 14px; border: 1px solid var(--border); border-radius: 8px; background: var(--surface); font-size: 14px; line-height: 1.5; }
article blockquote p { font-size: 14px; line-height: 1.5; margin: 0 0 6px; }
article blockquote p:last-child { margin-bottom: 0; }
.front-callout-warning, article blockquote.not-designed { background: var(--warning-soft); border-color: var(--warning-border); }

/* ---------- code blocks as the mock's cards ---------- */

figure.code { margin: 16px 0; min-width: 0; border: 1px solid var(--border); border-radius: 12px; background: var(--surface); overflow: hidden; }
figure.code figcaption { min-height: 40px; display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 0 8px 0 16px; border-bottom: 1px solid var(--border); }
figure.code .code-label { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font: 12px var(--font-mono); color: var(--text-muted); }
pre.code { margin: 0; padding: 12px 16px; max-width: 100%; overflow-x: auto; font: 13px/1.6 var(--font-mono); color: var(--text); }
pre.code code { display: block; width: max-content; min-width: 100%; background: none; padding: 0; border-radius: 0; font: inherit; white-space: pre; overflow-wrap: normal; }
pre.code .c { color: var(--code-comment); }
pre.code .s { color: var(--code-string); }
pre.code .n, pre.code .l { color: var(--code-number); }
pre.code .k { color: var(--code-keyword); }
pre.code .t { color: var(--code-type); }
pre.code .f { color: var(--code-fn); }
.compare { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 16px; align-items: start; margin: 16px 0; }
.compare figure.code { margin: 0; }
@media (max-width: 1023px) { .compare { grid-template-columns: minmax(0, 1fr); } }

/* ---------- tables ---------- */

.table { overflow-x: auto; max-width: 100%; margin: 0 0 12px; }
table { border-collapse: collapse; font-size: 14px; line-height: 1.45; }
th, td { text-align: left; vertical-align: top; padding: 8px 12px; border: 1px solid var(--border); }
th { background: var(--surface-2); font-weight: 600; }
td code { white-space: nowrap; }

/* ---------- previous/next ---------- */

.front-docs-pagenav { display: flex; justify-content: space-between; gap: 16px; margin-top: 40px; padding-top: 16px; border-top: 1px solid var(--border); }
.front-docs-pagenav a { color: var(--brand); text-decoration: none; font-size: 14px; }
.front-docs-pagenav a:hover { text-decoration: underline; }

/* ---------- phone ---------- */

@media (max-width: 1023px) {
  .front-docs-body { grid-template-columns: minmax(0, 1fr); gap: 16px; row-gap: 24px; align-content: start; }
  .front-docs-side { position: static; max-height: none; overflow-y: visible; }
  .front-docs-menu { display: inline-flex; }
  .front-docs-nav { display: none; margin-top: 8px; padding: 8px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface); }
  .front-docs-nav[data-open="true"] { display: grid; }
  .front-docs-toc { display: none; }
}
@media (max-width: 640px) {
  .front-eh { padding: 0 16px; gap: 12px; }
  .front-docs-body { padding: 16px 16px 48px; }
}
`

// The one inline script every docs page runs: the phone docs menu and the Copy
// button. Copy clones the block's code, drops hover popups, and writes the
// exact source text to the clipboard. The CSP allows it by its hash alone.
const pageScript = `document.addEventListener("click",function(event){
var target=event.target;
var button=target.closest&&target.closest("button.front-docs-menu,button.copy");
if(!button)return;
if(button.classList.contains("front-docs-menu")){
var nav=button.parentElement.querySelector(".front-docs-nav");
var open=nav.getAttribute("data-open")!=="true";
nav.setAttribute("data-open",String(open));
button.setAttribute("aria-expanded",String(open));
return;
}
var code=button.closest("figure.code").querySelector("pre.code code").cloneNode(true);
code.querySelectorAll(".docs-popup").forEach(function(popup){popup.remove()});
try{
var write=navigator.clipboard&&navigator.clipboard.writeText(code.textContent.replace(/\\n$/,""));
if(write)write.catch(function(){});
}catch(error){}
button.setAttribute("data-copied","");
button.querySelector("span").textContent="Copied";
clearTimeout(button._copyTimer);
button._copyTimer=setTimeout(function(){button.removeAttribute("data-copied");button.querySelector("span").textContent="Copy"},1500);
});`
const pageScriptHash = new Bun.CryptoHasher("sha256").update(pageScript).digest("base64")

// The mock's icons, 16px stroke SVGs; the Copy button shows the check while copied.
const ICON_COPY = `<svg class="i-copy" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="5.5" y="5.5" width="8" height="8" rx="1.5"/><path d="M10.5 5.5v-2a1.5 1.5 0 0 0-1.5-1.5H4A1.5 1.5 0 0 0 2.5 3.5v5A1.5 1.5 0 0 0 4 10h1.5"/></svg>`
const ICON_CHECK = `<svg class="i-check" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 8.5l3.2 3.2L13 4.8"/></svg>`
const ICON_CHEVRON = `<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 6.5l3 3 3-3"/></svg>`
const COPY_BUTTON = `<button type="button" class="front-btn front-btn-secondary front-btn-xs copy">${ICON_COPY}${ICON_CHECK}<span>Copy</span></button>`

// A code block's header-row label: the fence role's caption, else the mock's
// language labels (Terminal for shells, Output for text, VibeLang, TypeScript).
const codeLabel = (lang: string): string => {
  const language = lang.toLowerCase()
  if (["sh", "shell", "bash", "zsh", "console"].includes(language)) return "Terminal"
  if (["out", "output", "text", "plaintext", ""].includes(language)) return "Output"
  if (["vibe", "vibelang"].includes(language)) return "VibeLang"
  if (["ts", "typescript", "tsx"].includes(language)) return "TypeScript"
  if (language === "json") return "JSON"
  return lang
}

// group is the bold sidebar label a page is indented under, if any.
type Page = { path: string; label: string; group?: string }
type NavItem = { kind: "view" | "group"; label: string } | { kind: "page"; page: Page }
const inventory = readFileSync(join(root, "inventory.md"), "utf8")
const lines = inventory.split("\n")
const start = lines.findIndex(line => /^## V4 sidebar in order\s*$/.test(line))
if (start < 0) throw new Error('inventory.md: missing "V4 sidebar in order" section')
const navigation: NavItem[] = []
const order: Page[] = []
let lastGroup: string | undefined
for (const line of lines.slice(start + 1)) {
  if (/^## /.test(line)) break
  const view = /^### (.+)$/.exec(line)
  const group = /^- \*\*(.+)\*\*\s*$/.exec(line)
  const page = /^\s*- \[([^\]]+)\]\(((?:src|\.\.\/design\/src)\/[^)]+\.md)\)/.exec(line)
  if (view) navigation.push({ kind: "view", label: view[1]! })
  else if (group) navigation.push({ kind: "group", label: lastGroup = group[1]! })
  else if (page) {
    const path = keyOf(resolve(root, page[2]!))
    if (!path) throw new Error(`inventory.md: page outside docs/user/src and docs/design/src: ${page[2]}`)
    if (order.some(p => p.path === path)) throw new Error(`inventory.md: duplicate page: ${path}`)
    const entry = { path, label: page[1]!, group: /^\s/.test(line) ? lastGroup : undefined }
    order.push(entry)
    navigation.push({ kind: "page", page: entry })
  } else if (line.trim().startsWith("-")) throw new Error(`inventory.md: unrecognized sidebar entry: ${line.trim()}`)
}
if (!order.length) throw new Error("inventory.md: sidebar contains no pages")
function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name)).flatMap(entry =>
    entry.isDirectory() ? files(join(dir, entry.name)) : entry.isFile() && entry.name.endsWith(".md") ? [relative(src, join(dir, entry.name))] : [])
}
// Read a snapshot: writers can keep working, and validation and rendering use the same bytes.
const sources = new Map(files(src).map(path => [path, readFileSync(join(src, path), "utf8")]))
const errors: string[] = []
for (const name of readdirSync(design).filter(f => f.endsWith(".md")).sort()) {
  const markdown = readFileSync(join(design, name), "utf8").replace(/\{\{(\w+)\}\}/g, (match, fact: string) => {
    if (!(fact in facts)) errors.push(`docs/design/src/${name}: unknown placeholder ${match}`)
    return facts[fact] ?? match
  })
  sources.set(`spec/${name}`, markdown)
  if (!order.some(p => p.path === `spec/${name}`)) errors.push(`inventory.md: the sidebar is missing docs/design/src/${name}`)
}
for (const page of order) if (!sources.has(page.path)) errors.push(`inventory.md: missing ${where(page.path)}`)
if (errors.length) throw new Error(errors.join("\n"))
const external = (href: string) => /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(href)
// Returns the link to write, or undefined for a repository file that is not a
// page: the site publishes only pages, so such a link keeps its text and loses its target.
function pageLink(href: string, source: string, destination: string): string | undefined {
  if (!href || external(href) || href.startsWith("#") || href.startsWith("?")) return href
  const [, encoded = "", suffix = ""] = /^([^?#]*)([\s\S]*)$/.exec(href)!
  let path: string
  try { path = decodeURIComponent(encoded) } catch {
    errors.push(`${where(source)}: malformed internal link ${JSON.stringify(href)}`)
    return href
  }
  // A leading slash means the site root, not the host filesystem root.
  const target = path.startsWith("/") ? fileOf(join(".", path)) : resolve(dirname(fileOf(source)), path)
  const md = target.replace(/\.html$/i, ".md")
  const resolved = [md, extname(md) ? "" : md + ".md"].map(keyOf).find(key => key !== undefined && sources.has(key))
  if (resolved) return relative(dirname(destination), output(resolved)).split("/").map(encodeURIComponent).join("/") + suffix
  if (!path.startsWith("/") && target.startsWith(repo + "/") && existsSync(target)) return undefined
  errors.push(`${where(source)}: internal link ${JSON.stringify(href)} points to missing page ${relative(repo, md)}`)
  return href
}
let proposed = 0
let notDesigned = 0
// Filled while rendering: each output page's ids, and each specification chapter's heading id by number.
const ids = new Map<string, Set<string>>()
const chapterIds = new Map<string, string>()
// A specification part opener, such as "## Part I. The language by example", heads its first chapter's page.
const PART = /^Part [IVX]+\. /

async function render(source: string, markdown: string, destination: string, count: boolean): Promise<string> {
  const spec = source.startsWith("spec/")
  const toc: { id: string; text: string }[] = []
  const used = new Set<string>()
  // A specification page uses its sidebar label, such as "3. Errors: the E channel".
  let title = order.find(p => p.path === source)?.label ?? source.replace(/\.md$/, "")
  let firstHeading = !spec
  const marked = new Marked({
    gfm: true,
    renderer: {
      heading(this: any, token: Tokens.Heading) {
        const html = this.parser.parseInline(token.tokens)
        const plain = html.replace(/<[^>]*>/g, "")
        const base = slug(plain)
        let id = base
        let n = 0
        while (used.has(id)) id = `${base}-${++n}`
        used.add(id)
        if (spec && token.depth === 2 && PART.test(plain)) return `<p class="part" id="${escape(id)}"><a class="anchor" href="#${escape(id)}">${html}</a></p>\n`
        const depth = token.depth - shift
        if (depth === 1 && firstHeading) { title = unescape(plain); firstHeading = false }
        if (spec && depth === 1 && /^\d+-/.test(id)) chapterIds.set(id.split("-")[0]!, id)
        if (depth === 2) toc.push({ id, text: unescape(plain) })
        return `<h${depth} id="${escape(id)}"><a class="anchor" href="#${escape(id)}">${html}</a></h${depth}>\n`
      },
      code(token: Tokens.Code) {
        const [lang = "", role = ""] = (token.lang ?? "").split(/\s+/)
        const label = CAPTIONS[role] ?? codeLabel(lang)
        return `<figure class="code side-${escape(role || "none")}"><figcaption><span class="code-label">${escape(label)}</span>${COPY_BUTTON}</figcaption><pre class="code" data-lang="${escape(lang)}"><code>${highlight(token.text, lang)}</code></pre></figure>\n`
      },
      table(this: any, token: Tokens.Table) {
        const head = token.header.map(c => `<th>${this.parser.parseInline(c.tokens)}</th>`).join("")
        const rows = token.rows.map(r => `<tr>${r.map(c => `<td>${this.parser.parseInline(c.tokens)}</td>`).join("")}</tr>`).join("\n")
        return `<div class="table"><table><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>\n`
      },
      blockquote(this: any, token: Tokens.Blockquote) {
        const kind = /^\s*\*\*(Proposed\.|Not designed yet\.)\*\*/.exec(token.text)?.[1]
        if (count && kind === "Proposed.") proposed++
        if (count && kind === "Not designed yet.") notDesigned++
        return `<blockquote${kind ? ` class="callout ${kind === "Proposed." ? "proposed" : "not-designed"}"` : ""}>${this.parser.parse(token.tokens)}</blockquote>\n`
      }
    }
  })
  useHoverRenderer(marked, source, highlight)
  // Specification chapters are written as sections of one document, a chapter
  // under ###; each page's top heading becomes its h1.
  const depths = spec ? marked.lexer(markdown).flatMap(t => t.type === "heading" && !(t.depth === 2 && PART.test(t.text)) ? [(t as Tokens.Heading).depth] : []) : []
  const shift = depths.length ? Math.min(...depths) - 1 : 0
  const body = (marked.parse(markdown) as string).replace(
    /(<figure class="code side-(?:effect|long)">(?:(?!<\/figure>)[\s\S])*<\/figure>)\s*(<figure class="code side-es">(?:(?!<\/figure>)[\s\S])*<\/figure>)/g,
    '<div class="compare">$1$2</div>'
  )
  // Include reference-style links and raw HTML anchors in validation and rewriting.
  const rewritten = await new HTMLRewriter().on("a[href]", {
    element(element) {
      const link = pageLink(element.getAttribute("href")!, source, destination)
      if (link === undefined) element.removeAndKeepContent()
      else element.setAttribute("href", link)
    }
  }).on("img[src]", {
    element(element) {
      const url = element.getAttribute("src")!
      if (url.startsWith("data:")) return
      if (external(url)) { errors.push(`${where(source)}: external image ${JSON.stringify(url)} cannot be self-contained`); return }
      try {
        const path = resolve(dirname(fileOf(source)), decodeURIComponent(url))
        const mime: Record<string, string> = { ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml" }
        if (!mime[extname(path)]) throw new Error("unsupported image format")
        element.setAttribute("src", `data:${mime[extname(path)]};base64,${readFileSync(path).toString("base64")}`)
        element.removeAttribute("srcset")
      } catch { errors.push(`${where(source)}: missing or unsupported image ${JSON.stringify(url)}`) }
    }
  }).transform(new Response(body)).text()
  const href = (path: string) => escape(relative(dirname(destination), output(path)).split("/").map(encodeURIComponent).join("/"))
  const sidebar = navigation.map(item => item.kind === "page"
    ? `<a class="front-docs-item" href="${href(item.page.path)}"${item.page.path === source ? ' data-current="true" aria-current="page"' : ""}>${escape(item.page.label)}</a>`
    : `<p class="front-docs-section${item.kind === "view" ? " front-docs-view" : ""}">${escape(item.label)}</p>`).join("\n")
  const index = order.findIndex(p => p.path === source)
  const prev = index > 0 ? order[index - 1] : undefined
  const next = index >= 0 ? order[index + 1] : undefined
  const pager = (page: Page | undefined, direction: "previous" | "next") => page
    ? `<a class="front-docs-${direction === "previous" ? "prev" : "next"}" rel="${direction === "previous" ? "prev" : "next"}" href="${href(page.path)}">${direction === "previous" ? "← " : ""}${escape(page.label)}${direction === "next" ? " →" : ""}</a>`
    : "<span></span>"
  const home = href(order[0]!.path)
  const fonts = relative(dirname(destination), "fonts").split("/").map(encodeURIComponent).join("/")
  // The mock shows the "On this page" rail only when the page has at least 3 sections.
  const pageToc = toc.length >= 3
    ? `<aside class="front-docs-toc" aria-label="On this page"><p class="front-docs-toc-h">On this page</p>${toc.map(t => `<a class="front-docs-toc-item" href="#${escape(t.id)}">${escape(t.text)}</a>`).join("\n")}</aside>`
    : ""
  const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'sha256-${pageScriptHash}'; font-src 'self'; img-src data:; base-uri 'none'; form-action 'none'">
<title>${escape(title)} · ${spec ? "VibeLang Specification" : "VibeLang Docs"}</title>
<style>${(css + hoverCss).replaceAll("__FONTS__", fonts)}</style>
</head>
<body>
<a class="skip" href="#content">Skip to content</a>
<div class="front-page">
<header class="front-eh"><a class="brand" href="${home}">VibeLang</a><nav class="front-eh-nav" aria-label="Site"><a class="front-eh-link" href="${home}">Docs</a><!-- source-link --><a class="front-eh-link source-link" href="https://smithers.sh/effeck/effeck?utm_source=effeck.dev&amp;utm_medium=source&amp;utm_campaign=vibelang-launch">Source on Smithers</a><!-- /source-link --></nav></header>
<div class="front-docs-body">
<aside class="front-docs-side"><button type="button" class="front-docs-menu" aria-expanded="false">Menu ${ICON_CHEVRON}</button><nav class="front-docs-nav" data-open="false" aria-label="Documentation">${sidebar}</nav></aside>
<main class="front-docs-main" id="content"><p class="front-callout front-callout-warning" role="note"><strong>Warning:</strong> VibeLang is v0. The language will change in major, breaking ways. It is not suitable for production use.</p><article>${rewritten}</article><nav class="front-docs-pagenav" aria-label="Page navigation">${pager(prev, "previous")}${pager(next, "next")}</nav></main>
${pageToc}
</div>
<footer class="front-efoot"><a href="mailto:support@smithers.sh?subject=${escape(encodeURIComponent(`VibeLang docs: ${title} (${source})`))}">Report a bug</a><span aria-hidden="true">·</span><a href="${home}">Docs</a><!-- source-link --><span aria-hidden="true">·</span><a class="source-link" href="https://smithers.sh/effeck/effeck?utm_source=effeck.dev&amp;utm_medium=source&amp;utm_campaign=vibelang-launch">Source on Smithers</a><!-- /source-link --></footer>
</div>
<script>${pageScript}</script>
</body>
</html>\n`
  const found = new Set<string>()
  await new HTMLRewriter().on("[id]", { element(element) { found.add(element.getAttribute("id")!) } }).transform(new Response(page)).text()
  ids.set(destination, found)
  // The header and footer source links follow the landing's VIBELANG_LANDING_SOURCE gate.
  return sourceLinks(page, process.env.VIBELANG_LANDING_SOURCE ?? "none", process.env.VIBELANG_LANDING_GITHUB_URL)
}

// Complete validation before writing output, so a broken link cannot leave a partial build.
const rendered = new Map<string, string>()
for (const [source, markdown] of sources) rendered.set(output(source), await render(source, markdown, output(source), !source.startsWith("spec/")))
const first = order[0]!
rendered.set("index.html", await render(first.path, sources.get(first.path)!, "index.html", false))
// The landing is a static template; the checked hero pair remains its source of truth.
const landing = join(repo, "docs/landing/index.html")
// VIBELANG_LANDING=1 turns it on. Its copy is a draft until Will rewrites it, so a plain deploy keeps
// the onboarding page as the site's index.
if (process.env.VIBELANG_LANDING === "1" && existsSync(landing)) {
  const hero = JSON.parse(readFileSync(join(repo, "docs/site/converter/hero.json"), "utf8"))
  const html = await renderLanding(readFileSync(landing, "utf8"), hero, (code, lang) => highlight(code, lang, true), {
    hovers: process.env.VIBELANG_DOCS_HOVERS !== "0",
    source: process.env.VIBELANG_LANDING_SOURCE ?? "none",
    github: process.env.VIBELANG_LANDING_GITHUB_URL
  })
  rendered.set("index.html", html)
}
// /spec/ opens the specification's first page, so links to the old single-page specification still land on it.
const firstChapter = order.find(p => p.path.startsWith("spec/"))!
rendered.set("spec/index.html", await render(firstChapter.path, sources.get(firstChapter.path)!, "spec/index.html", false))
const fail = (what: string) => {
  if (errors.length) throw new Error(`Docs build failed ${what} (${new Set(errors).size} problems):\n${[...new Set(errors)].map(e => `  - ${e}`).join("\n")}`)
}
fail("while rendering")

// With every page's ids known, check each local link's page and id. The
// specification was one document, so its links name headings with a bare
// fragment; one that names a heading on another chapter's page moves there.
// "Chapter 9" and "Chapters 12 and 14" in specification prose become such
// links; code, headings, and existing links are left alone.
const owners = new Map<string, string[]>()
for (const [path, found] of ids) {
  if (!path.startsWith("spec/") || path === "spec/index.html") continue
  for (const id of found) owners.set(id, [...owners.get(id) ?? [], path])
}
const linkChapter = (n: string) => chapterIds.has(n) ? `<a href="#${chapterIds.get(n)}">${n}</a>` : n
const linkChapters = (article: string) => article.replace(
  /(<pre[\s\S]*?<\/pre>|<code>[\s\S]*?<\/code>|<a [\s\S]*?<\/a>|<h[1-6][\s\S]*?<\/h[1-6]>)|\b(Chapters?) (\d+(?:(?:, and |, | and )\d+)*)\b/g,
  (match, skip: string | undefined, word: string, numbers: string) =>
    skip !== undefined ? skip
      : /^\d+$/.test(numbers) && chapterIds.has(numbers) ? `<a href="#${chapterIds.get(numbers)}">${word} ${numbers}</a>`
      : `${word} ${numbers.replace(/\d+/g, linkChapter)}`
)
let crossLinks = 0
for (const [destination, html] of rendered) {
  const spec = destination.startsWith("spec/")
  const text = spec ? html.replace(/<article>[\s\S]*<\/article>/, linkChapters) : html
  rendered.set(destination, await new HTMLRewriter().on("a[href]", {
    element(element) {
      const href = element.getAttribute("href")!
      if (external(href)) return
      const [, encoded = "", fragment = ""] = /^([^?#]*)(?:\?[^#]*)?(?:#([\s\S]*))?$/.exec(href)!
      let target: string
      let id: string
      try {
        target = encoded ? join(dirname(destination), decodeURIComponent(encoded)) : destination
        id = decodeURIComponent(fragment)
      } catch { errors.push(`${destination}: malformed link ${JSON.stringify(href)}`); return }
      if (!rendered.has(target)) { errors.push(`${destination}: link ${JSON.stringify(href)} points to missing page ${target}`); return }
      if (!id || ids.get(target)!.has(id)) return
      const owner = spec && !encoded ? owners.get(id) : undefined
      if (owner?.length !== 1) { errors.push(`${destination}: link ${JSON.stringify(href)} names no heading on ${target}${owner ? `; ${owner.length} chapters have it` : ""}`); return }
      element.setAttribute("href", `${relative(dirname(destination), owner[0]!).split("/").map(encodeURIComponent).join("/")}#${fragment}`)
      crossLinks++
    }
  }).transform(new Response(text)).text())
}
fail("checking links")

// Every page is also published as Markdown, at its path with .md beside its
// HTML, for coding agents and other plain-text readers. A copy is the page's
// source text: each link to a page becomes that page's .md copy on the site, a
// bare specification fragment moves to its chapter as on the HTML page, and a
// link to a repository file that is not a page keeps only its text. Code keeps
// its text. llms.txt (llmstxt.org) lists the copies, one section per sidebar view.
// The host whose pages the links name. vibelang.sh is the docs domain; a deploy to a
// host that serves this build passes its own origin, so every link resolves there.
const origin = process.env.VIBELANG_DOCS_ORIGIN ?? "https://vibelang.sh"
// A code span, which stays as written, or an inline link, the only link form the sources use.
const LINK = /(?<!`)(`+)(?!`)[\s\S]*?(?<!`)\1(?!`)|\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\(([^()\s]*)\)/g
const lexer = new Marked({ gfm: true })
function markdownLink(href: string, source: string): string | undefined {
  if (!href || external(href) || href.startsWith("?")) return href
  if (href.startsWith("#")) {
    const id = decodeURIComponent(href.slice(1))
    const owner = source.startsWith("spec/") && !ids.get(output(source))!.has(id) ? owners.get(id) : undefined
    return owner?.length === 1 ? `${origin}/${owner[0]!.replace(/\.html$/, ".md")}${href}` : href
  }
  const link = pageLink(href, source, "index.html")
  return link === undefined ? undefined : `${origin}/${link.replace(/\.html(?=[?#]|$)/, ".md")}`
}
function markdownCopy(source: string, markdown: string): string {
  const found: string[] = []
  const out: string[] = []
  let prose: string[] = []
  let fence: string | undefined
  const flush = () => {
    if (prose.length) out.push(prose.join("\n").replace(LINK, (match, ticks: string | undefined, text: string, href: string) => {
      if (ticks !== undefined) return match
      found.push(href)
      const link = markdownLink(href, source)
      return link === undefined ? text : `[${text}](${link})`
    }))
    prose = []
  }
  for (const line of markdown.split("\n")) {
    const bare = line.replace(/^[\s>]*/, "").trimEnd()
    const marker = /^(`{3,}|~{3,})/.exec(bare)?.[1]
    if (fence === undefined && marker) { flush(); fence = marker; out.push(line) }
    else if (fence !== undefined) {
      out.push(line)
      if (marker?.[0] === fence[0] && marker.length >= fence.length && bare === marker) fence = undefined
    } else prose.push(line)
  }
  flush()
  // The scan must find exactly the links marked finds, or a link would keep its repository path.
  const expected: string[] = []
  lexer.walkTokens(lexer.lexer(markdown), token => {
    if (token.type === "link" && token.raw.startsWith("[")) expected.push((token as Tokens.Link).href)
  })
  if (found.join("\n") !== expected.join("\n")) errors.push(`${where(source)}: the Markdown copy finds ${found.length} links where marked finds ${expected.length}`)
  return out.join("\n")
}
const copies = new Map<string, string>()
for (const [source, markdown] of sources) copies.set(source, markdownCopy(source, markdown))
for (const [copy, page] of [["index.md", first], ["spec/index.md", firstChapter]] as const) {
  if (copies.has(copy)) errors.push(`${where(copy)} would collide with the Markdown copy of ${output(copy)}`)
  copies.set(copy, copies.get(page.path)!)
}
// The summary repeats the README's first paragraph. docs/user/llms-essentials.md
// follows it: the forms an agent needs before writing code, so it can start
// without reading pages first (check-snippets compiles its blocks). An agent
// harness may show only the first 2,000 bytes of a large file, so the title,
// summary and essentials come first and must fit in that.
const essentials = readFileSync(join(root, "llms-essentials.md"), "utf8").trim()
  .replace(/\]\((?!https?:)([^)]+\.md)\)/g, (_, page: string) => `](${origin}/${page})`)
const llms = [
  "# VibeLang",
  "",
  "> VibeLang is a language for building reliable TypeScript in the AI era, built on Effect. It gives Effect a simple, readable syntax that compiles to TypeScript, with first-class access to the TypeScript ecosystem.",
  "",
  essentials,
  "",
  `VibeLang is v0. The language will change in major, breaking ways. It is not suitable for production use. To set up VibeLang in a project, read [Set up VibeLang](${origin}/setup.md) and follow it. Each link below is a page's Markdown; replace \`.md\` with \`.html\` for the page with navigation.`,
  ...navigation.flatMap(item => item.kind === "view" ? ["", `## ${item.label}`, ""]
    : item.kind === "page" ? [`- [${item.page.label}](${origin}/${item.page.path})${item.page.group ? `: ${item.page.group}` : ""}`] : []),
  ""
].join("\n")
fail("copying pages as Markdown")
rmSync(site, { recursive: true, force: true })
for (const [path, html] of rendered) {
  const destination = join(site, path)
  mkdirSync(dirname(destination), { recursive: true })
  writeFileSync(destination, html)
}
for (const [path, text] of [...copies, ["llms.txt", llms] as const]) writeFileSync(join(site, path), text)
// The docs pages load Inter and IBM Plex Mono from /fonts/; the landing keeps
// its embedded fonts. The woff2 files and their OFL notice are vendored in
// docs/user/fonts and published beside the pages.
const FONT_FILES = [
  "inter-latin-400-normal.woff2", "inter-latin-500-normal.woff2", "inter-latin-600-normal.woff2",
  "ibm-plex-mono-latin-400-normal.woff2", "OFL.txt"
]
mkdirSync(join(site, "fonts"), { recursive: true })
for (const file of FONT_FILES) {
  const source = join(root, "fonts", file)
  if (!existsSync(source)) throw new Error(`docs/user/fonts/${file} is missing; the docs pages cannot load their fonts`)
  writeFileSync(join(site, "fonts", file), readFileSync(source))
}
const specPages = [...sources.keys()].filter(key => key.startsWith("spec/")).length
console.log(`Wrote ${rendered.size} pages: ${sources.size - specPages} user pages, ${specPages} specification pages, index.html, and spec/index.html.`)
console.log(`Specification: ${facts.testCount} kernel law tests passed; kernel ${facts.kernelLines} lines; ${crossLinks} links moved to another chapter's page.`)
console.log(`Callouts: ${proposed} Proposed; ${notDesigned} Not designed yet (user source pages, excluding duplicate index).`)
console.log(`Markdown: ${copies.size} copies beside the HTML pages, and llms.txt with ${order.length} links.`)
