// Copy TypeScript 5.9.3 into prototype/single-pass/typescript and patch its checker.
//
//   bun prototype/single-pass/patch/apply.ts
//
// Writes typescript/lib/typescript.js (patched) and patch/typescript.patch (a unified diff of
// lib/typescript.js against the original). Every edit is anchored on exact text; a missing or
// ambiguous anchor stops the script.
import { execFileSync } from "node:child_process"
import fs from "node:fs"
import path from "node:path"

const here = import.meta.dir
const root = path.resolve(here, "..")
const source = path.resolve(root, "../../experiments/node_modules/typescript")
const target = path.join(root, "typescript")

const version = JSON.parse(fs.readFileSync(path.join(source, "package.json"), "utf8")).version
if (version !== "5.9.3") throw new Error(`expected typescript 5.9.3, found ${version}`)

fs.rmSync(target, { recursive: true, force: true })
fs.cpSync(source, target, { recursive: true })

const file = path.join(target, "lib/typescript.js")
const original = fs.readFileSync(path.join(source, "lib/typescript.js"), "utf8")
let text = original
const fragment = fs.readFileSync(path.join(here, "es-checker.js"), "utf8")

const edit = (name: string, anchor: string, replacement: string) => {
  const first = text.indexOf(anchor)
  if (first < 0) throw new Error(`anchor not found: ${name}`)
  if (text.indexOf(anchor, first + 1) >= 0) throw new Error(`anchor not unique: ${name}`)
  text = text.slice(0, first) + replacement + text.slice(first + anchor.length)
}

// 5b. The signature a call's arguments are being checked against, including during error reporting:
//     the two functions are renamed and the fragment defines wrappers that note the signature.
edit("inferTypeArguments", "  function inferTypeArguments(node, signature, args, checkMode, context) {\n",
  "  function esInferTypeArgumentsWorker(node, signature, args, checkMode, context) {\n")
edit("getSignatureApplicabilityError",
  "  function getSignatureApplicabilityError(node, args, signature, relation, checkMode, reportErrors2, containingMessageChain) {\n",
  "  function esGetSignatureApplicabilityErrorWorker(node, args, signature, relation, checkMode, reportErrors2, containingMessageChain) {\n")

// 1. The experiment's state and functions, inside createTypeChecker.
edit("state", "function createTypeChecker(host) {\n  var deferredDiagnosticsCallbacks = [];\n",
  `function createTypeChecker(host) {\n  var deferredDiagnosticsCallbacks = [];\n${fragment}`)

// 2. Intrinsic calls are typed by the experiment.
edit("checkCallExpression", "  function checkCallExpression(node, checkMode) {\n    var _a, _b, _c;\n",
  "  function checkCallExpression(node, checkMode) {\n    var _a, _b, _c;\n    const esType = esCheckCall(node, checkMode);\n    if (esType !== void 0) return esType;\n")

// 3. A variant takes the contextual type of the dual it belongs to (and so on for each intrinsic's operand).
edit("getContextualType2",
  "    if (index >= 0) {\n      return contextualTypes[index];\n    }\n    const { parent: parent2 } = node;\n    switch (parent2.kind) {\n      case 261 /* VariableDeclaration */:",
  "    if (index >= 0) {\n      return contextualTypes[index];\n    }\n    if (esContextualOperand(node)) return getContextualType2(node.parent, contextFlags);\n    const { parent: parent2 } = node;\n    switch (parent2.kind) {\n      case 261 /* VariableDeclaration */:")

// 4. A dual is context sensitive when a variant is, so it is skipped in the first round like a lambda.
edit("isContextSensitive",
  "  function isContextSensitive(node) {\n    Debug.assert(node.kind !== 175 /* MethodDeclaration */ || isObjectLiteralMethod(node));\n",
  "  function isContextSensitive(node) {\n    Debug.assert(node.kind !== 175 /* MethodDeclaration */ || isObjectLiteralMethod(node));\n    if (esIsDual(node)) return esDualContextSensitive(node);\n")

// 5. chooseOverload: each candidate is noted before its arguments are checked; the winner after.
edit("chooseOverload single",
  "      if (isSingleNonGenericCandidate2) {\n        const candidate = candidates2[0];\n",
  "      if (isSingleNonGenericCandidate2) {\n        const candidate = candidates2[0];\n        esNoteCandidate(node, candidates2, candidate);\n")
edit("chooseOverload single winner",
  "          candidatesForArgumentError = [candidate];\n          return void 0;\n        }\n        return candidate;\n      }\n      for (let candidateIndex = 0;",
  "          candidatesForArgumentError = [candidate];\n          return void 0;\n        }\n        esNoteWinner(node, candidate);\n        return candidate;\n      }\n      for (let candidateIndex = 0;")
edit("chooseOverload loop",
  "        const candidate = candidates2[candidateIndex];\n        if (!hasCorrectTypeArgumentArity(candidate, typeArguments) || !hasCorrectArity(node, args, candidate, signatureHelpTrailingComma2)) {\n          continue;\n        }\n",
  "        const candidate = candidates2[candidateIndex];\n        if (!hasCorrectTypeArgumentArity(candidate, typeArguments) || !hasCorrectArity(node, args, candidate, signatureHelpTrailingComma2)) {\n          continue;\n        }\n        esNoteCandidate(node, candidates2, candidate);\n")
edit("chooseOverload winner",
  "        candidates2[candidateIndex] = checkCandidate;\n        return checkCandidate;\n",
  "        candidates2[candidateIndex] = checkCandidate;\n        esNoteWinner(node, candidate);\n        return checkCandidate;\n")
// 7. Narrowing sees through the intrinsics: a type guard in `if (Exit.isFailure(x))` narrows x
//    although the text is `(yield* $esBind(... .isFailure($esConv(x, ...))))`. The Go fork has no
//    wrappers; without this, narrowing would differ from the final text for an artificial reason.
edit("narrowType", "    function narrowType(type, expr, assumeTrue) {\n",
  "    function narrowType(type, expr, assumeTrue) {\n      const esInner = esUnwrapRun(expr);\n      if (esInner) return narrowType(type, esInner, assumeTrue);\n")
edit("getTypePredicateArgument",
  "      return callExpression.arguments[predicate.parameterIndex];\n    }\n    const invokedExpression = skipParentheses(callExpression.expression);\n    return isAccessExpression(invokedExpression) ? skipParentheses(invokedExpression.expression) : void 0;",
  "      return esUnwrapConv(callExpression.arguments[predicate.parameterIndex]);\n    }\n    const invokedExpression = skipParentheses(callExpression.expression);\n    return isAccessExpression(invokedExpression) ? esUnwrapSvc(skipParentheses(invokedExpression.expression)) : void 0;")

edit("hasMatchingArgument",
  "  function hasMatchingArgument(expression, reference) {\n    if (expression.arguments) {\n      for (const argument of expression.arguments) {\n",
  "  function hasMatchingArgument(expression, reference) {\n    if (expression.arguments) {\n      for (const argument0 of expression.arguments) {\n        const argument = esUnwrapConv(argument0);\n")

// 7b. The binder makes a flow condition only for a narrowing expression; let it see through
//     `yield* $esBind(call)` and `$esConv(reference)` and `(yield* $esSvc(receiver, ...))`.
edit("binder isNarrowingExpression", "  function isNarrowingExpression(expr) {\n    switch (expr.kind) {\n",
  "  function isNarrowingExpression(expr) {\n    if (expr.kind === 230 /* YieldExpression */ && expr.asteriskToken && expr.expression && expr.expression.kind === 214 /* CallExpression */ && expr.expression.expression.kind === 80 /* Identifier */ && expr.expression.expression.escapedText === \"$esBind\") return isNarrowingExpression(expr.expression.arguments[0]);\n    switch (expr.kind) {\n")
edit("binder hasNarrowableArgument",
  "      for (const argument of expr.arguments) {\n        if (containsNarrowableReference(argument)) {\n          return true;\n        }\n      }\n    }\n    if (expr.expression.kind === 212 /* PropertyAccessExpression */ && containsNarrowableReference(expr.expression.expression)) {",
  "      for (const argument0 of expr.arguments) {\n        const argument = argument0.kind === 214 /* CallExpression */ && argument0.expression.kind === 80 /* Identifier */ && argument0.expression.escapedText === \"$esConv\" ? argument0.arguments[0] : argument0;\n        if (containsNarrowableReference(argument)) {\n          return true;\n        }\n      }\n    }\n    if (expr.expression.kind === 212 /* PropertyAccessExpression */ && containsNarrowableReference(expr.expression.expression)) {")

// 6. Expose the decisions.
edit("checker object", "  const checker = {\n    getNodeCount:", "  const checker = {\n    __es: es,\n    getNodeCount:")

fs.writeFileSync(file, text)
const orig = path.join(root, "patch/.typescript.orig.js")
fs.writeFileSync(orig, original)
let diff = ""
try {
  execFileSync("diff", ["-u", "--label", "a/lib/typescript.js", "--label", "b/lib/typescript.js", orig, file], { encoding: "utf8" })
} catch (e: any) {
  diff = e.stdout as string
}
fs.rmSync(orig)
fs.writeFileSync(path.join(root, "patch/typescript.patch"), diff)
console.log(`patched ${path.relative(root, file)}; ${diff.split("\n").filter((l) => l.startsWith("+") && !l.startsWith("+++")).length} added lines in patch/typescript.patch`)
