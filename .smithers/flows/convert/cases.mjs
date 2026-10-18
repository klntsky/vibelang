import { readdirSync, readFileSync } from "node:fs"
import { join } from "node:path"

export function casesIn(dir) {
  return readdirSync(dir, { withFileTypes: true }).filter((entry) => entry.isDirectory()).map((entry) => entry.name).sort().map((name) => ({
    name,
    input: readFileSync(join(dir, name, "input.ts"), "utf8").trimEnd(),
    expected: readFileSync(join(dir, name, "expected.vibe"), "utf8").trimEnd()
  }))
}
