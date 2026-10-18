import { existsSync, readFileSync } from "node:fs"
import { resolve } from "node:path"
import { Marked, type Tokens } from "marked"
import type { ErrorNode, HoverNode, PageHovers } from "./hovers"

const escape = (text: string) => text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;")
const markdown = new Marked({ renderer: {
  html: ({ text }) => escape(text),
  // Documentation links often name declarations outside the published site.
  link(this: any, token: Tokens.Link) { return this.parser.parseInline(token.tokens) },
  image: ({ text }) => escape(text)
} })
const shortDocs = (docs: string) => {
  const summary = docs.trim().split(/\n\s*\n/).slice(0, 2).join("\n\n")
  const short = summary.length > 600 ? summary.slice(0, 599) + "…" : summary
  return short.split(/\n\s*\n/).filter(Boolean).map(p => `<span class="docs-paragraph">${markdown.parseInline(p)}</span>`).join("")
}

/** Offsets are UTF-16, as in the LSP cache. Invalid/stale ranges never eat code. */
export function renderHovers(code: string, nodes: (HoverNode | ErrorNode)[], highlight: (text: string) => string = escape): string {
  const valid = nodes.filter(n => Number.isInteger(n.start) && Number.isInteger(n.length) && n.start >= 0 && n.start <= code.length && n.length >= 0)
  const errors = valid.filter((n): n is ErrorNode => n.type === "error")
  const hovers = valid.filter((n): n is HoverNode => n.type === "hover" && n.length > 0 && n.start + n.length <= code.length && code.slice(n.start, n.start + n.length) === n.target)
    .sort((a, b) => a.start - b.start || b.length - a.length)
  const part = (start: number, end: number) => {
    const cuts = [...new Set([start, end, ...errors.flatMap(n => [n.start, n.start + n.length]).filter(p => p > start && p < end)])].sort((a, b) => a - b)
    return cuts.slice(0, -1).map((from, i) => {
      const to = cuts[i + 1]
      const active = errors.filter(n => n.start < to && n.start + n.length > from)
      const text = highlight(code.slice(from, to))
      return active.length ? `<span class="docs-error" title="${escape(active.map(n => `${n.code}: ${n.text}`).join("\n"))}">${text}</span>` : text
    }).join("")
  }
  let out = "", cursor = 0
  for (const node of hovers) {
    if (node.start < cursor) continue
    const end = node.start + node.length
    out += part(cursor, node.start)
    out += `<span class="docs-hover" tabindex="0">${part(node.start, end)}<span class="docs-popup" role="tooltip"><span class="docs-type">${escape(node.text)}</span>${node.docs ? `<span class="docs-docs">${shortDocs(node.docs)}</span>` : ""}</span></span>`
    cursor = end
  }
  out += part(cursor, code.length)
  for (const node of errors.filter(n => n.start === code.length)) {
    out += `<span class="docs-error docs-error-end" title="${escape(`${node.code}: ${node.text}`)}" aria-label="${escape(`${node.code}: ${node.text}`)}"></span>`
  }
  return out
}

// Fixed positioning escapes pre's overflow-x scrollport. A viewport-sized panel
// also keeps long compiler signatures readable on narrow screens, in both themes.
export const hoverCss = process.env.EFFECK_DOCS_HOVERS === "1" ? `
.docs-hover { cursor: help; text-decoration: underline dotted var(--muted); text-underline-offset: 3px; }
.docs-hover:focus-visible { outline: 2px solid var(--link); outline-offset: 2px; }
.docs-popup {
  display: none; position: fixed; z-index: 10; left: 50%; bottom: 16px; transform: translateX(-50%);
  width: max-content; max-width: min(60ch, calc(100vw - 32px)); max-height: 45vh; overflow: auto;
  padding: 12px 16px; border: 1px solid var(--rule); border-radius: 8px;
  background: var(--panel); color: var(--fg); box-shadow: 0 4px 20px color-mix(in srgb, var(--fg) 20%, transparent);
  font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  white-space: normal; overflow-wrap: anywhere; text-decoration: none; cursor: auto;
}
.docs-hover:hover > .docs-popup, .docs-hover:focus-within > .docs-popup { display: block; }
.docs-type { display: block; font: 13px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; white-space: pre-wrap; overflow-wrap: anywhere; }
.docs-docs { display: block; margin-top: 8px; border-top: 1px solid var(--rule); padding-top: 8px; }
.docs-paragraph { display: block; margin: 4px 0; }
pre.code .docs-popup code { white-space: normal; font-size: 13px; }
.docs-error { text-decoration: underline wavy var(--accent); text-underline-offset: 3px; }
.docs-error-end { display: inline-block; width: .6ch; border-bottom: 2px solid var(--accent); }
` : ""

/** One opt-in renderer swap per page; an absent cache leaves Marked untouched. */
export function useHoverRenderer(marked: Marked, source: string, highlight: (text: string, lang: string) => string): void {
  if (process.env.EFFECK_DOCS_HOVERS !== "1" || source.startsWith("spec/")) return
  const file = resolve(import.meta.dir, "../../.cache/docs-hovers", "src", source.replace(/\.md$/, ".json"))
  if (!existsSync(file)) return
  const page: PageHovers = JSON.parse(readFileSync(file, "utf8"))
  const blocks = new Map<string, typeof page.blocks>()
  const normalized = (code: string) => code.replace(/\n+$/, "")
  for (const block of page.blocks) {
    const key = normalized(block.code)
    blocks.set(key, [...blocks.get(key) ?? [], block])
  }
  const originalCode = marked.defaults.renderer!.code
  marked.use({ renderer: { code(token: Tokens.Code) {
    const [lang, role] = (token.lang ?? "").split(/\s+/)
    if (!["effeck", "ts"].includes(lang) || role === "effect") return false
    const block = blocks.get(normalized(token.text))?.shift()
    if (!block?.nodes.length) return false
    // Keep the existing caption, figure and pre markup byte for byte.
    const renderer = marked.defaults.renderer!
    const original = originalCode.call(renderer, token) as string
    return original.replace(/<code>[\s\S]*<\/code>/, () => `<code>${renderHovers(token.text, block.nodes, text => highlight(text, lang))}</code>`)
  } } })
}
