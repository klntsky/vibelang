// vibelang/eslint on VibeLang's own forms: every cheat-sheet form parses into
// a tree whose tokens are the source and whose nodes keep every name the
// author wrote, the rules Smithers lints with find nothing in clean code and
// each seeded violation, fixes land on the .vibe text, and a syntax error is
// ESLint's parsing error.
import { strict as assert } from "node:assert"
import { readFileSync } from "node:fs"
import { test } from "node:test"
import { Linter } from "eslint"
import { visitorKeys as typescriptKeys } from "@typescript-eslint/visitor-keys"
import vibelang, { parseForESLint, vibeVisitorKeys } from "../../eslint.mjs"
import { config } from "./config.mjs"
import { allVisitorKeys, problems, walk } from "./tree.mjs"

const fixture = (name) => {
  const filename = new URL(`./fixtures/${name}`, import.meta.url).pathname
  return { filename, code: readFileSync(filename, "utf8") }
}
const lint = ({ code, filename }) => new Linter({ configType: "flat", cwd: "/" }).verify(code, config, { filename })
const keys = allVisitorKeys(typescriptKeys)

test("every cheat-sheet form parses into a tree of the source's own tokens and names", () => {
  const { code, filename } = fixture("forms.vibe")
  const { ast } = parseForESLint(code, { filePath: filename })
  assert.deepEqual(problems(code, ast, keys), [])
})

test("VibeLang's forms take their own node types, which keep every source child", () => {
  const { code, filename } = fixture("forms.vibe")
  const { ast, visitorKeys } = parseForESLint(code, { filePath: filename })
  const types = new Map()
  walk(ast, (node) => types.set(node.type, (types.get(node.type) ?? 0) + 1), visitorKeys)
  for (const type of Object.keys(vibeVisitorKeys)) assert.ok(types.has(type), `forms.vibe has no ${type}`)
  assert.equal(types.get("VibeProvideStatement"), 1)
  assert.equal(types.get("VibeServiceProvider"), 1)
  assert.equal(types.get("VibeCatchExpression"), 2)
  assert.equal(types.get("VibeMaskStatement"), 3)
  assert.equal(types.get("VibePipeExpression"), 3)
  assert.equal(types.get("TopicReference"), 3)
  // The parser's long forms that read as TypeScript stay TypeScript.
  for (const type of ["ArrowFunctionExpression", "ClassDeclaration", "TryStatement", "TSTypeLiteral"]) {
    assert.ok(types.has(type), `forms.vibe has no ${type}`)
  }
})

test("comments and JSDoc survive with their text and place", () => {
  const { code, filename } = fixture("forms.vibe")
  const { ast } = parseForESLint(code, { filePath: filename })
  const header = ast.comments[0]
  assert.equal(header.type, "Block")
  assert.match(header.value, /^\*\n \* Every form of the VibeLang cheat sheet/)
  assert.deepEqual(header.loc.start, { line: 1, column: 0 })
  const lines = ast.comments.filter((comment) => comment.type === "Line").map((comment) => comment.value)
  assert.deepEqual(lines, [" Declarations", " Contracts", " Programs"])
  const jsdoc = ast.comments.filter((comment) => comment.type === "Block" && comment.value.startsWith("*"))
  assert.equal(jsdoc.length, code.match(/\/\*\*/g).length)
})

test("the rules Smithers lints with find nothing in clean VibeLang", () => {
  assert.deepEqual(lint(fixture("forms.vibe")), [])
})

test("each seeded violation is reported on its line and UTF-16 column", () => {
  const found = lint(fixture("violations.vibe")).map(({ line, column, ruleId }) => `${line}:${column} ${ruleId}`)
  assert.deepEqual(found, [
    "7:26 import/no-duplicates",
    "8:1 @typescript-eslint/consistent-type-imports",
    "8:22 import/no-duplicates",
    "10:1 jsdoc/require-jsdoc",
    "10:35 @typescript-eslint/array-type",
    "11:7 prefer-const",
    "12:9 @typescript-eslint/no-unused-vars",
    "13:29 no-console",
    "24:28 no-restricted-syntax",
    "33:33 no-restricted-syntax",
    "44:35 @typescript-eslint/no-unused-vars",
    "46:5 no-var",
    "53:1 jsdoc/no-restricted-syntax"
  ])
})

test("fixes rewrite the .vibe text, and the result parses", () => {
  const { code, filename } = fixture("fixable.vibe")
  const result = new Linter({ configType: "flat", cwd: "/" }).verifyAndFix(code, config, { filename })
  assert.deepEqual(result.messages, [])
  assert.equal(result.output, fixture("fixable.expected.vibe").code)
  assert.doesNotThrow(() => parseForESLint(result.output, { filePath: filename }))
})

test("a syntax error is ESLint's parsing error at its line and column", () => {
  const [message] = lint(fixture("broken.vibe"))
  assert.equal(message.fatal, true)
  assert.equal(message.ruleId, null)
  assert.equal(message.message, "Parsing error: '}' expected.")
  assert.deepEqual([message.line, message.column], [4, 1])
})

test("the parser declares its name and version for ESLint's cache", () => {
  const manifest = JSON.parse(readFileSync(new URL("../../package.json", import.meta.url), "utf8"))
  assert.deepEqual(vibelang.meta, { name: "vibelang/eslint", version: manifest.version })
})

test("names the parser writes are not variables a rule can report", () => {
  const code = "export function f(e: unknown) {\n  return match (e) {\n    when Exit.Success(let value): value;\n    default: 0;\n  }\n}\n"
  const { scopeManager } = parseForESLint(code, { filePath: "/f.vibe" })
  const names = scopeManager.scopes.flatMap((scope) => scope.variables.map((variable) => variable.name))
  assert.ok(names.includes("value"))
  assert.ok(!names.some((name) => name.startsWith("$")), names.join(", "))
})
