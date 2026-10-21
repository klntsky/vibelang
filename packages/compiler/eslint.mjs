// vibelang/eslint: an ESLint parser for .vibe files.
//
//   import vibelang from "vibelang/eslint"
//   export default [{ files: ["**/*.vibe"], languageOptions: { parser: vibelang } }]
//
// The compiler parses the file and converts its tree to ESTree
// (`vibelang --estree`, typescript/tsc/internal/vibelangestree): TypeScript
// forms take typescript-estree's shapes, so typescript-eslint's rules and
// every plugin that reads TypeScript work unchanged, and VibeLang's own forms
// take the node types in vibeVisitorKeys. Scope analysis is
// @typescript-eslint/scope-manager's, resolved from the typescript-eslint the
// project installs. There is no TypeScript program, so rules that need type
// information do not run on .vibe files; `vibelang -p` type-checks them.
import { spawnSync } from "node:child_process"
import { existsSync, readFileSync } from "node:fs"
import { createRequire } from "node:module"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const manifest = JSON.parse(readFileSync(new URL("./package.json", import.meta.url), "utf8"))

export const meta = { name: "vibelang/eslint", version: manifest.version }

/** Child keys of the ESTree node types VibeLang adds. */
export const vibeVisitorKeys = Object.freeze({
  TopicReference: [],
  VibeCatchExpression: ["expression", "handler"],
  VibeEffectType: ["success", "failure", "requirements"],
  VibeInferType: [],
  VibeMaskStatement: ["body"],
  VibePipeExpression: ["left", "right"],
  VibeProvideStatement: ["providers", "body"],
  VibeServiceProvider: ["key", "value"]
})

const executable = process.platform === "win32" ? "vibelang.exe" : "vibelang"

// nativeCompiler finds the compiler as bin/vibelang does, without starting
// Node for each file: VIBELANG_BINARY, then native/vibelang beside this
// package (a checkout build), then the installed platform package.
function nativeCompiler() {
  if (process.env.VIBELANG_BINARY) return resolve(process.env.VIBELANG_BINARY)
  const local = fileURLToPath(new URL(`./native/${executable}`, import.meta.url))
  if (existsSync(local)) return local
  const target = `${process.platform}-${process.arch}`
  try {
    const platform = createRequire(import.meta.url).resolve(`vibelang-${target}/package.json`)
    return join(dirname(platform), "bin", executable)
  } catch {
    throw new Error(`vibelang/eslint: no native compiler is installed for ${target}. Reinstall vibelang without omitting optional dependencies, or set VIBELANG_BINARY.`)
  }
}

// typescriptEslint loads scope analysis and visitor keys: beside this package
// when they are installed with it, else through the typescript-eslint the
// linted project installs, which depends on both.
const loaded = new Map()
function typescriptEslint(filePath) {
  const bases = [fileURLToPath(import.meta.url)]
  if (filePath) bases.push(resolve(filePath))
  bases.push(join(process.cwd(), "package.json"))
  for (const base of bases) {
    if (loaded.has(base)) return loaded.get(base)
    const found = tryLoad(createRequire(base))
    if (found) {
      loaded.set(base, found)
      return found
    }
  }
  throw new Error("vibelang/eslint needs typescript-eslint 8: install it beside eslint, as for TypeScript files.")
}

function tryLoad(require) {
  const attempt = (from) => {
    try {
      const scope = from.resolve("@typescript-eslint/scope-manager")
      const keys = from.resolve("@typescript-eslint/visitor-keys")
      return { analyze: from(scope).analyze, visitorKeys: from(keys).visitorKeys }
    } catch {
      return undefined
    }
  }
  const direct = attempt(require)
  if (direct) return direct
  for (const chain of [["@typescript-eslint/parser"], ["typescript-eslint", "@typescript-eslint/parser"]]) {
    let from = require
    try {
      for (const name of chain) from = createRequire(from.resolve(`${name}/package.json`))
    } catch {
      continue
    }
    const found = attempt(from)
    if (found) return found
  }
  return undefined
}

/**
 * Parses a .vibe file for ESLint.
 *
 * @param {string} code The file's text.
 * @param {{ filePath?: string, sourceType?: "module" | "script", lib?: string[] }} [options]
 */
export function parseForESLint(code, options = {}) {
  const filePath = options.filePath ?? "file.vibe"
  const result = spawnSync(nativeCompiler(), ["--estree", filePath], {
    input: code, encoding: "utf8", maxBuffer: 1024 * 1024 * 1024
  })
  if (result.error) throw new Error(`vibelang/eslint: cannot run the compiler: ${result.error.message}`)
  if (result.status !== 0) throw new Error(`vibelang/eslint: the compiler failed on ${filePath}: ${result.stderr.trim()}`)
  const { ast, error } = JSON.parse(result.stdout)
  if (error) {
    const failure = new SyntaxError(error.message)
    Object.assign(failure, { index: error.index, lineNumber: error.lineNumber, column: error.column })
    throw failure
  }
  finish(ast, code)
  ast.sourceType = options.sourceType ?? "module"
  const { analyze, visitorKeys: typescriptKeys } = typescriptEslint(options.filePath)
  const visitorKeys = { ...typescriptKeys, ...vibeVisitorKeys }
  const scopeManager = analyze(ast, {
    childVisitorKeys: visitorKeys,
    globalReturn: false,
    jsxPragma: null,
    lib: options.lib ?? ["esnext"],
    sourceType: ast.sourceType
  })
  hideSynthesized(scopeManager)
  // As @typescript-eslint/parser reports for a file outside any TypeScript
  // program: a rule that needs type information says so instead of crashing.
  const services = {
    emitDecoratorMetadata: undefined,
    experimentalDecorators: undefined,
    isolatedDeclarations: undefined,
    program: null,
    esTreeNodeToTSNodeMap: new WeakMap(),
    tsNodeToESTreeNodeMap: new WeakMap()
  }
  return { ast, scopeManager, services, visitorKeys }
}

// hideSynthesized removes the variables of names the parser wrote, such as a
// match's $match1 or a catch clause's $caught, so no rule reports a name the
// author never wrote. A reference to one resolves to nothing.
function hideSynthesized(scopeManager) {
  for (const scope of scopeManager.scopes) {
    const kept = []
    for (const variable of scope.variables) {
      if (variable.identifiers.length !== 0 && variable.identifiers.every((identifier) => identifier.synthesized === true)) {
        scope.set.delete(variable.name)
        for (const reference of variable.references) reference.resolved = null
      } else {
        kept.push(variable)
      }
    }
    scope.variables.splice(0, scope.variables.length, ...kept)
  }
}

/** Parses a .vibe file to its ESTree Program. */
export function parse(code, options) {
  return parseForESLint(code, options).ast
}

// finish adds what JSON cannot carry: each node's loc, and the values of
// bigint, regular expression, and non-finite number literals.
function finish(ast, code) {
  const starts = [0]
  for (const match of code.matchAll(/\r\n|[\r\n\u2028\u2029]/gu)) starts.push(match.index + match[0].length)
  const position = (offset) => {
    let low = 0
    let high = starts.length - 1
    while (low < high) {
      const middle = (low + high + 1) >> 1
      if (starts[middle] <= offset) low = middle
      else high = middle - 1
    }
    return { line: low + 1, column: offset - starts[low] }
  }
  const locate = (item) => {
    item.loc = { start: position(item.range[0]), end: position(item.range[1]) }
  }
  const visit = (node) => {
    if (Array.isArray(node)) {
      for (const item of node) if (item !== null && typeof item === "object") visit(item)
      return
    }
    if (typeof node.type !== "string") {
      for (const value of Object.values(node)) if (value !== null && typeof value === "object") visit(value)
      return
    }
    locate(node)
    if (node.type === "Literal") literal(node)
    for (const [key, value] of Object.entries(node)) {
      if (key === "range" || key === "loc" || value === null || typeof value !== "object") continue
      visit(value)
    }
  }
  visit(ast)
  for (const token of ast.tokens) locate(token)
  for (const comment of ast.comments) locate(comment)
}

function literal(node) {
  if (node.regex) {
    try {
      node.value = new RegExp(node.regex.pattern, node.regex.flags)
    } catch {
      node.value = null
    }
  } else if (typeof node.bigint === "string") {
    node.value = BigInt(node.bigint)
    node.bigint = String(node.value)
  } else if (node.value === null && node.raw !== "null") {
    node.value = Number(node.raw.replaceAll("_", ""))
  }
}

export default { meta, parse, parseForESLint }
