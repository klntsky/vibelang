import { expect, test } from "bun:test"
import { blockFilenames, classify, extract, markdownPages, parseDiagnostics } from "./check-snippets"

const block = extract('```ts\nconst answer = 42\n```')[0]
const diagnostic = (code: string, message = "example") => ({ code, message })
test("extracts effeck and ts fences, skips unnamed TypeScript examples, and preserves lines and directives", () => {
  const blocks = extract('heading\n```ts\n// TypeScript (.ts)\nconst ignored = 1\n```\n~~~effeck es\n// expect-error: EK2001 TS2322\nfunction f() {}\n~~~\n```json\n{}\n```\n```effeck\n// compiler-gap: return contract\nfunction g() {}\n```')
  expect(blocks.map(b => [b.number, b.line, b.firstLine])).toEqual([[2, 7, "function f() {}"], [3, 14, "function g() {}"]])
  expect(blocks[0].expected).toEqual(["EK2001", "TS2322"])
  expect(blocks[1].gap).toBe("return contract")
})
test("skips Effect comparison fences without numbering them", () => {
  const blocks = extract('```ts effect\nconst x = yield* work\n```\n```effeck es\nconst x = work()\n```')
  expect(blocks.map(b => [b.number, b.firstLine])).toEqual([[1, "const x = work()"]])
})
test("the fence decides the language: effeck is Effeck, ts is TypeScript", () => {
  const blocks = extract([
    "```effeck", "const a = 1", "```",
    "```ts", "const b: number = 2", "```",
    "```effeck es", "// main.effeck", "const c = 3", "```",
    "```ts", "// TypeScript (.ts)", "// helper.ts", "export const d = 4", "```",
    "```tsx", "const e = <div />", "```"
  ].join("\n"))
  expect(blocks.map(b => [b.number, b.language, b.filename])).toEqual([[1, "effeck", undefined], [2, "ts", undefined], [3, "effeck", "main.effeck"], [4, "ts", "helper.ts"]])
  expect([...blockFilenames(blocks).values()]).toEqual(["block-1.effeck", "block-2.ts", "main.effeck", "helper.ts"])
})
test("a filename caption must name a file of its fence's language", () => {
  expect(() => extract("```ts\n// main.effeck\nconst a = 1\n```")).toThrow("A ts fence names main.effeck")
  expect(() => extract("```effeck\n// helper.ts\nconst a = 1\n```")).toThrow("An effeck fence names helper.ts")
})
test("only unresolved names and local imports are automatic fragments", () => {
  expect(classify(block, []).status).toBe("ok")
  expect(classify(block, [diagnostic("TS2304")]).status).toBe("fragment")
  expect(classify(block, [diagnostic("TS2552")]).status).toBe("fragment")
  expect(classify(block, [diagnostic("TS2307", "Cannot find module './fixture.effeck' or its corresponding type declarations.")]).status).toBe("fragment")
  expect(classify(block, [diagnostic("TS2307", "Cannot find module 'missing-package'")]).status).toBe("fail")
  expect(classify(block, [diagnostic("TS2304"), diagnostic("TS2322")]).status).toBe("fail")
})
test("expected diagnostics must occur and do not hide other errors", () => {
  const expected = { ...block, expected: ["EK2001"] }
  expect(classify(expected, []).status).toBe("fail")
  expect(classify(expected, [diagnostic("EK2001")]).status).toBe("ok")
  expect(classify(expected, [diagnostic("EK2001"), diagnostic("TS2322")]).status).toBe("fail")
})
test("explicit gaps remain checked, and stale markers fail", () => {
  const gap = { ...block, gap: "D56 program conversion" }
  expect(classify(gap, [diagnostic("TS2345")]).status).toBe("compiler-gap")
  expect(classify(gap, []).status).toBe("fail")
  expect(classify({ ...gap, expected: ["EK2001"] }, []).status).toBe("compiler-gap")
  expect(classify({ ...gap, expected: ["EK2001"] }, [diagnostic("EK2001")]).status).toBe("fail")
  expect(classify(gap, [diagnostic("TS2304")]).status).toBe("fail")
  expect(classify({ ...block, fragment: true }, [diagnostic("TS1005")]).status).toBe("fragment")
})
test("parses native ES and TS errors and ignores notes and warnings", () => {
  expect(parseDiagnostics("block-2.effeck(4,8): error EK2001: failure escapes\n  Widen the contract.\nblock-3.effeck(1,1): warning EK2004: unreachable\nerror TS18003: No inputs were found")).toEqual([
    { file: "block-2.effeck", diagnostic: { code: "EK2001", message: "failure escapes", line: 4, column: 8 } },
    { diagnostic: { code: "TS18003", message: "No inputs were found" } }
  ])
})

import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { join, relative, resolve } from "node:path"
import { tmpdir } from "node:os"
const compiler = resolve(import.meta.dir, "../../typescript/built/local/tsc")
test.skipIf(!existsSync(compiler))("checks shared declarations, schema type/value names, captions, and expected errors together", async () => {
  const root = mkdtempSync(join(tmpdir(), "snippet-check-test-"))
  try {
    writeFileSync(join(root, "page.md"), [
      "```effeck", "// descriptor.effeck", "export const Item = Schema.Struct({ name: Schema.String })", "```",
      "```effeck", "type Item = typeof Item.Type", "function read(input: unknown) { return Schema.decodeUnknownEffect(Item)(input) }", "```",
      "```effeck", "import { Item } from './descriptor.effeck'", "const sample = { name: 'Ada' } satisfies typeof Item.Type", "```",
      "```effeck", "// expect-error: TS2322", "const invalid: number = 'text'", "```"
    ].join("\n"))
    const output = join(root, "report.json")
    const child = Bun.spawn(["bun", join(import.meta.dir, "check-snippets.ts"), "--root", root, "--compiler", relative(process.cwd(), compiler), "--json", output], { stdout: "pipe", stderr: "pipe" })
    const [stdout, stderr, status] = await Promise.all([new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited])
    expect({ status, stderr, failures: stdout.split("\n").filter(l => l.includes("fail")) }).toEqual({ status: 0, stderr: "", failures: [expect.stringContaining('"fail":0')] })
    expect(JSON.parse(readFileSync(output, "utf8")).totals).toMatchObject({ blocks: 4, ok: 4, fail: 0, fragment: 0 })
  } finally { rmSync(root, { recursive: true, force: true }) }
})


test("filename captions include TypeScript companions and only use the first two lines", () => {
  const blocks = extract([
    "```ts", "// TypeScript (.ts)", "// launch-options.ts", "export const code = 2", "```",
    "```effeck", "// main.effeck", "import { code } from './launch-options.ts'", "```",
    "```effeck", "// prose", "// nested/helper.effeck", "export const answer = 42", "```",
    "```effeck", "// prose", "// more prose", "// ignored.effeck", "const value = 1", "```"
  ].join("\n"))
  expect(blocks.map(b => b.filename)).toEqual(["launch-options.ts", "main.effeck", "nested/helper.effeck", undefined])
  expect([...blockFilenames(blocks).values()]).toEqual(["launch-options.ts", "main.effeck", "nested/helper.effeck", "block-4.effeck"])
  expect(() => extract("```ts\n// ../escape.ts\n``` ")).toThrow("Invalid snippet filename")
})

test("repeated captions stay independent and cannot overwrite a named file", () => {
  const blocks = extract("```effeck\n// main.effeck\nconst a = 1\n```\n```effeck\n// main.effeck\nconst a = 2\n```\n```effeck\n// block-1.effeck\nconst b = 3\n```")
  expect([...blockFilenames(blocks).values()]).toEqual(["block-1-2.effeck", "block-2.effeck", "block-1.effeck"])
})

test.skipIf(!existsSync(compiler))("checks named TypeScript companions and maps named-file errors to Markdown", async () => {
  const root = mkdtempSync(join(tmpdir(), "snippet-named-test-"))
  try {
    writeFileSync(join(root, "page.md"), [
      "```ts", "// TypeScript (.ts)", "// launch-options.ts", "export const launchOptions: { code: number } = { code: 2 }", "```",
      "```effeck", "// main.effeck", "import { launchOptions } from './launch-options.ts'", "const code: number = launchOptions.code", "```",
      "```ts", "// TypeScript (.ts)", "// invalid.ts", "// expect-error: TS2322", "export const invalid: number = 'text'", "```",
      "```effeck", "// nested/check.effeck", "// expect-error: TS2322", "const wrong: number = 'text'", "```"
    ].join("\n"))
    const output = join(root, "report.json")
    const child = Bun.spawn(["bun", join(import.meta.dir, "check-snippets.ts"), "--root", root, "--compiler", compiler, "--json", output], { stdout: "pipe", stderr: "pipe" })
    const [stdout, stderr, status] = await Promise.all([new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited])
    expect({ status, stderr }).toEqual({ status: 0, stderr: "" })
    const report = JSON.parse(readFileSync(output, "utf8"))
    expect(report.totals).toMatchObject({ blocks: 4, ok: 4, fragment: 0, fail: 0 })
    expect(report.pages[0].blocks[2].diagnostics[0].line).toBe(15)
    expect(report.pages[0].blocks[3].diagnostics[0].line).toBe(20)
  } finally { rmSync(root, { recursive: true, force: true }) }
})
test.skipIf(!existsSync(compiler))("compiles an effeck fence as Effeck and a ts fence as TypeScript", async () => {
  const root = mkdtempSync(join(tmpdir(), "snippet-language-test-"))
  try {
    writeFileSync(join(root, "page.md"), [
      "```effeck", "error Missing { id: string }", "```",
      "```ts", "// expect-error: TS1434", "error Missing { id: string }", "```"
    ].join("\n"))
    const output = join(root, "report.json")
    const child = Bun.spawn(["bun", join(import.meta.dir, "check-snippets.ts"), "--root", root, "--compiler", compiler, "--json", output], { stdout: "pipe", stderr: "pipe" })
    const [, stderr, status] = await Promise.all([new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited])
    expect({ status, stderr }).toEqual({ status: 0, stderr: "" })
    expect(JSON.parse(readFileSync(output, "utf8")).totals).toMatchObject({ blocks: 2, ok: 2, fail: 0 })
  } finally { rmSync(root, { recursive: true, force: true }) }
})
test("markdownPages skips the built site, which holds each page's Markdown copy", () => {
  const root = mkdtempSync(join(tmpdir(), "check-snippets-pages-"))
  try {
    for (const path of ["a.md", "guide/b.md", "site/a.md", "site/spec/c.md", "guide/site/d.md"]) {
      mkdirSync(join(root, path, ".."), { recursive: true })
      writeFileSync(join(root, path), "# Page\n")
    }
    expect(markdownPages(root).map(p => relative(root, p))).toEqual(["a.md", "guide/b.md", "guide/site/d.md"])
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
