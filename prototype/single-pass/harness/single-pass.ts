// The single-pass compiler front half: emit each `.es` file once, neutrally (emit-neutral.ts),
// check it once with the patched checker (../typescript), and read every lowering decision the
// checker made into the prototype's `Facts` shape. Nothing is re-emitted or re-checked here.
import path from "node:path"
import ts from "typescript"
import { containsShallow } from "../../esc/src/emit.ts"
import { compilerOptions } from "../../esc/src/compiler.ts"
import { calleeOrigin, computeDirect, effectParts, isEffectItself, isEffectType, isKeyType, type Unit, wireGraph } from "../../esc/src/facts.ts"
import { type CallFact, type ConvFact, emptyFacts, type EsFile, type Facts, type LambdaFact, type Span } from "../../esc/src/model.ts"
import { type CallCopy, NeutralEmitter, type NeutralEmitted, type VariantPath } from "./emit-neutral.ts"
import { discover, makeHost, relImport, RUNTIME, rewriterFor } from "./replica.ts"
// The patched TypeScript: same version, same AST shapes, plus checker.__es.
import tsp from "../typescript/lib/typescript.js"

export const SHIM = path.join(import.meta.dir, "es-shim.d.ts")
const patchedCache = new Map<string, ts.SourceFile>()

interface Selection {
  id: number
  kind: "dual" | "body"
  mode: "plain" | "effect" | "program"
  ctx: string
  via: string
  needs?: boolean
  pipe?: boolean
  owner?: ts.CallExpression
  cand?: ts.Node
}

interface EsState {
  selections: Selection[]
  final: Map<number, Selection>
  byCandidate: Map<number, Map<ts.Node | undefined, Selection>>
  intrinsics: Map<number, any>
  conflicts: Array<{ id: number; a: string; b: string }>
}

class NodeIndex {
  private readonly byKey = new Map<string, ts.Node[]>()
  constructor(sf: ts.SourceFile) {
    const visit = (n: ts.Node) => {
      const k = `${n.getStart(sf)}:${n.end}`
      const list = this.byKey.get(k)
      if (list) list.push(n)
      else this.byKey.set(k, [n])
      tsp.forEachChild(n, visit)
    }
    tsp.forEachChild(sf, visit)
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

export interface SinglePassUnit {
  file: EsFile
  genPath: string
  emitted: NeutralEmitted
}

export interface Anomaly {
  kind: string
  detail: string
}

export interface SinglePassResult {
  files: Map<string, EsFile>
  units: SinglePassUnit[]
  facts: Facts
  /** Per `.es` lambda: the mode the checker selected, how, and against which candidate. */
  modes: Map<ts.Node, { mode: string; ctx: string; via: string; candidates: number; perCandidate: string[] }>
  anomalies: Anomaly[]
  checkMs: number
  tsErrors: number
  neutralText: Map<string, string>
}

const variantIndex = (sel: Selection, method: boolean) =>
  sel.mode === "plain" ? 0 : sel.mode === "effect" ? 1 : method ? 1 : 2

export function runSinglePass(entries: string[]): SinglePassResult {
  const outDir = path.resolve(path.dirname(path.resolve(entries[0])), "out")
  const files = discover(entries, outDir)
  const rewrite = rewriterFor(files)
  const units: SinglePassUnit[] = [...files.values()].map((file) => ({
    file,
    genPath: file.outPath,
    emitted: new NeutralEmitter({
      file, files, facts: emptyFacts(), runtimeImport: relImport(file.outPath, RUNTIME), rewriteImport: (s) => rewrite(file, s), check: false
    }).emitFile()
  }))
  const virtual = new Map(units.map((u) => [u.genPath, u.emitted.text] as const))
  const program = tsp.createProgram({
    rootNames: [SHIM, ...virtual.keys()],
    options: compilerOptions as any,
    host: makeHost(tsp as any, compilerOptions, virtual, new Map(), patchedCache) as any
  }) as unknown as ts.Program
  const checker = program.getTypeChecker()

  // The one pass.
  const t0 = performance.now()
  let tsErrors = 0
  for (const u of units) tsErrors += program.getSemanticDiagnostics(program.getSourceFile(u.genPath)).length
  const checkMs = performance.now() - t0

  const es = (checker as any).__es as EsState
  const anomalies: Anomaly[] = []
  const methodDuals = new Set<number>()
  for (const u of units) for (const d of u.emitted.duals) if (d.method) methodDuals.add(d.id)

  // The selection that typed the program: for a lambda inside a call, the one made against the
  // candidate that resolution picked; elsewhere, the last one.
  const finalSel = (id: number): Selection | undefined => {
    const sel = es.final.get(id)
    if (!sel || !sel.owner) return sel
    const sig = checker.getResolvedSignature(sel.owner as any)
    const byCand = es.byCandidate.get(id)?.get(sig?.declaration as any)
    return byCand ?? sel
  }
  const live = (p: VariantPath) =>
    p.every(([d, v]) => {
      const sel = finalSel(d)
      return sel !== undefined && variantIndex(sel, methodDuals.has(d)) === v
    })

  const indexes = new Map<SinglePassUnit, NodeIndex>()
  for (const u of units) indexes.set(u, new NodeIndex(program.getSourceFile(u.genPath)!))
  const generated = new Set(units.map((u) => u.genPath))
  const byGen = new Map(units.map((u) => [u.genPath, u] as const))
  const facts = emptyFacts()

  // Every selection a dual received, to report any that changed after it typed the program.
  const seen = new Map<number, Set<string>>()
  for (const s of es.selections) {
    const set = seen.get(s.id) ?? new Set()
    set.add(s.mode)
    seen.set(s.id, set)
  }
  for (const c of es.conflicts) anomalies.push({ kind: "conflict", detail: `dual ${c.id} selected ${c.a} then ${c.b} against one candidate` })

  // 1. Calls: read from the live copy, exactly as the prototype's collectFacts reads them.
  const resolveEsFunction = (id: ts.Expression) => {
    if (!ts.isIdentifier(id)) return undefined
    let sym = checker.getSymbolAtLocation(id)
    if (!sym) return undefined
    if (sym.flags & ts.SymbolFlags.Alias) sym = checker.getAliasedSymbol(sym)
    const decl = sym.declarations?.[0]
    if (!decl) return undefined
    const unit = byGen.get(decl.getSourceFile().fileName)
    if (!unit) return undefined
    const name = sym.name.replace(/\$direct$/, "")
    if (!unit.file.functions.get(name)) return undefined
    const top = decl.parent?.parent?.parent
    if (!(ts.isFunctionDeclaration(decl) || (top && ts.isSourceFile(top)) || ts.isSourceFile(decl.parent))) return undefined
    return { file: unit.file.path, name }
  }
  const trustedThrows = (call: ts.CallExpression) => {
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
  const liveCall = new Map<ts.CallExpression, { u: SinglePassUnit; copy: CallCopy }>()
  for (const u of units) {
    for (const copy of u.emitted.calls) {
      if (!live(copy.path)) continue
      const prev = liveCall.get(copy.es)
      if (!prev) liveCall.set(copy.es, { u, copy })
    }
    for (const call of new Set(u.emitted.calls.map((c) => c.es))) {
      if (!liveCall.has(call)) anomalies.push({ kind: "no-live-copy", detail: `call ${call.getText(u.file.sf).slice(0, 50)}` })
    }
  }
  for (const [call, { u, copy }] of liveCall) {
    const idx = indexes.get(u)!
    const fact: CallFact = { type: "unknown", calleeEffectValue: false, serviceReceiver: false, origin: "unknown", calleeText: call.expression.getText(u.file.sf) }
    facts.calls.set(call, fact)
    if (copy.kind === "tail") continue
    const calleeNode = idx.find(copy.callee) as ts.Expression | undefined
    const callNode = idx.find(copy.call, ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
    if (ts.isPropertyAccessExpression(call.expression)) {
      const recv = idx.find(copy.receiver) as ts.Expression | undefined
      if (recv) {
        const rt = checker.getTypeAtLocation(recv)
        const shape = isKeyType(rt) ? rt.getProperty("Service") : undefined
        const member = call.expression.name.text
        if (shape && checker.getTypeOfSymbol(shape).getProperty(member)) fact.serviceReceiver = true
        else if (isKeyType(rt) && !rt.getProperty(member) && !checker.getApparentType(rt).getProperty(member)) fact.serviceReceiver = true
      }
    }
    if (calleeNode) {
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
      if (fact.type === "plain") fact.throws = trustedThrows(callNode)
      fact.esCallee = resolveEsFunction(callNode.expression)
    }
  }

  // 2. Lambdas, methods, and bodies: the selection the checker made.
  const modes: SinglePassResult["modes"] = new Map()
  const selected: Array<{ u: SinglePassUnit; es: ts.Node; sel: Selection }> = []
  const needs = (fn: ts.Node, plainCtx: boolean) => {
    const body = (fn as ts.FunctionLikeDeclaration).body
    if (!body) return { needs: false, pauses: undefined as string | undefined }
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
    for (const d of u.emitted.duals) {
      if (!live(d.path)) continue
      const sel = finalSel(d.id)
      if (!sel) {
        anomalies.push({ kind: "unchecked-dual", detail: `${d.es.getText(u.file.sf).slice(0, 50)}` })
        continue
      }
      const all = seen.get(d.id)!
      // Different classes under different candidates are the experiment working (reported as splits).
      if (all.size > 1 && !sel.owner) anomalies.push({ kind: "reselected", detail: `L${u.file.sf.getLineAndCharacterOfPosition(d.es.getStart(u.file.sf)).line + 1} ${d.es.getText(u.file.sf).slice(0, 40).replace(/\s+/g, " ")}: ${[...all].join(" / ")} (kept ${sel.mode})` })
      selected.push({ u, es: d.es, sel })
      if (sel.pipe && (ts.isArrowFunction(d.es) || ts.isFunctionExpression(d.es))) facts.pipePrograms.add(d.es as ts.Expression)
      const byCand = sel.owner ? es.byCandidate.get(d.id) : undefined
      const perCandidate = byCand ? [...byCand.values()].map((s) => s.mode) : []
      modes.set(d.es, { mode: sel.mode, ctx: sel.ctx, via: sel.via, candidates: byCand?.size ?? 0, perCandidate })
    }
  }

  // 3. Conversions: the in-pass decision of the live copy.
  for (const u of units) {
    for (const c of u.emitted.convs) {
      if (!live(c.path)) continue
      const d = es.intrinsics.get(c.id)
      if (!d) {
        anomalies.push({ kind: "unchecked-conv", detail: c.es.getText(u.file.sf).slice(0, 50) })
        continue
      }
      const conv: ConvFact = { program: d.program, programs: d.programs || undefined, stream: d.stream || undefined, direct: d.direct, rejectPlain: d.rejectPlain }
      if (conv.program || conv.programs || conv.stream || conv.direct || conv.rejectPlain) facts.convs.set(c.es, conv)
    }
  }

  // 4. Loops: the iterable's type.
  for (const u of units) {
    const idx = indexes.get(u)!
    for (const l of u.emitted.loops) {
      if (!live(l.path)) continue
      const node = idx.find(l.iterable) as ts.Expression | undefined
      if (!node) continue
      const t = checker.getTypeAtLocation(node)
      if (t.getProperty("~esc/Producer")) facts.producerLoops.add(l.es)
      else if (t.getProperty("~effect/Stream")) facts.streamLoops.add(l.es)
    }
  }

  // 5. wire(...) order from the arguments' Layer types (source order in the text).
  for (const u of units) {
    const idx = indexes.get(u)!
    for (const w of u.emitted.wireCalls) {
      if (!live(w.path)) continue
      const node = idx.find(w.call, ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
      if (!node) continue
      const prev = w.es.arguments.map((_, i) => i)
      const g = wireGraph(node, checker)
      const needsSrc = new Map<number, number[]>()
      for (const src of prev) {
        const ins = g.args[src].ins
        needsSrc.set(src, prev.filter((o) => o !== src && g.args[o].out.some((t) => ins.includes(t))))
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
      facts.wireOrders.set(w.es, out.length === prev.length && !g.cycle ? out : [...prev])
    }
  }

  // 6. D10, from the call decisions (the prototype's computeDirect).
  const protoUnits = units.map((u) => ({ file: u.file, genPath: u.genPath, emitted: undefined as any })) as Unit[]
  facts.direct = computeDirect(protoUnits, facts)
  for (const u of units) {
    for (const c of u.emitted.convs) {
      const conv = facts.convs.get(c.es)
      if (!conv?.direct && !conv?.rejectPlain) continue
      const es0 = resolveEsFunction(indexes.get(u)!.find(c.expr) as ts.Expression)
      if (!es0) continue
      const direct = facts.direct.has(`${es0.file}#${es0.name}`)
      if (direct !== conv.direct) anomalies.push({ kind: "direct-reread", detail: `${es0.name}: in pass ${conv.direct}, computeDirect ${direct}` })
    }
  }

  // 6b. A call to a function that compiles to plain JavaScript is emitted as `f$direct(...)`; the
  //     prototype reads its fact from that text, so its type is the type of the value it returns.
  for (const [call, f] of facts.calls) {
    if (!f.esCallee || !ts.isIdentifier(call.expression) || !facts.direct.has(`${f.esCallee.file}#${f.esCallee.name}`)) continue
    const live0 = liveCall.get(call)
    const node = live0 && (indexes.get(live0.u)!.find(live0.copy.call, ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined)
    const A = node ? effectParts(checker.getTypeAtLocation(node), checker)?.A : undefined
    if (!A) continue
    const tri = isEffectItself(A, checker)
    f.type = tri === "yes" ? "effect" : tri === "no" ? "plain" : "unknown"
    f.effectE = undefined
    f.effectR = undefined
  }

  // The decision that typed the program in the pass must be the one read back. (In Effect code a call
  // to a D10 function is typed as the Effect in the pass and read as its plain variant, as above.)
  for (const [call, { copy }] of liveCall) {
    const d = es.intrinsics.get(copy.id)
    const fact = facts.calls.get(call)!
    if (!d || !(copy.kind === "bind" || copy.kind === "bind0" || copy.kind === "val0" || copy.kind === "special")) continue
    const directRead = fact.esCallee !== undefined && facts.direct.has(`${fact.esCallee.file}#${fact.esCallee.name}`) && !d.direct
    if (directRead) continue
    const inPass = d.effectValue ? "effect-value" : d.type
    const readBack = fact.calleeEffectValue ? "effect-value" : fact.type
    if (inPass !== readBack) anomalies.push({ kind: "call-reread", detail: `${fact.calleeText}: in pass ${inPass}, read back ${readBack}` })
  }

  // 6c. Lambda facts, as the prototype computes them: the class, and whether the body pauses.
  for (const { u, es: fn, sel } of selected) {
    const n = needs(fn, sel.ctx === "plainFn")
    facts.lambdas.set(fn, { ctx: sel.ctx as LambdaFact["ctx"], needsEffect: n.needs, pauses: n.pauses })
    // For a body with no expected type, the in-pass reason must be the one read back.
    if (sel.via === "needs" && sel.needs !== n.needs) {
      anomalies.push({ kind: "needs-reread", detail: `${fn.getText(u.file.sf).slice(0, 40)}: in pass ${sel.needs}, read back ${n.needs}` })
    }
  }

  // 7. Recursive functions: the contract this pass inferred (the prototype's next forward binding).
  for (const u of units) {
    const sf = program.getSourceFile(u.genPath)!
    for (const name of u.file.recursive) {
      let decl: ts.VariableDeclaration | undefined
      for (const st of sf.statements) {
        if (!ts.isVariableStatement(st)) continue
        for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name) && d.name.text === name) decl = d
      }
      if (!decl?.initializer) continue
      const sig = checker.getTypeAtLocation(decl.initializer).getCallSignatures()[0]
      const parts = sig ? effectParts(sig.getReturnType(), checker) : undefined
      if (!parts) continue
      const print = (t: ts.Type) => checker.typeToString(t, decl, ts.TypeFormatFlags.NoTruncation | ts.TypeFormatFlags.UseFullyQualifiedType)
      facts.recTypes.set(`${u.file.path}#${name}`, { A: print(parts.A), E: print(parts.E), R: print(parts.R) })
    }
  }

  return { files, units, facts, modes, anomalies, checkMs, tsErrors, neutralText: virtual }
}
