// Replica of the prototype's pass loop (prototype/esc/src/compiler.ts `compile`),
// built from its exported modules without editing them. It records, per pass,
// every lowering decision keyed by `.es` node, so the single-pass checker can be
// compared with the prototype's final decisions, and so the per-pass diffs show
// which constructs force another pass.
import fs from "node:fs"
import path from "node:path"
import ts from "typescript"
import { compilerOptions } from "../../esc/src/compiler.ts"
import { FileEmitter } from "../../esc/src/emit.ts"
import { collectFacts, fingerprint, type Unit } from "../../esc/src/facts.ts"
import { emptyFacts, type EsFile, type Facts, type ServiceDecl } from "../../esc/src/model.ts"

export const ESC = path.resolve(import.meta.dir, "../../esc")
export const RUNTIME = path.resolve(ESC, "../../packages/runtime/src/index.ts")
// Service keys embed a cwd-relative path (compiler.ts `serviceKey`); run from the repo root as run-all does.
process.chdir(path.resolve(ESC, "../.."))

// ---- copied from compiler.ts (not exported there) -------------------------
const hasMod = (n: ts.Node, k: ts.SyntaxKind) => ts.canHaveModifiers(n) && (ts.getModifiers(n) ?? []).some((m) => m.kind === k)
const serviceKind = (cls: ts.ClassDeclaration): ServiceDecl["kind"] | undefined => {
  if (!hasMod(cls, ts.SyntaxKind.AbstractKeyword) || (cls.typeParameters?.length ?? 0) > 0) return undefined
  let kind: ServiceDecl["kind"] = "service"
  for (const h of cls.heritageClauses ?? []) {
    if (h.token === ts.SyntaxKind.ImplementsKeyword) return undefined
    const t = h.types[0]
    if (h.types.length === 1 && ts.isIdentifier(t.expression) && t.expression.text === "Control") kind = "control"
    else return undefined
  }
  const inst = cls.members.filter((m) => !hasMod(m, ts.SyntaxKind.StaticKeyword) && !ts.isSemicolonClassElement(m))
  if (inst.length === 0) return undefined
  if (!inst.every((m) => (ts.isMethodDeclaration(m) || ts.isPropertyDeclaration(m)) && hasMod(m, ts.SyntaxKind.AbstractKeyword))) return undefined
  return kind
}
const serviceKey = (file: string, name: string) => {
  let dir = path.dirname(file)
  while (dir !== path.dirname(dir)) {
    const pkg = path.join(dir, "package.json")
    if (fs.existsSync(pkg)) {
      const pkgName = (JSON.parse(fs.readFileSync(pkg, "utf8")).name as string | undefined) ?? path.basename(dir)
      return `${pkgName}/${path.relative(dir, file).replace(/\.es$/, "")}#${name}`
    }
    dir = path.dirname(dir)
  }
  return `${path.relative(process.cwd(), file).replace(/\.es$/, "")}#${name}`
}
export const loadEsFile = (p: string, outDir: string, root: string): EsFile => {
  const text = fs.readFileSync(p, "utf8")
  const sf = ts.createSourceFile(p, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const file: EsFile = { path: p, outPath: path.join(outDir, path.relative(root, p).replace(/\.es$/, ".ts")), text, sf,
    functions: new Map(), services: new Map(), topNames: new Set(), recursive: new Set(), esImports: new Map() }
  for (const st of sf.statements) {
    if (ts.isImportDeclaration(st)) {
      const spec = (st.moduleSpecifier as ts.StringLiteral).text
      const clause = st.importClause
      const abs = spec.startsWith(".") ? path.resolve(path.dirname(p), spec) : undefined
      if (clause?.name) file.topNames.add(clause.name.text)
      if (clause?.namedBindings) {
        if (ts.isNamespaceImport(clause.namedBindings)) file.topNames.add(clause.namedBindings.name.text)
        else for (const el of clause.namedBindings.elements) {
          file.topNames.add(el.name.text)
          if (abs?.endsWith(".es")) file.esImports.set(el.name.text, { file: abs, name: (el.propertyName ?? el.name).text })
        }
      }
    } else if (ts.isFunctionDeclaration(st) && st.name) {
      file.functions.set(st.name.text, st)
      file.topNames.add(st.name.text)
    } else if (ts.isClassDeclaration(st) && st.name) {
      file.topNames.add(st.name.text)
      const kind = serviceKind(st)
      if (kind) file.services.set(st.name.text, { name: st.name.text, node: st, kind, key: serviceKey(p, st.name.text) })
    } else if (ts.isVariableStatement(st)) {
      for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name)) file.topNames.add(d.name.text)
    } else if ((ts.isInterfaceDeclaration(st) || ts.isTypeAliasDeclaration(st) || ts.isEnumDeclaration(st)) && st.name) {
      file.topNames.add(st.name.text)
    }
  }
  const calls = new Map<string, Set<string>>()
  for (const [name, fn] of file.functions) {
    const out = new Set<string>()
    const visit = (n: ts.Node) => {
      if (ts.isCallExpression(n) && ts.isIdentifier(n.expression) && file.functions.has(n.expression.text)) out.add(n.expression.text)
      ts.forEachChild(n, visit)
    }
    if (fn.body) visit(fn.body)
    calls.set(name, out)
  }
  for (const name of file.functions.keys()) {
    const seen = new Set<string>()
    const stack = [...(calls.get(name) ?? [])]
    while (stack.length > 0) {
      const g = stack.pop()!
      if (g === name) { file.recursive.add(name); break }
      if (seen.has(g)) continue
      seen.add(g)
      stack.push(...(calls.get(g) ?? []))
    }
  }
  return file
}
export const discover = (entries: string[], outDir: string): Map<string, EsFile> => {
  const files = new Map<string, EsFile>()
  const root = path.dirname(path.resolve(entries[0]))
  const queue = entries.map((e) => path.resolve(e))
  while (queue.length > 0) {
    const p = queue.shift()!
    if (files.has(p)) continue
    const f = loadEsFile(p, outDir, root)
    files.set(p, f)
    for (const st of f.sf.statements) {
      if ((ts.isImportDeclaration(st) || ts.isExportDeclaration(st)) && st.moduleSpecifier && ts.isStringLiteral(st.moduleSpecifier)) {
        const spec = st.moduleSpecifier.text
        if (spec.startsWith(".") && spec.endsWith(".es")) queue.push(path.resolve(path.dirname(p), spec))
      }
    }
  }
  return files
}
export const relImport = (fromFile: string, target: string) => {
  let r = path.relative(path.dirname(fromFile), target)
  if (!r.startsWith(".")) r = `./${r}`
  return r
}
export const rewriterFor = (files: Map<string, EsFile>) => (file: EsFile, spec: string) => {
  if (!spec.startsWith(".")) return spec
  const abs = path.resolve(path.dirname(file.path), spec)
  if (abs.endsWith(".es")) {
    const target = files.get(abs)
    return target ? relImport(file.outPath, target.outPath) : spec.replace(/\.es$/, ".ts")
  }
  return relImport(file.outPath, abs)
}
const sfCache = new Map<string, ts.SourceFile>()
export const makeHost = (tsx: typeof ts, options: ts.CompilerOptions, virtual: Map<string, string>, prev: Map<string, ts.SourceFile>, cache = sfCache): ts.CompilerHost => {
  const base = tsx.createCompilerHost(options, true)
  return {
    ...base,
    fileExists: (f) => virtual.has(f) || base.fileExists(f),
    readFile: (f) => virtual.get(f) ?? base.readFile(f),
    getSourceFile: (f, lang, onError, shouldCreate) => {
      const v = virtual.get(f)
      if (v !== undefined) {
        const old = prev.get(f)
        if (old && old.text === v) return old
        const sf = tsx.createSourceFile(f, v, lang, true)
        prev.set(f, sf)
        return sf
      }
      const cached = cache.get(f)
      if (cached) return cached
      const sf = base.getSourceFile(f, lang, onError, shouldCreate)
      if (sf) cache.set(f, sf)
      return sf
    }
  }
}
// ---------------------------------------------------------------------------

/** A stable key for an `.es` node: file basename, start offset, and kind. */
export const nodeKey = (file: EsFile, n: ts.Node) => `${path.basename(file.path)}:${n.getStart(file.sf)}`
export const lineOf = (file: EsFile, n: ts.Node) => file.sf.getLineAndCharacterOfPosition(n.getStart(file.sf)).line + 1

export interface CallDecision {
  /** The lowering the emitter chose for this call in its mode. */
  emit: string
  mode: string
  /** The checker facts behind it. */
  type: string
  effectValue: boolean
  service: boolean
  direct: boolean
}

export interface Decisions {
  calls: Map<string, CallDecision & { file: EsFile; node: ts.CallExpression }>
  lambdas: Map<string, { mode: string; ctx: string; needs: boolean; file: EsFile; node: ts.Node }>
  convs: Map<string, { conv: string; file: EsFile; node: ts.Node }>
  loops: Map<string, { kind: string; file: EsFile; node: ts.Node }>
  direct: Set<string>
  pipes: Set<string>
  rec: Map<string, string>
  wire: Map<string, string>
}

/** The emission each call gets from `facts`, mirroring FileEmitter.call's choice. */
export const callEmit = (call: ts.CallExpression, mode: string, facts: Facts, file: EsFile) => {
  const f = facts.calls.get(call)
  const svcSelf = f ? f.calleeEffectValue : false
  if (svcSelf && call.arguments.length === 0) return mode === "effect" ? "run-value" : "value"
  const esCallee = f?.esCallee
  const direct = esCallee && ts.isIdentifier(call.expression) && facts.direct.has(`${esCallee.file}#${esCallee.name}`)
  if (direct) return "direct"
  if (mode !== "effect") return "call"
  if (f?.throws) return "trusted"
  const type = f?.type ?? "unknown"
  return type === "effect" ? "run" : type === "unknown" ? "bind" : "call"
}

export function decisionsOf(units: Unit[], facts: Facts): Decisions {
  const d: Decisions = { calls: new Map(), lambdas: new Map(), convs: new Map(), loops: new Map(), direct: new Set(facts.direct), pipes: new Set(), rec: new Map(), wire: new Map() }
  for (const u of units) {
    const file = u.file
    for (const [call, info] of u.emitted.callInfo) {
      const f = facts.calls.get(call)
      const emit = callEmit(call, info.mode, facts, file)
      const svc = f?.serviceReceiver ?? false
      d.calls.set(nodeKey(file, call), {
        emit: emit + (svc && info.mode === "effect" ? "+svc" : ""), mode: info.mode, type: f?.type ?? "unknown",
        effectValue: f?.calleeEffectValue ?? false, service: svc, direct: emit === "direct", file, node: call
      })
    }
    for (const [fn, mode] of u.emitted.lambdaModes) {
      const lf = facts.lambdas.get(fn)
      d.lambdas.set(nodeKey(file, fn), { mode, ctx: lf?.ctx ?? "-", needs: lf?.needsEffect ?? false, file, node: fn })
    }
    for (const [e, c] of facts.convs) {
      if (e.getSourceFile() !== file.sf) continue
      const conv = [c.program && "program", c.programs && "programs", c.stream && "stream", c.direct && "direct", c.rejectPlain && "rejectPlain"].filter(Boolean).join("+")
      d.convs.set(nodeKey(file, e), { conv, file, node: e })
    }
    for (const st of facts.producerLoops) if (st.getSourceFile() === file.sf) d.loops.set(nodeKey(file, st), { kind: "producer", file, node: st })
    for (const st of facts.streamLoops) if (st.getSourceFile() === file.sf) d.loops.set(nodeKey(file, st), { kind: "stream", file, node: st })
    for (const p of facts.pipePrograms) if (p.getSourceFile() === file.sf) d.pipes.add(nodeKey(file, p))
    for (const [c, o] of facts.wireOrders) if (c.getSourceFile() === file.sf) d.wire.set(nodeKey(file, c), o.join(","))
  }
  for (const [k, t] of facts.recTypes) d.rec.set(k.split("/").pop()!, `${t.A} / ${t.E} / ${t.R}`)
  return d
}

export interface PassRecord {
  pass: number
  decisions: Decisions
  text: Map<string, string>
}

export interface ReplicaResult {
  files: Map<string, EsFile>
  passes: PassRecord[]
  converged: boolean
  final: Decisions
  units: Unit[]
  facts: Facts
}

/** Run the prototype's loop exactly as `compile` does, recording every pass. */
export function runReplica(entries: string[], opts: { outDir?: string; maxPasses?: number } = {}): ReplicaResult {
  const outDir = path.resolve(opts.outDir ?? path.join(path.dirname(path.resolve(entries[0])), "out"))
  const files = discover(entries, outDir)
  const rewrite = rewriterFor(files)
  let facts: Facts = emptyFacts()
  let fp = ""
  let units: Unit[] = []
  let program: ts.Program | undefined
  let converged = false
  const prev = new Map<string, ts.SourceFile>()
  const passes: PassRecord[] = []
  const maxPasses = opts.maxPasses ?? 6
  for (let pass = 1; pass <= maxPasses; pass++) {
    units = [...files.values()].map((file) => ({
      file,
      genPath: file.outPath,
      emitted: new FileEmitter({ file, files, facts, runtimeImport: relImport(file.outPath, RUNTIME), rewriteImport: (s) => rewrite(file, s), check: true }).emitFile()
    }))
    // Decisions as emitted in this pass (from the previous pass's facts).
    passes.push({ pass, decisions: decisionsOf(units, facts), text: new Map(units.map((u) => [u.file.path, u.emitted.text.slice(0, u.emitted.bodyEnd)])) })
    const virtual = new Map(units.map((u) => [u.genPath, u.emitted.text] as const))
    program = ts.createProgram({ rootNames: [...virtual.keys()], options: compilerOptions, host: makeHost(ts, compilerOptions, virtual, prev), oldProgram: program })
    const next = collectFacts(program, units)
    const nfp = fingerprint(next, units)
    facts = next
    if (nfp === fp) { converged = true; break }
    fp = nfp
  }
  return { files, passes, converged, final: passes[passes.length - 1].decisions, units, facts }
}
