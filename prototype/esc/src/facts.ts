// Facts: what the type checker says about the previous pass's output.
//
// Each probe the emitter recorded maps one `.es` node to a span of generated
// TypeScript. Here we find the node at that span in the checked program and
// ask the checker the questions the next pass's lowering depends on.
import ts from "typescript"
import { containsShallow, type Emitted, isClause } from "./emit.ts"
import {
  type CallFact,
  type ConvFact,
  emptyFacts,
  type EsFile,
  type Facts,
  type LambdaContext,
  type LambdaFact,
  type Origin,
  type Span
} from "./model.ts"

export interface Unit {
  file: EsFile
  emitted: Emitted
  genPath: string
}

const EFFECT_ID = "~effect/Effect"
const SERVICE_ID = "~effect/Context/Service"
const PRODUCER_ID = "~esc/Producer"

export type Tri = "yes" | "no" | "unknown"

export const isEffectType = (t: ts.Type, checker: ts.TypeChecker, depth = 0): Tri => {
  if (depth > 5) return "unknown"
  if (t.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown)) return "unknown"
  if (t.flags & ts.TypeFlags.Never) return "no"
  if (t.isUnion()) {
    const parts = t.types.map((x) => isEffectType(x, checker, depth + 1))
    if (parts.every((p) => p === "yes")) return "yes"
    if (parts.every((p) => p === "no")) return "no"
    return "unknown"
  }
  if (t.flags & (ts.TypeFlags.TypeParameter | ts.TypeFlags.Conditional | ts.TypeFlags.IndexedAccess | ts.TypeFlags.Substitution)) {
    const c = checker.getBaseConstraintOfType(t)
    if (!c || c === t) return "unknown"
    return isEffectType(c, checker, depth + 1) === "yes" ? "yes" : "unknown"
  }
  return t.getProperty(EFFECT_ID) ? "yes" : "no"
}

export const isKeyType = (t: ts.Type) => t.getProperty(SERVICE_ID) !== undefined

/**
 * D24: a call runs its result only when the result's type is `Effect<A, E, R>`
 * itself. A subtype with structure of its own (Config, Exit, a service key, a
 * TaggedError instance) is data when a call returns it.
 */
export const isEffectItself = (t: ts.Type, checker: ts.TypeChecker, depth = 0): Tri => {
  if (depth > 5) return "unknown"
  if (t.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown)) return "unknown"
  if (t.flags & ts.TypeFlags.Never) return "no"
  if (t.isUnion()) {
    const parts = t.types.map((x) => isEffectItself(x, checker, depth + 1))
    if (parts.every((p) => p === "yes")) return "yes"
    if (parts.every((p) => p === "no")) return "no"
    return "unknown"
  }
  if (t.flags & (ts.TypeFlags.TypeParameter | ts.TypeFlags.Conditional | ts.TypeFlags.IndexedAccess | ts.TypeFlags.Substitution)) {
    const c = checker.getBaseConstraintOfType(t)
    if (!c || c === t) return "unknown"
    return isEffectItself(c, checker, depth + 1) === "yes" ? "yes" : "unknown"
  }
  if (!t.getProperty(EFFECT_ID)) return "no"
  const sym = t.getSymbol()
  const isEffectInterface = sym?.name === "Effect" &&
    (sym.declarations ?? []).some((d) => /node_modules\/effect\//.test(d.getSourceFile().fileName))
  return isEffectInterface ? "yes" : "no"
}

export interface EffectParts {
  A: ts.Type
  E: ts.Type
  R: ts.Type
}

/** A, E, R of an Effect type, read from its `Variance<A, E, R>` marker. */
export const effectParts = (t: ts.Type, checker: ts.TypeChecker): EffectParts | undefined => {
  const p = t.getProperty(EFFECT_ID)
  if (!p) return undefined
  const variance = checker.getTypeOfSymbol(p)
  const get = (name: string) => {
    const q = variance.getProperty(name)
    if (!q) return undefined
    const sig = checker.getTypeOfSymbol(q).getCallSignatures()[0]
    return sig?.getReturnType()
  }
  const A = get("_A")
  const E = get("_E")
  const R = get("_R")
  return A && E && R ? { A, E, R } : undefined
}

const LAYER_ID = "~effect/Layer"

/** ROut, E, RIn of a Layer type, read from its variance marker. */
export const layerParts = (t: ts.Type, checker: ts.TypeChecker): { Out: ts.Type; E: ts.Type; In: ts.Type } | undefined => {
  const p = t.getProperty(LAYER_ID)
  if (!p) return undefined
  const v = checker.getTypeOfSymbol(p)
  const sig = (name: string) => {
    const q = v.getProperty(name)
    return q ? checker.getTypeOfSymbol(q).getCallSignatures()[0] : undefined
  }
  const outSig = sig("_ROut")
  const Out = outSig?.getParameters()[0] ? checker.getTypeOfSymbol(outSig.getParameters()[0]) : undefined
  const E = sig("_E")?.getReturnType()
  const In = sig("_RIn")?.getReturnType()
  return Out && E && In ? { Out, E, In } : undefined
}

export const typeParts = (t: ts.Type): ts.Type[] => (t.isUnion() ? t.types : t.flags & ts.TypeFlags.Never ? [] : [t])

export interface WireGraph {
  args: Array<{ text: string; out: ts.Type[]; ins: ts.Type[]; opaque: boolean }>
  /** Argument order for the fold-right runtime: consumers before their providers. */
  order: number[]
  cycle?: number[]
  duplicates: Array<{ service: ts.Type; args: number[] }>
}

/** D27: the provider graph of a `wire(...)` call, from its arguments' Layer types. */
export const wireGraph = (call: ts.CallExpression, checker: ts.TypeChecker): WireGraph => {
  const args = call.arguments.map((a) => {
    const lp = layerParts(checker.getTypeAtLocation(a), checker)
    const opaque = !lp || (lp.Out.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown | ts.TypeFlags.Never)) !== 0
    return { text: a.getText(), out: lp && !opaque ? typeParts(lp.Out) : [], ins: lp ? typeParts(lp.In) : [], opaque }
  })
  const provides = (j: number, t: ts.Type) => args[j].out.some((o) => o === t)
  const needs = args.map((a, i) => args.map((_, j) => j).filter((j) => j !== i && a.ins.some((t) => provides(j, t))))
  const duplicates: WireGraph["duplicates"] = []
  const seen = new Map<ts.Type, number[]>()
  args.forEach((a, i) => a.out.forEach((t) => seen.set(t, [...(seen.get(t) ?? []), i])))
  for (const [service, idxs] of seen) if (idxs.length > 1) duplicates.push({ service, args: idxs })
  // Cycle detection.
  const state = new Map<number, "visiting" | "done">()
  let cycle: number[] | undefined
  const path: number[] = []
  const visit = (i: number) => {
    if (state.get(i) === "done" || cycle) return
    if (state.get(i) === "visiting") {
      cycle = [...path.slice(path.indexOf(i)), i]
      return
    }
    state.set(i, "visiting")
    path.push(i)
    for (const j of needs[i]) visit(j)
    path.pop()
    state.set(i, "done")
  }
  args.forEach((_, i) => visit(i))
  // Stable topological order (Kahn, smallest index first): a consumer comes before its providers.
  const indeg = args.map(() => 0)
  needs.forEach((js) => js.forEach((j) => indeg[j]++))
  const order: number[] = []
  const ready = args.map((_, i) => i).filter((i) => indeg[i] === 0)
  while (ready.length > 0) {
    ready.sort((a, b) => a - b)
    const i = ready.shift()!
    order.push(i)
    for (const j of needs[i]) if (--indeg[j] === 0) ready.push(j)
  }
  return { args, order: cycle || order.length !== args.length ? args.map((_, i) => i) : order, cycle, duplicates }
}

/** Find generated nodes by exact span. */
export class NodeIndex {
  private readonly byKey = new Map<string, ts.Node[]>()
  constructor(readonly sf: ts.SourceFile) {
    const visit = (n: ts.Node) => {
      const k = `${n.getStart(sf)}:${n.end}`
      const list = this.byKey.get(k)
      if (list) list.push(n)
      else this.byKey.set(k, [n])
      ts.forEachChild(n, visit)
    }
    ts.forEachChild(sf, visit)
  }
  find(span: Span | undefined, ...kinds: ts.SyntaxKind[]): ts.Node | undefined {
    if (!span) return undefined
    const list = this.byKey.get(`${span.start}:${span.end}`)
    if (!list) return undefined
    if (kinds.length === 0) return list[list.length - 1]
    for (let i = list.length - 1; i >= 0; i--) if (kinds.includes(list[i].kind)) return list[i]
    return undefined
  }
}

const PLATFORM_GLOBALS = new Set([
  "setTimeout", "setInterval", "clearTimeout", "clearInterval", "queueMicrotask", "fetch", "console", "process",
  "localStorage", "sessionStorage", "document", "window", "navigator", "XMLHttpRequest", "WebSocket", "require"
])

/** D5: where a plain callee comes from. */
export const calleeOrigin = (call: ts.CallExpression, checker: ts.TypeChecker, generated: Set<string>): Origin => {
  const sig = checker.getResolvedSignature(call)
  let decl: ts.Node | undefined = sig?.declaration
  if (!decl) {
    const sym = checker.getSymbolAtLocation(call.expression)
    decl = sym?.declarations?.[0]
  }
  if (!decl) return "unknown"
  const file = decl.getSourceFile().fileName
  if (generated.has(file)) return "es"
  // D23: `/** @effectscript trusted */` on a TypeScript declaration.
  if (ts.getJSDocTags(decl).some((t) => t.tagName.text === "effectscript" && /trusted/.test(String(t.comment ?? "")))) return "lib"
  const text = call.expression.getText()
  if (/^(Date\.now|Math\.random|crypto\.randomUUID|performance\.now)$/.test(text)) return "clock"
  if (/\/typescript\/lib\/lib\.dom/.test(file) || /\/@types\/node\//.test(file) || /bun-types/.test(file)) return "platform"
  if (/\/typescript\/lib\/lib\./.test(file)) {
    const root = text.split(/[.(]/)[0]
    return PLATFORM_GLOBALS.has(root) ? "platform" : "lib"
  }
  if (/\/node_modules\/(effect|@effect\/[^/]+)\//.test(file)) return "effect"
  if (/\/packages\/runtime\/src\//.test(file)) return "effect"
  return "ts"
}

/** D23: `@throws {T}` tags on a trusted (`@effectscript trusted`) TypeScript declaration. */
const trustedThrows = (call: ts.CallExpression, checker: ts.TypeChecker): Array<{ name: string; file: string }> | undefined => {
  const decls: ts.Node[] = []
  const d = checker.getResolvedSignature(call)?.declaration
  if (d && !ts.isJSDocSignature(d)) decls.push(d)
  const sym = checker.getSymbolAtLocation(call.expression)
  const target = sym && sym.flags & ts.SymbolFlags.Alias ? checker.getAliasedSymbol(sym) : sym
  for (const x of target?.declarations ?? []) decls.push(x)
  for (const decl of decls) {
    const tags = ts.getJSDocTags(decl)
    if (!tags.some((t) => t.tagName.text === "effectscript" && /trusted/.test(String(t.comment ?? "")))) continue
    const out = tags
      .filter((t): t is ts.JSDocThrowsTag => t.tagName.text === "throws")
      .map((t) => ({ name: t.typeExpression?.type.getText() ?? "", file: decl.getSourceFile().fileName }))
      .filter((t) => t.name !== "")
    return out.length > 0 ? out : undefined
  }
  return undefined
}

const unwrapSubstitution = (t: ts.Type): ts.Type =>
  t.flags & ts.TypeFlags.Substitution ? unwrapSubstitution((t as any).baseType as ts.Type) : t

/** Classify the type expected at a lambda's or value's position. */
export const classifyExpected = (types: readonly ts.Type[], checker: ts.TypeChecker): LambdaContext => {
  let program = false
  let effectFn = false
  let plainFn = false
  const visit = (t0: ts.Type, depth: number) => {
    const t = unwrapSubstitution(t0)
    if (depth > 4) return
    if (t.isUnion()) return t.types.forEach((x) => visit(x, depth + 1))
    if (t.flags & ts.TypeFlags.TypeParameter) {
      const c = checker.getBaseConstraintOfType(t)
      if (c && c !== t && !(c.flags & ts.TypeFlags.Unknown)) visit(c, depth + 1)
      return
    }
    const sigs = t.getCallSignatures()
    if (sigs.length === 0) {
      if (isEffectType(t, checker) === "yes") program = true
      return
    }
    for (const s of sigs) {
      const r = unwrapSubstitution(s.getReturnType())
      if (isEffectType(r, checker) === "yes") effectFn = true
      else plainFn = true
    }
  }
  types.forEach((t) => visit(t, 0))
  return program ? "program" : effectFn ? "effectFn" : plainFn ? "plainFn" : "none"
}

const isBareTypeParam = (t: ts.Type, checker: ts.TypeChecker) => {
  const u = unwrapSubstitution(t)
  if (!(u.flags & ts.TypeFlags.TypeParameter)) return false
  const c = checker.getBaseConstraintOfType(u)
  return !c || c === u || (c.flags & ts.TypeFlags.Unknown) !== 0
}

const requiredCount = (decl: ts.SignatureDeclaration) =>
  decl.parameters.filter((p) => !p.questionToken && !p.initializer && !p.dotDotDotToken).length

/** Declared (uninstantiated) parameter types at `index` of the call's resolved signature or arity-compatible candidates. */
const declaredParamTypes = (call: ts.CallExpression, index: number, checker: ts.TypeChecker): ts.Type[] => {
  const candidates: ts.Signature[] = []
  const resolved = checker.getResolvedSignature(call, candidates)
  const decls: ts.SignatureDeclaration[] = []
  const add = (d: ts.SignatureDeclaration | ts.JSDocSignature | undefined) => {
    if (d && !ts.isJSDocSignature(d) && !decls.includes(d)) decls.push(d)
  }
  add(resolved?.declaration)
  const typesFrom = (d: ts.SignatureDeclaration) => {
    const sig = checker.getSignatureFromDeclaration(d)
    if (!sig) return undefined
    const ps = sig.getParameters()
    const last = d.parameters[d.parameters.length - 1]
    const rest = last?.dotDotDotToken !== undefined
    if (index >= ps.length && !rest) return undefined
    const p = index < ps.length - (rest ? 1 : 0) ? ps[index] : ps[ps.length - 1]
    let t = checker.getTypeOfSymbol(p)
    if (rest && index >= ps.length - 1) {
      const n = checker.getIndexInfosOfType(t).find((i) => i.keyType.flags & ts.TypeFlags.Number)
      if (n) t = n.type
    }
    return t
  }
  const primary = decls.length > 0 ? typesFrom(decls[0]) : undefined
  if (primary && !isBareTypeParam(primary, checker) && classifyExpected([primary], checker) !== "none") return [primary]
  const out: ts.Type[] = primary ? [primary] : []
  for (const c of candidates) {
    const d = c.declaration
    if (!d || ts.isJSDocSignature(d)) continue
    if (requiredCount(d) > call.arguments.length) continue
    const hasRest = d.parameters.some((p) => p.dotDotDotToken)
    if (!hasRest && d.parameters.length < call.arguments.length) continue
    const t = typesFrom(d)
    if (t) out.push(t)
  }
  return out
}

const elementTypes = (t: ts.Type, checker: ts.TypeChecker): ts.Type[] => {
  const u = unwrapSubstitution(t)
  if (u.isUnion()) return u.types.flatMap((x) => elementTypes(x, checker))
  if (u.flags & ts.TypeFlags.TypeParameter) {
    const c = checker.getBaseConstraintOfType(u)
    return c && c !== u ? elementTypes(c, checker) : []
  }
  const out: ts.Type[] = []
  for (const info of checker.getIndexInfosOfType(u)) out.push(info.type)
  const ref = u as ts.TypeReference
  if (out.length === 0 && (ref.target !== undefined || u.symbol?.name === "Iterable")) {
    const args = ref.target ? checker.getTypeArguments(ref) : []
    if (args.length > 0) out.push(args[0])
  }
  return out
}

export const propertyTypes = (t: ts.Type, name: string, checker: ts.TypeChecker): ts.Type[] => {
  const u = unwrapSubstitution(t)
  if (u.isUnion()) return u.types.flatMap((x) => propertyTypes(x, name, checker))
  if (u.flags & ts.TypeFlags.TypeParameter) {
    const c = checker.getBaseConstraintOfType(u)
    return c && c !== u ? propertyTypes(c, name, checker) : []
  }
  const p = u.getProperty(name)
  if (p) return [checker.getTypeOfSymbol(p)]
  const s = checker.getIndexInfosOfType(u).find((i) => i.keyType.flags & ts.TypeFlags.String)
  return s ? [s.type] : []
}

/** The types expected at an expression's position, uninstantiated where the position is a call argument. */
export const expectedTypes = (node: ts.Expression, checker: ts.TypeChecker): ts.Type[] => {
  const parent = node.parent
  const fallback = () => {
    const c = checker.getContextualType(node)
    return c ? [c] : []
  }
  if (ts.isParenthesizedExpression(parent)) return expectedTypes(parent, checker)
  // A value returned from the generator of an `Effect.gen` program: the program's expected A.
  if (ts.isReturnStatement(parent)) {
    let fn: ts.Node | undefined = parent.parent
    while (fn && !ts.isFunctionLike(fn)) fn = fn.parent
    const gen = fn?.parent
    if (fn && ts.isFunctionExpression(fn) && fn.asteriskToken && gen && ts.isCallExpression(gen) && gen.expression.getText() === "Effect.gen") {
      const ctx = checker.getContextualType(gen)
      const as = (ctx ? (ctx.isUnion() ? ctx.types : [ctx]) : [])
        .map((t) => effectParts(t, checker)?.A)
        .filter((t): t is ts.Type => t !== undefined && !(t.flags & (ts.TypeFlags.Any | ts.TypeFlags.Unknown)))
      if (as.length > 0) return as
    }
  }
  if (ts.isCallExpression(parent) && parent.arguments.includes(node)) {
    const declared = declaredParamTypes(parent, parent.arguments.indexOf(node), checker)
    const useful = declared.filter((t) => !isBareTypeParam(t, checker))
    if (useful.length > 0 && classifyExpected(useful, checker) !== "none") return useful
    return fallback()
  }
  if (ts.isArrayLiteralExpression(parent)) {
    const outer = expectedTypes(parent, checker).flatMap((t) => elementTypes(t, checker))
    if (outer.length > 0 && classifyExpected(outer, checker) !== "none") return outer
    return fallback()
  }
  if (ts.isPropertyAssignment(parent) && ts.isObjectLiteralExpression(parent.parent)) {
    const name = parent.name.getText()
    const outer = expectedTypes(parent.parent, checker).flatMap((t) => propertyTypes(t, name, checker))
    if (outer.length > 0 && classifyExpected(outer, checker) !== "none") return outer
    return fallback()
  }
  return fallback()
}

const isZeroArgProgramFn = (t: ts.Type, checker: ts.TypeChecker) =>
  t.getCallSignatures().some((s) => {
    const d = s.declaration
    const required = d && !ts.isJSDocSignature(d) ? requiredCount(d) : s.getParameters().length
    return required === 0 && isEffectType(s.getReturnType(), checker) === "yes"
  })

/** Resolve an identifier in generated code to a top-level `.es` function. */
const resolveEsFunction = (
  id: ts.Expression,
  checker: ts.TypeChecker,
  byGen: Map<string, Unit>
): { file: string; name: string } | undefined => {
  if (!ts.isIdentifier(id)) return undefined
  let sym = checker.getSymbolAtLocation(id)
  if (!sym) return undefined
  if (sym.flags & ts.SymbolFlags.Alias) sym = checker.getAliasedSymbol(sym)
  const decl = sym.declarations?.[0]
  if (!decl) return undefined
  const unit = byGen.get(decl.getSourceFile().fileName)
  if (!unit) return undefined
  const name = sym.name.replace(/\$direct$/, "")
  const fn = unit.file.functions.get(name)
  if (!fn) return undefined
  // Only the top-level declaration itself (not a shadowing local).
  const top = decl.parent?.parent?.parent
  if (!(ts.isFunctionDeclaration(decl) || (top && ts.isSourceFile(top)) || ts.isSourceFile(decl.parent))) return undefined
  return { file: unit.file.path, name }
}

export function collectFacts(program: ts.Program, units: Unit[]): Facts {
  const checker = program.getTypeChecker()
  const facts = emptyFacts()
  const byGen = new Map(units.map((u) => [u.genPath, u] as const))
  const generated = new Set(units.map((u) => u.genPath))
  const indexes = new Map<Unit, NodeIndex>()
  for (const u of units) {
    const sf = program.getSourceFile(u.genPath)
    if (sf) indexes.set(u, new NodeIndex(sf))
  }

  // 1. Calls.
  for (const u of units) {
    const idx = indexes.get(u)
    if (!idx) continue
    for (const [call] of u.emitted.callInfo) {
      const calleeNode = idx.find(u.emitted.calleeProbes.get(call)) as ts.Expression | undefined
      const callNode = idx.find(u.emitted.callProbes.get(call), ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
      const fact: CallFact = {
        type: "unknown",
        calleeEffectValue: false,
        serviceReceiver: false,
        origin: "unknown",
        calleeText: call.expression.getText(u.file.sf)
      }
      if (ts.isPropertyAccessExpression(call.expression)) {
        const recv = idx.find(u.emitted.receiverProbes.get(call)) as ts.Expression | undefined
        if (recv) {
          // A member the service's shape declares wins over the key's own members (pipe, key, call, name, …).
          const rt = checker.getTypeAtLocation(recv)
          const shape = isKeyType(rt) ? rt.getProperty("Service") : undefined
          const member = call.expression.name.text
          if (shape && checker.getTypeOfSymbol(shape).getProperty(member)) fact.serviceReceiver = true
          else if (isKeyType(rt) && !rt.getProperty(member) && !checker.getApparentType(rt).getProperty(member)) fact.serviceReceiver = true
        }
      }
      if (calleeNode && ts.isExpression(calleeNode)) {
        const ct = checker.getTypeAtLocation(calleeNode)
        if (ct.getCallSignatures().length === 0 && isEffectType(ct, checker) === "yes" && call.arguments.length === 0) {
          fact.calleeEffectValue = true
          fact.type = "effect"
          const parts = effectParts(ct, checker)
          fact.effectE = parts?.E
          fact.effectR = parts?.R
        }
      }
      if (!fact.calleeEffectValue && callNode) {
        const t = checker.getTypeAtLocation(callNode)
        const tri = isEffectItself(t, checker)
        fact.type = tri === "yes" ? "effect" : tri === "no" ? "plain" : "unknown"
        if (fact.type === "effect") {
          const parts = effectParts(t, checker)
          fact.effectE = parts?.E
          fact.effectR = parts?.R
        }
        fact.origin = calleeOrigin(callNode, checker, generated)
        if (fact.type === "plain") fact.throws = trustedThrows(callNode, checker)
        fact.esCallee = resolveEsFunction(callNode.expression, checker, byGen)
      }
      facts.calls.set(call, fact)
    }
  }

  // 2. Lambdas and object-literal methods.
  const needs = (fn: ts.Node, plainCtx: boolean): { needs: boolean; pauses?: string } => {
    const body = (fn as ts.FunctionLikeDeclaration).body
    if (!body) return { needs: false }
    let pauses: string | undefined
    const found = containsShallow(body, (n) => {
      if (ts.isCallExpression(n)) {
        const f = facts.calls.get(n)
        if (!f) return false
        if (f.type === "effect" || f.calleeEffectValue || f.serviceReceiver || (!plainCtx && f.throws)) {
          pauses = f.calleeText
          return true
        }
        return false
      }
      if (!plainCtx && (ts.isThrowStatement(n) || ts.isTryStatement(n))) return true
      if (ts.isVariableStatement(n) && (n.declarationList.flags & 7) === 4) return true
      return false
    })
    return { needs: found, pauses }
  }
  for (const u of units) {
    const idx = indexes.get(u)
    if (!idx) continue
    for (const [fn, span] of u.emitted.lambdaProbes) {
      const node = idx.find(span, ts.SyntaxKind.ArrowFunction, ts.SyntaxKind.FunctionExpression, ts.SyntaxKind.CallExpression) as
        | ts.Expression
        | undefined
      const types = node ? expectedTypes(node, checker) : []
      const ctx = classifyExpected(types, checker)
      const n = needs(fn, ctx === "plainFn")
      const lf: LambdaFact = { ctx, needsEffect: n.needs, pauses: n.pauses }
      if (ctx === "plainFn" && node) lf.ctxText = describeCallee(node)
      facts.lambdas.set(fn, lf)
    }
    for (const [obj, span] of u.emitted.objectProbes) {
      const node = idx.find(span, ts.SyntaxKind.ObjectLiteralExpression) as ts.Expression | undefined
      const outer = node ? expectedTypes(node, checker) : []
      for (const p of obj.properties) {
        if (!ts.isMethodDeclaration(p)) continue
        const types = outer.flatMap((t) => propertyTypes(t, p.name.getText(u.file.sf), checker))
        const ctx = classifyExpected(types, checker)
        const n = needs(p, ctx === "plainFn")
        facts.lambdas.set(p, { ctx, needsEffect: n.needs, pauses: n.pauses })
      }
    }
    // Methods of a class that implements an interface (a service, D1): the interface member's type decides.
    for (const [cls, span] of u.emitted.classProbes) {
      const node = idx.find(span, ts.SyntaxKind.ClassDeclaration) as ts.ClassDeclaration | undefined
      if (!node) continue
      const impl = (node.heritageClauses ?? []).filter((h) => h.token === ts.SyntaxKind.ImplementsKeyword).flatMap((h) => h.types)
      const types = impl.map((t) => checker.getTypeAtLocation(t))
      for (const m of cls.members) {
        if (!ts.isMethodDeclaration(m) || !m.body) continue
        const ptypes = types.flatMap((t) => propertyTypes(t, m.name.getText(u.file.sf), checker))
        const ctx = classifyExpected(ptypes, checker)
        const n = needs(m, ctx === "plainFn")
        facts.lambdas.set(m, { ctx, needsEffect: n.needs, pauses: n.pauses })
      }
    }
    // Class methods and nested function declarations: no expected type; decided by their bodies.
    const visit = (n: ts.Node) => {
      if ((ts.isMethodDeclaration(n) && ts.isClassLike(n.parent)) || (ts.isFunctionDeclaration(n) && !ts.isSourceFile(n.parent))) {
        if ((n as ts.FunctionLikeDeclaration).body && !facts.lambdas.has(n)) {
          const r = needs(n, false)
          facts.lambdas.set(n, { ctx: "none", needsEffect: r.needs, pauses: r.pauses })
        }
      }
      ts.forEachChild(n, visit)
    }
    visit(u.file.sf)
  }

  // 3. Producer loops.
  for (const u of units) {
    const idx = indexes.get(u)
    if (!idx) continue
    for (const [st, span] of u.emitted.forOfProbes) {
      const node = idx.find(span) as ts.Expression | undefined
      if (!node) continue
      const t = checker.getTypeAtLocation(node)
      if (t.getProperty(PRODUCER_ID)) facts.producerLoops.add(st)
      else if (t.getProperty("~effect/Stream")) facts.streamLoops.add(st)
    }
  }

  // 3b. wire(...) argument order from Layer types (D27).
  for (const u of units) {
    const idx = indexes.get(u)
    if (!idx) continue
    for (const [call] of u.emitted.callInfo) {
      if (!ts.isIdentifier(call.expression) || call.expression.text !== "wire" || u.file.topNames.has("wire")) continue
      const node = idx.find(u.emitted.callProbes.get(call), ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
      if (!node) continue
      // The generated call carries the previous order: compute in source argument indexes, then sort stably.
      const prev = u.emitted.wireOrders.get(call) ?? call.arguments.map((_, i) => i)
      const g = wireGraph(node, checker)
      const genOfSource = new Map(prev.map((src, k) => [src, k] as const))
      const sourceOrder = g.order.map((k) => prev[k])
      // Recompute the topological order over source indexes so ties keep source order.
      const needsSrc = new Map<number, number[]>()
      for (const src of prev) {
        const k = genOfSource.get(src)!
        const ins = g.args[k].ins
        needsSrc.set(src, prev.filter((o) => o !== src && g.args[genOfSource.get(o)!].out.some((t) => ins.includes(t))))
      }
      const indeg = new Map(prev.map((p) => [p, 0] as const))
      for (const js of needsSrc.values()) for (const j of js) indeg.set(j, indeg.get(j)! + 1)
      const ready = prev.filter((p) => indeg.get(p) === 0)
      const out: number[] = []
      while (ready.length > 0) {
        ready.sort((a, b) => a - b)
        const i = ready.shift()!
        out.push(i)
        for (const j of needsSrc.get(i) ?? []) {
          indeg.set(j, indeg.get(j)! - 1)
          if (indeg.get(j) === 0) ready.push(j)
        }
      }
      facts.wireOrders.set(call, out.length === prev.length && !g.cycle ? out : [...prev].sort((a, b) => a - b))
      void sourceOrder
    }
  }

  // 4. Plain JavaScript functions (D10).
  facts.direct = computeDirect(units, facts)

  // 4b. Recursive functions: read this pass's inferred contract for the next pass's forward binding.
  for (const u of units) {
    const sf = program.getSourceFile(u.genPath)
    if (!sf) continue
    for (const name of u.file.recursive) {
      const decl = findConst(sf, name)
      if (!decl?.initializer) continue
      const sig = checker.getTypeAtLocation(decl.initializer).getCallSignatures()[0]
      const parts = sig ? effectParts(sig.getReturnType(), checker) : undefined
      if (!parts) continue
      const print = (t: ts.Type) =>
        checker.typeToString(t, decl, ts.TypeFormatFlags.NoTruncation | ts.TypeFormatFlags.UseFullyQualifiedType)
      facts.recTypes.set(`${u.file.path}#${name}`, { A: print(parts.A), E: print(parts.E), R: print(parts.R) })
    }
  }

  // 5. Values where a program or a plain function is expected (D7, D9), and pipe (D8).
  for (const u of units) {
    const idx = indexes.get(u)
    if (!idx) continue
    for (const [expr, spans] of u.emitted.convProbes) {
      const exprNode = idx.find(spans.expr) as ts.Expression | undefined
      const siteNode = idx.find(spans.site) as ts.Expression | undefined
      if (!exprNode || !siteNode) continue
      const ctx = classifyExpected(expectedTypes(siteNode, checker), checker)
      const own = checker.getTypeAtLocation(exprNode)
      const conv: ConvFact = { program: false, direct: false, rejectPlain: false }
      if (ctx === "program" && isZeroArgProgramFn(own, checker)) conv.program = true
      if (own.getProperty(PRODUCER_ID) && expectedTypes(siteNode, checker).some((t) => t.getProperty("~effect/Stream"))) conv.stream = true
      if (ctx === "none" && !conv.program && !conv.stream) {
        // An array of programs where an iterable of Effects is expected (prototype extension).
        const expectedEl = expectedTypes(siteNode, checker).flatMap((t) => elementTypes(t, checker))
        const ownEl = elementTypes(own, checker)
        if (classifyExpected(expectedEl, checker) === "program" && ownEl.length > 0 && ownEl.every((t) => isZeroArgProgramFn(t, checker))) {
          conv.programs = true
        }
      }
      if (ctx === "plainFn") {
        const es = resolveEsFunction(exprNode, checker, byGen)
        if (es) {
          if (facts.direct.has(`${es.file}#${es.name}`)) conv.direct = true
          else if (!byGen.get(units.find((x) => x.file.path === es.file)!.genPath)?.file.functions.get(es.name)?.asteriskToken) {
            conv.rejectPlain = true
          }
        }
      }
      if (conv.program || conv.programs || conv.stream || conv.direct || conv.rejectPlain) facts.convs.set(expr, conv)
    }
    // pipe(program, stage, ...): the first argument is a program when the first stage expects an Effect.
    for (const [call] of u.emitted.callInfo) {
      if (!ts.isIdentifier(call.expression) || call.expression.text !== "pipe" || call.arguments.length < 2) continue
      if (u.file.topNames.has("pipe")) continue
      const first = call.arguments[0]
      const stageSpan = u.emitted.convProbes.get(call.arguments[1] as ts.Expression)?.expr
      let stage = idx.find(stageSpan) as ts.Expression | undefined
      if (!stage) {
        const callNode = idx.find(u.emitted.callProbes.get(call), ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
        stage = callNode?.arguments[1]
      }
      if (!stage) continue
      const st = checker.getTypeAtLocation(stage)
      const p0 = st.getCallSignatures()[0]?.getParameters()[0]
      if (!p0) continue
      if (isEffectType(checker.getTypeOfSymbol(p0), checker) !== "yes") continue
      if (ts.isArrowFunction(first) || ts.isFunctionExpression(first)) facts.pipePrograms.add(first)
      else {
        const prev = facts.convs.get(first) ?? { program: false, direct: false, rejectPlain: false }
        facts.convs.set(first, { ...prev, program: true })
      }
    }
  }
  return facts
}

const findConst = (sf: ts.SourceFile, name: string): ts.VariableDeclaration | undefined => {
  for (const st of sf.statements) {
    if (!ts.isVariableStatement(st)) continue
    for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name) && d.name.text === name) return d
  }
  return undefined
}

const describeCallee = (node: ts.Node): string | undefined => {
  let p: ts.Node | undefined = node.parent
  while (p && ts.isParenthesizedExpression(p)) p = p.parent
  if (p && ts.isCallExpression(p)) {
    const t = p.expression.getText()
    return t.length > 40 ? t.slice(0, 40) + "…" : t
  }
  return undefined
}

/** D10: a function compiles to plain JavaScript when it performs no Effect operation at all. */
export function computeDirect(units: Unit[], facts: Facts): Set<string> {
  interface Cand {
    ok: boolean
    deps: Set<string>
  }
  const cands = new Map<string, Cand>()
  const producers = new Set<string>()
  for (const u of units) {
    for (const [name, fn] of u.file.functions) if (fn.asteriskToken) producers.add(`${u.file.path}#${name}`)
  }
  for (const u of units) {
    for (const [name, fn] of u.file.functions) {
      if (!fn.body || fn.asteriskToken) continue
      if ((ts.getModifiers(fn) ?? []).some((m) => m.kind === ts.SyntaxKind.AsyncKeyword)) continue
      const c: Cand = { ok: true, deps: new Set() }
      const bad = containsShallow(fn.body, (n) => {
        if (ts.isThrowStatement(n) || ts.isTryStatement(n) || ts.isYieldExpression(n) || ts.isAwaitExpression(n)) return true
        if (ts.isVariableStatement(n) && (n.declarationList.flags & 7) >= 4) return true
        if (ts.isForOfStatement(n) && (facts.producerLoops.has(n) || facts.streamLoops.has(n))) return true
        if (ts.isCallExpression(n)) {
          const f = facts.calls.get(n)
          if (!f) return true
          if (f.esCallee) {
            const key = `${f.esCallee.file}#${f.esCallee.name}`
            if (!producers.has(key)) c.deps.add(key)
            return false
          }
          if (ts.isIdentifier(n.expression) && ["fork", "handle", "resume", "resumeWith", "abandon"].includes(n.expression.text)) return true
          return f.calleeEffectValue || f.serviceReceiver || f.type !== "plain" || f.throws !== undefined
        }
        return false
      })
      if (bad) c.ok = false
      cands.set(`${u.file.path}#${name}`, c)
    }
  }
  let changed = true
  while (changed) {
    changed = false
    for (const c of cands.values()) {
      if (c.ok && [...c.deps].some((d) => !cands.get(d)?.ok)) {
        c.ok = false
        changed = true
      }
    }
  }
  return new Set([...cands].filter(([, c]) => c.ok).map(([k]) => k))
}

/** A stable rendering of the decisions, to detect convergence between passes. */
export function fingerprint(facts: Facts, units: Unit[]): string {
  const pos = (n: ts.Node) => `${n.getSourceFile().fileName}:${n.pos}`
  const parts: string[] = []
  for (const [c, f] of facts.calls) parts.push(`c${pos(c)}=${f.type}/${f.calleeEffectValue}/${f.serviceReceiver}/${f.esCallee?.name ?? ""}`)
  for (const [l, f] of facts.lambdas) parts.push(`l${pos(l)}=${f.ctx}/${f.needsEffect}`)
  for (const [e, f] of facts.convs) parts.push(`v${pos(e)}=${f.program}/${f.programs}/${f.stream}/${f.direct}/${f.rejectPlain}`)
  for (const s of facts.producerLoops) parts.push(`p${pos(s)}`)
  for (const s of facts.streamLoops) parts.push(`s${pos(s)}`)
  for (const d of facts.direct) parts.push(`d${d}`)
  for (const p of facts.pipePrograms) parts.push(`i${pos(p)}`)
  for (const [k, t] of facts.recTypes) parts.push(`r${k}=${t.A}/${t.E}/${t.R}`)
  for (const [c, o] of facts.wireOrders) parts.push(`w${pos(c)}=${o.join(",")}`)
  void units
  return parts.sort().join("\n")
}
