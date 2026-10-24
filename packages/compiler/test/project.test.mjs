import { strict as assert } from "node:assert"
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, utimesSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join, resolve } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"
import { findCompiler, formatDiagnostic, nearestProject, parseExplainFiles, parseOutput, projectBackend, projectImports, projects } from "../project.mjs"

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
    "dep/stray.vibe": "export const stray: number = 1\n",
    // A workspace package nested inside the app's directory, as packages/a/b
    // sits inside packages/a, with its own project.
    "app/packages/inner/package.json": '{"type":"module"}',
    "app/packages/inner/tsconfig.json": tsconfig,
    "app/packages/inner/src/inner.vibe": "export const fromInner: string = \"inner\"\n",
    // A nested package without a tsconfig.json of its own, such as a test fixture.
    "app/fixtures/plain/package.json": '{"type":"module"}'
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
  const modules = projects({ root: app, project: join(app, "tsconfig.json"), compiler, cache: false })
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

test("a module of a package nested inside root compiles with that package's own project", { skip }, async (t) => {
  const { app, compiler, compiles } = workspace(t)
  const modules = projects({ root: app, project: join(app, "tsconfig.json"), compiler, cache: false })
  const inner = await modules.load(join(app, "packages/inner/src/inner.vibe"))
  assert.equal(inner.project, join(app, "packages/inner/tsconfig.json"))
  assert.match(inner.code, /fromInner = "inner"/)
  assert.deepEqual([inner.errors, inner.error], [[], false])
  assert.ok(inner.map.sources.includes(join(app, "packages/inner/src/inner.vibe")))
  assert.deepEqual(compiles(), [join(app, "packages/inner/tsconfig.json")])
  // Root's own directories, and a nested package without a project of its own, stay in root's project.
  assert.equal(modules.projectOf(join(app, "src/deep/main.vibe")), join(app, "tsconfig.json"))
  assert.equal(modules.projectOf(join(app, "fixtures/plain/thing.vibe")), join(app, "tsconfig.json"))
})

test("a root module that imports a nested package through node_modules compiles, and so does that package's module", { skip }, async (t) => {
  const { root, app, compiler } = workspace(t)
  for (const [path, text] of Object.entries({
    "app/packages/inner/package.json": JSON.stringify({ name: "inner", type: "module", exports: { "./inner": "./src/inner.vibe" } }),
    "app/src/uses-inner.vibe": 'import { fromInner } from "inner/inner"\nexport const both: string = `${fromInner}!`\n'
  })) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    writeFileSync(join(root, path), text)
    utimesSync(join(root, path), new Date(Date.now() - 60_000), new Date(Date.now() - 60_000))
  }
  // A workspace link, as a package manager makes one.
  mkdirSync(join(app, "node_modules"), { recursive: true })
  symlinkSync(join(app, "packages/inner"), join(app, "node_modules/inner"), "dir")
  const modules = projects({ root: app, project: join(app, "tsconfig.json"), compiler, cache: false })
  const uses = await modules.load(join(app, "src/uses-inner.vibe"))
  assert.deepEqual([uses.errors, uses.error], [[], false])
  assert.match(uses.code, /from "inner\/inner"/)
  const inner = await modules.load(join(app, "packages/inner/src/inner.vibe"))
  assert.deepEqual([inner.errors, inner.error], [[], false])
  assert.match(inner.code, /fromInner = "inner"/)
})

test("a project whose rootDir holds sources above root compiles them, without TS6059", { skip }, async (t) => {
  const { root, app, compiler } = workspace(t)
  for (const [path, text] of Object.entries({
    "shared/clock.ts": "export const now = (): number => 7\n",
    // A test project that reaches a sibling package's source by a relative
    // path widens rootDir to their common directory.
    "app/tsconfig.test.json": JSON.stringify({ extends: "./tsconfig.json", compilerOptions: { rootDir: ".." } }),
    "app/src/main.vibe": 'import { now } from "../../shared/clock.ts"\nexport const fromApp: number = now()\n'
  })) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    writeFileSync(join(root, path), text)
    utimesSync(join(root, path), new Date(Date.now() - 60_000), new Date(Date.now() - 60_000))
  }
  const modules = projects({ root: app, project: join(app, "tsconfig.test.json"), compiler, cache: false })
  const main = await modules.load(join(app, "src/main.vibe"))
  assert.deepEqual([main.errors, main.error], [[], false])
  assert.match(main.code, /fromApp = now\(\)/)
  assert.ok(main.map.sources.includes(join(app, "src/main.vibe")))
})

test("a check-only project that imports .ts files compiles, without TS5096", { skip }, async (t) => {
  const { root, app, compiler } = workspace(t)
  for (const [path, text] of Object.entries({
    // Typechecked with `vibelang -p --noEmit`, it sets neither noEmit nor a rewrite.
    "app/tsconfig.check.json": JSON.stringify({
      extends: "./tsconfig.json",
      compilerOptions: { noEmit: false, rewriteRelativeImportExtensions: false }
    }),
    "app/src/helper.ts": "export const seven = (): number => 7\n",
    "app/src/main.vibe": 'import { seven } from "./helper.ts"\nexport const fromApp: number = seven()\n'
  })) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    writeFileSync(join(root, path), text)
    utimesSync(join(root, path), new Date(Date.now() - 60_000), new Date(Date.now() - 60_000))
  }
  const modules = projects({ root: app, project: join(app, "tsconfig.check.json"), compiler, cache: false })
  const main = await modules.load(join(app, "src/main.vibe"))
  assert.deepEqual([main.errors, main.error], [[], false])
  assert.match(main.code, /from "\.\/helper\.js"/)
})

test("a module declaration compiles into its file's output, and its body is no input file (D283)", { skip }, async (t) => {
  const { app, compiler, compiles } = workspace(t)
  const main = join(app, "src/main.vibe")
  writeFileSync(main, [
    "module tools {",
    "  export function shout(text: string): string { return text.toUpperCase() }",
    "}",
    "import { shout } from tools",
    "export function fromApp() {",
    "  return sync(() => shout(\"app\"))",
    "}",
    ""
  ].join("\n"))
  utimesSync(main, new Date(Date.now() - 60_000), new Date(Date.now() - 60_000))
  const modules = projects({ root: app, project: join(app, "tsconfig.json"), compiler })
  const loaded = await modules.load(main)
  assert.deepEqual([loaded.errors, loaded.error], [[], false])
  assert.match(loaded.code, /const tools_1 = \(\(\) => \{/)
  assert.ok(loaded.inputs.includes(main))
  assert.deepEqual(loaded.inputs.filter((input) => !existsSync(input)), [], "every input is a file on disk")
  await modules.load(main)
  assert.deepEqual(compiles(), [join(app, "tsconfig.json")], "a body that is not on disk never looks changed")
})

test("a failed compile marks its first module, and a module no project includes names the project it tried", { skip }, async (t) => {
  const { root, app, compiler } = workspace(t)
  writeFileSync(join(root, "dep/src/bad.vibe"), "export const count: number = \"three\"\n")
  utimesSync(join(root, "dep/src/bad.vibe"), new Date(Date.now() - 60_000), new Date(Date.now() - 60_000))
  const modules = projects({ root: app, project: join(app, "tsconfig.json"), compiler, cache: false })
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

test("explainFiles parsing keeps written imports with their resolved files and drops compiler-added ones", () => {
  const root = resolve("project")
  const input = [
    "bundled:///libs/lib.es5.d.ts",
    "   Library 'lib.es5.d.ts' specified in compilerOptions",
    "src/b.vibe",
    "   Matched by include pattern 'src/**/*' in 'tsconfig.json'",
    "   Imported via \"./b.vibe\" from file 'src/a.ts'",
    "   Imported via './b.vibe' from file 'src/it's.ts'",
    "../node_modules/effect/dist/index.d.ts",
    "   Imported via \"effect\" from file 'src/b.vibe' with packageId 'effect/dist/index.d.ts@4.0.5'",
    "   Imported via \"effect\" from file 'src/b.vibe' with packageId 'effect/dist/index.d.ts@4.0.5' to import 'jsx' and 'jsxs' factory functions",
    "src/a.ts",
    "   Matched by include pattern 'src/**/*' in 'tsconfig.json'",
    "   File is ECMAScript module because 'package.json' has field \"type\" with value \"module\"",
    "tsconfig.json(3,5): error TS5023: Unknown compiler option 'nope'.",
    "  detail of the option error"
  ].join("\r\n")
  assert.deepEqual(parseExplainFiles(input, root), {
    files: [join(root, "src/b.vibe"), resolve(root, "../node_modules/effect/dist/index.d.ts"), join(root, "src/a.ts")],
    imports: [
      { importer: join(root, "src/a.ts"), specifier: "./b.vibe", file: join(root, "src/b.vibe") },
      { importer: join(root, "src/it's.ts"), specifier: "./b.vibe", file: join(root, "src/b.vibe") },
      { importer: join(root, "src/b.vibe"), specifier: "effect", file: resolve(root, "../node_modules/effect/dist/index.d.ts") }
    ],
    diagnostics: [
      { file: join(root, "tsconfig.json"), line: 3, column: 5, category: "error", code: "TS5023", text: "Unknown compiler option 'nope'.\n  detail of the option error" }
    ]
  })
})

test("a project's imports name every program file and what each import resolved to, .vibe included", { skip }, (t) => {
  const { app, compiler } = workspace(t)
  for (const [path, text] of Object.entries({
    "src/main.vibe": "import { shout } from \"./loud.ts\"\nimport type { Volume } from \"./volume.vibe\"\nexport const fromApp: string = shout(\"app\")\n",
    "src/loud.ts": "export const shout = (text: string): string => text.toUpperCase()\nexport const later = () => import(\"./main.vibe\")\n",
    "src/volume.vibe": "export type Volume = \"quiet\" | \"loud\"\n"
  })) writeFileSync(join(app, path), text)
  const { files, imports, errors } = projectImports({ project: join(app, "tsconfig.json"), compiler })
  assert.deepEqual(errors, [])
  for (const file of ["src/main.vibe", "src/loud.ts", "src/volume.vibe"]) assert.ok(files.includes(join(app, file)), file)
  const own = imports.filter(({ importer, file }) => importer.startsWith(app + "/") && file.startsWith(app + "/"))
  assert.deepEqual(own.map(({ importer, specifier, file }) => [importer.slice(app.length + 1), specifier, file.slice(app.length + 1)]).sort(), [
    ["src/loud.ts", "./main.vibe", "src/main.vibe"],
    ["src/main.vibe", "./loud.ts", "src/loud.ts"],
    ["src/main.vibe", "./volume.vibe", "src/volume.vibe"]
  ])
})

test("explainFiles parsing gives a module declaration's imports to its .vibe file, the only file", () => {
  const root = resolve("project")
  const input = [
    "src/loud.ts",
    "   Imported via \"./loud.ts\" from file 'src/main.vibe.text.ts'",
    "   Matched by include pattern 'src/**/*' in 'tsconfig.json'",
    "src/main.vibe.text.ts",
    "   Supplemental virtual file produced by the content mapper for file 'src/main.vibe'.",
    "src/main.vibe",
    "   Matched by include pattern 'src/**/*' in 'tsconfig.json'"
  ].join("\n")
  assert.deepEqual(parseExplainFiles(input, root), {
    files: [join(root, "src/loud.ts"), join(root, "src/main.vibe")],
    imports: [{ importer: join(root, "src/main.vibe"), specifier: "./loud.ts", file: join(root, "src/loud.ts") }],
    diagnostics: []
  })
})

test("a module declaration's imports are its .vibe file's, and its body is not a file", { skip }, (t) => {
  const { app, compiler } = workspace(t)
  for (const [path, text] of Object.entries({
    "src/main.vibe": [
      "module text {",
      "  import { shout } from \"./loud.ts\"",
      "  export const loud = (value: string): string => shout(value)",
      "}",
      "import { loud } from text",
      "export const fromApp: string = loud(\"app\")",
      ""
    ].join("\n"),
    "src/loud.ts": "export const shout = (text: string): string => text.toUpperCase()\n"
  })) writeFileSync(join(app, path), text)
  const { files, imports, errors } = projectImports({ project: join(app, "tsconfig.json"), compiler })
  assert.deepEqual(errors, [])
  const own = (file) => file.startsWith(app + "/") && !file.includes("/node_modules/")
  assert.deepEqual(files.filter(own).map((file) => file.slice(app.length + 1)).sort(), ["src/loud.ts", "src/main.vibe"])
  assert.deepEqual(
    imports.filter(({ importer, file }) => own(importer) && own(file)).map(({ importer, specifier, file }) => [importer.slice(app.length + 1), specifier, file.slice(app.length + 1)]),
    [["src/main.vibe", "./loud.ts", "src/loud.ts"]]
  )
})

test("a project the compiler cannot read reports its diagnostics instead of an empty graph", { skip }, (t) => {
  const { app, compiler } = workspace(t)
  writeFileSync(join(app, "tsconfig.json"), JSON.stringify({ compilerOptions: { notAnOption: true }, include: ["src/**/*"] }))
  const { errors } = projectImports({ project: join(app, "tsconfig.json"), compiler })
  assert.equal(errors.length, 1)
  assert.match(formatDiagnostic(errors[0]), /tsconfig\.json:\d+:\d+ - error TS5023: Unknown compiler option 'notAnOption'/)
})
