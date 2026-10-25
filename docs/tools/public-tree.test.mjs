import { test } from "node:test"
import assert from "node:assert/strict"
import { existsSync, readFileSync } from "node:fs"

test("the public tree omits drafting history and keeps the referenced compiler-gap inventory", () => {
  assert.ok(!existsSync(new URL("../user/notes", import.meta.url)), "remove docs/user/notes drafting history")
  const inventory = new URL("../user/compiler-gaps.md", import.meta.url)
  assert.ok(existsSync(inventory), "preserve the referenced compiler-gap inventory")
  assert.ok(readFileSync(inventory, "utf8").includes("## Runtime disagreements"), "preserve the runtime findings")
  for (const path of ["README.md", "../user/STYLE.md", "../../scripts/rename-to-vibelang.mjs"]) {
    assert.ok(!readFileSync(new URL(path, import.meta.url), "utf8").includes("docs/user/notes/"), "stale drafting directory reference: " + path)
  }
  assert.ok(readFileSync(new URL("README.md", import.meta.url), "utf8").includes("docs/user/compiler-gaps.md"), "link the retained compiler-gap inventory")
})
