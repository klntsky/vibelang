import assert from "node:assert/strict"
import { createHash } from "node:crypto"
import { readFileSync } from "node:fs"
import { test } from "node:test"

const read = (path) => readFileSync(new URL("../../" + path, import.meta.url), "utf8")
test("the fork notice names the upstream commit and VibeLang's modifications before Microsoft's notices", () => {
  const upstream = /\| Upstream commit \| `([a-f0-9]{40})`/.exec(read("typescript/VIBELANG.md"))?.[1]
  assert.ok(upstream)
  const notice = read("typescript/NOTICE.txt")
  const microsoft = notice.indexOf("NOTICES AND INFORMATION")
  assert.ok(microsoft > 0, "VibeLang's modification notice must precede the preserved Microsoft notice")
  const own = notice.slice(0, microsoft)
  assert.match(own, /files under typescript\/ are a modified version of TypeScript/)
  assert.ok(own.includes(upstream))
  assert.match(own, /changed by the VibeLang authors/)
  assert.match(notice.slice(microsoft), /Microsoft Corporation/)
  assert.equal(createHash("sha256").update(notice.slice(microsoft)).digest("hex"), "317cd4d9d8a4de4971222bd232bc45c010424bf0dae61be830e49532e62e4d33", "preserve the Microsoft notice byte for byte")
})

test("fork project documents point readers to VibeLang and upstream rather than Microsoft's release instructions", () => {
  const targets = {
    "README.md": "../README.md",
    "SECURITY.md": "../.github/SECURITY.md",
    "CONTRIBUTING.md": "../CONTRIBUTING.md",
    "CODE_OF_CONDUCT.md": "../CONTRIBUTING.md",
    "SUPPORT.md": "../README.md",
  }
  for (const [file, target] of Object.entries(targets)) {
    const text = read("typescript/" + file)
    assert.ok(text.includes("](" + target + ")"), file)
    assert.ok(text.includes("https://github.com/microsoft/TypeScript"), file)
    assert.ok(text.length < 1200, file)
    assert.doesNotMatch(text, /actions\/workflows\/ci|npm install -D typescript/)
  }
})
