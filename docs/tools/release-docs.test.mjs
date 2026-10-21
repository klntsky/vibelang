import { test } from "node:test"
import assert from "node:assert/strict"
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import { releaseDocsProblems, renderReleasePage } from "./release-docs.mjs"

test("install pins, compiler output, prose and homepage badges derive from the compiler version", () => {
  const text = "Version 0.0.2; VibeLang 0.0.3; vibelang 0.0.1; In 0.0.2; Rehearsed for 0.0.2\n" +
    "vibelang@0.0.2 vibelang-runtime@0.0.2 vibelang-vite@0.0.2 vibelang-unplugin@0.0.2\n" +
    "Run npm view vibelang@0.0.2 version and expect \x600.0.2\x60.\n<strong>0.0.1</strong>"
  const next = renderReleasePage("site/src/pages/index.mdx", text, "9.8.7")
  assert.doesNotMatch(next, /0\.0\.[123]/)
  assert.equal(renderReleasePage("page.md", next, "9.8.7"), next)
})

test("historical release notes and ordinary addresses keep their original values", () => {
  const text = "Version 0.0.2; vibelang@0.0.2"
  assert.equal(renderReleasePage("docs/user/src/setup/release-notes.md", text, "9.8.7"), text)
  assert.equal(renderReleasePage("site/src/pages/setup/release-notes.mdx", text, "9.8.7"), text)
  assert.equal(renderReleasePage("page.md", 'fetch("http://127.0.0.1:8080") effect@4.0.5', "9.8.7"),
    'fetch("http://127.0.0.1:8080") effect@4.0.5')
})

test("the docs gate rejects stale source and deployed copies after a version bump", () => {
  const repo = mkdtempSync(join(tmpdir(), "release-docs-"))
  const put = (path, text) => { const file = join(repo, path); mkdirSync(dirname(file), { recursive: true }); writeFileSync(file, text) }
  try {
    put("packages/compiler/package.json", JSON.stringify({ version: "9.8.7" }))
    put("docs/user/src/setup.md", "VibeLang 0.0.2; vibelang@0.0.2")
    put("docs/user/notes/review.md", "Version 0.0.1; vibelang@0.0.1")
    put("site/src/pages/setup.mdx", "VibeLang 0.0.2; vibelang@0.0.2")
    assert.equal(releaseDocsProblems(repo).length, 2)
    releaseDocsProblems(repo, true)
    assert.equal(readFileSync(join(repo, "docs/user/src/setup.md"), "utf8"), "VibeLang 9.8.7; vibelang@9.8.7")
    assert.equal(releaseDocsProblems(repo).length, 1)
    put("site/src/pages/setup.mdx", "VibeLang 9.8.7; vibelang@9.8.7")
    assert.deepEqual(releaseDocsProblems(repo), [])
    put("packages/compiler/package.json", JSON.stringify({ version: "9.8.8" }))
    assert.equal(releaseDocsProblems(repo).length, 2)
  } finally { rmSync(repo, { recursive: true, force: true }) }
})

test("all current repository release references match the compiler manifest", () => {
  assert.deepEqual(releaseDocsProblems(fileURLToPath(new URL("../../", import.meta.url))), [])
})

test("install docs and the launcher describe the four published platforms and WSL", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url))
  for (const path of ["docs/user/src/getting-started/004-installation.md", "docs/user/src/setup.md",
    "docs/design/src/428-part4-ch28-tooling.md", ".smithers/instructions/app.md", "packages/compiler/README.md"]) {
    const text = readFileSync(join(root, path), "utf8")
    assert.match(text, /Windows.*not (?:available|published).*WSL/s, path)
    assert.doesNotMatch(text, /\| Windows on .*vibelang-win32/, path)
    assert.doesNotMatch(text, /ships all six|one of six/, path)
  }
  const launcher = readFileSync(join(root, "packages/compiler/bin/vibelang"), "utf8")
  assert.doesNotMatch(launcher, /const platforms = .*win32/)
})
