import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"

test("the Vite development lock leaves the compiler to the checkout fixture", () => {
  const root = new URL("../../", import.meta.url)
  const read = path => JSON.parse(readFileSync(new URL(path, root), "utf8"))
  const compiler = read("packages/compiler/package.json")
  const vite = read("packages/vite/package.json")
  const lock = readFileSync(new URL("packages/vite/bun.lock", root), "utf8")
  assert.equal(vite.peerDependencies.vibelang, compiler.version, "published compiler peer pin")
  assert.equal(vite.devDependencies.vibelang, undefined, "local consumers must not inherit a relative compiler devDependency")
  assert.ok(lock.includes('"vibelang": "' + compiler.version + '"'), "root peer pin")
  assert.equal(vite.overrides?.vibelang, undefined, "published metadata must not override its compiler peer")
  assert.doesNotMatch(lock, /"vibelang(?:-(?:darwin|linux|win32)-(?:arm64|x64))?": \[/, "the fixture supplies the compiler instead of a locked package")
})
