import assert from "node:assert/strict"
import { spawnSync } from "node:child_process"
import { mkdtempSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"
import * as AlchemyNode from "alchemy/Util/Node"

const site = fileURLToPath(new URL("../", import.meta.url))
// Use the installed CLI's loader. Older releases used native TypeScript.
const exec = fileURLToPath(new URL("../bin/exec.js", import.meta.resolve("alchemy")))
const loader = AlchemyNode.nodeLoaderArgs?.(exec) ?? AlchemyNode.transformTypesFlags?.() ?? []

function node(t, args) {
  const home = mkdtempSync(join(tmpdir(), "vibelang-deploy-load-"))
  t.after(() => rmSync(home, { recursive: true, force: true }))
  // No inherited credentials, Alchemy profile, or test-runner context.
  const result = spawnSync(process.execPath, [...loader, ...args], {
    cwd: site,
    env: { PATH: process.env.PATH, HOME: home, CI: "1" },
    encoding: "utf8",
    timeout: 60_000,
  })
  assert.equal(result.error, undefined, result.error?.message)
  assert.equal(result.status, 0, result.stdout + result.stderr)
  return result.stdout
}

test("Alchemy's real CLI entry imports and prints help without credentials", t => {
  const entry = new URL("../bin/alchemy.js", import.meta.resolve("alchemy")).href
  assert.match(node(t, ["--input-type=module", "-e", `
    process.argv = [process.execPath, process.argv[1], "--help"]
    await import(process.argv[1])
  `, entry]), /deploy/)
})

test("the real deployment stack constructs without credentials", t => {
  // Import only: evaluating this lazy Effect would require Cloudflare credentials.
  node(t, ["--input-type=module", "-e", `
    import assert from "node:assert/strict"
    import { isEffect } from "effect/Effect"
    const { default: stack } = await import("./alchemy.run.ts")
    assert.ok(isEffect(stack), "expected a lazy Alchemy stack")
  `])
})
