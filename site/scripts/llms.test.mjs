import assert from "node:assert/strict"
import { readFileSync, existsSync } from "node:fs"
import { resolve, join, dirname } from "node:path"
import { fileURLToPath } from "node:url"
import { test } from "node:test"
const repo = resolve(dirname(fileURLToPath(import.meta.url)), "../..")
const output = join(repo, "site/dist/public")
test("built llms.txt has essentials first and one section per sidebar view", () => {
  const llms = readFileSync(join(output, "llms.txt"), "utf8")
  const essentials = readFileSync(join(repo, "docs/user/llms-essentials.md"), "utf8").trim()
    .replace(/\]\((?!https?:)([^)]+\.md)\)/g, (_, page) => "](https://vibelang.sh/" + page + ")")
  assert.ok(llms.indexOf(essentials) > 0)
  assert.ok(llms.indexOf(essentials) < llms.indexOf("## Onboarding"))
  assert.ok(Buffer.byteLength(llms.slice(0, llms.indexOf(essentials) + essentials.length)) <= 2000)
  const inventory = readFileSync(join(repo, "docs/user/inventory.md"), "utf8").split("## V4 sidebar in order")[1].split(/\n## /)[0]
  const views = [...inventory.matchAll(/^### (.+)$/gm)].map(m => m[1])
  assert.deepEqual([...llms.matchAll(/^## (.+)$/gm)].map(m => m[1]).filter(v => v !== "Essentials"), views)
  const urls = [...llms.matchAll(/\]\((https?:[^)]+)\)/g)].map(m => new URL(m[1]))
  assert.ok(urls.length > 100)
  for (const url of urls) {
    assert.equal(url.origin, "https://vibelang.sh")
    assert.ok(url.pathname.endsWith(".md"))
    assert.ok(existsSync(join(output, decodeURIComponent(url.pathname))), url.href)
  }
})
test("built Markdown rewrites page links and aliases the first pages", () => {
  const page = readFileSync(join(output, "001-onboarding.md"), "utf8")
  assert.ok(page.includes("https://vibelang.sh/getting-started/004-installation.md"))
  for (const [alias, source] of [["index.md", "001-onboarding.md"], ["spec/index.md", "spec/000-intro.md"]]) {
    assert.equal(readFileSync(join(output, alias), "utf8"), readFileSync(join(output, source), "utf8"))
  }
  const spec = readFileSync(join(output, "spec/900-appendices.md"), "utf8")
  assert.ok(!spec.includes("{{testCount}}"))
})
