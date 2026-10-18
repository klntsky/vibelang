import { test, expect } from "bun:test"
import { readFileSync } from "node:fs"
import { renderLanding, sourceLinks } from "./render-landing"
const template = readFileSync(new URL("../landing/index.html", import.meta.url), "utf8")
const hero = { effect: "one\n\ntwo\n", effeck: "function nameOf() {}\nnameOf()\n" }
const highlight = (s: string) => s
const options = { hovers: true, source: "smithers", hover: async () => "compiler <type>" }
test("numbers, counts, hover and both source locations are derived", async () => {
 const html = await renderLanding(template, hero, highlight, options)
 expect(html.match(/class="source-link"|front-eh-link source-link/g)).toHaveLength(2)
 expect(html).toContain('>3 lines</div>')
 expect(html).toContain('>2 lines</div>')
 expect(html.match(/compiler &lt;type&gt;/g)).toHaveLength(2)
 expect(html).not.toContain('<!-- hero:')
})
test("hovers off removes tips and never calls compiler; none removes all source spans", async () => {
 const html = await renderLanding(template, hero, highlight, { ...options, source: "none", hovers: false, hover: async () => { throw Error("called") } })
 expect(html).not.toContain('class="front-tip"')
 expect(html).not.toContain('role="tooltip"')
 expect(html).not.toContain('hover:nameOf')
 expect(html).not.toContain('source-link')
})
test("missing compiler hover fails loudly", async () => {
 await expect(renderLanding(template, hero, highlight, { ...options, hover: async () => "" })).rejects.toThrow("compiler hover")
})
test("every required marker fails loudly", async () => {
 for (const marker of ['<!-- hero:effect -->','<!-- hero:effeck -->','<!-- lines:effect -->','<!-- lines:effeck -->','<!-- rows -->','<!-- hover:nameOf -->','<!-- source-link -->','<!-- /source-link -->']) {
  await expect(renderLanding(template.replace(marker, ""), hero, highlight, options)).rejects.toThrow()
 }
})
test("github changes both links and escapes attributes", () => {
 const html = sourceLinks(template, "github", 'https://github.com/org/repo?a="')
 expect(html.match(/Source on GitHub/g)).toHaveLength(2)
 expect(html).toContain('&quot;')
 expect(() => sourceLinks(template, "github")).toThrow()
})
test("CSP script hash is computed from the actual script", () => {
 const script = /<script>([\s\S]*?)<\/script>/.exec(template)![1]
 const hash = new Bun.CryptoHasher("sha256").update(script).digest("base64")
 expect(template).toContain(`script-src 'sha256-${hash}'`)
})
test("Copy acknowledges success and selects the command on clipboard failure", async () => {
 const { runInNewContext } = await import("node:vm")
 const script = /<script>([\s\S]*?)<\/script>/.exec(template)![1]
 for (const fails of [false, true]) {
  let click: () => Promise<void> = async () => {}, selected = false, focused = false, copied = ""
  const label = { textContent: "Copy" }, icon = { innerHTML: "copy" }
  const button = { addEventListener: (_: string, callback: any) => { click = () => callback.call(button) }, querySelector: (s: string) => s === "svg" ? icon : label }
  const input = { value: "npm create effeck@latest my-app", focus: () => { focused = true }, select: () => { selected = true } }
  runInNewContext(script, { document: { querySelectorAll: () => [], getElementById: (id: string) => id === "copy-button" ? button : input }, navigator: { clipboard: { writeText: async (text: string) => { if (fails) throw Error("denied"); copied = text } } }, setTimeout: () => 0 })
  await click()
  expect(selected).toBe(fails)
  expect(focused).toBe(fails)
  expect(label.textContent).toBe(fails ? "Copy" : "Copied")
  if (!fails) expect(copied).toBe(input.value)
 }
})
