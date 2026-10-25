import assert from "node:assert/strict"
import { spawnSync } from "node:child_process"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import { fileURLToPath } from "node:url"
import test from "node:test"

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..")
const compiler = path.join(root, "packages/compiler/native/vibelang")
const effect = path.join(root, "experiments/node_modules/effect")
const runtime = path.join(root, "packages/runtime")

test("synthetic benchmark projects compile without warnings", async (t) => {
  for (const [file, setup] of [
    [compiler, "sh scripts/build-vibelang.sh"],
    [path.join(effect, "package.json"), "(cd experiments && bun install)"],
    [path.join(runtime, "dist/index.d.ts"), "(cd packages/runtime && bun install && bun run build)"],
  ]) {
    if (!fs.existsSync(file)) return t.skip(`missing prerequisite ${file}; run ${setup}`)
  }
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), "vibelang-synthetic-test-"))
  t.after(() => fs.rmSync(temp, { recursive: true, force: true }))
  for (const shape of ["chain", "dense", "single"]) {
    await t.test(shape, () => {
      const out = path.join(temp, shape)
      const generated = spawnSync(process.execPath, [path.join(root, "scripts/bench/gen-synthetic.mjs"), out, "3", shape], { encoding: "utf8" })
      assert.equal(generated.status, 0, generated.stdout + generated.stderr)
      fs.mkdirSync(path.join(out, "node_modules"), { recursive: true })
      fs.symlinkSync(effect, path.join(out, "node_modules/effect"))
      fs.symlinkSync(runtime, path.join(out, "node_modules/vibelang-runtime"))
      const compiled = spawnSync(compiler, ["-p", path.join(out, "vibelang"), "--noEmit", "--pretty", "false"], { encoding: "utf8", timeout: 120_000 })
      assert.equal(compiled.status, 0, compiled.stdout + compiled.stderr)
      assert.equal(compiled.stdout + compiled.stderr, "", "benchmark must have zero diagnostics")
    })
  }
})
