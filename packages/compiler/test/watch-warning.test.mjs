import { strict as assert } from "node:assert"
import { spawn, spawnSync } from "node:child_process"
import { existsSync, mkdirSync, mkdtempSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const repo = fileURLToPath(new URL("../../../", import.meta.url))
const native = process.env.VIBELANG_BINARY || join(repo, "typescript/built/local/tsc")
const effect = join(process.env.EFFECT_NODE_MODULES || join(repo, "experiments/node_modules"), "effect")
const runtime = join(repo, "packages/runtime")
const missing = [native, effect, join(runtime, "dist/index.js")].find(path => !existsSync(path))
const skip = missing && `run typescript/vibelang/smoke.sh first; missing ${missing}`

test("watch counts warnings separately on initial and incremental builds", { skip, timeout: 60000 }, async (t) => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "vibelang-watch-warning-")))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  mkdirSync(join(root, "node_modules"))
  symlinkSync(realpathSync(effect), join(root, "node_modules/effect"))
  symlinkSync(runtime, join(root, "node_modules/vibelang-runtime"))
  writeFileSync(join(root, "package.json"), '{"type":"module"}')
  writeFileSync(join(root, "tsconfig.json"), JSON.stringify({
    compilerOptions: { target: "es2022", module: "nodenext", moduleResolution: "nodenext", strict: true, types: [], skipLibCheck: true, noEmit: true },
    include: ["*.vibe"]
  }))
  const source = 'function step(n: number) { return n + 1 }\nexport function f() { return step(1) catch 0 }\n'
  const file = join(root, "main.vibe")
  writeFileSync(file, source)
  const normal = spawnSync(native, ["--pretty", "false"], { cwd: root, encoding: "utf8" })
  assert.equal(normal.status, 0, normal.stdout + normal.stderr)
  assert.match(normal.stdout, /warning EK2004/)
  const child = spawn(native, ["-w", "--pretty", "false", "--preserveWatchOutput"], { cwd: root })
  t.after(async () => {
    if (child.exitCode === null) {
      const exited = new Promise(resolve => child.once("exit", resolve))
      child.kill()
      await exited
    }
  })
  let output = ""
  let pending
  const receive = chunk => { output += chunk; pending?.() }
  child.stdout.on("data", receive)
  child.stderr.on("data", receive)
  const cycle = () => new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`watch timeout:
${output}`)), 20000)
    pending = () => {
      if (!output.includes("Watching for file changes.")) return
      clearTimeout(timer)
      pending = undefined
      resolve(output)
    }
    pending()
  })
  const initial = await cycle()
  assert.match(initial, /Found 0 errors and 1 warning\. Watching for file changes\./)
  assert.match(initial, /warning EK2004/)
  output = ""
  writeFileSync(file, source + 'const broken: number = "wrong"\n')
  const incremental = await cycle()
  assert.match(incremental, /Found 1 error and 1 warning\. Watching for file changes\./)
  assert.match(incremental, /error TS2322/)
  output = ""
  writeFileSync(file, source)
  assert.match(await cycle(), /Found 0 errors and 1 warning\. Watching for file changes\./)
})
