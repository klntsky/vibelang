// The EffectScript prototype compiler driver.
//
// Pipeline: parse every `.es` file once; then repeat { emit TypeScript with the
// current facts; type-check it in memory; read new facts } until the facts stop
// changing. The last emission is the output. Contracts are read from the
// checker's inferred types of the generated `Effect.fnUntraced` functions.
import fs from "node:fs"
import path from "node:path"
import ts from "typescript"
import { type Emitted, FileEmitter } from "./emit.ts"
import { collectFacts, effectParts, expectedTypes, fingerprint, isEffectType, NodeIndex, propertyTypes, typeParts, type Unit, wireGraph } from "./facts.ts"
import { type Diagnostic, emptyFacts, type EsFile, type Facts, type ServiceDecl } from "./model.ts"

export interface CompileOptions {
  outDir?: string
  maxPasses?: number
  write?: boolean
}

export interface ChannelCompare {
  channel: "A" | "E" | "R" | "type"
  inferred: string
  claimed: string
  ok: boolean
}

export interface ContractReport {
  file: string
  name: string
  kind: "function" | "const"
  inferred: string
  direct: boolean
  contract: boolean
  claim?: { text: string; line: number; status: "match" | "mismatch" | "unresolved" | "text"; channels: ChannelCompare[]; note?: string }
}

export interface CompileResult {
  files: Array<{ es: string; out: string; text: string }>
  diagnostics: Diagnostic[]
  contracts: ContractReport[]
  passes: number
  converged: boolean
  ms: number
}

const RUNTIME = path.resolve(import.meta.dir, "../../../packages/runtime/src/index.ts")

export const compilerOptions: ts.CompilerOptions = {
  strict: true,
  target: ts.ScriptTarget.ES2022,
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  allowImportingTsExtensions: true,
  noEmit: true,
  skipLibCheck: true,
  types: [],
  lib: ["lib.esnext.d.ts", "lib.dom.d.ts"],
  allowUnreachableCode: true,
  useDefineForClassFields: true
}

// Library and node_modules source files are parsed once per process.
const sfCache = new Map<string, ts.SourceFile>()

const makeHost = (virtual: Map<string, string>, prev: Map<string, ts.SourceFile>): ts.CompilerHost => {
  const base = ts.createCompilerHost(compilerOptions, true)
  return {
    ...base,
    fileExists: (f) => virtual.has(f) || base.fileExists(f),
    readFile: (f) => virtual.get(f) ?? base.readFile(f),
    getSourceFile: (f, lang, onError, shouldCreate) => {
      const v = virtual.get(f)
      if (v !== undefined) {
        const old = prev.get(f)
        if (old && old.text === v) return old
        const sf = ts.createSourceFile(f, v, lang, true)
        prev.set(f, sf)
        return sf
      }
      const cached = sfCache.get(f)
      if (cached) return cached
      const sf = base.getSourceFile(f, lang, onError, shouldCreate)
      if (sf) sfCache.set(f, sf)
      return sf
    }
  }
}

// ---------------------------------------------------------------------------
// Loading `.es` files
// ---------------------------------------------------------------------------

const hasMod = (n: ts.Node, k: ts.SyntaxKind) => ts.canHaveModifiers(n) && (ts.getModifiers(n) ?? []).some((m) => m.kind === k)

/** D1, D2: an abstract class with only abstract instance members is a service; `extends Control` makes it a control. */
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

/** The service key: package, module path, and class name (D1). */
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

const loadEsFile = (p: string, outDir: string, root: string): EsFile => {
  const text = fs.readFileSync(p, "utf8")
  const sf = ts.createSourceFile(p, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const file: EsFile = {
    path: p,
    outPath: path.join(outDir, path.relative(root, p).replace(/\.es$/, ".ts")),
    text,
    sf,
    functions: new Map(),
    services: new Map(),
    topNames: new Set(),
    recursive: new Set(),
    esImports: new Map()
  }
  for (const st of sf.statements) {
    if (ts.isImportDeclaration(st)) {
      const spec = (st.moduleSpecifier as ts.StringLiteral).text
      const clause = st.importClause
      const abs = spec.startsWith(".") ? path.resolve(path.dirname(p), spec) : undefined
      if (clause?.name) file.topNames.add(clause.name.text)
      if (clause?.namedBindings) {
        if (ts.isNamespaceImport(clause.namedBindings)) file.topNames.add(clause.namedBindings.name.text)
        else {
          for (const el of clause.namedBindings.elements) {
            file.topNames.add(el.name.text)
            if (abs?.endsWith(".es")) file.esImports.set(el.name.text, { file: abs, name: (el.propertyName ?? el.name).text })
          }
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
  // Recursion: a top-level function that reaches itself through calls in this file.
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
      if (g === name) {
        file.recursive.add(name)
        break
      }
      if (seen.has(g)) continue
      seen.add(g)
      stack.push(...(calls.get(g) ?? []))
    }
  }
  return file
}

const discover = (entries: string[], outDir: string): Map<string, EsFile> => {
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

const relImport = (fromFile: string, target: string) => {
  let r = path.relative(path.dirname(fromFile), target)
  if (!r.startsWith(".")) r = `./${r}`
  return r
}

// ---------------------------------------------------------------------------
// Diagnostics
// ---------------------------------------------------------------------------

const mapBack = (maps: Emitted["maps"], pos: number): ts.Node | undefined => {
  let best: ts.Node | undefined
  let width = Infinity
  for (const m of maps) {
    if (m.gs <= pos && pos < Math.max(m.ge, m.gs + 1) && m.ge - m.gs < width) {
      best = m.node
      width = m.ge - m.gs
    }
  }
  return best
}

const unionParts = (t: ts.Type): ts.Type[] => (t.isUnion() ? t.types : t.flags & ts.TypeFlags.Never ? [] : [t])

/** Syntactic misuse of service classes (D1). */
const serviceMisuse = (file: EsFile, files: Map<string, EsFile>, out: Diagnostic[]) => {
  const isService = (e: ts.Expression) => {
    if (!ts.isIdentifier(e)) return false
    if (file.services.has(e.text)) return true
    const imp = file.esImports.get(e.text)
    return imp !== undefined && files.get(imp.file)?.services.has(imp.name) === true
  }
  const visit = (n: ts.Node) => {
    const d = (code: string, message: string) =>
      out.push({ code, severity: "error", file: file.path, start: n.getStart(file.sf), end: n.end, message })
    if (ts.isNewExpression(n) && isService(n.expression)) {
      d("ES3003", `${n.expression.getText(file.sf)} is a service: get its implementation with ${n.expression.getText(file.sf)}(), or implement it with an object literal or a class that \`implements\` it.`)
    }
    if (ts.isBinaryExpression(n) && n.operatorToken.kind === ts.SyntaxKind.InstanceOfKeyword && isService(n.right)) {
      d("ES3004", `instanceof ${n.right.getText(file.sf)} is always false: a service is a Context key, not a base class.`)
    }
    if (ts.isClassDeclaration(n) && n.name && file.services.get(n.name.text)?.node === n) {
      // A static member that shares a name with a service member makes `S.m` ambiguous.
      const inst = new Set(n.members.filter((m) => !hasMod(m, ts.SyntaxKind.StaticKeyword)).map((m) => m.name?.getText(file.sf)))
      for (const m of n.members) {
        if (hasMod(m, ts.SyntaxKind.StaticKeyword) && m.name && inst.has(m.name.getText(file.sf))) {
          out.push({ code: "ES3006", severity: "error", file: file.path, start: m.getStart(file.sf), end: m.end,
            message: `${n.name.text}.${m.name.getText(file.sf)} is both a static member and a service member; rename one.` })
        }
      }
    }
    if (ts.isClassDeclaration(n)) {
      for (const h of n.heritageClauses ?? []) {
        if (h.token === ts.SyntaxKind.ExtendsKeyword && h.types.some((t) => isService(t.expression))) {
          d("ES3002", `a class cannot extend a service; write \`implements ${h.types[0].expression.getText(file.sf)}\` instead.`)
        }
      }
      if (hasMod(n, ts.SyntaxKind.AbstractKeyword) && n.typeParameters && n.members.every((m) => hasMod(m, ts.SyntaxKind.AbstractKeyword))) {
        d("ES3005", `a generic abstract class cannot be a service: one Context key cannot carry type parameters.`)
      }
    }
    ts.forEachChild(n, visit)
  }
  visit(file.sf)
}

// ---------------------------------------------------------------------------
// Compile
// ---------------------------------------------------------------------------

export function compile(entries: string[], opts: CompileOptions = {}): CompileResult {
  const t0 = performance.now()
  const outDir = path.resolve(opts.outDir ?? path.join(path.dirname(path.resolve(entries[0])), "out"))
  fs.mkdirSync(outDir, { recursive: true })
  const files = discover(entries, outDir)
  const diagnostics: Diagnostic[] = []

  // Service keys are unique program-wide (D1).
  const keys = new Map<string, ServiceDecl>()
  for (const f of files.values()) {
    for (const s of f.services.values()) {
      const prev = keys.get(s.key)
      if (prev) {
        diagnostics.push({
          code: "ES3001",
          severity: "error",
          file: f.path,
          start: s.node.getStart(f.sf),
          end: s.node.end,
          message: `two services share the key ${s.key}`
        })
      }
      keys.set(s.key, s)
    }
    serviceMisuse(f, files, diagnostics)
  }

  const rewriteImport = (file: EsFile, spec: string) => {
    if (!spec.startsWith(".")) return spec
    const abs = path.resolve(path.dirname(file.path), spec)
    if (abs.endsWith(".es")) {
      const target = files.get(abs)
      return target ? relImport(file.outPath, target.outPath) : spec.replace(/\.es$/, ".ts")
    }
    return relImport(file.outPath, abs)
  }

  let facts: Facts = emptyFacts()
  let fp = ""
  let units: Unit[] = []
  let program: ts.Program | undefined
  let converged = false
  let passes = 0
  const prev = new Map<string, ts.SourceFile>()
  const maxPasses = opts.maxPasses ?? 6
  while (passes < maxPasses) {
    passes++
    units = [...files.values()].map((file) => ({
      file,
      genPath: file.outPath,
      emitted: new FileEmitter({
        file,
        files,
        facts,
        runtimeImport: relImport(file.outPath, RUNTIME),
        rewriteImport: (s) => rewriteImport(file, s),
        check: true
      }).emitFile()
    }))
    const virtual = new Map(units.map((u) => [u.genPath, u.emitted.text] as const))
    program = ts.createProgram({ rootNames: [...virtual.keys()], options: compilerOptions, host: makeHost(virtual, prev), oldProgram: program })
    const next = collectFacts(program, units)
    const nfp = fingerprint(next, units)
    facts = next
    if (nfp === fp) {
      converged = true
      break
    }
    fp = nfp
  }
  const checker = program!.getTypeChecker()
  const contracts: ContractReport[] = []
  const suppress = new Set<ts.Node>()

  for (const u of units) {
    for (const d of u.emitted.diagnostics) {
      diagnostics.push(d)
      if (d.covers) suppress.add(d.covers)
    }
    esChecks(u, facts, checker, program!, diagnostics, suppress, contracts, [...files.values()])
  }
  for (const u of units) {
    const sf = program!.getSourceFile(u.genPath)!
    for (const d of [...program!.getSyntacticDiagnostics(sf), ...program!.getSemanticDiagnostics(sf)]) {
      if (d.start === undefined || d.start >= u.emitted.bodyEnd) continue
      const node = mapBack(u.emitted.maps, d.start)
      let covered = false
      for (let n: ts.Node | undefined = node; n; n = n.parent) if (suppress.has(n)) covered = true
      if (covered) continue
      diagnostics.push({
        code: `TS${d.code}`,
        severity: d.category === ts.DiagnosticCategory.Error ? "error" : "warning",
        file: u.file.path,
        start: node ? node.getStart(u.file.sf) : 0,
        end: node ? node.end : 0,
        message: ts.flattenDiagnosticMessageText(d.messageText, "\n  ")
      })
    }
  }
  if (!converged) {
    diagnostics.push({ code: "ES0002", severity: "warning", file: [...files.keys()][0], start: 0, end: 0, message: `lowering decisions did not converge in ${passes} passes` })
  }

  const outputs = units.map((u) => ({ es: u.file.path, out: u.genPath, text: u.emitted.text.slice(0, u.emitted.bodyEnd) }))
  if (opts.write !== false) {
    for (const o of outputs) {
      fs.mkdirSync(path.dirname(o.out), { recursive: true })
      fs.writeFileSync(o.out, o.text)
    }
  }
  diagnostics.sort((a, b) => (a.file === b.file ? a.start - b.start : a.file < b.file ? -1 : 1))
  return { files: outputs, diagnostics, contracts, passes, converged, ms: performance.now() - t0 }
}

// ---------------------------------------------------------------------------
// Checks that need the final program: D5 boundaries, ES1001, contracts
// ---------------------------------------------------------------------------

function esChecks(
  u: Unit,
  facts: Facts,
  checker: ts.TypeChecker,
  program: ts.Program,
  out: Diagnostic[],
  suppress: Set<ts.Node>,
  contracts: ContractReport[],
  allFiles: EsFile[]
) {
  const sf = program.getSourceFile(u.genPath)!
  const idx = new NodeIndex(sf)
  const f = u.file
  const print = (t: ts.Type, at?: ts.Node) =>
    checker
      .typeToString(t, at, ts.TypeFormatFlags.NoTruncation | ts.TypeFormatFlags.UseAliasDefinedOutsideCurrentScope)
      .replace(/import\("[^"]*"\)\./g, "")
  const diag = (code: string, node: ts.Node, message: string, notes?: string[]) =>
    out.push({ code, severity: "error", file: f.path, start: node.getStart(f.sf), end: node.end, message, notes })

  // ES1001 and D5 (ES4401-ES4403).
  for (const [call, info] of u.emitted.callInfo) {
    const fact = facts.calls.get(call)
    if (!fact) continue
    const name = fact.calleeText
    if (info.mode === "top") {
      if (fact.type === "effect" || fact.calleeEffectValue) {
        diag("ES1001", call, `${name}() runs a program, but a module's top level is not inside one.`, [
          "Export a function instead, or provide the value as a service with a Layer."
        ])
      }
      continue
    }
    if (fact.type !== "plain" || fact.calleeEffectValue) continue
    if (fact.origin === "ts" && !info.boundary) {
      diag("ES4401", call, `${name} is a plain TypeScript function: EffectScript cannot know whether it throws.`, [
        `Call it through Effect.try(() => ${call.getText(f.sf)}), or declare a trusted contract for it.`
      ])
    } else if (fact.origin === "platform") {
      diag("ES4402", call, `${name} is a platform API and is not callable from EffectScript; use Effect's module for it (D6).`)
    } else if (fact.origin === "clock") {
      diag("ES4403", call, `${name}() reads the clock or randomness; use Clock, DateTime, or Random so tests control it.`)
    }
  }

  // Contracts of top-level functions.
  const genDecls = new Map<string, ts.VariableDeclaration>()
  for (const st of sf.statements) {
    if (!ts.isVariableStatement(st)) continue
    for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name)) genDecls.set(d.name.text, d)
  }
  const inferredOf = (name: string) => {
    const d = genDecls.get(name)
    if (!d?.initializer) return undefined
    const t = checker.getTypeAtLocation(d.initializer)
    const sig = t.getCallSignatures()[0]
    if (!sig) return { decl: d, type: t, sig: undefined, parts: effectParts(t, checker) }
    return { decl: d, type: t, sig, parts: effectParts(sig.getReturnType(), checker) }
  }
  const reported = new Set<string>()
  for (const [name, info] of u.emitted.fnDecls) {
    const inf = inferredOf(name)
    if (!inf) continue
    const at = inf.decl
    let inferred: string
    const generic = (inf.sig?.getTypeParameters()?.length ?? 0) > 0
    if (inf.parts && !generic) {
      inferred = `Effect<${print(inf.parts.A, at)}, ${print(inf.parts.E, at)}, ${print(inf.parts.R, at)}>`
    } else if (inf.sig && generic) {
      // A generic function: its contract is its whole signature (Chapter 14).
      inferred = checker
        .signatureToString(inf.sig, at, ts.TypeFormatFlags.NoTruncation | ts.TypeFormatFlags.WriteArrowStyleSignature)
        .replace(/import\("[^"]*"\)\./g, "")
    } else inferred = (inf.sig ? checker.signatureToString(inf.sig, at, ts.TypeFormatFlags.NoTruncation) : print(inf.type, at)).replace(/import\("[^"]*"\)\./g, "")
    const report: ContractReport = { file: f.path, name, kind: "function", inferred, direct: info.direct, contract: info.contract }
    contracts.push(report)
    reported.add(name)

    // ES2001 / ES2002: the body against a written contract (Chapter 12).
    if (info.contract && inf.parts && at.type) {
      const declared = checker.getTypeFromTypeNode(at.type).getCallSignatures()[0]
      const dparts = declared ? effectParts(declared.getReturnType(), checker) : undefined
      if (dparts) {
        const escE = unionParts(inf.parts.E).filter((x) => !checker.isTypeAssignableTo(x, dparts.E))
        const escR = unionParts(inf.parts.R).filter((x) => !checker.isTypeAssignableTo(x, dparts.R))
        const fixed = `Effect<${print(dparts.A, at)}, ${print(inf.parts.E, at)}, ${print(inf.parts.R, at)}>`
        for (const x of escE) {
          const site = findSite(u, facts, checker, idx, name, "E", x)
          diag(
            "ES2001",
            site?.node ?? info.node.name ?? info.node,
            `${print(x, at)} can escape ${name}, but its contract says E = ${print(dparts.E, at)}.`,
            [
              ...(site ? [`${site.what} can fail with ${print(x, at)}`] : []),
              `Catch it inside ${name}, or change the contract to ${fixed}.`
            ]
          )
        }
        for (const x of escR) {
          const site = findSite(u, facts, checker, idx, name, "R", x)
          diag(
            "ES2002",
            site?.node ?? info.node.name ?? info.node,
            `${name} needs ${print(x, at)}, but its contract says R = ${print(dparts.R, at)}.`,
            [
              ...(site ? [`${site.what} needs ${print(x, at)}`] : []),
              `Provide ${print(x, at)} inside ${name}, or add it: ${fixed}.`
            ]
          )
        }
        if (escE.length > 0 || escR.length > 0) suppress.add(info.node)
      }
    }
  }

  // ES4201: a control operation inside a combinator that runs its argument on another fiber (Chapter 14).
  const controls = new Set<string>()
  for (const file of allFiles) for (const svc of file.services.values()) if (svc.kind === "control") controls.add(svc.name)
  const RACING = new Set(["race", "raceFirst", "raceAll", "timeout", "timeoutOption", "timeoutOrElse", "forkChild", "forkIn", "forkScoped", "forkDetach", "fork"])
  const optionsSay = (call: ts.CallExpression, prop: string, isOn: (v: ts.Expression) => boolean) =>
    call.arguments.some((a) => ts.isObjectLiteralExpression(a) && a.properties.some((p) =>
      ts.isPropertyAssignment(p) && p.name.getText() === prop && isOn(p.initializer)))
  const concurrent = (v: ts.Expression) => !(ts.isNumericLiteral(v) && v.text === "1")
  for (const [call] of u.emitted.callInfo) {
    const callee = call.expression
    let what: string | undefined
    if (ts.isIdentifier(callee) && callee.text === "fork" && !f.topNames.has("fork")) what = "fork runs its program on a new fiber"
    else if (ts.isPropertyAccessExpression(callee) && callee.expression.getText(f.sf) === "Effect") {
      const n = callee.name.text
      if (RACING.has(n)) what = `Effect.${n} runs its program on another fiber`
      else if ((n === "forEach" || n === "all") && optionsSay(call, "concurrency", concurrent)) {
        const c = call.arguments.flatMap((a) => ts.isObjectLiteralExpression(a) ? a.properties : []).find((p) => p.name?.getText(f.sf) === "concurrency")
        what = `${n} with concurrency ${c && ts.isPropertyAssignment(c) ? c.initializer.getText(f.sf) : ""} runs its ${n === "forEach" ? "callback" : "programs"} on other fibers`
      } else if (n === "zip" && optionsSay(call, "concurrent", (v) => v.kind === ts.SyntaxKind.TrueKeyword)) what = "zip with concurrent: true runs both programs on other fibers"
    }
    if (!what) continue
    const gen = idx.find(u.emitted.callProbes.get(call), ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
    if (!gen) continue
    gen.arguments.forEach((a, i) => {
      const t = checker.getTypeAtLocation(a)
      const sig = t.getCallSignatures()[0]
      const parts = effectParts(sig ? sig.getReturnType() : t, checker)
      if (!parts) return
      for (const r of typeParts(parts.R)) {
        const name = r.getSymbol()?.name
        if (!name || !controls.has(name)) continue
        diag("ES4201", call.arguments[i] ?? call, `${name} is a control operation, and ${what}, where the ${name} handler on this fiber cannot be reached.`, [
          `Handle ${name} inside the ${sig ? "callback" : "program"}, or run it on this fiber.`
        ])
      }
    })
  }

  // D27: wire's provider graph (ES3202 duplicate, ES3203 cycle, ES3204 opaque).
  for (const [call] of u.emitted.callInfo) {
    if (!ts.isIdentifier(call.expression) || call.expression.text !== "wire" || f.topNames.has("wire")) continue
    const gen = idx.find(u.emitted.callProbes.get(call), ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
    if (!gen) continue
    const g = wireGraph(gen, checker)
    g.args.forEach((a, i) => {
      if (a.opaque) diag("ES3204", call, `${a.text} does not name its outputs in its type, so wire cannot place it.`, ["Compose it with Layer.provide instead."])
      void i
    })
    for (const d of g.duplicates) {
      diag("ES3202", call, `two providers for ${print(d.service, gen)}: ${d.args.map((i) => g.args[i].text).join(" and ")}. Pass one.`)
    }
    if (g.cycle) {
      diag("ES3203", call, `providers form a cycle: ${g.cycle.map((i) => g.args[i].text).join(" → ")}`)
    }
  }

  // ES3201: a runner whose program still needs services (D27: reported where R must be empty).
  for (const [call] of u.emitted.callInfo) {
    const t = call.expression.getText(f.sf)
    if (!/^(Effect\.run(Promise|PromiseExit|Fork|Sync|SyncExit)|NodeRuntime\.runMain|runMain)$/.test(t)) continue
    const gen = idx.find(u.emitted.callProbes.get(call), ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
    const arg = gen?.arguments[0]
    if (!arg) continue
    const parts = effectParts(checker.getTypeAtLocation(arg), checker)
    if (!parts || parts.R.flags & ts.TypeFlags.Never) continue
    const missing = typeParts(parts.R).map((x) => print(x, gen))
    const paths: string[] = []
    for (const [wcall] of u.emitted.callInfo) {
      if (!ts.isIdentifier(wcall.expression) || wcall.expression.text !== "wire") continue
      const wgen = idx.find(u.emitted.callProbes.get(wcall), ts.SyntaxKind.CallExpression) as ts.CallExpression | undefined
      if (!wgen) continue
      for (const a of wireGraph(wgen, checker).args) {
        const needs = a.ins.map((x) => print(x, wgen)).filter((x) => missing.includes(x))
        if (needs.length > 0) paths.push(`${a.text} needs ${needs.join(", ")}`)
      }
    }
    diag("ES3201", call, `no provider for ${missing.join(", ")}.`, paths)
    suppress.add(call)
    for (const a of call.arguments) suppress.add(a)
  }

  // Implementations against their service contract (Chapter 4): ES2001 and ES3101.
  const hasTypeParam = (t: ts.Type): boolean =>
    (t.flags & ts.TypeFlags.TypeParameter) !== 0 || (t.isUnion() && t.types.some(hasTypeParam))
  for (const [obj, span] of u.emitted.objectProbes) {
    const genObj = idx.find(span, ts.SyntaxKind.ObjectLiteralExpression) as ts.ObjectLiteralExpression | undefined
    if (!genObj) continue
    const expected = expectedTypes(genObj, checker)
    if (expected.length !== 1) continue
    const owner = expected[0].getSymbol()?.name ?? "the service"
    for (const p of obj.properties) {
      const member = p.name?.getText(f.sf)
      if (!member || !(ts.isMethodDeclaration(p) || (ts.isPropertyAssignment(p) && (ts.isArrowFunction(p.initializer) || ts.isFunctionExpression(p.initializer))))) continue
      const genMember = genObj.properties.find((q) => q.name?.getText() === member)
      if (!genMember) continue
      const declaredT = propertyTypes(expected[0], member, checker)[0]
      const dsig = declaredT?.getCallSignatures()[0]
      const dparts = dsig ? effectParts(dsig.getReturnType(), checker) : undefined
      const isig = checker.getTypeAtLocation(ts.isPropertyAssignment(genMember) ? genMember.initializer : genMember).getCallSignatures()[0]
      const iparts = isig ? effectParts(isig.getReturnType(), checker) : undefined
      if (!dparts || !iparts || hasTypeParam(dparts.E) || hasTypeParam(dparts.R)) continue
      const holder = enclosingName(obj) ?? "this provider"
      const contract = `${owner}.${member}${checker.signatureToString(dsig!, genObj, ts.TypeFormatFlags.NoTruncation)}`
      const escE = unionParts(iparts.E).filter((x) => !checker.isTypeAssignableTo(x, dparts.E))
      const escR = unionParts(iparts.R).filter((x) => !checker.isTypeAssignableTo(x, dparts.R))
      for (const x of escE) {
        diag("ES2001", p, `${print(x, genObj)} can escape ${holder}.${member}, but ${owner}.${member}'s contract allows only ${print(dparts.E, genObj)}.`, [
          `Handle it, add ${print(x, genObj)} to ${owner}.${member}'s contract, or make it a defect with Effect.orDie.`
        ])
      }
      for (const x of escR) {
        const svc = print(x, genObj)
        diag("ES3101", p, `${holder}.${member} looks up ${svc} each time it is called, but ${contract} allows ${print(dparts.R, genObj) === "never" ? "no call-time requirements" : `only ${print(dparts.R, genObj)}`}.`, [
          `Capture it once when the layer is built: const ${svc.charAt(0).toLowerCase() + svc.slice(1)} = ${svc}()`,
          `or declare it: ${member}(…): Effect<${print(dparts.A, genObj)}, ${print(dparts.E, genObj)}, ${svc}>`
        ])
      }
      if (escE.length > 0 || escR.length > 0) {
        // The same mismatch surfaces as TS2345/TS2322 on the enclosing argument: report it once.
        let n: ts.Node | undefined = obj
        while (n && !ts.isSourceFile(n)) {
          suppress.add(n)
          n = n.parent
          if (n && ts.isVariableStatement(n)) break
        }
      }
    }
  }

  // Claimed contracts: `// Effect<…>` comments in the source (Part I examples).
  for (const c of u.emitted.claims) {
    let report = contracts.find((r) => r.file === f.path && r.name === c.name)
    if (!report) {
      const d = genDecls.get(c.name)
      if (!d) continue
      report = { file: f.path, name: c.name, kind: "const", inferred: print(checker.getTypeAtLocation(d.name), d), direct: false, contract: false }
      contracts.push(report)
    }
    const claim: NonNullable<ContractReport["claim"]> = { text: c.text, line: c.line, status: "text", channels: [] }
    report.claim = claim
    if (!c.alias) continue
    const alias = sf.statements.find((s): s is ts.TypeAliasDeclaration => ts.isTypeAliasDeclaration(s) && s.name.text === c.alias)
    if (!alias) continue
    const aliasDiags = program.getSemanticDiagnostics(sf).filter((d) => d.start !== undefined && d.start >= alias.pos && d.start < alias.end)
    if (aliasDiags.length > 0) {
      // The claim names a type not in scope here: compare normalized text per channel.
      const why = ts.flattenDiagnosticMessageText(aliasDiags[0].messageText, " ")
      const inf = splitEffect(report.inferred)
      const cl = splitEffect(c.text)
      if (c.kind === "effect" && inf && cl) {
        for (const [i, ch] of (["A", "E", "R"] as const).entries()) {
          claim.channels.push({ channel: ch, inferred: inf[i], claimed: cl[i], ok: normType(inf[i]) === normType(cl[i]) })
        }
        claim.status = claim.channels.every((x) => x.ok) ? "match" : "mismatch"
        claim.note = `compared as text: ${why}`
      } else {
        claim.status = "unresolved"
        claim.channels.push({ channel: "type", inferred: report.inferred, claimed: `${c.text}  (${why})`, ok: false })
      }
      continue
    }
    const claimed = checker.getTypeAtLocation(alias.name)
    const isAny = (t: ts.Type) => (t.flags & ts.TypeFlags.Any) !== 0
    const same = (a: ts.Type, b: ts.Type) =>
      isAny(a) === isAny(b) && checker.isTypeAssignableTo(a, b) && checker.isTypeAssignableTo(b, a)
    if (c.kind === "effect") {
      const inf = inferredOf(c.name)
      const cp = effectParts(claimed, checker)
      if (!inf?.parts || !cp) {
        claim.status = "unresolved"
        continue
      }
      for (const ch of ["A", "E", "R"] as const) {
        claim.channels.push({ channel: ch, inferred: print(inf.parts[ch], inf.decl), claimed: print(cp[ch], alias), ok: same(inf.parts[ch], cp[ch]) })
      }
    } else {
      const d = genDecls.get(c.name)!
      const t = checker.getTypeAtLocation(d.name)
      claim.channels.push({ channel: "type", inferred: print(t, d), claimed: print(claimed, alias), ok: same(t, claimed) })
    }
    claim.status = claim.channels.every((x) => x.ok) ? "match" : "mismatch"
  }
  // Signature claims (generic functions): compare normalized text.
  for (const r of contracts) {
    if (r.file !== f.path || !r.claim || r.claim.status !== "text") continue
    const ok = normSig(r.inferred) === normSig(r.claim.text)
    r.claim.channels.push({ channel: "type", inferred: r.inferred, claimed: r.claim.text, ok })
    r.claim.status = ok ? "match" : "mismatch"
    r.claim.note = "signature compared as text"
  }
}

/** Normalize a signature: D12's fresh E$n/R$n names, array spellings, arrow style. */
export const normSig = (text: string) =>
  text
    .replace(/\bE\$\d+\b/g, "E")
    .replace(/\bR\$\d+\b/g, "R")
    .replace(/ReadonlyArray<(\w+)>/g, "readonly $1[]")
    .replace(/Array<(\w+)>/g, "$1[]")
    .replace(/\bEffect\.Effect</g, "Effect<")
    .replace(/\s+/g, " ")
    .trim()

/** The name of the top-level declaration that encloses `n`. */
const enclosingName = (n: ts.Node): string | undefined => {
  let p: ts.Node | undefined = n
  while (p && !ts.isSourceFile(p)) {
    if (ts.isVariableDeclaration(p) && ts.isIdentifier(p.name)) return p.name.text
    if ((ts.isFunctionDeclaration(p) || ts.isClassDeclaration(p)) && p.name) return p.name.text
    if (ts.isPropertyDeclaration(p) && ts.isClassDeclaration(p.parent) && p.parent.name) return `${p.parent.name.text}.${p.name.getText()}`
    p = p.parent
  }
  return undefined
}

/** Split `Effect<A, E, R>` text into its three arguments (defaults E = R = never). */
export const splitEffect = (text: string): [string, string, string] | undefined => {
  const t = text.trim().replace(/^<[^>]*>(?=Effect<)/, "")
  if (!t.startsWith("Effect<") || !t.endsWith(">")) return undefined
  const inner = t.slice("Effect<".length, -1)
  const parts: string[] = []
  let depth = 0
  let cur = ""
  for (const ch of inner) {
    if ("<({[".includes(ch)) depth++
    if (">)}]".includes(ch) && !(ch === ">" && cur.endsWith("="))) depth--
    if (ch === "," && depth === 0) {
      parts.push(cur.trim())
      cur = ""
    } else cur += ch
  }
  parts.push(cur.trim())
  while (parts.length < 3) parts.push("never")
  return parts.length === 3 ? [parts[0], parts[1], parts[2]] : undefined
}

/** Normalize type text: qualifiers, separators, and top-level union order. */
export const normType = (text: string) => {
  const t = text
    .replace(/import\("[^"]*"\)\./g, "")
    .replace(/\bScope\.Scope\b/g, "Scope")
    .replace(/\bEffect\.Effect</g, "Effect<")
    .replace(/;\s*}/g, " }")
    .replace(/\s+/g, " ")
    .trim()
  const members: string[] = []
  let depth = 0
  let cur = ""
  for (const ch of t) {
    if ("<({[".includes(ch)) depth++
    if (">)}]".includes(ch) && !(ch === ">" && cur.endsWith("="))) depth--
    if (ch === "|" && depth === 0) {
      members.push(cur.trim())
      cur = ""
    } else cur += ch
  }
  members.push(cur.trim())
  return members.filter(Boolean).sort().join(" | ")
}

/** The first call or throw in `fn` that contributes `x` to its E or R. */
function findSite(
  u: Unit,
  facts: Facts,
  checker: ts.TypeChecker,
  idx: NodeIndex,
  fn: string,
  channel: "E" | "R",
  x: ts.Type
): { node: ts.Node; what: string } | undefined {
  const hits: Array<{ node: ts.Node; what: string }> = []
  const overlaps = (t: ts.Type | undefined) =>
    t !== undefined && unionParts(t).some((p) => checker.isTypeAssignableTo(p, x) || checker.isTypeAssignableTo(x, p))
  for (const [call, info] of u.emitted.callInfo) {
    if (info.fn !== fn || info.inCatchTry) continue
    const fact = facts.calls.get(call)
    if (!fact) continue
    if (channel === "R" && fact.serviceReceiver && ts.isPropertyAccessExpression(call.expression)) {
      const recv = idx.find(u.emitted.receiverProbes.get(call))
      const rt = recv ? checker.getTypeAtLocation(recv) : undefined
      const ident = rt ? effectParts(rt, checker)?.R : undefined
      if (overlaps(ident)) hits.push({ node: call, what: call.expression.getText(u.file.sf) })
      continue
    }
    if (overlaps(channel === "E" ? fact.effectE : fact.effectR)) hits.push({ node: call, what: call.expression.getText(u.file.sf) })
  }
  if (channel === "E") {
    for (const [st, info] of u.emitted.throwInfo) {
      if (info.fn !== fn || info.inCatchTry) continue
      const n = idx.find(u.emitted.throwProbes.get(st))
      if (n && overlaps(checker.getTypeAtLocation(n))) hits.push({ node: st, what: "throw" })
    }
  }
  hits.sort((a, b) => a.node.getStart() - b.node.getStart() || b.node.end - a.node.end)
  // Prefer the innermost call at the earliest position.
  return hits.sort((a, b) => a.node.end - a.node.getStart() - (b.node.end - b.node.getStart()))[0]
}

export { isEffectType }
