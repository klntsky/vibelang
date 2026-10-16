// Build the design document: concatenate docs/design/src/*.md into
// docs/design/effectscript-design.md, then render a self-contained HTML page
// (inline CSS, no external requests) next to it. Run: bun render.ts
import { Marked, type Tokens } from "marked"
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs"
import { join, resolve } from "node:path"

const designDir = join(import.meta.dir, "..", "design")
const srcDir = join(designDir, "src")
const experimentsDir = join(import.meta.dir, "..", "..", "experiments")
const parts = readdirSync(srcDir).filter((f) => f.endsWith(".md")).sort()

// Facts the document states are derived here, not typed by hand. The build
// refuses to run if the law suite fails: no [Kernel] claim outlives a broken law.
// The 100,000-element stress cases also run while build lanes share the host.
const suite = Bun.spawnSync(["bun", "test", "--timeout=30000"], { cwd: experimentsDir, stdout: "pipe", stderr: "pipe" })
const suiteOutput = suite.stdout.toString() + suite.stderr.toString()
const ran = /Ran (\d+) tests? across (\d+) files?/.exec(suiteOutput)
const failed = /(\d+) fail/.exec(suiteOutput)
if (suite.exitCode !== 0 || ran === null || failed === null || failed[1] !== "0") {
  console.error(suiteOutput.split("\n").slice(-30).join("\n"))
  throw new Error("experiments test suite failed; refusing to build the design document")
}
const lines = (path: string) => readFileSync(join(experimentsDir, path), "utf8").split("\n").length - 1
const facts: Record<string, string> = {
  testCount: ran[1]!,
  testFiles: ran[2]!,
  kernelLines: String(lines("../packages/runtime/src/kernel/kernel.ts") + lines("../packages/runtime/src/kernel/internals.ts"))
}

const markdown = parts
  .map((f) => readFileSync(join(srcDir, f), "utf8").trimEnd())
  .join("\n\n")
  .replace(/\{\{(\w+)\}\}/g, (match, name: string) => {
    if (!(name in facts)) throw new Error(`unknown placeholder ${match}`)
    return facts[name]!
  }) + "\n"

// GitHub-compatible heading slugs, so the in-document links work in both outputs.
const slug = (text: string) =>
  text
    .toLowerCase()
    .replace(/<[^>]+>/g, "")
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .trim()
    .replace(/\s/g, "-")

const toc: Array<{ depth: number; id: string; text: string }> = []
const slugCounts = new Map<string, number>()
const escape = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")

// Code block roles, written after the language in the fence: ```ts effect
const CAPTIONS: Record<string, string> = {
  effect: "Effect today (TypeScript)",
  es: "EffectScript",
  gen: "Generated TypeScript",
  dts: "Generated declaration",
  ts: "TypeScript",
  sh: "Shell",
  out: "Output"
}

// A small build-time TypeScript highlighter: comments, strings, numbers,
// keywords, literals, capitalized names, and called functions.
const KEYWORDS = new Set([
  "abstract", "as", "async", "await", "break", "case", "catch", "class", "const", "continue", "control",
  "declare", "default", "do", "else", "enum", "export", "extends", "finally", "for", "from", "function",
  "get", "if", "implements", "import", "in", "instanceof", "interface", "keyof", "let", "namespace", "new",
  "of", "private", "protected", "public", "readonly", "return", "satisfies", "service", "set", "static",
  "switch", "throw", "try", "type", "typeof", "using", "var", "void", "while", "yield"
])
const LITERALS = new Set([
  "true", "false", "null", "undefined", "this", "never", "unknown", "any", "string", "number", "boolean",
  "bigint", "symbol", "object"
])
const TOKEN =
  /(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|("(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|`(?:[^`\\]|\\.)*`)|(\b\d[\d_]*(?:\.\d+)?\b)|([A-Za-z_$][\w$]*)|(\s+)|([\s\S])/g
const highlight = (code: string, lang: string): string => {
  if (!["ts", "tsx", "js", "typescript"].includes(lang)) return escape(code)
  let out = ""
  for (const m of code.matchAll(TOKEN)) {
    const [text, comment, str, num, ident] = m
    if (comment !== undefined) out += `<span class="c">${escape(text)}</span>`
    else if (str !== undefined) out += `<span class="s">${escape(text)}</span>`
    else if (num !== undefined) out += `<span class="n">${escape(text)}</span>`
    else if (ident !== undefined) {
      if (KEYWORDS.has(ident)) out += `<span class="k">${ident}</span>`
      else if (LITERALS.has(ident)) out += `<span class="l">${ident}</span>`
      else if (/^[A-Z]/.test(ident)) out += `<span class="t">${ident}</span>`
      else if (/^\s*\(/.test(code.slice(m.index! + text.length))) out += `<span class="f">${ident}</span>`
      else out += escape(ident)
    } else out += escape(text)
  }
  return out
}

const marked = new Marked({
  gfm: true,
  renderer: {
    heading(this: any, token: Tokens.Heading) {
      const html = this.parser.parseInline(token.tokens)
      // Repeated headings get -1, -2, ... suffixes, as on GitHub.
      const base = slug(token.text)
      const seen = slugCounts.get(base) ?? 0
      slugCounts.set(base, seen + 1)
      const id = seen === 0 ? base : `${base}-${seen}`
      if (token.depth === 2 || token.depth === 3) toc.push({ depth: token.depth, id, text: html })
      return `<h${token.depth} id="${id}"><a class="anchor" href="#${id}">${html}</a></h${token.depth}>\n`
    },
    code(token: Tokens.Code) {
      const [lang = "", role = ""] = (token.lang ?? "").split(/\s+/)
      const caption = CAPTIONS[role]
      const body = `<pre class="code" data-lang="${escape(lang)}"><code>${highlight(token.text, lang)}</code></pre>`
      const label = caption ? `<figcaption>${caption}</figcaption>` : ""
      return `<figure class="code side-${escape(role || "none")}">${label}${body}</figure>\n`
    },
    table(this: any, token: Tokens.Table) {
      const head = token.header.map((c) => `<th>${this.parser.parseInline(c.tokens)}</th>`).join("")
      const rows = token.rows
        .map((r) => `<tr>${r.map((c) => `<td>${this.parser.parseInline(c.tokens)}</td>`).join("")}</tr>`)
        .join("\n")
      return `<div class="table"><table><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>\n`
    }
  }
})

// An "Effect today" block immediately followed by an "EffectScript" block
// renders side by side.
const body = (marked.parse(markdown) as string).replace(
  /(<figure class="code side-effect">(?:(?!<\/figure>)[\s\S])*<\/figure>)\s*(<figure class="code side-es">(?:(?!<\/figure>)[\s\S])*<\/figure>)/g,
  '<div class="compare">$1$2</div>'
)
// "Chapter 9" and "Chapters 12 and 14" in prose link to their chapters.
// Code, headings, and existing links are left alone.
const chapterIds = new Map(toc.filter((t) => t.depth === 3).map((t) => [t.id.split("-")[0]!, t.id]))
const linkChapter = (n: string) => (chapterIds.has(n) ? `<a href="#${chapterIds.get(n)}">${n}</a>` : n)
const linked = body.replace(
  /(<pre[\s\S]*?<\/pre>|<code>[\s\S]*?<\/code>|<a [\s\S]*?<\/a>|<h[1-6][\s\S]*?<\/h[1-6]>)|\b(Chapters?) (\d+(?:(?:, and |, | and )\d+)*)\b/g,
  (match, skip: string | undefined, word: string, numbers: string) =>
    skip !== undefined
      ? skip
      : /^\d+$/.test(numbers) && chapterIds.has(numbers)
        ? `<a href="#${chapterIds.get(numbers)}">${word} ${numbers}</a>`
        : `${word} ${numbers.replace(/\d+/g, linkChapter)}`
)
const nav = toc
  .map((t) => `<a class="toc-${t.depth}" href="#${t.id}">${t.text}</a>`)
  .join("\n")

const html = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>EffectScript Design</title>
<style>
:root {
  --bg: #fbfbf9; --fg: #1d1d1f; --muted: #5d6066; --rule: #dedcd6; --panel: #f2f1ed;
  --code-bg: #f4f3ef; --code-fg: #1d1d1f; --link: #1f5fbf; --accent: #8a4b00; --mark: #fff3c4;
  --hl-comment: #6b6f76; --hl-string: #0b6b3a; --hl-number: #9a3412; --hl-keyword: #7c2d92;
  --hl-literal: #9a3412; --hl-type: #1d4ed8; --hl-fn: #0e6e8c;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg: #15161a; --fg: #e6e4df; --muted: #a2a3a8; --rule: #33353b; --panel: #1d1f24;
    --code-bg: #1b1d22; --code-fg: #e6e4df; --link: #7fb0ff; --accent: #f0b46a; --mark: #3a3420;
    --hl-comment: #8b8f96; --hl-string: #8fd19e; --hl-number: #f2a477; --hl-keyword: #d7a6f2;
    --hl-literal: #f2a477; --hl-type: #8cb4ff; --hl-fn: #7fd3e8;
  }
}
:root[data-theme="dark"] {
  --bg: #15161a; --fg: #e6e4df; --muted: #a2a3a8; --rule: #33353b; --panel: #1d1f24;
  --code-bg: #1b1d22; --code-fg: #e6e4df; --link: #7fb0ff; --accent: #f0b46a; --mark: #3a3420;
  --hl-comment: #8b8f96; --hl-string: #8fd19e; --hl-number: #f2a477; --hl-keyword: #d7a6f2;
  --hl-literal: #f2a477; --hl-type: #8cb4ff; --hl-fn: #7fd3e8;
}
* { box-sizing: border-box; }
html { scroll-padding-top: 16px; }
body {
  margin: 0; background: var(--bg); color: var(--fg);
  font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
}
.layout { display: grid; grid-template-columns: 290px minmax(0, 1fr); gap: 40px; max-width: 1480px; margin: 0 auto; padding: 0 24px; }
nav {
  position: sticky; top: 0; align-self: start; max-height: 100vh; overflow-y: auto;
  padding: 28px 0 40px; font-size: 13.5px; line-height: 1.45; border-right: 1px solid var(--rule);
}
nav a { display: block; color: var(--muted); text-decoration: none; padding: 3px 12px 3px 0; }
nav a:hover { color: var(--link); }
nav .toc-2 { color: var(--fg); font-weight: 600; margin-top: 10px; }
nav .toc-3 { padding-left: 14px; }
main { min-width: 0; padding: 28px 0 96px; }
h1 { font-size: 34px; line-height: 1.2; margin: 8px 0 12px; letter-spacing: -0.01em; }
h2 { font-size: 28px; margin: 80px 0 8px; padding-top: 18px; border-top: 3px solid var(--fg); letter-spacing: -0.01em; }
h3 { font-size: 22px; margin: 48px 0 10px; padding-top: 10px; border-top: 1px solid var(--rule); }
h4 { font-size: 17px; margin: 30px 0 8px; }
h1 a.anchor, h2 a.anchor, h3 a.anchor, h4 a.anchor { color: inherit; text-decoration: none; }
p, li { max-width: 860px; }
a { color: var(--link); }
code { font: 13.5px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; background: var(--code-bg); padding: 1px 4px; border-radius: 4px; }
figure.code { margin: 14px 0 18px; min-width: 0; }
figure.code figcaption {
  font: 600 11.5px/1.4 -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  letter-spacing: 0.04em; text-transform: uppercase; color: var(--muted); margin: 0 0 6px 2px;
}
figure.side-es figcaption { color: var(--accent); }
pre.code {
  background: var(--code-bg); color: var(--code-fg); border: 1px solid var(--rule); border-radius: 8px;
  padding: 14px 16px; overflow-x: auto; margin: 0; max-width: 100%;
}
figure.side-es pre.code { border-color: color-mix(in srgb, var(--accent) 45%, var(--rule)); }
pre.code code { background: none; padding: 0; font-size: 13px; white-space: pre; }
pre.code[data-lang="text"] { background: var(--panel); }
pre.code .c { color: var(--hl-comment); font-style: italic; }
pre.code .s { color: var(--hl-string); }
pre.code .n { color: var(--hl-number); }
pre.code .k { color: var(--hl-keyword); font-weight: 600; }
pre.code .l { color: var(--hl-literal); }
pre.code .t { color: var(--hl-type); }
pre.code .f { color: var(--hl-fn); }
.compare { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 14px; align-items: start; margin: 14px 0 18px; }
.compare figure.code { margin: 0; }
@media (max-width: 1180px) { .compare { grid-template-columns: minmax(0, 1fr); } }
blockquote { margin: 16px 0; padding: 10px 18px; border-left: 4px solid var(--accent); background: var(--panel); border-radius: 0 8px 8px 0; }
blockquote p { margin: 6px 0; }
.table { overflow-x: auto; margin: 14px 0 20px; border: 1px solid var(--rule); border-radius: 8px; }
table { border-collapse: collapse; width: 100%; font-size: 14px; line-height: 1.45; }
th, td { text-align: left; vertical-align: top; padding: 8px 12px; border-bottom: 1px solid var(--rule); }
th { background: var(--panel); font-weight: 600; white-space: nowrap; }
tr:last-child td { border-bottom: none; }
td code { font-size: 12.5px; }
strong { font-weight: 650; }
hr { border: none; border-top: 1px solid var(--rule); margin: 32px 0; }
@media (max-width: 960px) {
  .layout { grid-template-columns: 1fr; gap: 0; padding: 0 16px; }
  nav { position: static; max-height: none; border-right: none; border-bottom: 1px solid var(--rule); padding: 16px 0; }
  nav .toc-3 { display: none; }
  h1 { font-size: 28px; }
}
</style>
</head>
<body>
<div class="layout">
<nav aria-label="Contents">
${nav}
</nav>
<main>
${linked}
</main>
</div>
</body>
</html>
`
// Check the rendered targets, including generated navigation and chapter links.
// Relative file links resolve beside the combined design, not its source parts.
const ids = new Set<string>()
const hrefs = new Set<string>()
await new HTMLRewriter()
  .on("[id]", { element: (element) => { ids.add(element.getAttribute("id")!) } })
  .on("a[href]", { element: (element) => { hrefs.add(element.getAttribute("href")!) } })
  .transform(new Response(html)).text()
const broken: string[] = []
for (const href of hrefs) {
  if (/^(?:[a-z][a-z\d+.-]*:|\/\/)/i.test(href)) continue
  try {
    const [target, fragment] = href.split("#", 2)
    const path = decodeURIComponent(target!.split("?", 1)[0]!)
    if (path ? !existsSync(resolve(designDir, path)) : fragment && !ids.has(decodeURIComponent(fragment))) {
      broken.push(href)
    }
  } catch {
    broken.push(href)
  }
}
if (broken.length) throw new Error(`Design build has broken internal links:\n${broken.map(href => `  - ${href}`).join("\n")}`)
writeFileSync(join(designDir, "effectscript-design.md"), markdown)
writeFileSync(join(designDir, "effectscript-design.html"), html)
console.log(`wrote effectscript-design.md (${markdown.length} chars) and effectscript-design.html from ${parts.length} parts; ${facts.testCount} tests passed; kernel ${facts.kernelLines} lines`)
