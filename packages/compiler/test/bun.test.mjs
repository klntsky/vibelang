import { strict as assert } from "node:assert"
import { spawnSync } from "node:child_process"
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { test } from "node:test"
import { setTimeout as sleep } from "node:timers/promises"
import { fileURLToPath } from "node:url"

const repo = fileURLToPath(new URL("../../../", import.meta.url))
const compilerPackage = fileURLToPath(new URL("../", import.meta.url))
const preload = join(compilerPackage, "bun.mjs")
const native = process.env.VIBELANG_BINARY || join(repo, "typescript/built/local/tsc")
const effect = join(process.env.EFFECT_NODE_MODULES || join(repo, "experiments/node_modules"), "effect")
const runtime = join(repo, "packages/runtime")
const hasBun = spawnSync("bun", ["--version"]).status === 0
const missing = [native, effect, join(runtime, "dist/index.js")].find((path) => !existsSync(path))
const skip = !hasBun ? "bun is not on PATH" : missing !== undefined && `run typescript/vibelang/smoke.sh first; missing ${missing}`

const tsconfig = (include) => JSON.stringify({
  compilerOptions: {
    target: "ES2022", module: "NodeNext", moduleResolution: "NodeNext", strict: true, types: [], skipLibCheck: true,
    allowImportingTsExtensions: true, rewriteRelativeImportExtensions: true, noEmit: true
  },
  include
})

// A scratch project, with this package installed as vibelang, whose compiler
// records each invocation in calls.jsonl.
function project(t, files) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "vibelang-bun-")))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  mkdirSync(join(root, "node_modules"))
  symlinkSync(realpathSync(effect), join(root, "node_modules/effect"))
  symlinkSync(runtime, join(root, "node_modules/vibelang-runtime"))
  symlinkSync(compilerPackage, join(root, "node_modules/vibelang"))
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
  const bun = (args, env = {}) =>
    spawnSync("bun", args, {
      cwd: root, encoding: "utf8",
      env: { ...process.env, VIBELANG_BINARY: compiler, VIBELANG_PROJECT: "", VIBELANG_CACHE_DIR: join(root, ".cache"), NO_COLOR: "1", ...env }
    })
  const compilations = () => existsSync(calls) ? readFileSync(calls, "utf8").trim().split("\n").length : 0
  return { root, bun, compilations }
}

const greeting = {
  "src/suffix.ts": "export const suffix = \"!\"\n",
  "src/shout.vibe": "export function shout(text: string): string { return text.toUpperCase() }\n",
  "src/where.vibe": "export function where(): string {\n  return new Error(\"here\").stack ?? \"\"\n}\n",
  "src/greet.vibe": [
    "import { shout } from \"./shout.vibe\"",
    "import { suffix } from \"./suffix.ts\"",
    "",
    "export function greet(name: string) {",
    "  return shout(`Hello, ${name}${suffix}`)",
    "}",
    ""
  ].join("\n")
}

test("bun runs .vibe modules imported from TypeScript, with one compile per project", { skip }, (t) => {
  const { bun, compilations } = project(t, {
    ...greeting,
    "main.ts": [
      "import { Effect } from \"effect\"",
      "import { greet } from \"./src/greet.vibe\"",
      "import { shout } from \"./src/shout.vibe\"",
      "import { where } from \"./src/where.vibe\"",
      "console.log(JSON.stringify({ text: Effect.runSync(greet(\"Ada\")), plain: shout(\"ok\"), named: /where\\.vibe:\\d+:\\d+/.test(where()) }))",
      ""
    ].join("\n")
  })
  const result = bun(["--preload", preload, "main.ts"])
  assert.equal(result.status, 0, result.stderr)
  assert.deepEqual(JSON.parse(result.stdout), { text: "HELLO, ADA!", plain: "OK", named: true }, "a frame names the .vibe module")
  assert.equal(compilations(), 1, "every .vibe module of a project comes from one compile")
})

test("bun test loads .vibe modules through a bunfig.toml preload of vibelang/bun", { skip }, async (t) => {
  const { bun, compilations } = project(t, {
    ...greeting,
    "bunfig.toml": "preload = [\"vibelang/bun\"]\n\n[test]\npreload = [\"vibelang/bun\"]\n",
    "greet.test.ts": [
      "import { expect, test } from \"bun:test\"",
      "import { Effect } from \"effect\"",
      "import { greet } from \"./src/greet.vibe\"",
      "test(\"greets\", () => {",
      "  expect(Effect.runSync(greet(\"Bun\"))).toBe(\"HELLO, BUN!\")",
      "})",
      ""
    ].join("\n"),
    "main.ts": "import { shout } from \"./src/shout.vibe\"\nconsole.log(shout(\"run\"))\n"
  })
  // A source as new as a compile is not cached, since it may have changed
  // while the compiler read it; wait past that.
  await sleep(120)
  const tested = bun(["test", "./greet.test.ts"])
  assert.equal(tested.status, 0, tested.stdout + tested.stderr)
  assert.match(tested.stdout + tested.stderr, /1 pass/)
  const ran = bun(["main.ts"])
  assert.equal(ran.status, 0, ran.stderr)
  assert.equal(ran.stdout, "RUN\n")
  assert.equal(compilations(), 1, "the second process loads the first one's compile from the cache")
})

test("bun reports the compiler's diagnostics when the project does not compile", { skip }, (t) => {
  const { bun } = project(t, {
    "src/bad.vibe": "export const count: number = \"three\"\n",
    "main.ts": "import { count } from \"./src/bad.vibe\"\nconsole.log(count)\n",
    "bad.test.ts": "import { expect, test } from \"bun:test\"\nimport { count } from \"./src/bad.vibe\"\ntest(\"count\", () => expect(count).toBe(3))\n"
  })
  for (const args of [["--preload", preload, "main.ts"], ["test", "--preload", preload, "./bad.test.ts"]]) {
    const result = bun(args)
    assert.notEqual(result.status, 0, args.join(" "))
    assert.match(result.stdout + result.stderr, /did not compile/)
    assert.match(result.stdout + result.stderr, /bad\.vibe:1:\d+ - error TS2322/)
  }
})

test("names VIBELANG_PROJECT when the nearest tsconfig does not include a module, and honors it", { skip }, (t) => {
  const { root, bun } = project(t, {
    "tsconfig.tools.json": tsconfig(["src/**/*", "tools/**/*"]),
    "src/index.ts": "export {}\n",
    "tools/answer.vibe": "export const answer: number = 42\n",
    "tools/run.ts": "import { answer } from \"./answer.vibe\"\nconsole.log(answer)\n"
  })
  const refused = bun(["--preload", preload, "tools/run.ts"])
  assert.notEqual(refused.status, 0)
  assert.match(refused.stderr, /does not include .*answer\.vibe; set VIBELANG_PROJECT/)
  const accepted = bun(["--preload", preload, "tools/run.ts"], { VIBELANG_PROJECT: join(root, "tsconfig.tools.json") })
  assert.equal(accepted.status, 0, accepted.stderr)
  assert.equal(accepted.stdout, "42\n")
})

test("under Node, vibelang/bun names vibelang/register", async () => {
  await assert.rejects(import(preload), /vibelang\/bun is a Bun preload; under Node, use node --import vibelang\/register/)
})
