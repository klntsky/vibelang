// vibelang --estree converts TypeScript exactly as typescript-estree does, so
// a rule reads a .vibe file's TypeScript forms as it reads a .ts file's. The
// corpus is fixtures/typescript.ts, which holds every TypeScript form, and
// every .ts file in this repository.
import { strict as assert } from "node:assert"
import { readFileSync } from "node:fs"
import { test } from "node:test"
import { parse as typescriptEstree } from "@typescript-eslint/typescript-estree"
import { parseForESLint } from "../../eslint.mjs"
import { firstDifference, plain, relativeToRepo, repositoryFiles } from "./helpers.mjs"

const fixture = new URL("./fixtures/typescript.ts", import.meta.url).pathname

function expected(code, filePath) {
  const ast = typescriptEstree(code, {
    comment: true, filePath, jsx: false, loc: false, range: true, tokens: true, suppressDeprecatedPropertyWarnings: true
  })
  return plain({ ...ast, sourceType: undefined })
}

function actual(code, filePath) {
  const { ast } = parseForESLint(code, { filePath })
  return plain({ ...ast, sourceType: undefined })
}

/** Compares the two trees of one file; undefined means they agree. */
export function compare(filePath) {
  const code = readFileSync(filePath, "utf8")
  let reference
  try {
    reference = expected(code, filePath)
  } catch {
    return { skipped: true }
  }
  let converted
  try {
    converted = actual(code, filePath)
  } catch (error) {
    return { difference: `vibelang --estree failed: ${error.message}` }
  }
  return { difference: firstDifference(converted, reference) }
}

test("fixtures/typescript.ts converts as typescript-estree converts it", () => {
  const { skipped, difference } = compare(fixture)
  assert.equal(skipped, undefined, "typescript-estree must parse the fixture")
  assert.equal(difference, undefined)
})

test("every .ts file in the repository converts as typescript-estree converts it", () => {
  const files = repositoryFiles(".ts").filter((file) => !file.includes("/test/eslint/fixtures/"))
  const failures = []
  let compared = 0
  for (const file of files) {
    const { skipped, difference } = compare(file)
    if (skipped) continue
    compared++
    if (difference) failures.push(`${relativeToRepo(file)}\n  ${difference}`)
  }
  assert.ok(compared > 300, `compared only ${compared} files`)
  assert.deepEqual(failures, [])
})
