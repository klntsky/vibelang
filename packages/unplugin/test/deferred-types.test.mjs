// The deferred entry must also stay dependency-free during type checking.
import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

test("deferred declarations import nothing outside the package", async () => {
  const visited = new Set()
  async function visit(file) {
    if (visited.has(file.href)) return
    visited.add(file.href)
    const source = await readFile(file, "utf8")
    // Static imports/exports, import types, and triple-slash references.
    const specifiers = /(?:\bfrom\s*|\bimport\s*(?:\(\s*)?|<reference\s+(?:types|path)=)["']([^"']+)["']/g
    for (const [, specifier] of source.matchAll(specifiers)) {
      assert.ok(specifier.startsWith("./") || specifier.startsWith("../"),
        `${fileURLToPath(file)} imports external declaration ${specifier}`)
      const declaration = specifier.replace(/\.js$/, ".d.ts")
      assert.ok(declaration.endsWith(".d.ts"), `expected a declaration: ${specifier}`)
      await visit(new URL(declaration, file))
    }
  }
  await visit(new URL("../src/deferred.d.ts", import.meta.url))
})
