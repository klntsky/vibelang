import { strict as assert } from "node:assert"
import { chmodSync, copyFileSync, mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { spawn, spawnSync } from "node:child_process"
import { once } from "node:events"
import { fileURLToPath } from "node:url"
import { test } from "node:test"

const launcher = fileURLToPath(new URL("../bin/effeck", import.meta.url))
test("forwards arguments, input, streams, working directory, and exit status", () => {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-launcher-"))
  try {
    const binary = join(scratch, "native effeck")
    writeFileSync(binary, `#!/usr/bin/env node
import fs from "node:fs"
console.log(JSON.stringify({ args: process.argv.slice(2), cwd: process.cwd(), input: fs.readFileSync(0, "utf8") }))
console.error("native stderr")
process.exit(23)
`)
    chmodSync(binary, 0o755)
    const result = spawnSync(process.execPath, [launcher, "--pretty", "a b", "$literal"], {
      cwd: scratch, env: { ...process.env, EFFECK_BINARY: binary }, input: "native stdin", encoding: "utf8"
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
    env: { ...process.env, EFFECK_BINARY: "/nonexistent/effeck/effeck" }, encoding: "utf8"
  })
  assert.equal(result.status, 1)
  assert.match(result.stderr, /scripts\/build-effeck.sh/)
  assert.match(result.stderr, /EFFECK_BINARY/)
})

test("says Windows binaries are not published yet, without a stack trace", () => {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-win32-"))
  try {
    // A copy outside the checkout finds neither native/effeck nor a platform package.
    mkdirSync(join(scratch, "bin"))
    writeFileSync(join(scratch, "package.json"), '{"type":"module"}')
    copyFileSync(launcher, join(scratch, "bin", "effeck"))
    const env = { ...process.env }
    delete env.EFFECK_BINARY
    const asWindows = 'data:text/javascript,Object.defineProperty(process,"platform",{value:"win32"})'
    const result = spawnSync(process.execPath, ["--import", asWindows, join(scratch, "bin", "effeck"), "--version"], {
      env, encoding: "utf8"
    })
    assert.equal(result.status, 1)
    assert.match(result.stderr, /Windows binaries are not published yet/)
    assert.match(result.stderr, /EFFECK_BINARY/)
    assert.doesNotMatch(result.stderr, /\n\s+at /)
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("preserves native termination signals", () => {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-signal-"))
  try {
    const binary = join(scratch, "native effeck")
    writeFileSync(binary, '#!/usr/bin/env node\nprocess.kill(process.pid, "SIGTERM")\n')
    chmodSync(binary, 0o755)
    const result = spawnSync(process.execPath, [launcher], { env: { ...process.env, EFFECK_BINARY: binary } })
    assert.equal(result.signal, "SIGTERM")
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("forwards termination to a running native watcher", { timeout: 5000 }, async () => {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-watcher-"))
  let wrapper
  let nativePid
  try {
    const binary = join(scratch, "native effeck")
    writeFileSync(binary, '#!/usr/bin/env node\nconsole.log(process.pid)\nsetInterval(() => {}, 1000)\n')
    chmodSync(binary, 0o755)
    wrapper = spawn(process.execPath, [launcher, "--watch"], {
      env: { ...process.env, EFFECK_BINARY: binary }, stdio: ["ignore", "pipe", "inherit"]
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

// installed lays out a copy of the launcher as an installed package, with an
// optional native/effeck beside it and an optional platform package.
function installed(scratch, { native = false, platform = false } = {}) {
  const pkg = join(scratch, "node_modules", "effeck")
  mkdirSync(join(pkg, "bin"), { recursive: true })
  writeFileSync(join(pkg, "package.json"), JSON.stringify({ name: "effeck", type: "module" }))
  copyFileSync(launcher, join(pkg, "bin", "effeck"))
  const fake = (path, label) => {
    mkdirSync(dirname(path), { recursive: true })
    writeFileSync(path, `#!/usr/bin/env node\nconsole.log(${JSON.stringify(label)}, ...process.argv.slice(2))\n`)
    chmodSync(path, 0o755)
  }
  if (native) fake(join(pkg, "native", "effeck"), "native")
  if (platform) {
    const target = join(scratch, "node_modules", `effeck-${process.platform}-${process.arch}`)
    mkdirSync(target, { recursive: true })
    writeFileSync(join(target, "package.json"), JSON.stringify({ name: `effeck-${process.platform}-${process.arch}` }))
    fake(join(target, "bin", "effeck"), "platform")
  }
  return join(pkg, "bin", "effeck")
}

function withoutOverride() {
  const env = { ...process.env }
  delete env.EFFECK_BINARY
  return env
}

test("runs the installed platform package's compiler", { skip: process.platform === "win32" }, () => {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-platform-"))
  try {
    const result = spawnSync(process.execPath, [installed(scratch, { platform: true }), "--version"], { env: withoutOverride(), encoding: "utf8" })
    assert.equal(result.status, 0, result.stderr)
    assert.equal(result.stdout, "platform --version\n")
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("prefers native/effeck beside the launcher, as in a checkout", { skip: process.platform === "win32" }, () => {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-native-"))
  try {
    const result = spawnSync(process.execPath, [installed(scratch, { native: true, platform: true }), "-p", "."], { env: withoutOverride(), encoding: "utf8" })
    assert.equal(result.status, 0, result.stderr)
    assert.equal(result.stdout, "native -p .\n")
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})

test("names the supported platforms when no compiler is installed", () => {
  const scratch = mkdtempSync(join(tmpdir(), "effeck-missing-"))
  try {
    const result = spawnSync(process.execPath, [installed(scratch)], { env: withoutOverride(), encoding: "utf8" })
    assert.equal(result.status, 1)
    assert.match(result.stderr, /no native compiler is installed for /)
    assert.match(result.stderr, /darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-arm64, win32-x64/)
    assert.match(result.stderr, /EFFECK_BINARY/)
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
})
