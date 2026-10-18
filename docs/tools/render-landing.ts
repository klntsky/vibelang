import { mkdtempSync, writeFileSync, rmSync } from "node:fs"
import { join, resolve } from "node:path"
import { pathToFileURL } from "node:url"
import { startServer } from "./lsp"
import { linkModules } from "./check-snippets"
import { parseHover } from "./hovers"

const escape = (s: string) => s.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;")
export function replaceOne(html: string, marker: string, value: string): string {
  if (html.split(marker).length !== 2) throw new Error(`Landing requires one ${marker}`)
  return html.replace(marker, () => value)
}
export function sourceLinks(html: string, source: string, github?: string): string {
  const open = "<!-- source-link -->", close = "<!-- /source-link -->"
  // Both locations are required; a missing or unmatched marker must not publish.
  const spans = [...html.matchAll(/<!-- source-link -->([\s\S]*?)<!-- \/source-link -->/g)]
  if (spans.length !== 2 || html.split(open).length !== 3 || html.split(close).length !== 3 || spans.some(s => s[1].includes(open))) throw new Error("Landing requires paired header and footer source-link markers")
  if (!["none", "smithers", "github"].includes(source)) throw new Error(`Invalid VIBELANG_LANDING_SOURCE: ${source}`)
  if (source === "github" && !github?.startsWith("https://github.com/")) throw new Error("VIBELANG_LANDING_SOURCE=github needs VIBELANG_LANDING_GITHUB_URL, a https://github.com/ URL")
  return html.replace(/<!-- source-link -->([\s\S]*?)<!-- \/source-link -->/g, (_, link: string) => source === "none" ? "" : source === "smithers" ? link : link.replace(/href="[^"]*"/, `href="${escape(github!)}"`).replace("Source on Smithers", "Source on GitHub"))
}
export async function nameOfHover(code: string): Promise<string> {
  // Keep the project in the workspace, including all generated files.
  const temp = mkdtempSync(resolve(import.meta.dir, "../../.cache/landing-hover-"))
  let server: ReturnType<typeof startServer> | undefined
  try {
    linkModules(temp)
    writeFileSync(join(temp, "package.json"), '{"type":"module"}')
    writeFileSync(join(temp, "tsconfig.json"), JSON.stringify({ compilerOptions: { strict: true, target: "ESNext", module: "NodeNext", moduleResolution: "NodeNext", skipLibCheck: true }, files: ["hero.vibelang"] }))
    writeFileSync(join(temp, "hero.vibelang"), code)
    // Deploy provides its release binary; direct builds resolve vibelang from PATH.
    server = startServer([process.env.VIBELANG_BINARY || "vibelang", "--lsp", "--stdio"])
    await server.request("initialize", { processId: process.pid, rootUri: pathToFileURL(temp).href, capabilities: { textDocument: { hover: { contentFormat: ["markdown"] } } } })
    server.notify("initialized", {})
    const uri = pathToFileURL(join(temp, "hero.vibelang")).href
    server.notify("textDocument/didOpen", { textDocument: { uri, languageId: "vibe", version: 1, text: code } })
    const start = code.indexOf("nameOf")
    if (start < 0) throw new Error("Landing hero has no nameOf")
    const before = code.slice(0, start).split("\n")
    const hover = await server.request("textDocument/hover", { textDocument: { uri }, position: { line: before.length - 1, character: before.at(-1)!.length } })
    const parsed = hover && parseHover(hover.contents?.value ?? String(hover.contents), [temp])
    if (!parsed?.text.trim()) throw new Error("Landing requires compiler hover for nameOf")
    return parsed.text
  } finally {
    await server?.close()
    rmSync(temp, { recursive: true, force: true })
  }
}

export async function renderLanding(template: string, hero: Record<"effect" | "vibelang", string>, highlight: (code: string, lang: string) => string, options: { hovers: boolean; source: string; github?: string; hover?: (code: string) => Promise<string> }): Promise<string> {
  let html = template
  const tip = /<template id="nameOf-tip">([\s\S]*?)<\/template>/g
  const tips = [...html.matchAll(tip)]
  if (tips.length !== 1) throw new Error("Landing requires one nameOf-tip template")
  let body = tips[0][1]
  if (body.split("<!-- hover:nameOf -->").length !== 2) throw new Error("Landing requires one hover:nameOf marker")
  if (options.hovers) {
    const hover = await (options.hover ?? nameOfHover)(hero.vibelang)
    if (!hover?.trim()) throw new Error("Landing requires compiler hover for nameOf")
    body = replaceOne(body, "<!-- hover:nameOf -->", escape(hover))
  }
  html = html.replace(tip, "")
  const counts = {} as Record<"effect" | "vibelang", number>
  for (const side of ["effect", "vibelang"] as const) {
    if (typeof hero[side] !== "string" || !hero[side].trim()) throw new Error(`Landing requires nonempty hero.${side}`)
    const lines = hero[side].replace(/\n$/, "").split("\n")
    counts[side] = lines.length
    const code = lines.map((line, i) => {
      let rendered = highlight(line, side === "vibelang" ? "vibelang" : "ts")
      if (side === "vibelang" && options.hovers) rendered = rendered.replace(/(<span class="f">)?\bnameOf\b(<\/span>)?/g, match => `<span class="front-tip" tabindex="0">${match}<span class="front-tip-body" role="tooltip">${body}</span></span>`)
      return `<span class="front-code-line"><span class="front-code-num" aria-hidden="true">${i + 1}</span><span class="front-code-src">${rendered}\n</span></span>`
    }).join("")
    html = replaceOne(html, `<!-- hero:${side} -->`, code)
    html = replaceOne(html, `<!-- lines:${side} -->`, String(lines.length))
  }
  html = replaceOne(html, "<!-- rows -->", String(Math.max(counts.effect, counts.vibe)))
  return sourceLinks(html, options.source, options.github)
}
