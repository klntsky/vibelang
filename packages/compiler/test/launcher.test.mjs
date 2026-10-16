import { strict as assert } from "node:assert"
import { chmodSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { spawn, spawnSync } from "node:child_process"
import { once } from "node:events"
import { fileURLToPath } from "node:url"
import { test } from "node:test"

const launcher = fileURLToPath(new URL("../bin/esc", import.meta.url))
test("forwards arguments, input, streams, working directory, and exit status", () => {
  const scratch = mkdtempSync(join(tmpdir(), "esc-launcher-"))
  try {
    const binary = join(scratch, "native esc")
    writeFileSync(binary, `#!/usr/bin/env node
import fs from "node:fs"
console.log(JSON.stringify({ args: process.argv.slice(2), cwd: process.cwd(), input: fs.readFileSync(0, "utf8") }))
console.error("native stderr")
process.exit(23)
`)
    chmodSync(binary, 0o755)
    const result = spawnSync(process.execPath, [launcher, "--pretty", "a b", "$literal"], {
      cwd: scratch, env: { ...process.env, ESC_BINARY: binary }, input: "native stdin", encoding: "utf8"
    })
    assert.equal(result.status, 23)
    assert.deepEqual(JSON.parse(result.stdout), {
      args: ["--pretty", "a b", "$literal"], cwd: realpathSync(scratch), input: "native stdin"
    })
    assert.equal(result.stderr, "native stderr\n")
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("explains how to supply a missing native compiler", () => {
  const result = spawnSync(process.execPath, [launcher], {
    env: { ...process.env, ESC_BINARY: "/nonexistent/effectscript/esc" }, encoding: "utf8"
  })
  assert.equal(result.status, 1)
  assert.match(result.stderr, /scripts\/build-esc.sh/)
  assert.match(result.stderr, /ESC_BINARY/)
})

test("preserves native termination signals", () => {
  const scratch = mkdtempSync(join(tmpdir(), "esc-signal-"))
  try {
    const binary = join(scratch, "native esc")
    writeFileSync(binary, '#!/usr/bin/env node\nprocess.kill(process.pid, "SIGTERM")\n')
    chmodSync(binary, 0o755)
    const result = spawnSync(process.execPath, [launcher], { env: { ...process.env, ESC_BINARY: binary } })
    assert.equal(result.signal, "SIGTERM")
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("forwards termination to a running native watcher", { timeout: 5000 }, async () => {
  const scratch = mkdtempSync(join(tmpdir(), "esc-watcher-"))
  let wrapper
  let nativePid
  try {
    const binary = join(scratch, "native esc")
    writeFileSync(binary, '#!/usr/bin/env node\nconsole.log(process.pid)\nsetInterval(() => {}, 1000)\n')
    chmodSync(binary, 0o755)
    wrapper = spawn(process.execPath, [launcher, "--watch"], {
      env: { ...process.env, ESC_BINARY: binary }, stdio: ["ignore", "pipe", "inherit"]
    })
    const [data] = await once(wrapper.stdout, "data")
    nativePid = Number(data.toString().trim())
    assert.ok(nativePid > 0)
    const closed = once(wrapper, "close")
    wrapper.kill("SIGTERM")
    const [code, signal] = await closed
    assert.equal(code, null)
    assert.equal(signal, "SIGTERM")
    assert.throws(() => process.kill(nativePid, 0), { code: "ESRCH" })
  } finally {
    wrapper?.kill("SIGKILL")
    if (nativePid) {
      try { process.kill(nativePid, "SIGKILL") } catch {}
    }
    rmSync(scratch, { recursive: true, force: true })
  }
})
