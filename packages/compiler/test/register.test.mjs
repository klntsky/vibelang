import { strict as assert } from "node:assert"
import { spawnSync } from "node:child_process"
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const repo = fileURLToPath(new URL("../../../", import.meta.url))
const register = fileURLToPath(new URL("../register.mjs", import.meta.url))
const native = process.env.VIBELANG_BINARY || join(repo, "typescript/built/local/tsc")
const effect = join(process.env.EFFECT_NODE_MODULES || join(repo, "experiments/node_modules"), "effect")
const runtime = join(repo, "packages/runtime")
const missing = [native, effect, join(runtime, "dist/index.js")].find((path) => !existsSync(path))
const skip = missing !== undefined && `run typescript/vibelang/smoke.sh first; missing ${missing}`

const tsconfig = (include) => JSON.stringify({
  compilerOptions: {
    target: "ES2022", module: "NodeNext", moduleResolution: "NodeNext", strict: true, types: [], skipLibCheck: true,
    allowImportingTsExtensions: true, rewriteRelativeImportExtensions: true, noEmit: true
  },
  include
})

// A scratch project whose compiler records each invocation in calls.jsonl.
function project(t, files) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "vibelang-register-")))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  mkdirSync(join(root, "node_modules"))
  symlinkSync(realpathSync(effect), join(root, "node_modules/effect"))
  symlinkSync(runtime, join(root, "node_modules/vibelang-runtime"))
  const calls = join(root, "calls.jsonl")
  const compiler = join(root, "compiler.mjs")
  writeFileSync(compiler, `import { appendFileSync } from "node:fs"
import { spawnSync } from "node:child_process"
const args = process.argv.slice(2)
appendFileSync(${JSON.stringify(calls)}, JSON.stringify(args) + "\\n")
process.exit(spawnSync(${JSON.stringify(native)}, args, { stdio: "inherit" }).status ?? 1)
`)
  for (const [path, text] of Object.entries({ "package.json": '{"type":"module"}', "tsconfig.json": tsconfig(["src/**/*"]), ...files })) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    writeFileSync(join(root, path), text)
  }
  const run = (entry, env = {}, flags = ["--enable-source-maps"]) =>
    spawnSync(process.execPath, [...flags, "--import", register, entry], {
      cwd: root, encoding: "utf8",
      env: { ...process.env, VIBELANG_BINARY: compiler, VIBELANG_PROJECT: "", VIBELANG_CACHE_DIR: join(root, ".cache"), ...env }
    })
  const compilations = () => existsSync(calls) ? readFileSync(calls, "utf8").trim().split("\n").length : 0
  return { root, run, compilations }
}

test("runs .vibe modules imported from TypeScript, with one compile and one copy of each module", { skip }, (t) => {
  const { run, compilations } = project(t, {
    "src/helper.ts": "export const marker = { id: \"helper\" }\n",
    "src/shout.vibe": "export function shout(text: string): string { return text.toUpperCase() }\n",
    // A plain function never fails (D282); a JavaScript exception from a
    // property access still propagates, with its .vibe frame.
    "src/boom.vibe": "export function boom(): number {\n  const broken: any = undefined; return broken.length\n}\n",
    "src/greet.vibe": [
      "import { marker } from \"./helper.ts\"",
      "import { shout } from \"./shout.vibe\"",
      "export const seen = marker",
      "export function greet(name: string) {",
      "  return shout(`Hello, ${name}`)",
      "}",
      ""
    ].join("\n"),
    "src/main.ts": [
      "import { Effect } from \"effect\"",
      "import { boom } from \"./boom.vibe\"",
      "import { greet, seen } from \"./greet.vibe\"",
      "import { marker } from \"./helper.ts\"",
      "let stack = \"\"",
      "try { boom() } catch (error) { stack = String((error as Error).stack) }",
      "console.log(JSON.stringify({ text: Effect.runSync(greet(\"Ada\")), same: seen === marker, mapped: /boom\\.vibe:2/.test(stack) }))",
      ""
    ].join("\n")
  })
  const result = run("src/main.ts")
  assert.equal(result.stderr, "")
  assert.equal(result.status, 0)
  assert.deepEqual(JSON.parse(result.stdout), { text: "HELLO, ADA", same: true, mapped: true })
  assert.equal(compilations(), 1, "every .vibe module of a project comes from one compile")
})

test("loads a .vibe module whose module declaration holds TypeScript, with stack frames inside it (D283)", { skip }, (t) => {
  const { run, compilations } = project(t, {
    "src/text.vibe": [
      "module tools {",
      "  export function clean(value: string): string {",
      "    if (value.trim() === \"\") throw new Error(\"empty\")",
      "    return value.trim()",
      "  }",
      "}",
      "import { clean } from tools",
      "export function tidy(value: string) {",
      "  return sync((value) => clean(value), value)",
      "}",
      ""
    ].join("\n"),
    "src/main.ts": [
      "import { Cause, Effect, Exit } from \"effect\"",
      "import { tidy } from \"./text.vibe\"",
      "const exit = Effect.runSyncExit(tidy(\"  \"))",
      "const defect = Exit.isFailure(exit) ? Cause.squash(exit.cause) as Error : undefined",
      "console.log(JSON.stringify({ text: Effect.runSync(tidy(\"  a  \")), defect: defect?.message, mapped: /text\\.vibe:3:/.test(String(defect?.stack)) }))",
      ""
    ].join("\n")
  })
  const result = run("src/main.ts")
  assert.equal(result.stderr, "")
  assert.equal(result.status, 0)
  assert.deepEqual(JSON.parse(result.stdout), { text: "a", defect: "empty", mapped: true })
  assert.equal(compilations(), 1)
})

test("reports the compiler's diagnostics when the project does not compile", { skip }, (t) => {
  const { run } = project(t, {
    "src/bad.vibe": "export const count: number = \"three\"\n",
    "src/main.ts": "import { count } from \"./bad.vibe\"\nconsole.log(count)\n"
  })
  const result = run("src/main.ts")
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /did not compile/)
  assert.match(result.stderr, /bad\.vibe:1:\d+ - error TS2322/)
})

test("names VIBELANG_PROJECT when the nearest tsconfig does not include a module, and honors it", { skip }, (t) => {
  const { root, run } = project(t, {
    "tsconfig.tools.json": tsconfig(["src/**/*", "tools/**/*"]),
    "src/index.ts": "export {}\n",
    "tools/answer.vibe": "export const answer: number = 42\n",
    "tools/run.ts": "import { answer } from \"./answer.vibe\"\nconsole.log(answer)\n"
  })
  const refused = run("tools/run.ts")
  assert.notEqual(refused.status, 0)
  assert.match(refused.stderr, /does not include .*answer\.vibe; set VIBELANG_PROJECT/)
  const accepted = run("tools/run.ts", { VIBELANG_PROJECT: join(root, "tsconfig.tools.json") })
  assert.equal(accepted.stderr, "")
  assert.equal(accepted.stdout, "42\n")
})

test("VIBELANG_PROJECT compiles its own modules; another workspace package's source compiles with its own tsconfig", { skip }, (t) => {
  const { root, run } = project(t, {
    "app/package.json": '{"type":"module"}',
    "app/tsconfig.json": tsconfig(["src/**/*"]),
    "app/tsconfig.tools.json": tsconfig(["tools/**/*"]),
    "app/tools/answer.vibe": "import { value } from \"../../lib/src/value.vibe\"\nexport const answer: number = value + 1\n",
    "app/tools/run.ts": "import { answer } from \"./answer.vibe\"\nconsole.log(answer)\n",
    "lib/package.json": '{"type":"module"}',
    "lib/tsconfig.json": tsconfig(["src/**/*"]),
    "lib/src/value.vibe": "export const value: number = 41\n"
  })
  const result = run("app/tools/run.ts", { VIBELANG_PROJECT: join(root, "app/tsconfig.tools.json") })
  assert.equal(result.stderr, "")
  assert.equal(result.stdout, "42\n")
})

test("a long-running process loads a .vibe module written or edited after its project compiled", { skip }, (t) => {
  const { root, run, compilations } = project(t, {
    "src/first.vibe": "export function first(): string { return \"first\" }\n",
    // Outside the project's include, so the compiler does not check it.
    "tools/main.ts": [
      "import { writeFileSync } from \"node:fs\"",
      "import { first } from \"../src/first.vibe\"",
      "const seen = [first()]",
      "// A module that did not exist when the project compiled, as a registry's content-pinned copy.",
      "writeFileSync(new URL(\"../src/later.vibe\", import.meta.url), \"export function later(): string { return \\\"later\\\" }\\n\")",
      "seen.push((await import(\"../src/later.vibe\")).later())",
      "// An edited module, imported again under a new URL, as a watcher does.",
      "writeFileSync(new URL(\"../src/first.vibe\", import.meta.url), \"export function first(): string { return \\\"edited\\\" }\\n\")",
      "seen.push((await import(\"../src/first.vibe?edit=1\")).first())",
      "console.log(JSON.stringify(seen))",
      ""
    ].join("\n")
  })
  const result = run("tools/main.ts")
  assert.equal(result.stderr, "")
  assert.deepEqual(JSON.parse(result.stdout), ["first", "later", "edited"])
  assert.equal(compilations(), 3)
  assert.ok(existsSync(join(root, "src/later.vibe")))
})

test("a failed compile and a module outside the project throw errors with a code", { skip }, (t) => {
  const { run } = project(t, {
    "src/good.vibe": "export const good: number = 1\n",
    "broken/tsconfig.json": tsconfig(["src/**/*"]),
    "broken/src/bad.vibe": "export const count: number = \"three\"\n",
    "elsewhere/stray.vibe": "export const stray: number = 1\n",
    "tools/check.ts": [
      "for (const specifier of [\"../src/good.vibe\", \"../broken/src/bad.vibe\", \"../elsewhere/stray.vibe\"]) {",
      "  try { await import(specifier); console.log(\"loaded\") } catch (error) { console.log((error as { code?: string }).code) }",
      "}",
      ""
    ].join("\n")
  })
  const result = run("tools/check.ts")
  assert.equal(result.status, 0, result.stderr)
  assert.deepEqual(result.stdout.trim().split("\n"), ["loaded", "ERR_VIBELANG_COMPILE", "ERR_VIBELANG_PROJECT"])
})

test("stack traces name .vibe lines without --enable-source-maps", { skip }, (t) => {
  const { run } = project(t, {
    // The error declaration emits more lines than it takes, so an unmapped
    // frame would name a later line of boom.vibe.
    "src/boom.vibe": "error Refused { reason: string }\n\nexport function boom(): Effect<number, Refused> {\n  throw new Refused({ reason: \"boom\" })\n}\n",
    "src/main.ts": [
      "import { Effect } from \"effect\"",
      "import { boom } from \"./boom.vibe\"",
      "const failure = Effect.runSync(Effect.flip(boom()))",
      "console.log(String(failure.stack).split(\"\\n\").filter((line) => line.includes(\"boom.vibe\")).join(\"\\n\"))",
      ""
    ].join("\n")
  })
  const result = run("src/main.ts", {}, [])
  assert.equal(result.stderr, "")
  assert.match(result.stdout, /boom\.vibe:4:\d+\)?$/m)
})

