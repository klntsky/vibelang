// vibelang --estree converts TypeScript exactly as typescript-estree does, so
// a rule reads a .vibe file's TypeScript forms as it reads a .ts file's. The
// corpus is fixtures/typescript.ts, which holds every TypeScript form, and
// every .ts file in this repository; the fixture is also the body of a
// module declaration (D283).
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

// A module declaration's body is TypeScript at the .vibe file's offsets:
// typescript-estree's tree of the same text, with everything outside the
// body blanked, is the body's, node for node and token for token.
const blank = (text) => text.replace(/[^\n\r\u2028\u2029]/g, " ")

test("a module declaration's body converts as typescript-estree converts its TypeScript", () => {
  const head = "// VibeLang around a module, after non-ASCII text: é😀\nerror Gone { id: string }\n\nmodule text {"
  const body = `\n${readFileSync(fixture, "utf8")}`
  const tail = "}\n\nimport { answer } from text\n\nexport function main(): Effect<number, Gone> {\n  return answer |> % + 1\n}\n"
  const code = head + body + tail
  const start = head.length
  const end = start + body.length
  const { ast } = parseForESLint(code, { filePath: "/module.vibe" })
  const module = ast.body.find((node) => node.type === "VibeModuleDeclaration")
  assert.deepEqual(module.body.range, [start - 1, end + 1])
  const reference = expected(blank(code.slice(0, start)) + body + blank(code.slice(end)), "/module.ts")
  const inside = ({ range }) => range[0] >= start && range[1] <= end
  assert.equal(firstDifference(plain(module.body.body), reference.body), undefined)
  assert.equal(firstDifference(plain(ast.tokens.filter(inside)), reference.tokens), undefined)
  assert.equal(firstDifference(plain(ast.comments.filter(inside)), reference.comments), undefined)
})
