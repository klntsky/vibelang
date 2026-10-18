import { expect, test } from "bun:test"
import { blockFilenames, classify, defaultCompiler, extract, markdownPages, normalizeOutput, parseDiagnostics } from "./check-snippets"

const block = extract('```ts\nconst answer = 42\n```')[0]
const diagnostic = (code: string, message = "example") => ({ code, message })
test("extracts vibe and ts fences, skips unnamed TypeScript examples, and preserves lines and directives", () => {
  const blocks = extract('heading\n```ts\n// TypeScript (.ts)\nconst ignored = 1\n```\n~~~vibe es\n// expect-error: EK2001 TS2322\nfunction f() {}\n~~~\n```json\n{}\n```\n```vibe\n// compiler-gap: return contract\nfunction g() {}\n```')
  expect(blocks.map(b => [b.number, b.line, b.firstLine])).toEqual([[2, 7, "function f() {}"], [3, 14, "function g() {}"]])
  expect(blocks[0].expected).toEqual(["EK2001", "TS2322"])
  expect(blocks[1].gap).toBe("return contract")
})
test("skips Effect comparison fences without numbering them", () => {
  const blocks = extract('```ts effect\nconst x = yield* work\n```\n```vibe es\nconst x = work()\n```')
  expect(blocks.map(b => [b.number, b.firstLine])).toEqual([[1, "const x = work()"]])
})
test("the fence decides the language: vibe is VibeLang, ts is TypeScript", () => {
  const blocks = extract([
    "```vibe", "const a = 1", "```",
    "```ts", "const b: number = 2", "```",
    "```vibe es", "// main.vibe", "const c = 3", "```",
    "```ts", "// TypeScript (.ts)", "// helper.ts", "export const d = 4", "```",
    "```tsx", "const e = <div />", "```"
  ].join("\n"))
  expect(blocks.map(b => [b.number, b.language, b.filename])).toEqual([[1, "vibe", undefined], [2, "ts", undefined], [3, "vibe", "main.vibe"], [4, "ts", "helper.ts"]])
  expect([...blockFilenames(blocks).values()]).toEqual(["block-1.vibe", "block-2.ts", "main.vibe", "helper.ts"])
})
test("a filename caption must name a file of its fence's language", () => {
  expect(() => extract("```ts\n// main.vibe\nconst a = 1\n```")).toThrow("A ts fence names main.vibe")
  expect(() => extract("```vibe\n// helper.ts\nconst a = 1\n```")).toThrow("A vibe fence names helper.ts")
})
test("only unresolved names and local imports are automatic fragments", () => {
  expect(classify(block, []).status).toBe("ok")
  expect(classify(block, [diagnostic("TS2304")]).status).toBe("fragment")
  expect(classify(block, [diagnostic("TS2552")]).status).toBe("fragment")
  expect(classify(block, [diagnostic("TS2307", "Cannot find module './fixture.vibe' or its corresponding type declarations.")]).status).toBe("fragment")
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
  expect(parseDiagnostics("block-2.vibe(4,8): error EK2001: failure escapes\n  Widen the contract.\nblock-3.vibe(1,1): warning EK2004: unreachable\nerror TS18003: No inputs were found")).toEqual([
    { file: "block-2.vibe", diagnostic: { code: "EK2001", message: "failure escapes", line: 4, column: 8 } },
    { diagnostic: { code: "TS18003", message: "No inputs were found" } }
  ])
})

import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { join, relative, resolve } from "node:path"
import { tmpdir } from "node:os"
const compiler = defaultCompiler()
test.skipIf(!existsSync(compiler))("checks shared declarations, schema type/value names, captions, and expected errors together", async () => {
  const root = mkdtempSync(join(tmpdir(), "snippet-check-test-"))
  try {
    writeFileSync(join(root, "page.md"), [
      "```vibe", "// descriptor.vibe", "export const Item = Schema.Struct({ name: Schema.String })", "```",
      "```vibe", "type Item = typeof Item.Type", "function read(input: unknown) { return Schema.decodeUnknownEffect(Item)(input) }", "```",
      "```vibe", "import { Item } from './descriptor.vibe'", "const sample = { name: 'Ada' } satisfies typeof Item.Type", "```",
      "```vibe", "// expect-error: TS2322", "const invalid: number = 'text'", "```"
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
    "```vibe", "// main.vibe", "import { code } from './launch-options.ts'", "```",
    "```vibe", "// prose", "// nested/helper.vibe", "export const answer = 42", "```",
    "```vibe", "// prose", "// more prose", "// ignored.vibe", "const value = 1", "```"
  ].join("\n"))
  expect(blocks.map(b => b.filename)).toEqual(["launch-options.ts", "main.vibe", "nested/helper.vibe", undefined])
  expect([...blockFilenames(blocks).values()]).toEqual(["launch-options.ts", "main.vibe", "nested/helper.vibe", "block-4.vibe"])
  expect(() => extract("```ts\n// ../escape.ts\n``` ")).toThrow("Invalid snippet filename")
})

test("repeated captions stay independent and cannot overwrite a named file", () => {
  const blocks = extract("```vibe\n// main.vibe\nconst a = 1\n```\n```vibe\n// main.vibe\nconst a = 2\n```\n```vibe\n// block-1.vibe\nconst b = 3\n```")
  expect([...blockFilenames(blocks).values()]).toEqual(["block-1-2.vibe", "block-2.vibe", "block-1.vibe"])
})

test.skipIf(!existsSync(compiler))("checks named TypeScript companions and maps named-file errors to Markdown", async () => {
  const root = mkdtempSync(join(tmpdir(), "snippet-named-test-"))
  try {
    writeFileSync(join(root, "page.md"), [
      "```ts", "// TypeScript (.ts)", "// launch-options.ts", "export const launchOptions: { code: number } = { code: 2 }", "```",
      "```vibe", "// main.vibe", "import { launchOptions } from './launch-options.ts'", "const code: number = launchOptions.code", "```",
      "```ts", "// TypeScript (.ts)", "// invalid.ts", "// expect-error: TS2322", "export const invalid: number = 'text'", "```",
      "```vibe", "// nested/check.vibe", "// expect-error: TS2322", "const wrong: number = 'text'", "```"
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
test.skipIf(!existsSync(compiler))("compiles a vibe fence as VibeLang and a ts fence as TypeScript", async () => {
  const root = mkdtempSync(join(tmpdir(), "snippet-language-test-"))
  try {
    writeFileSync(join(root, "page.md"), [
      "```vibe", "error Missing { id: string }", "```",
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


test("extracts exact output fences and inline output, with optional run expressions", () => {
  const blocks = extract([
    "```vibe", "// run: Console.log(greet())", "function greet() { return 'hello' }", "```", "",
    "~~~output", "hello", "  indented  ", "~~~",
    "```vibe", "export function main() { Console.log('a'); Console.log('b') }", "// Output: a", "// Output: b", "```",
    "```vibe", "const silent = 1", "// Output:", "```",
    "```vibe", "const displayOnly = 1", "```", "```out", "not checked", "```"
  ].join("\n"))
  expect(blocks.map(b => [b.run, b.output])).toEqual([
    ["Console.log(greet())", "hello\n  indented  "], [undefined, "a\nb"], [undefined, ""], [undefined, undefined]
  ])
})

test("rejects ambiguous output and run markers without an assertion", () => {
  expect(() => extract("```vibe\n// Output: a\n```\n```output\na\n```")).toThrow("both")
  expect(() => extract("```vibe\n// run: work()\nfunction work() {}\n```")).toThrow("output")
})

// Exercise the real compiler, emitted JavaScript, local runtime, and Node.
// Intentionally wrong output and a live fiber must fail instead of passing a
// compile-only check. An unrelated expected error must not block emission.
test.skipIf(!existsSync(compiler))("runs marked blocks and reports output mismatches, exits, timeouts, and unrun fragments", async () => {
  const root = mkdtempSync(join(tmpdir(), "snippet-run-test-"))
  try {
    const pages: Record<string, string[]> = {
      "pass": [
        "```vibe", "// expect-error: TS2322", "const wrong: number = 'text'", "```",
        "```vibe", "import { missingFixture } from './missing.vibe'", "function incomplete() { return missingFixture() }", "```",
        "```vibe", "// names.vibe", "function salutation() { return 'Hello' }", "export function name() { return 'Ada' }", "```",
        "```vibe", "// run: Console.log(greet())", "function greet() { return `${salutation()}, ${name()}!` }", "```",
        "```output", "Hello, Ada!", "```",
        "```vibe", "export function main() { Console.log('native entry') }", "// Output: native entry", "```",
        "```ts", "// TypeScript (.ts)", "// plain.ts", "console.log('plain entry')", "// Output: plain entry", "```"
      ],
      "mismatch": ["```vibe", "export function main() { Console.log('actual') }", "// Output: claimed", "```"],
      "exit": ["```vibe", "export function main() { Effect.die('failure') }", "// Output:", "```"],
      "timeout": ["```vibe", "export function main() { Effect.never() }", "// Output:", "```"],
      "fragment": ["```vibe", "// fragment", "missing()", "// Output: impossible", "```"],
      "expected": ["```vibe", "// expect-error: TS2322", "const invalid: number = 'text'", "// Output:", "```"],
      "compile": ["```vibe", "// run: missing()", "const valid = 1", "// Output:", "```"]
    }
    for (const [name, lines] of Object.entries(pages)) writeFileSync(join(root, `${name}.md`), lines.join("\n"))
    const output = join(root, "report.json")
    const child = Bun.spawn(["bun", join(import.meta.dir, "check-snippets.ts"), "--root", root, "--compiler", compiler, "--run", "--timeout", "3000", "--json", output], { stdout: "pipe", stderr: "pipe" })
    const [stdout, stderr, status] = await Promise.all([new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited])
    expect({ status, stderr }).toEqual({ status: 1, stderr: "" })
    const report = JSON.parse(readFileSync(output, "utf8"))
    const page = (name: string) => report.pages.find((p: any) => p.page.endsWith(`/${name}.md`)).blocks
    expect(page("pass").map((b: any) => b.status)).toEqual(["ok", "fragment", "ok", "ok", "ok", "ok"])
    expect(page("pass").slice(3).map((b: any) => b.execution.stdout)).toEqual(["Hello, Ada!\n", "native entry\n", "plain entry\n"])
    expect(page("mismatch")[0]).toMatchObject({ status: "fail", reason: expect.stringContaining("stdout mismatch"), execution: { stdout: "actual\n", expectedStdout: "claimed" } })
    expect(page("exit")[0]).toMatchObject({ status: "fail", execution: { exitCode: 1 } })
    expect(page("timeout")[0]).toMatchObject({ status: "fail", execution: { timedOut: true } })
    expect(page("fragment")[0]).toMatchObject({ status: "fail", reason: expect.stringContaining("cannot run") })
    expect(page("expected")[0]).toMatchObject({ status: "fail", reason: expect.stringContaining("cannot run") })
    expect(page("compile")[0]).toMatchObject({ status: "fail", execution: { phase: "compile" } })
    expect(stdout).toContain('"passed":3')
  } finally { rmSync(root, { recursive: true, force: true }) }
}, 30000)


test("output normalization preserves spaces and extra blank lines", () => {
  expect(normalizeOutput("one\r\ntwo\r\n")).toBe("one\ntwo")
  expect(normalizeOutput("one \n\n")).toBe("one \n")
  expect(normalizeOutput("  one  ")).toBe("  one  ")
  expect(normalizeOutput("")).toBe("")
})
