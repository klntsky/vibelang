// Entry declarations must not pull in unrelated bundlers during type checking.
import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

async function checkDeclarations(entry, allowedExternal = []) {
  const visited = new Set()
  async function visit(file) {
    if (visited.has(file.href)) return
    visited.add(file.href)
    const source = await readFile(file, "utf8")
    // Static imports/exports, import types, and triple-slash references.
    const specifiers = /(?:\bfrom\s*|\bimport\s*(?:\(\s*)?|<reference\s+(?:types|path)=)["']([^"']+)["']/g
    for (const [, specifier] of source.matchAll(specifiers)) {
      if (allowedExternal.includes(specifier)) continue
      assert.ok(specifier.startsWith("./") || specifier.startsWith("../"),
        `${fileURLToPath(file)} imports external declaration ${specifier}`)
      const declaration = specifier.replace(/\.js$/, ".d.ts")
      assert.ok(declaration.endsWith(".d.ts"), `expected a declaration: ${specifier}`)
      await visit(new URL(declaration, file))
    }
  }
  await visit(new URL(`../src/${entry}.d.ts`, import.meta.url))
}

test("deferred declarations import nothing outside the package", async () => {
  await checkDeclarations("deferred")
})

for (const [entry, dependency] of Object.entries({
  esbuild: "esbuild",
  rolldown: "rolldown",
  rollup: "rollup",
  vite: "vite",
  webpack: "webpack",
  rspack: "@rspack/core",
})) {
  test(`${entry} declarations import only their own bundler types`, async () => {
    await checkDeclarations(entry, [dependency])
  })
}
