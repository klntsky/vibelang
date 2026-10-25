import { test } from "node:test"
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"

test("the scope guide and current release notes explain the Disposable interruption window and safe acquisition", () => {
  for (const path of ["../user/src/resource-management/030-scope.md", "../user/src/setup/release-notes.md"]) {
    const text = readFileSync(new URL(path, import.meta.url), "utf8")
    assert.ok(/JavaScript Disposable/.test(text), path + ": missing Disposable warning")
    assert.ok(/interruption is pending/.test(text), path + ": missing pending condition")
    assert.ok(/mask ends, before `using` registers/.test(text), path + ": missing registration window")
    assert.ok(/Effect\.acquireRelease/.test(text), path + ": missing safe pattern")
    assert.ok(/\["acquired"\].*\["acquired","disposed"\]/s.test(text), path + ": missing reproduction output")
  }
})
