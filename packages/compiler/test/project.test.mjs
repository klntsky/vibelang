import { strict as assert } from "node:assert"
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, utimesSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join, resolve } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"
import { findCompiler, formatDiagnostic, nearestProject, parseOutput, projectBackend, projects } from "../project.mjs"

const repo = fileURLToPath(new URL("../../../", import.meta.url))
const native = process.env.VIBELANG_BINARY || join(repo, "typescript/built/local/tsc")
const effect = join(process.env.EFFECT_NODE_MODULES || join(repo, "experiments/node_modules"), "effect")
const runtime = join(repo, "packages/runtime")
const missing = [native, effect, join(runtime, "dist/index.js")].find((path) => !existsSync(path))
const skip = missing !== undefined && `run typescript/vibelang/smoke.sh first; missing ${missing}`

function fixture(t) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "vibelang-project-test-")))
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

test("the nearest project is the closest tsconfig.json above a file, or none", (t) => {
  const root = fixture(t)
  mkdirSync(join(root, "app/src/nested"), { recursive: true })
  writeFileSync(join(root, "app/tsconfig.json"), "{}")
  assert.equal(nearestProject(join(root, "app/src/nested/main.vibe")), join(root, "app/tsconfig.json"))
  assert.equal(nearestProject(join(root, "elsewhere.vibe")), undefined)
})

const tsconfig = JSON.stringify({
  compilerOptions: {
    target: "ES2022", module: "NodeNext", moduleResolution: "NodeNext", strict: true, types: [], skipLibCheck: true,
    allowImportingTsExtensions: true, rewriteRelativeImportExtensions: true, noEmit: true
  },
  include: ["src/**/*"]
})

// A workspace: an app and a package beside it, each with its own project, and
// a compiler that records each compile's --project in calls.jsonl.
function workspace(t) {
  const root = fixture(t)
  mkdirSync(join(root, "node_modules"))
  symlinkSync(realpathSync(effect), join(root, "node_modules/effect"))
  symlinkSync(runtime, join(root, "node_modules/vibelang-runtime"))
  const calls = join(root, "calls.jsonl")
  const compiler = join(root, "compiler.mjs")
  writeFileSync(compiler, `import { appendFileSync } from "node:fs"
import { spawnSync } from "node:child_process"
const args = process.argv.slice(2)
appendFileSync(${JSON.stringify(calls)}, JSON.stringify(args[args.indexOf("--project") + 1]) + "\\n")
process.exit(spawnSync(${JSON.stringify(native)}, args, { stdio: "inherit" }).status ?? 1)
`)
  for (const [path, text] of Object.entries({
    "app/package.json": '{"type":"module"}',
    "app/tsconfig.json": tsconfig,
    "app/src/main.vibe": "export const fromApp: string = \"app\"\n",
    "dep/package.json": '{"type":"module"}',
    "dep/tsconfig.json": tsconfig,
    "dep/src/lib.vibe": "export function shout(text: string): string { return text.toUpperCase() }\n",
    "dep/stray.vibe": "export const stray: number = 1\n"
  })) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    writeFileSync(join(root, path), text)
    // A source newer than a compile's start makes it compile again (a save
    // during the compile); keep the fixture older than every compile.
    utimesSync(join(root, path), new Date(Date.now() - 60_000), new Date(Date.now() - 60_000))
  }
  const app = join(root, "app")
  const compiles = () => existsSync(calls) ? readFileSync(calls, "utf8").trim().split("\n").map((line) => JSON.parse(line)) : []
  return { root, app, compiler, compiles }
}

test("a module outside root compiles with its own package's project, once per revision", { skip }, async (t) => {
  const { root, app, compiler, compiles } = workspace(t)
  const modules = projects({ root: app, project: join(app, "tsconfig.json"), compiler })
  const main = await modules.load(join(app, "src/main.vibe"))
  const lib = await modules.load(join(root, "dep/src/lib.vibe"))
  assert.equal(main.project, join(app, "tsconfig.json"))
  assert.equal(lib.project, join(root, "dep/tsconfig.json"))
  assert.match(main.code, /fromApp = "app"/)
  assert.match(lib.code, /toUpperCase/)
  assert.deepEqual([main.errors, main.error, lib.errors, lib.error], [[], false, [], false])
  assert.ok(lib.map.sources.includes(join(root, "dep/src/lib.vibe")))
  await modules.load(join(root, "dep/src/lib.vibe"))
  assert.deepEqual(compiles(), [join(app, "tsconfig.json"), join(root, "dep/tsconfig.json")])
  modules.invalidate()
  await modules.load(join(root, "dep/src/lib.vibe"))
  assert.deepEqual(compiles().slice(2), [join(root, "dep/tsconfig.json")], "a new revision compiles only the project asked for")
})

test("a failed compile marks its first module, and a module no project includes names the project it tried", { skip }, async (t) => {
  const { root, app, compiler } = workspace(t)
  writeFileSync(join(root, "dep/src/bad.vibe"), "export const count: number = \"three\"\n")
  utimesSync(join(root, "dep/src/bad.vibe"), new Date(Date.now() - 60_000), new Date(Date.now() - 60_000))
  const modules = projects({ root: app, project: join(app, "tsconfig.json"), compiler })
  const first = await modules.load(join(root, "dep/src/lib.vibe"))
  const second = await modules.load(join(root, "dep/src/bad.vibe"))
  assert.equal(first.errors.length, 1)
  assert.match(formatDiagnostic(first.errors[0]), /bad\.vibe:1:\d+ - error TS2322/)
  assert.deepEqual([first.first, second.first], [true, false], "only the first module of a compile is first")
  assert.deepEqual(second.errors, first.errors, "every module of a failed compile carries its errors")
  assert.match(second.code, /did not compile/)
  rmSync(join(root, "dep/src/bad.vibe"))
  modules.invalidate()
  const outside = await modules.load(join(root, "dep/stray.vibe"))
  assert.match(outside.error.message, /dep\/stray\.vibe is not part of .*dep\/tsconfig\.json/)
})
