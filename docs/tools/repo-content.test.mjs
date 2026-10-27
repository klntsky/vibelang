import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { join } from "node:path"
import { test } from "node:test"
import { fileURLToPath } from "node:url"

const root = process.env.VIBELANG_HYGIENE_ROOT ?? fileURLToPath(new URL("../../", import.meta.url))
const read = path => readFileSync(join(root, path), "utf8")
const home = () => JSON.parse(read(".smithers/home.json"))
const blocks = () => home().sections.flatMap(section => section.blocks)

test("first-run entry points lead with npm and show Bun beside it", () => {
  for (const path of [
    "README.md",
    "docs/user/src/getting-started/004-installation.md",
    "docs/user/src/013-getting-started.md",
    "docs/user/src/setup/new-project.md",
    "site/src/pages/index.mdx",
  ]) {
    const source = read(path)
    assert.equal(source.match(/(?:npm|bun) create vibelang@latest my-app/)?.[0],
      "npm create vibelang@latest my-app", path)
    const commands = [...source.matchAll(/(?:^|\n)\s*```(?:sh|bash)(?: \[\w+\])?\n([\s\S]*?)\n\s*```/g)]
      .map(match => match[1])
    assert.ok(commands.some(command => command.includes("npm create vibelang@latest my-app")), path + ": npm example")
    assert.ok(commands.some(command => command.includes("bun create vibelang@latest my-app")), path + ": Bun alternative")
  }
  assert.equal(blocks().find(block => block.type === "hero").install, "npm create vibelang@latest my-app")
  assert.ok(blocks().some(block => block.type === "code" && block.title === "Bun alternative" &&
    block.code.startsWith("bun create vibelang@latest my-app")), "repo hero: adjacent Bun alternative")
})
