import assert from "node:assert/strict"
import { mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { test } from "node:test"
import { findCompiler, formatDiagnostic, parseOutput, projectBackend } from "../src/backend.js"

function fixture(t) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "vibelang-backend-test-")))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  return root
}

test("compiler selection prefers explicit options, then environment, then installed package", (t) => {
  const root = fixture(t)
  const previous = process.env.VIBELANG_BINARY
  t.after(() => { if (previous === undefined) delete process.env.VIBELANG_BINARY; else process.env.VIBELANG_BINARY = previous })
  process.env.VIBELANG_BINARY = "environment/compiler"
  assert.equal(findCompiler(root, "configured/compiler"), join(root, "configured/compiler"))
  assert.equal(findCompiler(root), join(root, "environment/compiler"))
  delete process.env.VIBELANG_BINARY
  const installed = join(root, "node_modules/vibelang")
  mkdirSync(installed, { recursive: true })
  writeFileSync(join(installed, "package.json"), '{"name":"vibelang"}')
  assert.equal(findCompiler(root), join(installed, "bin/vibelang"))
})

test("diagnostic parsing preserves multiline detail, CRLF, global messages and dependency files", () => {
  const root = resolve("project with spaces")
  const input = [
    "src/main.vibe(2,17): error TS2322: Type 'string' is not assignable.",
    "  Type detail", "    Nested detail", "warning EK2004: unreachable catch",
    join(root, "src/main.vibe"), join(root, "src/type-only.ts"), "", "ordinary banner"
  ].join("\r\n")
  assert.deepEqual(parseOutput(input, root), {
    diagnostics: [
      { file: join(root, "src/main.vibe"), line: 2, column: 17, category: "error", code: "TS2322", text: "Type 'string' is not assignable.\n  Type detail\n    Nested detail" },
      { category: "warning", code: "EK2004", text: "unreachable catch" }
    ], files: [join(root, "src/main.vibe"), join(root, "src/type-only.ts")]
  })
})

test("wide diagnostic coordinates and messages survive parsing and formatting", () => {
  const root = resolve("project")
  for (let i = 1; i <= 300; i++) {
    const file = `source ${i}.vibe`, line = i * 19, column = i * 31
    const category = i % 2 ? "error" : "warning", code = i % 2 ? "TS2322" : "EK2004"
    const text = `message ${i}: punctuation (x,y), unicode λ, trailing space `
    const result = parseOutput(`${file}(${line},${column}): ${category} ${code}: ${text}\n`, root)
    const expected = { file: join(root, file), line, column, category, code, text }
    assert.deepEqual(result.diagnostics, [expected])
    assert.equal(formatDiagnostic(expected), `${expected.file}:${line}:${column} - ${category} ${code}: ${text}`)
  }
  assert.equal(formatDiagnostic({ category: "error", code: "TS5058", text: "no config" }), "error TS5058: no config")
})

test("a compiler crash without diagnostics reports its output and never returns stale modules", async (t) => {
  const root = fixture(t)
  const compiler = join(root, "crash.mjs")
  writeFileSync(compiler, 'console.error("compiler stopped unexpectedly"); process.exit(17)\n')
  const result = await projectBackend({ root, compiler }).compile(join(root, "tsconfig.json"))
  assert.deepEqual([...result.outputs], [])
  assert.deepEqual(result.errors, [{ category: "error", code: "vibelang", text: "compiler stopped unexpectedly" }])
  assert.deepEqual(result.warnings, [])
})

test("a silent compiler crash reports its exit status", async (t) => {
  const root = fixture(t)
  const compiler = join(root, "silent.mjs")
  writeFileSync(compiler, 'process.exit(19)\n')
  const result = await projectBackend({ root, compiler }).compile(join(root, "tsconfig.json"))
  assert.deepEqual(result.errors, [{ category: "error", code: "vibelang", text: "exited with 19" }])
})

test("a missing compiler reports the command that failed to spawn", async (t) => {
  const root = fixture(t)
  const compiler = join(root, "missing")
  await assert.rejects(projectBackend({ root, compiler }).compile(join(root, "tsconfig.json")), (error) => {
    assert.ok(error.message.includes(`Compiler: ${compiler}`))
    assert.match(error.message, /ENOENT/)
    return true
  })
})
