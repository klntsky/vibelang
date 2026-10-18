import { afterAll, expect, test } from "bun:test"
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { type PageHovers, sanitize, trimDocs, updateHovers, words } from "./hovers"
import { leaks } from "./leaks"

test("hovers identifiers, keywords, the topic, and template substitutions, not comments or strings", () => {
  const code = 'const n = f("x y") // note\nconst s = `${n.length} and ${g({ a: 1 })}`\nreturn % + 1'
  expect(words(code).map(w => [w.target, code.slice(w.start, w.start + w.target.length)]).map(([t, s]) => t === s ? t : `${t}!=${s}`))
    .toEqual(["const", "n", "f", "const", "s", "n", "length", "g", "a", "return", "%"])
})

test("keeps Effeck's notes and the JSDoc's first paragraph", () => {
  expect(trimDocs("Runs here and returns `void`.\n\nLogs one or more messages.\nAt the default level.\n\n**Example**\n\n```ts\nEffect.log(1)\n```\n\n*@since* — 2.0.0"))
    .toBe("Runs here and returns `void`.\n\nLogs one or more messages.\nAt the default level.")
  expect(trimDocs("*@category* — constructors")).toBe("")
  expect(trimDocs("")).toBe("")
})

test("strips local links and paths", () => {
  const raw = "Like [`String`](file:///Users/someone/effectscript/packages/runtime/node_modules/effect/src/Config.ts#1439,17-1439,23) " +
    "from /Users/someone/effectscript/packages/runtime/node_modules/effect/dist/Config.d.ts in /private/var/folders/x/T/effeck-hovers-1/3/block-1.effeck"
  expect(leaks(raw)).not.toEqual([])
  const clean = sanitize(raw, ["/private/var/folders/x/T/effeck-hovers-1/3"])
  expect(clean).toBe("Like `String` from effect/dist/Config.d.ts in ./block-1.effeck")
  expect(leaks(clean)).toEqual([])
})

// The cache tests drive the local compiler through a wrapper script: the
// cache key hashes the compiler file, so editing the wrapper stands in for a
// rebuilt binary without copying it.
const compiler = resolve(import.meta.dir, "../../typescript/built/local/tsc")
const root = mkdtempSync(join(tmpdir(), "hovers-test-"))
const cache = join(root, "cache")
const wrapper = join(root, "effeck")
const page = join(root, "src/page.md")
const writeWrapper = (build: number) => {
  writeFileSync(wrapper, `#!/bin/sh\n# build ${build}\nexec "${compiler}" "$@"\n`)
  chmodSync(wrapper, 0o755)
}
const writePage = (name: string) => {
  mkdirSync(join(root, "src"), { recursive: true })
  writeFileSync(page, ["# Page", "", "```ts", 'import { Effect } from "effect"', "",
    "/** Wraps {@link Effect.succeed} for the docs. */", `function ${name}(n: number) {`, "  return n + 1", "}", `const answer = ${name}(41)`, "```", ""].join("\n"))
}
const run = () => updateHovers({ compiler: wrapper, root, cache })
const entry = (): PageHovers => JSON.parse(readFileSync(join(cache, "src/page.json"), "utf8"))
afterAll(() => rmSync(root, { recursive: true, force: true }))

test.skipIf(!existsSync(compiler))("a second run reuses the page's entry", async () => {
  writeWrapper(1)
  writePage("wrap")
  expect(await run()).toMatchObject({ pages: 1, hits: 0, misses: 1 })
  const first = readFileSync(join(cache, "src/page.json"), "utf8")
  expect(await run()).toMatchObject({ pages: 1, hits: 1, misses: 0 })
  expect(readFileSync(join(cache, "src/page.json"), "utf8")).toBe(first)
  const hover = entry().blocks[0].nodes.find(n => n.type === "hover" && n.target === "wrap")
  expect(hover).toMatchObject({ type: "hover", line: 3, character: 9, text: "function wrap(n: number): number" })
}, 60_000)

test.skipIf(!existsSync(compiler))("editing a block refreshes its page", async () => {
  writePage("increment")
  expect(await run()).toMatchObject({ hits: 0, misses: 1 })
  expect(entry().blocks[0].nodes.some(n => n.type === "hover" && n.target === "increment")).toBe(true)
  expect(entry().blocks[0].nodes.some(n => n.type === "hover" && n.target === "wrap")).toBe(false)
}, 60_000)

test.skipIf(!existsSync(compiler))("a rebuilt compiler refreshes every page", async () => {
  expect(await run()).toMatchObject({ hits: 1, misses: 0 })
  writeWrapper(2)
  expect(await run()).toMatchObject({ hits: 0, misses: 1 })
}, 60_000)

test.skipIf(!existsSync(compiler))("no cached text contains a forbidden string", async () => {
  const text = readFileSync(join(cache, "src/page.json"), "utf8")
  expect(leaks(text)).toEqual([])
  expect(text).not.toContain("file:")
  // The server links Effect.succeed to its source in the build machine's
  // node_modules; the link's text remains.
  const hover = entry().blocks[0].nodes.find(n => n.type === "hover" && n.target === "increment" && n.line === 3)
  expect(hover?.type === "hover" && hover.docs).toBe(
    "Plain JavaScript: Effeck code calls it directly. TypeScript callers see `Effect.Effect<number, never, never>`.\n\nWraps Effect.succeed for the docs.")
})
