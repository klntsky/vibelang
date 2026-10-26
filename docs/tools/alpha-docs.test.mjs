import { test } from "node:test"
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = process.env.DOCS_CONTRACT_ROOT || fileURLToPath(new URL("../../", import.meta.url))
const read = path => readFileSync(resolve(root, path), "utf8")
const supportedEffect = JSON.parse(readFileSync(new URL("../../packages/runtime/package.json", import.meta.url), "utf8")).peerDependencies.effect

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

test("2: first-screen compatibility covers exact peers, checker and editor costs", () => {
  for (const path of ["README.md", "site/src/pages/index.mdx"]) {
    const text = read(path)
    const box = text.slice(text.indexOf("compatibility:start"), text.indexOf("compatibility:end"))
    for (const term of ["effect@" + supportedEffect, "@effect/platform-node", "@effect/vitest", "override", "tsc", "vue-tsc", "VS Code", "TypeScript files"]) assert.ok(box.includes(term), path + ": " + term)
    if (path === "README.md") assert.ok(text.indexOf("compatibility:start") < text.indexOf("## Why VibeLang"))
  }
  for (const path of ["docs/user/src/setup/existing-project.md", "docs/user/src/getting-started/004-installation.md"]) {
    assert.match(read(path), /bun add --exact [^\n]*effect@/)
    assert.match(read(path), /bun add -d --exact [^\n]*@effect\/vitest@/)
  }
  assert.match(read("packages/compiler/README.md"), /exact.*fiber internals/s)
  assert.match(read("site/src/pages/index.mdx"), /libraries compatible with the supported Effect version/)
})
