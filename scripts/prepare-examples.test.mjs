import assert from "node:assert/strict"
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, existsSync, rmSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { test } from "node:test"
import { prepareExample } from "./prepare-examples.mjs"

test("CI stages examples with local packages and leaves published sources unchanged", t => {
  const scratch = mkdtempSync(join(tmpdir(), "vibelang-examples-test-"))
  t.after(() => rmSync(scratch, { recursive: true, force: true }))
  const source = join(scratch, "source")
  mkdirSync(join(source, "src"), { recursive: true })
  mkdirSync(join(source, "node_modules"))
  const manifest = JSON.stringify({ dependencies: { "vibelang-runtime": "9.8.7", effect: "4.0.5" }, devDependencies: { vibelang: "9.8.7", "vibelang-vite": "9.8.7" } })
  writeFileSync(join(source, "package.json"), manifest)
  writeFileSync(join(source, "src/main.vibe"), "export function main() {}")
  writeFileSync(join(source, "bun.lock"), "old")
  const destination = join(scratch, "staged")
  const packages = { vibelang: join(scratch, "compiler"), "vibelang-runtime": join(scratch, "runtime.tgz"), "vibelang-vite": join(scratch, "vite") }
  prepareExample(source, destination, packages, "4.0.5")
  assert.equal(readFileSync(join(source, "package.json"), "utf8"), manifest)
  const result = JSON.parse(readFileSync(join(destination, "package.json"), "utf8"))
  assert.equal(result.dependencies["vibelang-runtime"], "file:" + packages["vibelang-runtime"])
  assert.equal(result.devDependencies.vibelang, "file:" + packages.vibelang)
  assert.equal(result.devDependencies["vibelang-vite"], "file:" + packages["vibelang-vite"])
  assert.equal(result.overrides["@effect/platform-node-shared"], "4.0.5")
  assert.equal(result.overrides.vibelang, "file:" + packages.vibelang)
  assert.ok(existsSync(join(destination, "src/main.vibe")))
  assert.ok(!existsSync(join(destination, "node_modules")))
  assert.ok(!existsSync(join(destination, "bun.lock")))
})
