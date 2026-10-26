import assert from "node:assert/strict"
import { execFileSync } from "node:child_process"
import { lstatSync, readFileSync, readdirSync } from "node:fs"
import { resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { test } from "node:test"
import { publicHygiene } from "./public-hygiene.mjs"

const root = process.env.VIBELANG_HYGIENE_ROOT ?? fileURLToPath(new URL("../../", import.meta.url))
const files = process.env.VIBELANG_HYGIENE_ROOT
  ? readdirSync(root, { recursive: true })
  : execFileSync("jj", ["file", "list"], { cwd: root, encoding: "utf8", maxBuffer: 16 * 1024 * 1024 }).trimEnd().split("\n")
const findings = files.flatMap(file => {
  const path = resolve(root, file)
  if (!lstatSync(path).isFile()) return []
  const bytes = readFileSync(path)
  if (bytes.includes(0)) return []
  return publicHygiene(file, bytes.toString("utf8")).map(problem => file + ": " + problem)
})

test("the public tree contains no retired hosted service or client", () => {
  assert.deepEqual(findings.filter(p => p.endsWith("a removed hosted compile service")), [])
})

test("the hygiene check rejects references as well as service files", () => {
  const retired = ["converter", "worker"].join("-")
  assert.equal(publicHygiene("packages/" + retired + "/README.md", "").length, 1)
  assert.equal(publicHygiene("scripts/check.mjs", "import '../packages/" + retired + "/compile.mjs'").length, 1)
  assert.deepEqual(publicHygiene("packages/compiler/index.mjs", "export const version = '0.0.9'"), [])
})
