import { test } from "node:test"
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = process.env.DOCS_CONTRACT_ROOT || fileURLToPath(new URL("../../", import.meta.url))
const read = path => readFileSync(resolve(root, path), "utf8")

test("1: onboarding explains calls into local TypeScript and npm at the boundary", () => {
  for (const path of ["README.md", "docs/user/src/setup/existing-project.md"]) {
    const text = read(path)
    assert.match(text, /your own.*\.ts.*npm/)
    for (const term of ["try", "trusted", "trustedPackages", "EK4401", "EK4402", "EK4403"]) assert.ok(text.includes(term), path + ": " + term)
    assert.match(text, /114-vibelang-for-effect-users/)
    assert.match(text, /012-devtools/)
    assert.match(text, /import \{ parseSettings \} from "\.\/settings\.ts"\nfunction settings\(text: string\) \{ return try\(\(text\) => parseSettings\(text\), text\) \}/)
  }
})
