import { strict as assert } from "node:assert"
import { spawn } from "node:child_process"
import { once } from "node:events"
import { test } from "node:test"
import { proveNoEgress } from "../compile.mjs"

test("detects a reachable endpoint, a blocked endpoint, and a missing isolation tool", async () => {
  // A separate process serves TCP while the synchronous startup check runs.
  const server = spawn(process.execPath, ["-e", 'const s=require("net").createServer(c=>c.end());s.listen(0,"127.0.0.1",()=>console.log(s.address().port))'], { stdio: ["ignore", "pipe", "inherit"] })
  try {
    const [output] = await once(server.stdout, "data")
    const target = { host: "127.0.0.1", port: Number(output.toString().trim()) }
    assert.equal(proveNoEgress({ network: "policy" }, target), false)
    server.kill()
    await once(server, "exit")
    assert.equal(proveNoEgress({ network: "policy" }, target), true)
    const previous = process.env.PATH
    try {
      process.env.PATH = "/nonexistent"
      assert.equal(proveNoEgress({ network: "unshare" }, target), false)
    } finally {
      process.env.PATH = previous
    }
  } finally {
    server.kill()
  }
})
