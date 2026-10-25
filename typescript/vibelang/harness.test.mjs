import assert from "node:assert/strict"
import { spawnSync } from "node:child_process"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import { fileURLToPath } from "node:url"
import test from "node:test"

const here = path.dirname(fileURLToPath(import.meta.url))

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "vibelang-harness-test-"))
  t.after(() => fs.rmSync(root, { recursive: true, force: true }))
  const dir = path.join(root, "vibelang")
  fs.mkdirSync(dir, { recursive: true })
  for (const file of ["harness.mjs", "harness-command.mjs"]) {
    if (fs.existsSync(path.join(here, file))) fs.copyFileSync(path.join(here, file), path.join(dir, file))
  }
  const compiler = path.join(root, "built/local/tsc")
  fs.mkdirSync(path.dirname(compiler), { recursive: true })
  fs.writeFileSync(compiler, `#!/usr/bin/env node
import fs from "node:fs"
const scenario = fs.readFileSync("scenario", "utf8")
console.log("compile stdout " + scenario)
console.error("compile stderr " + scenario)
if (scenario === "compile") {
  console.log("main.vibe(1,1): error TS9999: deliberate failure")
  process.exit(1)
}
fs.mkdirSync("out", { recursive: true })
fs.writeFileSync("out/main.js", 'console.log("wrong " + ' + JSON.stringify(scenario) + '); console.error("runtime stderr"); process.exit(7)')
`, { mode: 0o755 })
  fs.writeFileSync(path.join(root, "package.json"), '{"type":"module"}')
  for (const name of ["compile", "runtime"]) {
    const at = path.join(dir, "cases", name)
    fs.mkdirSync(path.join(at, "expected"), { recursive: true })
    fs.writeFileSync(path.join(at, "scenario"), name)
    fs.writeFileSync(path.join(at, "tsconfig.json"), "{}")
    fs.writeFileSync(path.join(at, "expected/stdout.txt"), "expected\n")
  }
  return { root, harness: path.join(dir, "harness.mjs"), compiler }
}

test("status failures retain per-case command evidence across reruns", (t) => {
  const { root, harness, compiler } = fixture(t)
  const env = { ...process.env, EFFECT_NODE_MODULES: path.join(root, "modules"), VIBELANG_RUNTIME: path.join(root, "runtime") }
  const invoke = () => spawnSync(process.execPath, [harness, "--status", "--jobs", "2"], { env, encoding: "utf8", timeout: 10_000 })
  const result = invoke()
  assert.equal(result.status, 1, result.stdout + result.stderr)
  const paths = [...result.stdout.matchAll(/log: (.+?)(?:\)|$)/gm)].map((m) => m[1])
  assert.equal(paths.length, 2, "each failing status line must name its log:\n" + result.stdout)
  const logs = paths.map((file) => JSON.parse(fs.readFileSync(file, "utf8")))
  for (const record of logs) {
    assert.equal(record.case, path.basename(record.commands[0].cwd))
    assert.ok(record.detail)
    const compile = record.commands[0]
    assert.equal(compile.phase, "compile")
    assert.equal(compile.command, compiler)
    assert.deepEqual(compile.argv, ["-p", ".", "--pretty", "false"])
    assert.equal(compile.timeoutMs, 120_000)
    assert.equal(Date.parse(compile.deadline) - Date.parse(compile.startedAt), 120_000)
    assert.match(compile.stdoutTail, new RegExp("compile stdout " + record.case))
    assert.match(compile.stderrTail, new RegExp("compile stderr " + record.case))
    assert.equal(compile.exitCode, record.case === "compile" ? 1 : 0)
    assert.equal(compile.timedOut, false)
    if (record.case === "runtime") {
      const run = record.commands[1]
      assert.equal(run.phase, "run")
      assert.equal(run.command, "node")
      assert.equal(run.exitCode, 7)
      assert.equal(run.timeoutMs, 60_000)
      assert.match(run.stdoutTail, /wrong runtime/)
      assert.match(run.stderrTail, /runtime stderr/)
    }
  }
  const again = invoke()
  assert.equal(again.status, 1, again.stdout + again.stderr)
  const secondPaths = [...again.stdout.matchAll(/log: (.+?)(?:\)|$)/gm)].map((m) => m[1])
  assert.equal(secondPaths.length, 2)
  assert.ok(secondPaths.every((file) => !paths.includes(file)), "reruns must preserve prior evidence")
  for (const file of paths) assert.ok(fs.existsSync(file))
})

test("command evidence records deadline kills, spawn errors, and bounded tails", async () => {
  const { CaseCommands } = await import("./harness-command.mjs")
  const commands = new CaseCommands()
  const timed = await commands.run(process.execPath, ["-e", "setInterval(() => {}, 1000)"], { phase: "run", timeout: 30 })
  assert.equal(timed.code, "killed by SIGKILL")
  assert.equal(commands.records[0].timedOut, true)
  assert.equal(commands.records[0].signal, "SIGKILL")
  assert.equal(commands.records[0].exitCode, null)
  const missing = await commands.run(path.join(os.tmpdir(), "missing-vibelang-command", "tsc"), [], { phase: "compile", timeout: 1000 })
  assert.equal(missing.code, "ENOENT")
  assert.match(commands.records[1].error, /ENOENT/)
  await commands.run(process.execPath, ["-e", 'console.log("x".repeat(50000) + "stdout end"); console.error("y".repeat(50000) + "stderr end")'], { phase: "run", timeout: 1000 })
  assert.ok(commands.records[2].stdoutTail.length <= 16_384)
  assert.ok(commands.records[2].stderrTail.length <= 16_384)
  assert.match(commands.records[2].stdoutTail, /stdout end\n$/)
  assert.match(commands.records[2].stderrTail, /stderr end\n$/)
})
