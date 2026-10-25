import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"

test("the internal TypeScript mapper cannot be published accidentally", () => {
  const pkg = JSON.parse(readFileSync(new URL("../../packages/ts-mapper/package.json", import.meta.url), "utf8"))
  assert.equal(pkg.private, true, pkg.name + " is not a release package and must be private")
})
