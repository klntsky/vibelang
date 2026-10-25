import { expect, test } from "bun:test"
import { mkdtempSync, readFileSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { buildMarkdown } from "./markdown"

test("published Markdown omits harness directives and preserves example outputs", () => {
  const dir = mkdtempSync(join(tmpdir(), "vibelang-markdown-"))
  try {
    buildMarkdown(dir, { testCount: "123", testFiles: "4", kernelLines: "567" })
    const page = readFileSync(join(dir, "setup/vite.md"), "utf8")
    expect(page).not.toMatch(/^\s*\/\/ run:/m)
    expect(page).toContain("// Output: Hello, Vite!")
    expect(page).toContain('export function greet(name: string)')
  } finally { rmSync(dir, { recursive: true, force: true }) }
})
