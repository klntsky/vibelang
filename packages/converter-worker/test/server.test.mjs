// Real HTTP surface with the Linux compile/isolation boundary replaced through
// Node's loader hooks. No customer program or compiler is executed here.
import assert from "node:assert/strict"
import { spawn } from "node:child_process"
import { once } from "node:events"
import { mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { createServer } from "node:net"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath, pathToFileURL } from "node:url"

const worker = new URL("../server.mjs", import.meta.url)
const boundary = `
export const maxSourceBytes = 8192
export const settings = () => ({ network: "policy" })
export const identity = () => ({ vibelangVersion: "test-version", compilerDigest: "sha256:compiler", dependenciesDigest: "sha256:deps" })
export const proveNoEgress = () => !process.env.TEST_EGRESS
let active = 0, ordinal = 0
export async function compile(source) {
 if (Buffer.byteLength(source) > maxSourceBytes) return { refusal: "too_large" }
 if (source.includes("\\0")) return { refusal: "invalid_source" }
 if (source === "worker-fault") throw new Error("private worker fault detail")
 const order = ++ordinal
 const concurrency = ++active
 await new Promise(resolve => setTimeout(resolve, source === "slow" ? 40 : 0))
 active--
 return { exitCode: 0, timedOut: false, diagnostics: [], truncated: false, order, concurrency }
}
`
async function launch(t, env = {}) {
  const scratch = mkdtempSync(join(tmpdir(), "vibelang-worker-http-"))
  t.after(() => rmSync(scratch, { recursive: true, force: true }))
  const hook = join(scratch, "hook.mjs")
  const boundaryFile = join(scratch, "boundary.mjs")
  writeFileSync(boundaryFile, boundary)
  writeFileSync(hook, `import { registerHooks } from "node:module"; process.on("SIGTERM", () => process.exit(0)); registerHooks({ resolve(specifier, context, next) { if(specifier === "./compile.mjs" && context.parentURL === ${JSON.stringify(worker.href)}) return { url: ${JSON.stringify(pathToFileURL(boundaryFile).href)}, shortCircuit: true }; return next(specifier, context) } });`)
  const listener = createServer()
  listener.listen(0, "127.0.0.1")
  await once(listener, "listening")
  const port = listener.address().port
  await new Promise((resolve) => listener.close(resolve))
  const child = spawn("node", ["--import", hook, fileURLToPath(worker)], {
    env: { ...process.env, PORT: String(port), ...env }, stdio: ["ignore", "pipe", "pipe"]
  })
  const closed = once(child, "close")
  let stdout = "", stderr = ""
  child.stdout.on("data", (chunk) => { stdout += chunk })
  child.stderr.on("data", (chunk) => { stderr += chunk })
  t.after(async () => { child.kill(); await closed })
  const deadline = Date.now() + 5000
  while (!stdout.includes('"event":"listening"') && child.exitCode === null && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 10))
  }
  return { base: `http://127.0.0.1:${port}`, child, closed, logs: () => stdout + stderr }
}
const post = (server, body) => fetch(`${server.base}/compile`, { method: "POST", body: JSON.stringify(body), signal: AbortSignal.timeout(5000) })

test("HTTP routes, request validation, receipts, serialization, privacy and fault recovery", { timeout: 15_000 }, async (t) => {
  const server = await launch(t)
  const health = await fetch(`${server.base}/healthz`)
  assert.equal(health.status, 200)
  assert.equal(health.headers.get("content-type"), "application/json")
  assert.equal(health.headers.get("cache-control"), "no-store")
  assert.deepEqual(await health.json(), { ok: true, vibelangVersion: "test-version", compilerDigest: "sha256:compiler", dependenciesDigest: "sha256:deps", network: "policy" })
  for (const [method, path] of [["GET", "/compile"], ["POST", "/healthz"], ["GET", "/missing"]]) {
    const response = await fetch(server.base + path, { method })
    assert.equal(response.status, 404)
    assert.deepEqual(await response.json(), { error: "not_found" })
  }
  const invalid = await fetch(`${server.base}/compile`, { method: "POST", body: "{" })
  assert.equal(invalid.status, 400)
  assert.deepEqual(await invalid.json(), { error: "invalid_json" })
  for (const body of [{}, { source: 42 }, { source: null }]) {
    const response = await post(server, body)
    assert.equal(response.status, 400)
    assert.deepEqual(await response.json(), { error: "source_required" })
  }
  for (const [source, refusal] of [["x".repeat(8193), "too_large"], ["text\0text", "invalid_source"]]) {
    const response = await post(server, { source })
    assert.equal(response.status, 422)
    assert.deepEqual(await response.json(), { refusal })
  }
  const oversized = await post(server, { source: "x".repeat(20_000) })
  assert.equal(oversized.status, 413)
  assert.deepEqual(await oversized.json(), { refusal: "too_large" })
  const slow = post(server, { source: "slow" })
  await new Promise((resolve) => setTimeout(resolve, 10))
  const quick = post(server, { source: "request-marker-private" })
  const [first, second] = await Promise.all([slow.then((r) => r.json()), quick.then((r) => r.json())])
  assert.equal(first.concurrency, 1)
  assert.equal(second.concurrency, 1)
  assert.equal(second.order, first.order + 1)
  assert.equal(second.compilerDigest, "sha256:compiler")
  assert.ok(!server.logs().includes("request-marker-private"))
  assert.match(server.logs(), /"event":"compile"/)

  const fault = await post(server, { source: "worker-fault" })
  assert.equal(fault.status, 500)
  assert.deepEqual(await fault.json(), { error: "compile_failed" })
  assert.ok(!server.logs().includes("private worker fault detail"))
  assert.match(server.logs(), /"event":"compile_failed"/)
  const recovered = await post(server, { source: "good after fault" })
  assert.equal(recovered.status, 200)
})

test("startup refuses to serve when the compile environment has network access", async (t) => {
  const server = await launch(t, { TEST_EGRESS: "1" })
  const [code] = await server.closed
  assert.equal(code, 1)
  assert.match(server.logs(), /refusing to start/)
  assert.ok(!server.logs().includes('"event":"listening"'))
})
