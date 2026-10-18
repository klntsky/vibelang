import { expect, test } from "bun:test"
import type { ErrorNode, HoverNode } from "./hovers"
import { renderHovers } from "./render-hovers"

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

test("renders one keyboard-focusable hover with a type and short Markdown docs", () => {
  const html = renderHovers("let x = 1", [hover("x", 4, { docs: "Runs **here** and returns `number`." })])
  expect(html.match(/tabindex="0"/g)).toHaveLength(1)
  expect(html).toContain('>x<span class="docs-popup" role="tooltip">')
  expect(html).toContain('<span class="docs-type">const x: number</span>')
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
  expect(renderHovers("x", nodes).match(/tabindex="0"/g)).toHaveLength(1)
  expect(renderHovers("😀 < &", [])).toBe("😀 &lt; &amp;")
  expect(renderHovers("😀 x", [hover("x", 3)])).toContain('😀 <span class="docs-hover"')
})
