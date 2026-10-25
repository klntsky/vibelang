import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"

test("the Vite lockfile resolves its compiler peer to the current manifest version", () => {
  const root = new URL("../../", import.meta.url)
  const version = JSON.parse(readFileSync(new URL("packages/compiler/package.json",root),"utf8")).version
  const lock = readFileSync(new URL("packages/vite/bun.lock",root),"utf8")
  assert.ok(lock.includes('"vibelang": "'+version+'"'),"root peer pin")
  assert.ok(lock.includes('"vibelang@'+version+'"'),"resolved compiler")
  assert.doesNotMatch(lock, /vibelang(?:-(?:darwin|linux|win32)-(?:arm64|x64))?@0\.0\.[12]"/)
})
