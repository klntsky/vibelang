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

test("Contribute gives short actions while retaining every process fact", () => {
  const section = home().sections.find(section => section.title === "Contribute")
  const cards = section.blocks.find(block => block.type === "cards").cards
  const copy = cards.map(card => card.title + " " + card.text).join(" ")
  assert.ok(copy.split(/\s+/).length <= 80, "keep the contributor actions within 80 words")
  for (const fact of ["minimal reproduction", "needs-repro", "accepted", "discussion",
    "maintainer", "timeline", "weeks", "months", "unscheduled", "commitment",
    "committed", "open", "undecided", "proof of concept", "draft", "poc: #123",
    "issue 123", "Check out and run", "focused", "accepted issue", "person",
    "Agents never approve or merge"]) {
    assert.ok(copy.toLowerCase().includes(fact.toLowerCase()), "lost process fact: " + fact)
  }
})

test("benchmark content provides runnable instructions without implying results", () => {
  const benchmark = blocks().find(block => block.title === "Run benchmarks")
  assert.ok(benchmark, "label the section as instructions")
  assert.equal(benchmark.type, "code")
  assert.equal(benchmark.language, "sh")
  for (const command of ["sh scripts/setup.sh", "sh scripts/bench/compiler.sh",
    'VIBELANG="$PWD/packages/compiler/native/vibelang"', "--runs 5 --out .cache/bench",
    "node scripts/bench/lsp.mjs packages/compiler/native/vibelang .cache/bench/synth/vibelang"]) {
    assert.ok(benchmark.code.includes(command), "missing benchmark setup or command: " + command)
  }
  assert.ok(!blocks().some(block => block.title === "Benchmarks"), "remove the unmeasured results section")
})
