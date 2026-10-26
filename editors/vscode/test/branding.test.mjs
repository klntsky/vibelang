import assert from "node:assert/strict"
import { readFileSync, readdirSync } from "node:fs"
import { extname, join, relative } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const editors = fileURLToPath(new URL("../../", import.meta.url))
// Cover integration manifests, docs, setup examples and test instructions.
// EK diagnostic codes and TypeScript scopes intentionally retain their names.
const extensions = new Set([".md", ".json", ".lua", ".mjs", ".sh", ".toml", ".svg"])
const skip = new Set(["node_modules", ".vscode-test", "dist"])
function sources(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const file = join(dir, entry.name)
    if (entry.isDirectory()) return skip.has(entry.name) ? [] : sources(file)
    return entry.isFile() && extensions.has(extname(file)) ? [file] : []
  })
}

test("editor integrations use the current VibeLang name in metadata and setup guidance", () => {
  const stale = sources(editors).filter((file) => /effect[ -]?(?:script|kit|lang)/i.test(readFileSync(file, "utf8")))
  assert.deepEqual(stale.map((file) => relative(editors, file)), [], "remove old project names from editor integration sources")
})
