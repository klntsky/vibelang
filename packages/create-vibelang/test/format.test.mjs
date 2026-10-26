import { strict as assert } from "node:assert"
import { spawnSync } from "node:child_process"
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const create = fileURLToPath(new URL("../bin/create-vibelang.mjs", import.meta.url))
const compiler = fileURLToPath(new URL("../../compiler/", import.meta.url))
const dprint = process.env.VIBELANG_DPRINT
const skip = !dprint ? "set VIBELANG_DPRINT to a dprint binary"
  : !existsSync(join(compiler, "dprint-plugin.json")) ? "run sh scripts/build-vibelang.sh" : false

test("a freshly generated starter passes dprint check with the documented configuration", { skip }, (t) => {
  const scratch = mkdtempSync(join(tmpdir(), "vibelang-starter-format-"))
  t.after(() => rmSync(scratch, { recursive: true, force: true }))
  const app = join(scratch, "hello app")
  const created = spawnSync(process.execPath, [create, app, "--no-install"], { encoding: "utf8" })
  assert.equal(created.status, 0, created.stdout + created.stderr)
  // Use the guide's actual config so its settings and plugins cannot drift.
  const guide = readFileSync(new URL("../../../docs/user/src/setup/formatting.md", import.meta.url), "utf8")
  const config = JSON.parse(guide.match(/```json\n([\s\S]*?)\n```/)[1])
  writeFileSync(join(app, "dprint.json"), JSON.stringify(config, null, 2) + "\n")
  mkdirSync(join(app, "node_modules"))
  symlinkSync(compiler, join(app, "node_modules/vibelang"), process.platform === "win32" ? "junction" : "dir")
  const checked = spawnSync(dprint, ["check"], {
    cwd: app, encoding: "utf8", timeout: 120_000, env: { ...process.env, NO_COLOR: "1" }
  })
  assert.equal(checked.error, undefined, checked.error?.message)
  assert.equal(checked.status, 0, checked.stdout + checked.stderr)
})
