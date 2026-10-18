import { expect, test } from "bun:test"
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { join, relative, resolve } from "node:path"
import { Marked } from "marked"
import type { BlockHovers, ErrorNode, HoverNode, PageHovers } from "./hovers"
import { renderHovers, useHoverRenderer } from "./render-hovers"

const hover = (target: string, start = 0, extra: Partial<HoverNode> = {}): HoverNode => ({ type: "hover", start, length: target.length, line: 0, character: start, target, text: `const ${target}: number`, ...extra })
const error = (start: number, length: number, text = "Expected a number"): ErrorNode => ({ type: "error", start, length, line: 0, character: start, code: "TS2322", level: "error", text })

test("escapes source, type text, diagnostic attributes, and raw Markdown HTML", () => {
  const html = renderHovers('x <img src="x" onerror="bad()"> & \'', [
    hover("x", 0, { text: '<script>"&\'</script>', docs: '<img src=x onerror="bad()">\n\n**Safe** `a<b` [link](javascript:bad) ![alt](data:bad)' }),
    error(0, 1, '\" onmouseover="bad() < &')
  ])
  expect(html).toContain("&lt;script&gt;&quot;&amp;&#39;&lt;/script&gt;")
  expect(html).toContain("&lt;img")
  expect(html).toContain("&quot; onmouseover=&quot;bad() &lt; &amp;")
  expect(html).toContain("<strong>Safe</strong>")
  expect(html).toContain("<code>a&lt;b</code>")
  expect(html).not.toMatch(/<(script|img|a)\b|data:|javascript:/)
})

test("renders pointer hovers outside the tab order with a type and short Markdown docs", () => {
  const html = renderHovers("let x = 1", [hover("x", 4, { docs: "Runs **here** and returns `number`." })])
  expect(html).not.toContain("tabindex")
  expect(html).toContain('>x<span class="docs-popup" aria-hidden="true">const x: number')
  expect(html).toContain("Runs <strong>here</strong> and returns <code>number</code>.")
  expect(html.endsWith(" = 1")).toBe(true)
  expect(renderHovers("x", [hover("x", 0, { docs: "a".repeat(1000) })])).not.toContain("a".repeat(601))
})

test("underlines error ranges, including ranges overlapping a hover, without data URIs", () => {
  const html = renderHovers("bad + x", [error(0, 5), hover("bad"), error(2, 5)])
  expect(html).toContain('class="docs-error"')
  expect(html).toContain("TS2322: Expected a number")
  expect(html).not.toContain("data:")
  // Removing popups and tags recovers every source character exactly once.
  expect(html.replace(/<span class="docs-popup"[\s\S]*?<\/span><\/span>/g, "").replace(/<[^>]*>/g, "")).toBe("bad + x")
})

test("keeps a hover ending at the block's end and represents an EOF diagnostic", () => {
  const html = renderHovers("x", [hover("x"), error(1, 1, "Missing semicolon")])
  expect(html).toContain('>x<span class="docs-popup"')
  expect(html).toContain('class="docs-error docs-error-end"')
  expect(html).toContain('aria-label="TS2322: Missing semicolon"')
  expect(renderHovers("", [error(0, 0)])).toContain("docs-error-end")
})

test("ignores stale, invalid and overlapping hover ranges without dropping code", () => {
  const nodes = [hover("old"), hover("x", -1), hover("x", 0, { length: 20 }), hover("x"), hover("x")]
  expect(renderHovers("x", nodes).match(/class="docs-hover"/g)).toHaveLength(1)
  expect(renderHovers("x", nodes)).not.toContain("tabindex")
  expect(renderHovers("😀 < &", [])).toBe("😀 &lt; &amp;")
  expect(renderHovers("😀 x", [hover("x", 3)])).toContain('😀 <span class="docs-hover"')
})

// Exercise the opt-in renderer with a real page cache, without depending on a
// compiler or the developer's generated cache. Every call owns its fixture.
function page(blocks: BlockHovers[], markdown: string, flag: string | null = "1", cached = true): string {
  const cache = resolve(import.meta.dir, "../../.cache/docs-hovers/src")
  mkdirSync(cache, { recursive: true })
  const dir = mkdtempSync(join(cache, "render-hovers-test-"))
  const source = `${relative(cache, dir)}/page.md`
  const entry: PageHovers = { key: "test", page: `src/${source}`, blocks, failures: [] }
  if (cached) writeFileSync(join(dir, "page.json"), JSON.stringify(entry))
  const previous = process.env.VIBELANG_DOCS_HOVERS
  try {
    if (flag === null) delete process.env.VIBELANG_DOCS_HOVERS
    else process.env.VIBELANG_DOCS_HOVERS = flag
    const marked = new Marked({ renderer: { code: ({ text, lang }) => `<figure class="code"><figcaption>${lang}</figcaption><pre class="code"><code>${text}</code></pre></figure>\n` } })
    useHoverRenderer(marked, source, text => text)
    return marked.parse(markdown) as string
  } finally {
    if (previous === undefined) delete process.env.VIBELANG_DOCS_HOVERS
    else process.env.VIBELANG_DOCS_HOVERS = previous
    rmSync(dir, { recursive: true, force: true })
  }
}

test("makes each block with hovers one labeled stop linked to its own keyboard type list", async () => {
  const blocks = [
    { number: 1, code: "x + y", nodes: [hover("x", 0, { docs: "Runs **here**." }), hover("y", 4)] },
    { number: 2, code: "z", nodes: [hover("z")] },
    { number: 3, code: "bad", nodes: [error(0, 3)] },
    { number: 4, code: "stale", nodes: [hover("old")] }
  ]
  const html = page(blocks, blocks.map(b => `\`\`\`ts\n${b.code}\n\`\`\``).join("\n\n"))
  expect(html.match(/tabindex="0"/g)).toHaveLength(2)
  expect(html.match(/class="docs-hover"/g)).toHaveLength(3)
  expect(html.match(/class="docs-popup"/g)).toHaveLength(3)
  expect(html.match(/aria-label="Code example with types"/g)).toHaveLength(2)
  expect(html).not.toMatch(/<(?:span|code|div)\b[^>]*tabindex/)
  expect(html).not.toMatch(/<(?:a|button|input|summary)\b/)
  const described: string[] = [], lists: string[] = []
  await new HTMLRewriter().on("pre[tabindex]", { element(element) {
    expect(element.getAttribute("role")).toBe("group")
    described.push(element.getAttribute("aria-describedby")!)
  } }).on(".docs-types", { element(element) { lists.push(element.getAttribute("id")!) } })
    .transform(new Response(html)).text()
  expect(new Set(described).size).toBe(2)
  expect(described).toEqual(lists)
  expect(html.match(/<\/pre><div class="docs-types"/g)).toHaveLength(2)
  expect(html).toContain("<p>Types in this example</p><dl>")
  expect(html).toContain('<dt><code>x</code></dt><dd>const x: number<span class="docs-docs">Runs <strong>here</strong>.</span></dd>')
  expect(html).toContain('<dt><code>y</code></dt><dd>const y: number</dd>')
  expect(html).toContain('class="docs-error"')
})

test("deduplicates identical type entries while preserving different types and docs", () => {
  const nodes = [hover("x"), hover("x", 4), hover("x", 8, { text: "const x: string" }), hover("x", 12, { docs: "Different **scope**." })]
  const html = page([{ number: 1, code: "x + x + x + x", nodes }], "```ts\nx + x + x + x\n```")
  expect(html.match(/class="docs-popup"/g)).toHaveLength(4)
  expect(html.match(/<dt>/g)).toHaveLength(3)
  expect(html).toContain('<dd>const x: string</dd>')
  expect(html).toContain("Different <strong>scope</strong>.")
})

test("escapes the keyboard list and shares the popup's documentation limits", () => {
  const node = hover("x", 0, { text: "<script>&\"'</script>", docs: '<img src=x onerror="bad()">\n\n' + "a".repeat(1000) })
  const html = page([{ number: 1, code: "x", nodes: [node] }], "```ts\nx\n```")
  expect(html).toContain('<dd>&lt;script&gt;&amp;&quot;&#39;&lt;/script&gt;')
  expect(html).not.toMatch(/<(?:script|img)\b/)
  expect(html).not.toContain("a".repeat(601))
})

test("leaves the original renderer unchanged without the flag or a matching cache", () => {
  const blocks = [{ number: 1, code: "x", nodes: [hover("x")] }]
  const markdown = "```ts\nx\n```"
  const original = '<figure class="code"><figcaption>ts</figcaption><pre class="code"><code>x</code></pre></figure>\n'
  expect(page(blocks, markdown, null)).toBe(original)
  expect(page(blocks, markdown, "0")).toBe(original)
  expect(page(blocks, markdown, "1", false)).toBe(original)
  expect(page(blocks, "```ts effect\nx\n```", "1")).not.toContain("docs-hover")
  expect(page(blocks, "```js\nx\n```", "1")).not.toContain("docs-hover")
})

test("CSS reveals the adjacent type list on block keyboard focus and keeps pointer popups", () => {
  const child = Bun.spawnSync(["bun", "--eval", `import { hoverCss } from ${JSON.stringify(resolve(import.meta.dir, "render-hovers.ts"))}; process.stdout.write(hoverCss)`], {
    env: { ...process.env, VIBELANG_DOCS_HOVERS: "1" }, stdout: "pipe", stderr: "pipe"
  })
  expect(child.exitCode).toBe(0)
  const css = child.stdout.toString()
  expect(css).toMatch(/\.docs-types\s*\{\s*display:\s*none;/)
  expect(css).toContain("pre.code:focus-visible + .docs-types { display: block; }")
  expect(css).toContain("pre.code[tabindex]:focus-visible { outline: 2px solid var(--link);")
  expect(css).toContain(".docs-hover:hover > .docs-popup { display: block; }")
  expect(css).not.toContain(".docs-hover:focus")
})
