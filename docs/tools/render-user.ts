// Run: cd docs/tools && bun render-user.ts
// Builds the one docs site in docs/user/site: the user pages in docs/user/src
// and, under spec/, one page per specification chapter in docs/design/src.
// docs/user/inventory.md orders the sidebar. Each page also gets a Markdown copy
// beside its HTML, and llms.txt lists the copies. Uses the package.json pin: marked 18.0.12.
import { Marked, type Tokens } from "marked"
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { dirname, extname, join, relative, resolve } from "node:path"
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
const slug = (s: string) => s.toLowerCase().replace(/<[^>]+>/g, "").replace(/[^\p{L}\p{N}\s-]/gu, "").trim().replace(/\s/g, "-")
const output = (s: string) => s.replace(/\.md$/i, ".html")
// Code block roles, written after the language in the fence: ```ts effect
const CAPTIONS: Record<string, string> = {
  effect: "Effect today (TypeScript)",
  long: "Effeck long form",
  es: "Effeck",
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


// Styles copied from render.ts; user-doc layout overrides follow.
const css = `:root {
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

`
const userCss = `
.layout { grid-template-columns: 258px minmax(0, 1fr) 200px; max-width: 1540px; gap: 32px; }
main, article, aside, nav { min-width: 0; }
main { grid-column: 2; grid-row: 1; }
.sidebar { grid-column: 1; grid-row: 1; }
.page-toc { grid-column: 3; grid-row: 1; }
body { overflow-wrap: anywhere; }
.sidebar { padding-right: 16px; }
.sidebar .brand { color: var(--fg); font-size: 18px; font-weight: 700; margin-bottom: 20px; }
.sidebar .view { margin: 22px 0 8px; padding: 0; border: 0; color: var(--fg); font-size: 14px; text-transform: uppercase; letter-spacing: .06em; }
.sidebar .group { margin: 16px 0 6px; padding: 0; border: 0; color: var(--fg); font-size: 13px; }
.sidebar a { padding: 5px 8px; border-radius: 5px; }
.sidebar a[aria-current="page"] { background: var(--panel); color: var(--link); font-weight: 650; box-shadow: inset 3px 0 var(--link); }
.page-toc { border: 0; }
.page-toc h2 { font-size: 13.5px; border: 0; margin: 0 0 10px; padding: 0; }
.page-toc a { padding: 5px 0; }
.pager { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 16px; border: 0; border-top: 1px solid var(--rule); margin-top: 48px; padding: 24px 0 0; position: static; max-height: none; }
.pager a { border: 1px solid var(--rule); border-radius: 8px; padding: 12px; color: var(--link); }
.pager span { display: block; font-size: 12px; color: var(--muted); }
.pager .next { text-align: right; grid-column: 2; }
.callout { border: 1px solid var(--rule); border-left: 4px solid var(--accent); background: var(--mark); }
.callout.not-designed { border-left-style: double; border-left-width: 6px; }
.callout strong:first-child { color: var(--accent); }
.v0-warning { margin: 0 0 24px; padding: 10px 18px; border-radius: 0 8px 8px 0; }
article > .part { margin: 8px 0 18px; color: var(--accent); font-size: 13px; font-weight: 650; letter-spacing: .06em; text-transform: uppercase; }
.part a.anchor { color: inherit; text-decoration: none; }
img, svg, video { max-width: 100%; height: auto; }
pre { max-width: 100%; overflow-x: auto; overflow-wrap: normal; }
.table { max-width: 100%; }
.skip { position: absolute; left: 16px; top: -100px; background: var(--bg); padding: 8px; z-index: 1; }
.skip:focus { top: 8px; }
@media (max-width: 1200px) {
  .layout { grid-template-columns: 240px minmax(0, 1fr); }
  .page-toc { position: static; grid-column: 2; grid-row: 1; max-height: none; padding-bottom: 0; }
  .sidebar { grid-row: 1 / 3; }
  main { grid-column: 2; grid-row: 2; }
}
@media (max-width: 760px) {
  .layout { display: flex; flex-direction: column; gap: 0; padding: 0 16px; }
  .sidebar { width: 100%; max-height: 260px; overflow-y: auto; padding: 16px 0; }
  .page-toc { padding: 20px 0 0; }
  main { width: 100%; padding-top: 16px; }
  h1 { font-size: 28px; }
  h2 { font-size: 24px; margin-top: 48px; }
  .pager { gap: 8px; }
}
`

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
        if (depth === 1 && firstHeading) { title = plain; firstHeading = false }
        if (spec && depth === 1 && /^\d+-/.test(id)) chapterIds.set(id.split("-")[0]!, id)
        if (depth === 2) toc.push({ id, text: plain })
        return `<h${depth} id="${escape(id)}"><a class="anchor" href="#${escape(id)}">${html}</a></h${depth}>\n`
      },
      code(token: Tokens.Code) {
        const [lang = "", role = ""] = (token.lang ?? "").split(/\s+/)
        const caption = CAPTIONS[role]
        return `<figure class="code side-${escape(role || "none")}">${caption ? `<figcaption>${caption}</figcaption>` : ""}<pre class="code" data-lang="${escape(lang)}"><code>${highlight(token.text, lang)}</code></pre></figure>\n`
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
    ? `<a href="${href(item.page.path)}"${item.page.path === source ? ' aria-current="page"' : ""}>${escape(item.page.label)}</a>`
    : `<${item.kind === "view" ? "h2" : "h3"} class="${item.kind}">${escape(item.label)}</${item.kind === "view" ? "h2" : "h3"}>`).join("\n")
  const index = order.findIndex(p => p.path === source)
  const prev = index > 0 ? order[index - 1] : undefined
  const next = index >= 0 ? order[index + 1] : undefined
  const pager = (page: Page | undefined, direction: "previous" | "next") => page ? `<a class="${direction}" rel="${direction === "previous" ? "prev" : "next"}" href="${href(page.path)}"><span>${direction === "previous" ? "Previous" : "Next"}</span>${escape(page.label)}</a>` : ""
  const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'">
<title>${escape(title)} · ${spec ? "Effeck Specification" : "Effeck Docs"}</title>
<style>${css}${userCss}</style>
</head>
<body>
<a class="skip" href="#content">Skip to content</a>
<div class="layout">
<nav class="sidebar" aria-label="Documentation"><a class="brand" href="${escape(relative(dirname(destination), "index.html"))}">Effeck Docs</a>${sidebar}</nav>
<nav class="page-toc" aria-label="On this page"><h2>On this page</h2>${toc.map(t => `<a href="#${escape(t.id)}">${escape(t.text)}</a>`).join("\n")}</nav>
<main id="content"><p class="callout v0-warning" role="note"><strong>Warning:</strong> Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.</p><article>${rewritten}</article><nav class="pager" aria-label="Page navigation">${pager(prev, "previous")}${pager(next, "next")}</nav></main>
</div>
</body>
</html>\n`
  const found = new Set<string>()
  await new HTMLRewriter().on("[id]", { element(element) { found.add(element.getAttribute("id")!) } }).transform(new Response(page)).text()
  ids.set(destination, found)
  return page
}

// Complete validation before writing output, so a broken link cannot leave a partial build.
const rendered = new Map<string, string>()
for (const [source, markdown] of sources) rendered.set(output(source), await render(source, markdown, output(source), !source.startsWith("spec/")))
const first = order[0]!
rendered.set("index.html", await render(first.path, sources.get(first.path)!, "index.html", false))
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
const origin = "https://effeck.dev"
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
// The summary repeats the README's first paragraph.
const llms = [
  "# Effeck",
  "",
  "> Effeck is a language for building reliable TypeScript in the AI era, built on Effect. It gives Effect a simple, readable syntax that compiles to TypeScript, with first-class access to the TypeScript ecosystem.",
  "",
  `Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use. To set up Effeck in a project, read [Set up Effeck](${origin}/setup.md) and follow it. Each link below is a page's Markdown; replace \`.md\` with \`.html\` for the page with navigation.`,
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
const specPages = [...sources.keys()].filter(key => key.startsWith("spec/")).length
console.log(`Wrote ${rendered.size} pages: ${sources.size - specPages} user pages, ${specPages} specification pages, index.html, and spec/index.html.`)
console.log(`Specification: ${facts.testCount} kernel law tests passed; kernel ${facts.kernelLines} lines; ${crossLinks} links moved to another chapter's page.`)
console.log(`Callouts: ${proposed} Proposed; ${notDesigned} Not designed yet (user source pages, excluding duplicate index).`)
console.log(`Markdown: ${copies.size} copies beside the HTML pages, and llms.txt with ${order.length} links.`)
