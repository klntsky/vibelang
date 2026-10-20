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
  const run = (entry, env = {}) =>
    spawnSync(process.execPath, ["--enable-source-maps", "--import", register, entry], {
      cwd: root, encoding: "utf8", env: { ...process.env, VIBELANG_BINARY: compiler, VIBELANG_PROJECT: "", ...env }
    })
  const compilations = () => existsSync(calls) ? readFileSync(calls, "utf8").trim().split("\n").length : 0
  return { root, run, compilations }
}

test("runs .vibe modules imported from TypeScript, with one compile and one copy of each module", { skip }, (t) => {
  const { run, compilations } = project(t, {
    "src/helper.ts": "export const marker = { id: \"helper\" }\n",
    "src/shout.vibe": "export function shout(text: string): string { return text.toUpperCase() }\n",
    "src/boom.vibe": "export function boom(): number {\n  throw new Error(\"boom\")\n}\n",
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
