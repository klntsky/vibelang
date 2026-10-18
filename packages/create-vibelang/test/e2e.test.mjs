// Uses the current checkout's compiler, runtime and Vite plugin. Run the
// documented workspace setup first, then node --test test/e2e.test.mjs.
import assert from "node:assert/strict"
import { spawnSync } from "node:child_process"
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const root = fileURLToPath(new URL("../../../", import.meta.url))
const binary = process.env.VIBELANG_BINARY || join(root, "typescript/built/local/tsc")

function run(command, args, cwd, extraEnv = {}) {
  const result = spawnSync(command, args, {
    cwd, encoding: "utf8", timeout: 120_000,
    env: { ...process.env, VIBELANG_BINARY: binary, NO_COLOR: "1", ...extraEnv }
  })
  assert.equal(result.error, undefined, result.error?.message)
  return { ...result, output: result.stdout + result.stderr }
}

// The test is deliberately opt-in until workspace setup builds the compiler
// and runtime. CLI unit tests remain runnable in an unpacked npm package.
test("a scaffold installs, tests source without a build, builds, starts, and reports assertion positions", {
  skip: !existsSync(binary) || !existsSync(join(root, "packages/runtime/dist/index.js"))
    ? "run the documented workspace setup to build the compiler and runtime" : false,
  timeout: 180_000
}, (t) => {
  const scratch = mkdtempSync(join(tmpdir(), "vibelang-scaffold-e2e-"))
  t.after(() => rmSync(scratch, { recursive: true, force: true }))
  const app = join(scratch, "hello app")
  const created = run(process.execPath, [join(root, "packages/create-vibelang/bin/create-vibelang.mjs"), app, "--no-install"], scratch)
  assert.equal(created.status, 0, created.output)
  const manifestPath = join(app, "package.json")
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"))
  // Pack rather than symlink the runtime: its Effect import must resolve to
  // the same physical installation as the app's Effect import (D120).
  const packed = run("npm", ["pack", join(root, "packages/runtime"), "--pack-destination", scratch, "--ignore-scripts", "--json", "--silent"], scratch)
  assert.equal(packed.status, 0, packed.output)
  manifest.dependencies["vibelang-runtime"] = `file:${join(scratch, JSON.parse(packed.stdout)[0].filename)}`
  manifest.devDependencies.vibelang = `file:${join(root, "packages/compiler")}`
  manifest.devDependencies["vibelang-vite"] = `file:${join(root, "packages/vite")}`
  writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n")
  const installed = run("bun", ["install"], app)
  assert.equal(installed.status, 0, installed.output)

  assert.ok(!existsSync(join(app, "dist")), "source tests start without emitted files")
  const tested = run("bun", ["run", "test"], app)
  assert.equal(tested.status, 0, tested.output)
  assert.match(tested.output, /2 passed/)

  const built = run("bun", ["run", "build"], app)
  assert.equal(built.status, 0, built.output)
  assert.ok(existsSync(join(app, "dist/main.js")))
  assert.ok(existsSync(join(app, "dist/greeting.js.map")))
  const started = run("bun", ["run", "start"], app)
  assert.equal(started.status, 0, started.output)
  assert.match(started.output, /Hello, VibeLang!/)
  assert.match(started.output, /Recovered from EmptyName/)
  const greeting = readFileSync(join(app, "dist/greeting.js"), "utf8")
  assert.doesNotMatch(greeting, /\.vibe["']/)

  const testFile = join(app, "src/greeting.test.vibe")
  writeFileSync(testFile, readFileSync(testFile, "utf8").replace('"Hello, Ada!"', '"deliberately wrong"'))
  const failed = run("bun", ["run", "test"], app)
  assert.equal(failed.status, 1, failed.output)
  assert.match(failed.output, /1 failed/)
  assert.match(failed.output, /greeting\.test\.vibe:5:\d+/)
})
