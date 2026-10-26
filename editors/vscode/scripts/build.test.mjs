import assert from "node:assert/strict"
import { spawnSync } from "node:child_process"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { test } from "node:test"

test("the editor build preserves the MIT section when its heading starts LICENSE", () => {
  const root = fileURLToPath(new URL("../", import.meta.url))
  const source = readFileSync(new URL("../../../LICENSE", import.meta.url), "utf8")
  assert.ok(source.startsWith("MIT License\n"), "repository reproduction has its MIT heading first")
  const result = spawnSync(process.execPath, ["scripts/build.mjs"], { cwd: root, encoding: "utf8" })
  assert.equal(result.status, 0, result.stdout + result.stderr)
  assert.equal(readFileSync(new URL("../LICENSE", import.meta.url), "utf8"), source)
})
