// Cloud Build runs this inside the final image with no network, 1 CPU, 512 MiB,
// a read-only root, and a writable /tmp. The source mount contains no credentials.
import { strict as assert } from "node:assert"
import { spawn } from "node:child_process"
import { once } from "node:events"
import { existsSync, readFileSync } from "node:fs"

assert.equal(process.getuid(), 1000)
for (const shell of ["/bin/sh", "/bin/bash", "/bin/dash"]) assert.equal(existsSync(shell), false)
const server = spawn(process.execPath, ["/opt/converter/server.mjs"], { stdio: ["ignore", "pipe", "inherit"] })
try {
  await Promise.race([
    once(server.stdout, "data"),
    once(server, "exit").then(([code]) => { throw new Error(`server exited ${code}`) }),
    new Promise((_, reject) => setTimeout(() => reject(new Error("server startup timeout")), 15_000).unref())
  ])
  const health = await fetch("http://127.0.0.1:8080/healthz")
  assert.equal(health.status, 200)
  assert.equal((await health.json()).network, "policy")
  const post = async (source) => {
    const response = await fetch("http://127.0.0.1:8080/compile", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ source }) })
    assert.equal(response.status, 200)
    return response.json()
  }
  const hero = readFileSync("/workspace/.smithers/flows/convert/golden/01-hero-batch-lookup/expected.vibe", "utf8")
  const good = await post(hero)
  assert.equal(good.exitCode, 0, JSON.stringify(good))
  assert.equal(good.timedOut, false)
  assert.deepEqual(good.diagnostics, [])
  const bad = await post('export function f(): Effect<number> { return "no" }')
  assert.notEqual(bad.exitCode, 0)
  assert.ok(bad.diagnostics.length > 0)
  assert.match(bad.diagnostics[0].code, /^(TS|EK)\d+$/)
  console.log(JSON.stringify({ event: "image-smoke", uid: process.getuid(), heroExitCode: good.exitCode, typeErrorDiagnostics: bad.diagnostics.length, compilerDigest: good.compilerDigest, dependenciesDigest: good.dependenciesDigest }))
} finally {
  server.kill()
  await once(server, "exit")
}
