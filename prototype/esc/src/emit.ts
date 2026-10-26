// The lowering emitter: one `.es` file to Effect TypeScript.
//
// Every decision that depends on types (does this call return an Effect? is
// this lambda a program, an Effect function, or a plain callback?) is read
// from `facts`, which the compiler computes by type-checking the previous
// pass's output. With no facts (pass 1) the emitter is optimistic: calls go
// through `$bind`, whose conditional type types the program either way.
import ts from "typescript"
import {
  type Diagnostic,
  type EsFile,
  type Facts,
  HELPER_NAMES,
  PRELUDE_NAMES,
  type ServiceDecl,
  type Span
} from "./model.ts"

export interface Claim {
  node: ts.Node
  name: string
  kind: "effect" | "layer" | "text"
  text: string
  alias?: string
  /** Generated span of the inferred-type probe expression for consts. */
  line: number
}

export interface CallInfo {
  mode: Mode
  boundary: boolean
  fn: string | undefined
  inCatchTry: boolean
}

export interface Emitted {
  text: string
  /** The generated module ends here; claim probes follow (check mode only). */
  bodyEnd: number
  maps: Array<{ gs: number; ge: number; node: ts.Node }>
  callProbes: Map<ts.CallExpression, Span>
  calleeProbes: Map<ts.CallExpression, Span>
  receiverProbes: Map<ts.CallExpression, Span>
  lambdaProbes: Map<ts.Node, Span>
  objectProbes: Map<ts.ObjectLiteralExpression, Span>
  classProbes: Map<ts.ClassDeclaration, Span>
  convProbes: Map<ts.Expression, { expr: Span; site: Span }>
  throwProbes: Map<ts.ThrowStatement, Span>
  forOfProbes: Map<ts.ForOfStatement, Span>
  callInfo: Map<ts.CallExpression, CallInfo>
  throwInfo: Map<ts.ThrowStatement, CallInfo>
  lambdaModes: Map<ts.Node, "program" | "effect" | "plain">
  wireOrders: Map<ts.CallExpression, number[]>
  fnDecls: Map<string, { node: ts.FunctionDeclaration; direct: boolean; contract: boolean }>
  claims: Claim[]
  diagnostics: Diagnostic[]
}

type Mode = "effect" | "plain" | "top"

interface Region {
  ret: boolean
  brk: Set<string>
  cont: Set<string>
}

interface Ctx {
  mode: Mode
  region?: Region
  breakDepth: number
  contDepth: number
  labels: string[]
  boundary: boolean
  inCatchTry: boolean
  fn: string | undefined
  scopeVar?: string
  producer: boolean
  /** Inside a handler clause: `return resume(k, v)` in tail position lowers to the tail shape (D18). */
  clause: boolean
}

export interface EmitOptions {
  file: EsFile
  files: Map<string, EsFile>
  facts: Facts
  runtimeImport: string
  /** Rewrites a relative import specifier of this file. */
  rewriteImport: (spec: string) => string
  check: boolean
}

const isUsingList = (list: ts.VariableDeclarationList) => (list.flags & 7) === 4
const isAwaitUsingList = (list: ts.VariableDeclarationList) => (list.flags & 7) === 6
const isUsingStatement = (s: ts.Statement): s is ts.VariableStatement =>
  ts.isVariableStatement(s) && (isUsingList(s.declarationList) || isAwaitUsingList(s.declarationList))

const hasModifier = (n: ts.Node, kind: ts.SyntaxKind) =>
  ts.canHaveModifiers(n) && (ts.getModifiers(n) ?? []).some((m) => m.kind === kind)

const isFunctionLike = (n: ts.Node) =>
  ts.isArrowFunction(n) ||
  ts.isFunctionExpression(n) ||
  ts.isFunctionDeclaration(n) ||
  ts.isMethodDeclaration(n) ||
  ts.isGetAccessor(n) ||
  ts.isSetAccessor(n) ||
  ts.isConstructorDeclaration(n) ||
  ts.isClassDeclaration(n) ||
  ts.isClassExpression(n)

/** Does `node` (not descending into nested functions) satisfy `pred`? */
export const containsShallow = (node: ts.Node, pred: (n: ts.Node) => boolean): boolean => {
  let found = false
  const visit = (n: ts.Node) => {
    if (found) return
    if (pred(n)) {
      found = true
      return
    }
    if (isFunctionLike(n)) return
    ts.forEachChild(n, visit)
  }
  // The node itself counts (an expression body), then its children.
  if (pred(node)) return true
  ts.forEachChild(node, visit)
  return found
}

/** Syntactic: can this statement complete normally (fall off its end)? Conservative: true when unsure. */
export const canCompleteNormally = (s: ts.Statement | undefined): boolean => {
  if (s === undefined) return true
  if (ts.isReturnStatement(s) || ts.isThrowStatement(s) || ts.isBreakStatement(s) || ts.isContinueStatement(s)) {
    return false
  }
  if (ts.isBlock(s)) return s.statements.every(canCompleteNormally)
  if (ts.isIfStatement(s)) return s.elseStatement === undefined || canCompleteNormally(s.thenStatement) || canCompleteNormally(s.elseStatement)
  if (ts.isTryStatement(s)) {
    const body = canCompleteNormally(s.tryBlock) || (s.catchClause !== undefined && canCompleteNormally(s.catchClause.block))
    return body && (s.finallyBlock === undefined || canCompleteNormally(s.finallyBlock))
  }
  return true
}

const EFFECT_TYPE_REF = /(?<![.\w$])Effect</g
export const rewriteEffectTypeText = (text: string) => text.replace(EFFECT_TYPE_REF, "Effect.Effect<")

export class FileEmitter {
  out = ""
  private readonly throwsImports = new Map<string, string>()
  maps: Emitted["maps"] = []
  callProbes = new Map<ts.CallExpression, Span>()
  calleeProbes = new Map<ts.CallExpression, Span>()
  receiverProbes = new Map<ts.CallExpression, Span>()
  lambdaProbes = new Map<ts.Node, Span>()
  objectProbes = new Map<ts.ObjectLiteralExpression, Span>()
  /** Classes with methods: their `implements` types decide whether a method is an Effect function. */
  classProbes = new Map<ts.ClassDeclaration, Span>()
  convProbes = new Map<ts.Expression, { expr: Span; site: Span }>()
  throwProbes = new Map<ts.ThrowStatement, Span>()
  forOfProbes = new Map<ts.ForOfStatement, Span>()
  callInfo = new Map<ts.CallExpression, CallInfo>()
  throwInfo = new Map<ts.ThrowStatement, CallInfo>()
  lambdaModes = new Map<ts.Node, "program" | "effect" | "plain">()
  wireOrders = new Map<ts.CallExpression, number[]>()
  fnDecls: Emitted["fnDecls"] = new Map()
  claims: Claim[] = []
  diagnostics: Diagnostic[] = []
  private counter = 0
  private readonly sf: ts.SourceFile
  private readonly text: string
  private readonly facts: Facts

  constructor(private readonly o: EmitOptions) {
    this.sf = o.file.sf
    this.text = o.file.text
    this.facts = o.facts
  }

  // ---------------------------------------------------------------------------
  // Output helpers
  // ---------------------------------------------------------------------------

  private push(s: string) {
    this.out += s
  }

  private fresh(base: string) {
    return `${base}${++this.counter}`
  }

  private diag(code: string, node: ts.Node, message: string, severity: "error" | "warning" = "error") {
    this.diagnostics.push({ code, severity, file: this.o.file.path, start: node.getStart(this.sf), end: node.end, message })
  }

  private mapped(node: ts.Node, f: () => void) {
    const gs = this.out.length
    f()
    this.maps.push({ gs, ge: this.out.length, node })
  }

  private src(n: ts.Node) {
    return this.text.slice(n.getStart(this.sf), n.end)
  }

  /** Copy `n`'s source text, re-emitting each child node in `ctx`. */
  private generic(n: ts.Node, ctx: Ctx) {
    this.mapped(n, () => {
      let pos = n.getStart(this.sf)
      ts.forEachChild(n, (child) => {
        const cs = child.getStart(this.sf)
        if (cs < pos) return
        this.push(this.text.slice(pos, cs))
        this.any(child, ctx)
        pos = child.end
      })
      this.push(this.text.slice(pos, n.end))
    })
  }

  private any(n: ts.Node, ctx: Ctx): void {
    switch (n.kind) {
      case ts.SyntaxKind.CallExpression:
        return this.call(n as ts.CallExpression, ctx, false)
      case ts.SyntaxKind.ArrowFunction:
      case ts.SyntaxKind.FunctionExpression:
        return this.lambda(n as ts.ArrowFunction | ts.FunctionExpression, ctx)
      case ts.SyntaxKind.ObjectLiteralExpression:
        return this.objectLiteral(n as ts.ObjectLiteralExpression, ctx)
      case ts.SyntaxKind.ArrayLiteralExpression:
        return this.arrayLiteral(n as ts.ArrayLiteralExpression, ctx)
      case ts.SyntaxKind.TypeReference:
        return this.typeRef(n as ts.TypeReferenceNode, ctx)
      case ts.SyntaxKind.AwaitExpression:
        this.diag("ES1003", n, "`await` is not part of EffectScript: a call already waits. Use Effect.tryPromise for a Promise.")
        return this.generic(n, ctx)
      case ts.SyntaxKind.YieldExpression:
        return this.yieldExpr(n as ts.YieldExpression, ctx)
      case ts.SyntaxKind.Block:
        return this.block(n as ts.Block, ctx)
      case ts.SyntaxKind.ReturnStatement:
        return this.returnStmt(n as ts.ReturnStatement, ctx)
      case ts.SyntaxKind.ThrowStatement:
        return this.throwStmt(n as ts.ThrowStatement, ctx)
      case ts.SyntaxKind.TryStatement:
        return this.tryStmt(n as ts.TryStatement, ctx)
      case ts.SyntaxKind.BreakStatement:
      case ts.SyntaxKind.ContinueStatement:
        return this.jump(n as ts.BreakOrContinueStatement, ctx)
      case ts.SyntaxKind.ForOfStatement:
        return this.forOf(n as ts.ForOfStatement, ctx)
      case ts.SyntaxKind.ForStatement:
      case ts.SyntaxKind.ForInStatement:
      case ts.SyntaxKind.WhileStatement:
      case ts.SyntaxKind.DoStatement:
        return this.generic(n, { ...ctx, breakDepth: ctx.breakDepth + 1, contDepth: ctx.contDepth + 1 })
      case ts.SyntaxKind.SwitchStatement:
        return this.generic(n, { ...ctx, breakDepth: ctx.breakDepth + 1 })
      case ts.SyntaxKind.LabeledStatement:
        return this.generic(n, { ...ctx, labels: [...ctx.labels, (n as ts.LabeledStatement).label.text] })
      case ts.SyntaxKind.FunctionDeclaration:
        return this.nestedFunction(n as ts.FunctionDeclaration, ctx)
      case ts.SyntaxKind.VariableStatement:
        if (isUsingStatement(n as ts.Statement)) {
          // A `using` outside a block we lowered as a region (top level, plain code).
          this.diag("ES1004", n, "`using` is supported only inside an EffectScript function body.")
        }
        return this.generic(n, ctx)
      case ts.SyntaxKind.VariableDeclaration: {
        // Regions move assignments into closures, which TypeScript's definite-assignment
        // analysis cannot follow: `let x: T` becomes `let x!: T` in Effect functions.
        const d = n as ts.VariableDeclaration
        if (ctx.mode === "effect" && !d.initializer && d.type && !d.exclamationToken && ts.isIdentifier(d.name) &&
          ts.isVariableDeclarationList(d.parent) && (d.parent.flags & ts.NodeFlags.Let) !== 0) {
          this.mapped(d, () => this.push(`${d.name.getText(this.sf)}!: ${this.typeText(d.type!)}`))
          return
        }
        return this.generic(n, ctx)
      }
      case ts.SyntaxKind.MethodDeclaration:
        return this.classMethod(n as ts.MethodDeclaration, ctx)
      default:
        return this.generic(n, ctx)
    }
  }

  // ---------------------------------------------------------------------------
  // File
  // ---------------------------------------------------------------------------

  emitFile(): Emitted {
    const f = this.o.file
    const prelude = PRELUDE_NAMES.filter((n) => !f.topNames.has(n))
    this.push(`// Generated by esc from ${f.path.split("/").pop()}. Do not edit.\n`)
    if (prelude.length > 0) this.push(`import { ${prelude.join(", ")} } from ${JSON.stringify(this.o.runtimeImport)}\n`)
    this.push(`import { ${HELPER_NAMES.join(", ")} } from ${JSON.stringify(this.o.runtimeImport)}\n`)
    const top: Ctx = {
      mode: "top",
      breakDepth: 0,
      contDepth: 0,
      labels: [],
      boundary: false,
      inCatchTry: false,
      fn: undefined,
      producer: false,
      clause: false
    }
    let pos = this.sf.statements.length > 0 ? this.sf.statements[0].getFullStart() : 0
    for (const st of this.sf.statements) {
      this.push(this.text.slice(pos, st.getStart(this.sf)))
      this.topStatement(st, top)
      this.terminate(st)
      pos = st.end
    }
    this.push(this.text.slice(pos))
    this.push("\n")
    // Imports are hoisted, so the classes `$trusted` tests can follow the body without moving its spans.
    for (const [key, local] of this.throwsImports) {
      const at = key.lastIndexOf("#")
      this.push(`import { ${key.slice(at + 1)} as ${local} } from ${JSON.stringify(key.slice(0, at))}\n`)
    }
    const bodyEnd = this.out.length
    this.collectClaims()
    if (this.o.check) this.emitClaimProbes()
    return {
      text: this.out,
      bodyEnd,
      maps: this.maps,
      callProbes: this.callProbes,
      calleeProbes: this.calleeProbes,
      receiverProbes: this.receiverProbes,
      lambdaProbes: this.lambdaProbes,
      objectProbes: this.objectProbes,
      classProbes: this.classProbes,
      convProbes: this.convProbes,
      throwProbes: this.throwProbes,
      forOfProbes: this.forOfProbes,
      callInfo: this.callInfo,
      throwInfo: this.throwInfo,
      lambdaModes: this.lambdaModes,
      wireOrders: this.wireOrders,
      fnDecls: this.fnDecls,
      claims: this.claims,
      diagnostics: this.diagnostics
    }
  }

  private topStatement(st: ts.Statement, ctx: Ctx) {
    if (ts.isImportDeclaration(st)) return this.importDecl(st)
    if (ts.isFunctionDeclaration(st) && st.name) return this.topFunction(st)
    if (ts.isClassDeclaration(st) && st.name) {
      const svc = this.o.file.services.get(st.name.text)
      if (svc && svc.node === st) return this.serviceClass(st, svc)
      const s = this.out.length
      this.any(st, ctx)
      if (this.classProbes.has(st)) this.classProbes.set(st, { start: s, end: this.out.length })
      return
    }
    if (ts.isExportDeclaration(st) && st.moduleSpecifier && ts.isStringLiteral(st.moduleSpecifier)) {
      const spec = st.moduleSpecifier
      this.mapped(st, () => {
        this.push(this.text.slice(st.getStart(this.sf), spec.getStart(this.sf)))
        this.push(JSON.stringify(this.o.rewriteImport(spec.text)))
        this.push(this.text.slice(spec.end, st.end))
      })
      return
    }
    this.any(st, ctx)
  }

  private importDecl(st: ts.ImportDeclaration) {
    const spec = st.moduleSpecifier as ts.StringLiteral
    const target = this.o.rewriteImport(spec.text)
    const esFile = this.esTarget(spec.text)
    this.mapped(st, () => {
      const clause = st.importClause
      // Add `name$direct` imports for direct functions of another .es module (D10).
      if (esFile && clause?.namedBindings && ts.isNamedImports(clause.namedBindings) && !clause.isTypeOnly) {
        const extra: string[] = []
        for (const el of clause.namedBindings.elements) {
          const imported = (el.propertyName ?? el.name).text
          if (this.facts.direct.has(`${esFile.path}#${imported}`)) {
            extra.push(`${imported}$direct${el.propertyName ? ` as ${el.name.text}$direct` : ""}`)
          }
        }
        this.push(this.text.slice(st.getStart(this.sf), clause.namedBindings.elements.end))
        if (extra.length > 0) this.push(`, ${extra.join(", ")}`)
        this.push(this.text.slice(clause.namedBindings.elements.end, spec.getStart(this.sf)))
      } else {
        this.push(this.text.slice(st.getStart(this.sf), spec.getStart(this.sf)))
      }
      this.push(JSON.stringify(target))
      this.push(this.text.slice(spec.end, st.end))
    })
  }

  /** A module specifier for `file` relative to this file's generated output. */
  private relTo(file: string) {
    const from = this.o.file.outPath.slice(0, this.o.file.outPath.lastIndexOf("/"))
    const a = from.split("/")
    const b = file.split("/")
    let i = 0
    while (i < a.length && i < b.length && a[i] === b[i]) i++
    const rel = [...a.slice(i).map(() => ".."), ...b.slice(i)].join("/")
    return rel.startsWith(".") ? rel : `./${rel}`
  }

  private esTarget(spec: string): EsFile | undefined {
    if (!spec.startsWith(".") || !spec.endsWith(".es")) return undefined
    const dir = this.o.file.path.slice(0, this.o.file.path.lastIndexOf("/"))
    const abs = new URL(spec, `file://${dir}/`).pathname
    return this.o.files.get(abs)
  }

  // ---------------------------------------------------------------------------
  // Functions
  // ---------------------------------------------------------------------------

  private exportPrefix(n: ts.Node) {
    return hasModifier(n, ts.SyntaxKind.ExportKeyword) ? "export " : ""
  }

  /** `Effect<A, E, R>` (or `Effect.Effect<…>`) with three type arguments. */
  static effectTypeArgs(t: ts.TypeNode | undefined): readonly ts.TypeNode[] | undefined {
    if (!t || !ts.isTypeReferenceNode(t)) return undefined
    const name = t.typeName
    const isEffect = (ts.isIdentifier(name) && name.text === "Effect") ||
      (ts.isQualifiedName(name) && name.right.text === "Effect" && ts.isIdentifier(name.left) && name.left.text === "Effect")
    return isEffect ? (t.typeArguments ?? []) : undefined
  }

  private typeText(t: ts.Node) {
    const start = this.out.length
    this.any(t, this.topCtx())
    const s = this.out.slice(start)
    this.out = this.out.slice(0, start)
    return s
  }

  private topCtx(): Ctx {
    return {
      mode: "top",
      breakDepth: 0,
      contDepth: 0,
      labels: [],
      boundary: false,
      inCatchTry: false,
      fn: undefined,
      producer: false,
      clause: false
    }
  }

  /** Emit a node into a string without recording it (for text reused twice). */
  private capture(f: () => void) {
    const start = this.out.length
    const maps = this.maps.length
    f()
    const s = this.out.slice(start)
    this.out = this.out.slice(0, start)
    this.maps.length = maps
    return s
  }

  /**
   * D12: a callback parameter whose function type has no Effect contract
   * accepts any contract. `(a: A) => B` becomes `(a: A) => Effect<B, E$n, R$n>`
   * with fresh type parameters on the enclosing function.
   */
  private d12(fn: ts.FunctionDeclaration) {
    const extra: string[] = []
    const params = fn.parameters.map((p) => {
      const t = p.type
      if (t && ts.isFunctionTypeNode(t) && !FileEmitter.effectTypeArgs(t.type)) {
        const n = extra.length / 2 + 1
        extra.push(`E$${n}`, `R$${n}`)
        const head = this.capture(() => {
          this.push(this.text.slice(p.getStart(this.sf), t.getStart(this.sf)))
        })
        const tps = t.typeParameters ? `<${t.typeParameters.map((tp) => this.typeText(tp)).join(", ")}>` : ""
        const ps = t.parameters.map((q) => this.typeText(q)).join(", ")
        return `${head}${tps}(${ps}) => Effect.Effect<${this.typeText(t.type)}, E$${n}, R$${n}>${this.text.slice(t.end, p.end)}`
      }
      return this.typeText(p)
    })
    const tps = [...(fn.typeParameters ?? []).map((tp) => this.typeText(tp)), ...extra]
    return { params: params.join(", "), typeParams: tps.length > 0 ? `<${tps.join(", ")}>` : "", rewritten: extra.length > 0 }
  }

  private topFunction(fn: ts.FunctionDeclaration) {
    const name = fn.name!.text
    const exp = this.exportPrefix(fn)
    if (hasModifier(fn, ts.SyntaxKind.DefaultKeyword)) {
      this.diag("ES9001", fn, "`export default function` is not supported by the prototype; use a named export.")
    }
    if (hasModifier(fn, ts.SyntaxKind.AsyncKeyword)) {
      this.diag("ES1003", fn, "`async` functions are not part of EffectScript: a call already waits.")
    }
    const key = `${this.o.file.path}#${name}`
    const contractArgs = FileEmitter.effectTypeArgs(fn.type)
    if (fn.type && !contractArgs) {
      this.diag("ES2003", fn.type, `${name}'s return annotation must be a full contract Effect<A, E, R> (prototype rule).`)
    }
    if (!fn.body) {
      // A declaration without a body states its full contract (Chapter 12).
      if (!contractArgs || contractArgs.length !== 3) {
        this.diag("ES2101", fn, `${name} has no body to infer from, so its contract must be written in full: Effect<A, E, R>.`)
      }
      const { params, typeParams } = this.d12(fn)
      this.mapped(fn, () => {
        this.push(`${exp}declare const ${name}: ${typeParams}(${params}) => ${fn.type ? this.typeText(fn.type) : "Effect.Effect<unknown, unknown, unknown>"}`)
      })
      return
    }
    if (fn.asteriskToken) return this.producerFunction(fn, name, exp)
    const direct = this.facts.direct.has(key)
    this.fnDecls.set(name, { node: fn, direct, contract: contractArgs !== undefined })
    const { params, typeParams } = this.d12(fn)
    const contract = contractArgs && contractArgs.length === 3
      ? `Effect.Effect<${contractArgs.map((a) => this.typeText(a)).join(", ")}>`
      : undefined
    const annotation = contract ? `: ${typeParams}(${params}) => ${contract}` : ""
    const ctx: Ctx = { ...this.topCtx(), mode: direct ? "plain" : "effect", fn: name }
    this.mapped(fn, () => {
      if (direct) {
        const ret = contractArgs && contractArgs.length === 3 ? `: ${this.typeText(contractArgs[0])}` : ""
        this.push(`${exp}function ${name}$direct${typeParams}(${params})${ret} {`)
        this.fnBody(fn.body!, ctx)
        this.push("}\n")
        const simple = fn.parameters.every((p) => ts.isIdentifier(p.name) && !p.dotDotDotToken)
        const args = simple ? fn.parameters.map((p) => (p.name as ts.Identifier).text).join(", ") : "...args"
        const wparams = simple ? params : `...args: Parameters<typeof ${name}$direct>`
        this.push(`${exp}const ${name}${annotation} = ${typeParams}(${wparams}) => Effect.sync(() => ${name}$direct(${args}))`)
      } else {
        // A recursive function without a written contract: recursive calls go
        // through a forward binding typed with the previous pass's inference,
        // so E and R are a fixpoint over the cycle (Chapter 12).
        const rec = this.recBinding(fn, name)
        if (rec) this.push(`let ${name}$rec: ${typeParams}(${params}) => Effect.Effect<${rec.A}, ${rec.E}, ${rec.R}>;\n`)
        this.push(`${exp}const ${name}${annotation} = Effect.fnUntraced(function* ${typeParams}(${params}) {`)
        this.fnBody(fn.body!, ctx)
        this.push("})")
        if (rec) this.push(`;\n${name}$rec = ${name} as any`)
      }
    })
  }

  /** The forward binding's contract for a recursive function, or undefined when it needs none. */
  private recBinding(fn: ts.FunctionDeclaration, name: string) {
    if (!this.o.file.recursive.has(name) || FileEmitter.effectTypeArgs(fn.type) || fn.typeParameters) return undefined
    return this.facts.recTypes.get(`${this.o.file.path}#${name}`) ?? { A: "any", E: "never", R: "never" }
  }

  private recTarget(callee: ts.Expression): string | undefined {
    if (!ts.isIdentifier(callee)) return undefined
    const fn = this.o.file.functions.get(callee.text)
    if (!fn || this.facts.direct.has(`${this.o.file.path}#${callee.text}`)) return undefined
    return this.recBinding(fn, callee.text) ? `${callee.text}$rec` : undefined
  }

  /** A function declaration inside a body: keeps hoisting; Effect functions return a generator Effect. */
  private nestedFunction(fn: ts.FunctionDeclaration, ctx: Ctx) {
    if (!fn.body || !fn.name) return this.generic(fn, ctx)
    const mode = this.lambdaKind(fn, ctx)
    this.lambdaModes.set(fn, mode === "program" ? "effect" : mode)
    const tps = fn.typeParameters ? `<${fn.typeParameters.map((t) => this.typeText(t)).join(", ")}>` : ""
    const ps = fn.parameters.map((p) => this.typeText(p)).join(", ")
    this.mapped(fn, () => {
      if (mode === "plain") {
        this.push(`function ${fn.name!.text}${tps}(${ps}) {`)
        this.fnBody(fn.body!, { ...this.innerCtx(ctx), mode: "plain" })
        this.push("}")
      } else {
        this.push(`function ${fn.name!.text}${tps}(${ps}) { return Effect.gen(function* () {`)
        this.fnBody(fn.body!, { ...this.innerCtx(ctx), mode: "effect" })
        this.push("}) }")
      }
    })
  }

  private innerCtx(ctx: Ctx): Ctx {
    return { ...ctx, region: undefined, breakDepth: 0, contDepth: 0, labels: [], scopeVar: undefined, producer: false, clause: false }
  }

  private fnBody(body: ts.ConciseBody, ctx: Ctx) {
    if (!ts.isBlock(body)) {
      this.push(" return ")
      if (ctx.mode === "effect" && ctx.clause && this.resumeKind(body)) this.tailResume(body as ts.CallExpression, ctx)
      else this.any(body, ctx)
      this.push("; ")
      return
    }
    const stmts = body.statements
    if (ctx.mode === "effect" && needsScope(stmts)) {
      // The whole body is the Scope region: returns stay direct.
      const s = this.fresh("$scope")
      this.push(` return yield* $region((${s}) => Effect.gen(function* () {`)
      this.statements(stmts, { ...ctx, scopeVar: s }, body.statements.pos)
      this.push(" }));")
      return
    }
    this.statements(stmts, ctx, body.statements.pos)
  }

  private statements(stmts: ts.NodeArray<ts.Statement>, ctx: Ctx, from: number) {
    let pos = from
    for (const st of stmts) {
      this.push(this.text.slice(pos, st.getStart(this.sf)))
      if (isUsingStatement(st) && ctx.mode === "effect" && ctx.scopeVar) this.usingDecl(st, ctx)
      else this.any(st, ctx)
      this.terminate(st)
      pos = st.end
    }
    // Trailing whitespace and comments before the closing brace.
    const parent = stmts.length > 0 ? stmts[0].parent : undefined
    const close = parent && ts.isBlock(parent) ? parent.end - 1 : pos
    if (close > pos) this.push(this.text.slice(pos, close))
  }

  /** Generated statements may begin with `(`: end each one explicitly so ASI cannot join them. */
  private terminate(st: ts.Statement) {
    if (
      ts.isExpressionStatement(st) || ts.isVariableStatement(st) || ts.isReturnStatement(st) || ts.isThrowStatement(st) ||
      ts.isBreakStatement(st) || ts.isContinueStatement(st) || ts.isDoStatement(st)
    ) {
      if (!/[;}]\s*$/.test(this.out.slice(-4))) this.push(";")
      else if (/}\s*$/.test(this.out.slice(-2)) && !this.out.trimEnd().endsWith(";")) this.push(";")
    }
  }

  // ---------------------------------------------------------------------------
  // Statements in Effect functions
  // ---------------------------------------------------------------------------

  private block(b: ts.Block, ctx: Ctx) {
    if (ctx.mode !== "effect" || !needsScope(b.statements)) {
      this.mapped(b, () => {
        this.push("{")
        this.statements(b.statements, ctx, b.statements.pos)
        this.push("}")
      })
      return
    }
    // A block with `using`: a Scope region with completion records (D14).
    const c = this.fresh("$c")
    const s = this.fresh("$scope")
    const region: Region = { ret: false, brk: new Set(), cont: new Set() }
    const normal = b.statements.every(canCompleteNormally)
    this.mapped(b, () => {
      this.push(`{ const ${c} = yield* $region((${s}) => Effect.gen(function* () {`)
      this.statements(b.statements, { ...ctx, region, breakDepth: 0, contDepth: 0, labels: [], scopeVar: s }, b.statements.pos)
      if (normal) this.push(" return $normal;")
      this.push(" }));")
      this.dispatch(c, region, ctx, normal)
      this.push(" }")
    })
  }

  private usingDecl(st: ts.VariableStatement, ctx: Ctx) {
    if (isAwaitUsingList(st.declarationList)) {
      this.diag("ES1003", st, "`await using` is not part of EffectScript; `using` already waits.")
    }
    this.mapped(st, () => {
      for (const d of st.declarationList.declarations) {
        this.push(`const ${this.src(d.name)} = yield* $using(${ctx.scopeVar}, `)
        const init = d.initializer
        if (init && ts.isCallExpression(init)) {
          const f = this.facts.calls.get(init)
          const raw = f && f.type === "plain" && !f.serviceReceiver
          if (raw) this.push("$bind(")
          this.call(init, ctx, true)
          if (raw) this.push(")")
        } else if (init) {
          this.push("$bind(")
          this.any(init, ctx)
          this.push(")")
        } else {
          this.push("$bind(undefined)")
        }
        this.push(");")
      }
    })
  }

  private returnStmt(st: ts.ReturnStatement, ctx: Ctx) {
    if (ctx.mode !== "effect" || !ctx.region) {
      if (ctx.mode === "effect" && ctx.clause && st.expression && this.resumeKind(st.expression)) {
        this.mapped(st, () => {
          this.push("return ")
          this.tailResume(st.expression as ts.CallExpression, ctx)
          this.push(";")
        })
        return
      }
      return this.generic(st, ctx)
    }
    ctx.region.ret = true
    this.mapped(st, () => {
      this.push(`return { kind: "return" as const, value: `)
      if (st.expression) this.any(st.expression, ctx)
      else this.push("undefined")
      this.push(" };")
    })
  }

  private throwStmt(st: ts.ThrowStatement, ctx: Ctx) {
    if (ctx.mode !== "effect") return this.generic(st, ctx)
    this.throwInfo.set(st, { mode: ctx.mode, boundary: ctx.boundary, fn: ctx.fn, inCatchTry: ctx.inCatchTry })
    this.mapped(st, () => {
      this.push("return yield* Effect.fail(")
      const s = this.out.length
      this.any(st.expression, ctx)
      this.throwProbes.set(st, { start: s, end: this.out.length })
      this.push(");")
    })
  }

  private jump(st: ts.BreakOrContinueStatement, ctx: Ctx) {
    const label = st.label?.text ?? ""
    const isBreak = ts.isBreakStatement(st)
    const local = label !== "" ? ctx.labels.includes(label) : isBreak ? ctx.breakDepth > 0 : ctx.contDepth > 0
    if (ctx.mode !== "effect" || !ctx.region || local) return this.generic(st, ctx)
    ;(isBreak ? ctx.region.brk : ctx.region.cont).add(label)
    this.mapped(st, () => {
      this.push(`return { kind: ${isBreak ? '"break"' : '"continue"'} as const, label: ${JSON.stringify(label)} as const };`)
    })
  }

  /** Dispatch a region's completion record in the enclosing frame. */
  private dispatch(c: string, region: Region, ctx: Ctx, canNormal: boolean) {
    const parts: string[] = []
    if (region.ret) {
      if (ctx.region) {
        ctx.region.ret = true
        parts.push(`if (${c}.kind === "return") return ${c};`)
      } else parts.push(`if (${c}.kind === "return") return ${c}.value;`)
    }
    for (const [kind, labels] of [["break", region.brk], ["continue", region.cont]] as const) {
      for (const label of labels) {
        const local = label !== "" ? ctx.labels.includes(label) : kind === "break" ? ctx.breakDepth > 0 : ctx.contDepth > 0
        const test = `${c}.kind === "${kind}" && ${c}.label === ${JSON.stringify(label)}`
        if (local || !ctx.region) parts.push(`if (${test}) ${kind}${label ? ` ${label}` : ""};`)
        else {
          ;(kind === "break" ? ctx.region.brk : ctx.region.cont).add(label)
          parts.push(`if (${test}) return ${c};`)
        }
      }
    }
    if (!canNormal) parts.push(`return ${c} as never;`)
    if (parts.length > 0) this.push(" " + parts.join(" "))
  }

  /**
   * `try`/`catch`/`finally` compile to Effect frames, never to a JavaScript
   * `try` (Chapter 24). `catch` handles expected failures only (D13).
   */
  private tryStmt(st: ts.TryStatement, ctx: Ctx) {
    if (ctx.mode !== "effect") return this.generic(st, ctx)
    const c = this.fresh("$c")
    const region: Region = { ret: false, brk: new Set(), cont: new Set() }
    const inner: Ctx = { ...ctx, region, breakDepth: 0, contDepth: 0, labels: [], scopeVar: undefined }
    const tryNormal = canCompleteNormally(st.tryBlock)
    const catchNormal = st.catchClause ? canCompleteNormally(st.catchClause.block) : false
    const normal = (tryNormal || catchNormal) && canCompleteNormally(st.finallyBlock)
    this.mapped(st, () => {
      this.push(`const ${c} = yield* `)
      if (st.finallyBlock) this.push("$finally(")
      if (st.catchClause) this.push("$catch(")
      this.push("Effect.gen(function* () {")
      this.regionBody(st.tryBlock, { ...inner, inCatchTry: ctx.inCatchTry || st.catchClause !== undefined })
      if (tryNormal) this.push(" return $normal;")
      this.push(" })")
      if (st.catchClause) {
        const cc = st.catchClause
        const param = cc.variableDeclaration ? this.typeText(cc.variableDeclaration) : "_e"
        this.push(`, (${param}) => Effect.gen(function* () {`)
        this.regionBody(cc.block, inner)
        if (catchNormal) this.push(" return $normal;")
        this.push(" }))")
      }
      if (st.finallyBlock) {
        const fin: Region = { ret: false, brk: new Set(), cont: new Set() }
        this.push(", Effect.gen(function* () {")
        this.regionBody(st.finallyBlock, { ...ctx, region: fin, breakDepth: 0, contDepth: 0, labels: [], scopeVar: undefined })
        this.push(" }))")
        if (fin.ret || fin.brk.size > 0 || fin.cont.size > 0) {
          this.diag("ES9002", st.finallyBlock, "return, break, or continue out of a finally block is not supported by the prototype.")
        }
      }
      this.push(";")
      this.dispatch(c, region, ctx, normal)
    })
  }

  /** A region generator's statements; a `using` inside opens its own Scope region. */
  private regionBody(b: ts.Block, ctx: Ctx) {
    if (needsScope(b.statements)) {
      this.block(b, ctx)
      return
    }
    this.statements(b.statements, ctx, b.statements.pos)
  }

  // ---------------------------------------------------------------------------
  // Calls
  // ---------------------------------------------------------------------------

  private syntacticService(e: ts.Expression): ServiceDecl | undefined {
    if (!ts.isIdentifier(e)) return undefined
    const local = this.o.file.services.get(e.text)
    if (local) return local
    const imp = this.o.file.esImports.get(e.text)
    if (imp) return this.o.files.get(imp.file)?.services.get(imp.name)
    return undefined
  }

  /** `resume(k, v)` or `resumeWith(k, p)` from the prelude. */
  private resumeKind(e: ts.Expression): "resume" | "resumeWith" | undefined {
    if (!ts.isCallExpression(e) || !ts.isIdentifier(e.expression)) return undefined
    const t = e.expression.text
    if ((t === "resume" || t === "resumeWith") && !this.o.file.topNames.has(t)) return t
    return undefined
  }

  private directName(call: ts.CallExpression): string | undefined {
    const f = this.facts.calls.get(call)
    if (!f?.esCallee || !ts.isIdentifier(call.expression)) return undefined
    if (!this.facts.direct.has(`${f.esCallee.file}#${f.esCallee.name}`)) return undefined
    return `${call.expression.text}$direct`
  }

  /**
   * A call. In an Effect function, a call whose type is an Effect runs it
   * (`yield*`), a plain call stays a call, and an undecided call goes through
   * `$bind`. `raw` emits the call expression without running it (for `using`).
   */
  private call(call: ts.CallExpression, ctx: Ctx, raw: boolean): void {
    if (this.specialCall(call, ctx)) return
    const f = this.facts.calls.get(call)
    const callee = call.expression
    this.callInfo.set(call, { mode: ctx.mode, boundary: ctx.boundary, fn: ctx.fn, inCatchTry: ctx.inCatchTry })
    const svcSelf = f ? f.calleeEffectValue : this.syntacticService(callee) !== undefined
    const gs = this.out.length
    if (svcSelf && call.arguments.length === 0) {
      // `p()` where `p` is an Effect value (a service key included): run it.
      if (ctx.mode === "effect" && !raw) {
        this.push("(yield* ")
        const s = this.out.length
        const recvSvc = f?.serviceReceiver ?? (ts.isPropertyAccessExpression(callee) && this.syntacticService(callee.expression) !== undefined)
        if (ts.isPropertyAccessExpression(callee)) {
          // `S.member()` where the member is an Effect value: `yield* (yield* S).member`.
          if (recvSvc) this.push("(yield* ")
          const r = this.out.length
          this.any(callee.expression, ctx)
          this.receiverProbes.set(call, { start: r, end: this.out.length })
          if (recvSvc) this.push(")")
          this.push(`.${callee.name.text}`)
        } else this.any(callee, ctx)
        this.calleeProbes.set(call, { start: s, end: this.out.length })
        this.push(")")
      } else {
        const s = this.out.length
        this.any(callee, ctx)
        this.calleeProbes.set(call, { start: s, end: this.out.length })
      }
      this.maps.push({ gs, ge: this.out.length, node: call })
      return
    }
    const direct = this.directName(call)
    const type = f?.type ?? "unknown"
    const trusted = ctx.mode === "effect" && !raw && f?.throws ? f.throws : undefined
    const wrap = trusted ? "trusted" : ctx.mode === "effect" && !raw && !direct ? (type === "effect" ? "yield" : type === "unknown" ? "bind" : "none") : "none"
    if (wrap === "yield") this.push("(yield* ")
    else if (wrap === "bind") this.push("(yield* $bind(")
    else if (trusted) {
      // D23: the declared exceptions become typed failures; any other exception stays a defect.
      this.push(`(yield* $trusted(() => `)
    }
    const rs = this.out.length
    const serviceRecv = f ? f.serviceReceiver : ts.isPropertyAccessExpression(callee) && this.syntacticService(callee.expression) !== undefined
    const cs = this.out.length
    const rec = direct ? undefined : this.recTarget(callee)
    if (direct) {
      this.push(direct)
    } else if (rec) {
      this.push(rec)
    } else if (ts.isPropertyAccessExpression(callee) && ctx.mode === "effect") {
      if (serviceRecv) this.push("(yield* ")
      const s = this.out.length
      this.any(callee.expression, ctx)
      this.receiverProbes.set(call, { start: s, end: this.out.length })
      if (serviceRecv) this.push(")")
      this.push(this.text.slice(callee.expression.end, callee.name.getStart(this.sf)).trim() || ".")
      this.push(callee.name.text)
    } else if (ts.isPropertyAccessExpression(callee)) {
      const s = this.out.length
      this.any(callee.expression, ctx)
      this.receiverProbes.set(call, { start: s, end: this.out.length })
      this.push(this.text.slice(callee.expression.end, callee.name.getStart(this.sf)).trim() || ".")
      this.push(callee.name.text)
    } else {
      this.any(callee, ctx)
    }
    this.calleeProbes.set(call, { start: cs, end: this.out.length })
    if (call.typeArguments) this.push(`<${call.typeArguments.map((t) => this.typeText(t)).join(", ")}>`)
    this.push(call.questionDotToken ? "?.(" : "(")
    const boundary = ctx.boundary || this.isBoundaryCallee(callee)
    const argCtx = boundary === ctx.boundary ? ctx : { ...ctx, boundary }
    // D27: `wire` arguments in dependency order (consumers first), from the previous pass's Layer types.
    const isWire = ts.isIdentifier(callee) && callee.text === "wire" && !this.o.file.topNames.has("wire")
    const order = isWire ? this.facts.wireOrders.get(call) : undefined
    if (isWire) this.wireOrders.set(call, order ?? call.arguments.map((_, i) => i))
    const args = order ? order.map((i) => call.arguments[i]) : [...call.arguments]
    args.forEach((a, i) => {
      if (i > 0) this.push(", ")
      this.argLike(a, argCtx)
    })
    this.push(")")
    this.callProbes.set(call, { start: rs, end: this.out.length })
    if (wrap === "yield") this.push(")")
    else if (wrap === "bind") this.push("))")
    else if (trusted) this.push(`, [${trusted.map((t) => this.throwsClass(t)).join(", ")}]))`)
    this.maps.push({ gs, ge: this.out.length, node: call })
  }

  /** D51: a `@throws` class, imported from the trusted function's module so `$trusted` can test `instanceof`. */
  private throwsClass(t: { name: string; file: string }) {
    const key = `${this.relTo(t.file)}#${t.name}`
    let local = this.throwsImports.get(key)
    if (!local) {
      const taken = this.o.file.topNames.has(t.name) || [...this.throwsImports.values()].includes(t.name)
      local = taken ? `$throws${this.throwsImports.size}_${t.name}` : t.name
      this.throwsImports.set(key, local)
    }
    return local
  }

  /** D5, D23: `Effect.try`, `Effect.tryPromise`, `Effect.sync`, and `Effect.promise` are the explicit boundary to plain code. */
  private isBoundaryCallee(callee: ts.Expression) {
    return ts.isPropertyAccessExpression(callee) && ts.isIdentifier(callee.expression) &&
      callee.expression.text === "Effect" && ["try", "tryPromise", "sync", "promise"].includes(callee.name.text)
  }

  /** Arguments, array elements, and property values: where program conversion (D7) and D9 apply. */
  private argLike(a: ts.Expression, ctx: Ctx) {
    const conv = this.facts.convs.get(a)
    const candidate = ts.isIdentifier(a) || ts.isPropertyAccessExpression(a) || ts.isElementAccessExpression(a) || ts.isCallExpression(a)
    if (!candidate) return this.any(a, ctx)
    if (conv?.stream || (conv?.programs === undefined && false)) {
      const site = this.out.length
      this.push("$toStream(")
      const s = this.out.length
      this.any(a, ctx)
      const e = this.out.length
      this.push(")")
      this.convProbes.set(a, { expr: { start: s, end: e }, site: { start: site, end: this.out.length } })
      return
    }
    if (conv?.programs) {
      const site = this.out.length
      this.push("$programs(")
      const s = this.out.length
      this.any(a, ctx)
      const e = this.out.length
      this.push(")")
      this.convProbes.set(a, { expr: { start: s, end: e }, site: { start: site, end: this.out.length } })
      return
    }
    if (conv?.program) {
      const site = this.out.length
      this.push("$program(")
      const s = this.out.length
      this.any(a, ctx)
      const e = this.out.length
      this.push(")")
      this.convProbes.set(a, { expr: { start: s, end: e }, site: { start: site, end: this.out.length } })
      return
    }
    if (conv?.direct && ts.isIdentifier(a)) {
      const s = this.out.length
      this.mapped(a, () => this.push(`${a.text}$direct`))
      this.convProbes.set(a, { expr: { start: s, end: this.out.length }, site: { start: s, end: this.out.length } })
      return
    }
    if (conv?.rejectPlain) {
      this.diag("ES4301", a, `${this.src(a)} can pause, but it is passed where a plain JavaScript function is expected, which calls it synchronously.`)
    }
    const s = this.out.length
    this.any(a, ctx)
    this.convProbes.set(a, { expr: { start: s, end: this.out.length }, site: { start: s, end: this.out.length } })
  }

  private arrayLiteral(arr: ts.ArrayLiteralExpression, ctx: Ctx) {
    this.mapped(arr, () => {
      this.push("[")
      arr.elements.forEach((el, i) => {
        if (i > 0) this.push(", ")
        if (ts.isSpreadElement(el)) {
          this.push("...")
          this.any(el.expression, ctx)
        } else this.argLike(el, ctx)
      })
      this.push("]")
    })
  }

  private objectLiteral(obj: ts.ObjectLiteralExpression, ctx: Ctx) {
    const s = this.out.length
    this.mapped(obj, () => {
      this.push("{")
      obj.properties.forEach((p, i) => {
        if (i > 0) this.push(",")
        this.push(" ")
        if (ts.isPropertyAssignment(p)) {
          this.mapped(p, () => {
            this.push(this.src(p.name))
            this.push(": ")
            this.argLike(p.initializer, ctx)
          })
        } else if (ts.isMethodDeclaration(p)) {
          this.objectMethod(p, obj, ctx)
        } else {
          this.any(p, ctx)
        }
      })
      this.push(" }")
    })
    this.objectProbes.set(obj, { start: s, end: this.out.length })
  }

  // ---------------------------------------------------------------------------
  // Lambdas
  // ---------------------------------------------------------------------------

  private syntacticNeedsEffect(fn: ts.SignatureDeclaration & { body?: ts.ConciseBody }) {
    if (!fn.body) return false
    return containsShallow(fn.body, (n) => ts.isCallExpression(n) || ts.isThrowStatement(n) || ts.isTryStatement(n) || isUsingStatementNode(n))
  }

  /** Decide how a lambda compiles: a program (D7), an Effect function, or a plain callback. */
  private lambdaKind(fn: ts.SignatureDeclaration & { body?: ts.ConciseBody }, ctx: Ctx): "program" | "effect" | "plain" {
    const lf = this.facts.lambdas.get(fn)
    const noParams = fn.parameters.length === 0
    if (this.facts.pipePrograms.has(fn as unknown as ts.Expression) && noParams) return "program"
    if (!lf) return this.syntacticNeedsEffect(fn) ? "effect" : "plain"
    if (lf.ctx === "program" && noParams) return "program"
    if (lf.ctx === "program") return "effect"
    if (lf.ctx === "effectFn") return "effect"
    if (lf.ctx === "plainFn") {
      if (lf.needsEffect) {
        this.diag(
          "ES4301",
          fn,
          `this callback can pause (${lf.pauses ?? "an Effect call"}), but ${lf.ctxText ?? "its caller"} calls it synchronously. Use Effect.forEach or a for...of loop.`
        )
        this.diagnostics[this.diagnostics.length - 1].covers = fn
      }
      return "plain"
    }
    return lf.needsEffect ? "effect" : "plain"
  }

  private lambdaParams(fn: ts.ArrowFunction | ts.FunctionExpression | ts.MethodDeclaration, keepReturn: boolean) {
    const tps = fn.typeParameters ? `<${fn.typeParameters.map((t) => this.typeText(t)).join(", ")}>` : ""
    const ps = fn.parameters.map((p) => this.typeText(p)).join(", ")
    const ret = keepReturn && fn.type ? `: ${this.typeText(fn.type)}` : ""
    return `${tps}(${ps})${ret}`
  }

  private genOpen(fn: ts.Node) {
    const usesThis = containsShallowThis(fn)
    return usesThis ? "Effect.gen({ self: this }, function* () {" : "Effect.gen(function* () {"
  }

  private lambda(fn: ts.ArrowFunction | ts.FunctionExpression, ctx: Ctx) {
    const kind = this.lambdaKind(fn, ctx)
    this.lambdaModes.set(fn, kind)
    if (hasModifier(fn, ts.SyntaxKind.AsyncKeyword)) {
      this.diag("ES1003", fn, "`async` functions are not part of EffectScript: a call already waits.")
    }
    const s = this.out.length
    const clause = isClause(fn)
    const inner: Ctx = { ...this.innerCtx(ctx), mode: kind === "plain" ? "plain" : "effect", clause }
    this.mapped(fn, () => {
      if (kind === "program") {
        this.push(this.genOpen(fn))
        this.fnBody(fn.body, inner)
        this.push("})")
      } else if (kind === "effect") {
        const contract = FileEmitter.effectTypeArgs(fn.type) !== undefined
        if (ts.isArrowFunction(fn)) this.push(`${this.lambdaParams(fn, contract)} => `)
        else this.push(`function${fn.name ? " " + fn.name.text : ""}${this.lambdaParams(fn, contract)} { return `)
        this.push(this.genOpen(fn))
        this.fnBody(fn.body, inner)
        this.push("})")
        if (!ts.isArrowFunction(fn)) this.push(" }")
      } else {
        // Plain: copy, with calls left as calls.
        if (ts.isArrowFunction(fn)) {
          this.push(`${this.lambdaParams(fn, true)} =>`)
          if (ts.isBlock(fn.body)) this.block(fn.body, inner)
          else {
            this.push(" ")
            const needsParens = ts.isObjectLiteralExpression(skipParens(fn.body))
            if (needsParens) this.push("(")
            this.any(fn.body, inner)
            if (needsParens) this.push(")")
          }
        } else {
          this.push(`function${fn.name ? " " + fn.name.text : ""}${this.lambdaParams(fn, true)} `)
          this.block(fn.body, inner)
        }
      }
    })
    this.lambdaProbes.set(fn, { start: s, end: this.out.length })
  }

  /** A method in an object literal: an Effect function when its expected type returns an Effect. */
  private objectMethod(m: ts.MethodDeclaration, obj: ts.ObjectLiteralExpression, ctx: Ctx) {
    if (!m.body) return this.any(m, ctx)
    const kind = this.lambdaKind(m, ctx)
    this.lambdaModes.set(m, kind)
    const clause = isClause(m)
    const inner: Ctx = { ...this.innerCtx(ctx), mode: kind === "plain" ? "plain" : "effect", clause }
    this.mapped(m, () => {
      this.push(`${this.src(m.name)}${this.lambdaParams(m, kind === "plain" || FileEmitter.effectTypeArgs(m.type) !== undefined)} {`)
      if (kind === "plain") {
        this.statements(m.body!.statements, inner, m.body!.statements.pos)
      } else {
        this.push(" return ")
        this.push(this.genOpen(m))
        this.fnBody(m.body!, inner)
        this.push("})")
        this.push(";")
      }
      this.push(" }")
    })
  }

  /** A method of an ordinary class: an Effect function when its body needs one. */
  private classMethod(m: ts.MethodDeclaration, ctx: Ctx) {
    if (!m.body || !ts.isClassLike(m.parent)) return this.generic(m, ctx)
    const cls = m.parent
    if (ts.isClassDeclaration(cls) && !this.classProbes.has(cls)) this.classProbes.set(cls, { start: 0, end: 0 })
    const kind = this.lambdaKind(m, ctx)
    this.lambdaModes.set(m, kind)
    if (kind === "plain") return this.generic(m, { ...this.innerCtx(ctx), mode: "plain" })
    const mods = (ts.getModifiers(m) ?? []).map((x) => x.getText(this.sf)).join(" ")
    this.mapped(m, () => {
      this.push(`${mods ? mods + " " : ""}${this.src(m.name)}${this.lambdaParams(m, FileEmitter.effectTypeArgs(m.type) !== undefined)} { return ${this.genOpen(m)}`)
      this.fnBody(m.body!, { ...this.innerCtx(ctx), mode: "effect" })
      this.push("}) }")
    })
  }

  // ---------------------------------------------------------------------------
  // Types
  // ---------------------------------------------------------------------------

  private typeRef(t: ts.TypeReferenceNode, ctx: Ctx) {
    if (ts.isIdentifier(t.typeName) && t.typeName.text === "Effect") {
      this.mapped(t, () => {
        this.push("Effect.Effect")
        if (t.typeArguments) this.push(`<${t.typeArguments.map((a) => this.typeText(a)).join(", ")}>`)
      })
      return
    }
    this.generic(t, ctx)
  }

  // ---------------------------------------------------------------------------
  // Services and control operations (D1, D2)
  // ---------------------------------------------------------------------------

  /**
   * D1: a service class compiles to an interface (the class name as a type is
   * the implementation's shape, so `implements Users` and `db: Database` work)
   * merged with a class whose static side is an Effect 4 Context key. An
   * optional brand carrying the key keeps two services with the same members
   * distinct in R (the structural collapse in tests/service-identity/probe.ts).
   */
  private serviceClass(cls: ts.ClassDeclaration, svc: ServiceDecl) {
    const name = svc.name
    const exp = this.exportPrefix(cls)
    const ops: string[] = []
    this.mapped(cls, () => {
      this.push(`${exp}interface ${name} {`)
      for (const m of cls.members) {
        if (hasModifier(m, ts.SyntaxKind.StaticKeyword)) continue
        if (ts.isMethodDeclaration(m)) {
          const args = FileEmitter.effectTypeArgs(m.type)
          if (!args || args.length !== 3) {
            this.diag(
              "ES2101",
              m,
              `${name}.${this.src(m.name)} has no body to infer from, so its contract must be written in full: Effect<A, E, R>.`
            )
          }
          ops.push(this.src(m.name))
          this.mapped(m, () => {
            const tps = m.typeParameters ? `<${m.typeParameters.map((t) => this.typeText(t)).join(", ")}>` : ""
            const ps = m.parameters.map((p) => this.typeText(p)).join(", ")
            this.push(`\n  ${this.src(m.name)}${tps}(${ps}): ${m.type ? this.typeText(m.type) : "unknown"};`)
          })
        } else if (ts.isPropertyDeclaration(m)) {
          const ro = hasModifier(m, ts.SyntaxKind.ReadonlyKeyword) ? "readonly " : ""
          this.mapped(m, () => {
            this.push(`\n  ${ro}${this.src(m.name)}${m.questionToken ? "?" : ""}: ${m.type ? this.typeText(m.type) : "unknown"};`)
          })
        }
      }
      this.push(`\n  readonly "~esc/service"?: ${JSON.stringify(svc.key)};\n}\n`)
      this.push(`${exp}class ${name} extends $service<${name}>(${JSON.stringify(svc.key)}) {`)
      for (const m of cls.members) {
        if (!hasModifier(m, ts.SyntaxKind.StaticKeyword)) continue
        this.push("\n  ")
        this.any(m, this.topCtx())
      }
      this.push("\n}")
      if (svc.kind === "control") this.push(`\n$control(${name}, ${JSON.stringify(ops)});`)
    })
  }

  // ---------------------------------------------------------------------------
  // Handlers and producers (Chapters 8, 9)
  // ---------------------------------------------------------------------------

  /** Built-ins with their own lowering. Returns true when handled. */
  private specialCall(call: ts.CallExpression, ctx: Ctx): boolean {
    const callee = call.expression
    if (!ts.isIdentifier(callee) || this.o.file.topNames.has(callee.text)) return false
    if (callee.text === "fork" && ctx.mode === "effect") {
      // D19: block-owned fiber. Attach to the innermost `using` Scope region, else the caller's Scope.
      this.callInfo.set(call, { mode: ctx.mode, boundary: ctx.boundary, fn: ctx.fn, inCatchTry: ctx.inCatchTry })
      this.mapped(call, () => {
        this.push(`(yield* `)
        const s = this.out.length
        this.push(`$fork(`)
        call.arguments.forEach((a, i) => {
          if (i > 0) this.push(", ")
          this.argLike(a, ctx)
        })
        this.push(ctx.scopeVar ? `, ${ctx.scopeVar})` : ")")
        this.callProbes.set(call, { start: s, end: this.out.length })
        this.push(")")
      })
      return true
    }
    return false
  }

  /**
   * D18: a clause's `return resume(k, v)` outside every region returns the
   * kernel's tail-resume request as the clause's value; the kernel performs it
   * after the clause's generator frame is gone (KERNEL-CHANGES.md section 1).
   */
  private tailResume(call: ts.CallExpression, ctx: Ctx) {
    const kind = this.resumeKind(call)!
    this.callInfo.set(call, { mode: ctx.mode, boundary: ctx.boundary, fn: ctx.fn, inCatchTry: ctx.inCatchTry })
    this.mapped(call, () => {
      this.push(kind === "resume" ? "$tailResume(" : "$tailResumeWith(")
      call.arguments.forEach((a, i) => {
        if (i > 0) this.push(", ")
        this.argLike(a, ctx)
      })
      this.push(")")
    })
  }

  private yieldExpr(y: ts.YieldExpression, ctx: Ctx) {
    if (!ctx.producer || ctx.mode !== "effect") {
      this.diag("ES1002", y, "`yield` is allowed only directly inside a producer (function*).")
      return this.generic(y, ctx)
    }
    if (y.asteriskToken) {
      this.diag("ES9003", y, "`yield*` in a producer is not supported by the prototype; use for...of and yield each value.")
    }
    this.mapped(y, () => {
      this.push("(yield* $yield(")
      if (y.expression) this.any(y.expression, ctx)
      else this.push("undefined")
      this.push("))")
    })
  }

  /** `function*` is a producer: a cold program performing Yield once per element (Chapter 24). */
  private producerFunction(fn: ts.FunctionDeclaration, name: string, exp: string) {
    this.fnDecls.set(name, { node: fn, direct: false, contract: false })
    const { params, typeParams } = this.d12(fn)
    const ctx: Ctx = { ...this.topCtx(), mode: "effect", fn: name, producer: true }
    this.mapped(fn, () => {
      this.push(`${exp}const ${name} = ${typeParams}(${params}) => $produce(Effect.fnUntraced(function* () {`)
      this.fnBody(fn.body!, ctx)
      this.push("}))")
    })
  }

  /** `for (x of producer)`: internal iteration under a Yield handler; plain iterables keep JavaScript's meaning. */
  private forOf(st: ts.ForOfStatement, ctx: Ctx) {
    const isProducer = this.facts.producerLoops.has(st) || this.facts.streamLoops.has(st)
    if (ctx.mode !== "effect" || !isProducer) {
      // JavaScript's loop. Record the iterable's span so the next pass can tell a producer from an iterable.
      const loop: Ctx = { ...ctx, breakDepth: ctx.breakDepth + 1, contDepth: ctx.contDepth + 1 }
      this.mapped(st, () => {
        this.push(`for ${st.awaitModifier ? "await " : ""}(`)
        this.any(st.initializer, ctx)
        this.push(" of ")
        const s = this.out.length
        this.any(st.expression, ctx)
        this.forOfProbes.set(st, { start: s, end: this.out.length })
        this.push(") ")
        this.any(st.statement, loop)
      })
      return
    }
    if (st.awaitModifier) this.diag("ES1003", st, "`for await` is not part of EffectScript.")
    const c = this.fresh("$c")
    const region: Region = { ret: false, brk: new Set(), cont: new Set() }
    const decl = st.initializer
    const binding = ts.isVariableDeclarationList(decl) ? this.src(decl.declarations[0].name) : this.src(decl)
    const label = ts.isLabeledStatement(st.parent) ? st.parent.label.text : undefined
    this.mapped(st, () => {
      this.push(`const ${c} = yield* ${this.facts.streamLoops.has(st) ? "$forStream" : "$forOf"}(`)
      const s = this.out.length
      this.any(st.expression, ctx)
      this.forOfProbes.set(st, { start: s, end: this.out.length })
      this.push(`, (${binding}) => Effect.gen(function* () {`)
      // The loop body is a region: its own break/continue complete the clause.
      const body: Ctx = { ...ctx, region, breakDepth: 0, contDepth: 0, labels: [], scopeVar: undefined }
      if (ts.isBlock(st.statement)) this.regionBody(st.statement, body)
      else {
        this.any(st.statement, body)
        this.push(";")
      }
      this.push(` return $normal; })${label ? `, ${JSON.stringify(label)}` : ""});`)
      // break/continue that target this loop are handled by $forOf.
      const own = (l: string) => l === "" || l === label
      const outer: Region = {
        ret: region.ret,
        brk: new Set([...region.brk].filter((l) => !own(l))),
        cont: new Set([...region.cont].filter((l) => !own(l)))
      }
      this.dispatch(c, outer, ctx, true)
    })
  }

  // ---------------------------------------------------------------------------
  // Claimed contracts: `// Effect<…>` comments after declarations
  // ---------------------------------------------------------------------------

  private collectClaims() {
    for (const st of this.sf.statements) {
      let name: string | undefined
      if (ts.isFunctionDeclaration(st) && st.name) name = st.name.text
      else if (ts.isVariableStatement(st) && st.declarationList.declarations.length === 1) {
        const d = st.declarationList.declarations[0]
        if (ts.isIdentifier(d.name)) name = d.name.text
      }
      if (!name) continue
      const comments = ts.getLeadingCommentRanges(this.text, st.end) ?? []
      const endLine = this.sf.getLineAndCharacterOfPosition(st.end).line
      for (const c of comments) {
        if (c.kind !== ts.SyntaxKind.SingleLineCommentTrivia) continue
        const line = this.sf.getLineAndCharacterOfPosition(c.pos).line
        if (line > endLine + 1) break
        const text = this.text.slice(c.pos + 2, c.end).trim()
        const kind = text.startsWith("Effect<") ? "effect" : text.startsWith("Layer<") ? "layer" : text.startsWith("<") || text.startsWith("(") ? "text" : undefined
        if (kind) this.claims.push({ node: st, name, kind, text, line: line + 1 })
        break
      }
    }
  }

  private emitClaimProbes() {
    this.push("\n// ---- contract claim probes (check only) ----\n")
    this.claims.forEach((c, i) => {
      if (c.kind === "text") return
      const alias = `$claim${i}`
      c.alias = alias
      const t = c.kind === "layer" ? c.text.replace(/^Layer</, "Layer.Layer<") : c.text
      this.push(`type ${alias} = ${rewriteEffectTypeText(t)};\n`)
    })
  }
}

/** A handler clause: a method or property of the object literal passed as `handle`'s third argument. */
export const isClause = (fn: ts.Node): boolean => {
  const obj = ts.isMethodDeclaration(fn) ? fn.parent : ts.isPropertyAssignment(fn.parent) ? fn.parent.parent : undefined
  if (!obj || !ts.isObjectLiteralExpression(obj)) return false
  const call = obj.parent
  return ts.isCallExpression(call) && ts.isIdentifier(call.expression) && call.expression.text === "handle" && call.arguments[2] === obj
}

/** A fork call in this statement, not looking into nested blocks or functions (D19). */
const containsFork = (st: ts.Node): boolean => {
  let found = false
  const visit = (n: ts.Node) => {
    if (found || ts.isBlock(n) || isFunctionLikeNode(n)) return
    if (ts.isCallExpression(n) && ts.isIdentifier(n.expression) && n.expression.text === "fork") {
      found = true
      return
    }
    ts.forEachChild(n, visit)
  }
  ts.forEachChild(st, visit)
  return found
}

const isFunctionLikeNode = (n: ts.Node) =>
  ts.isArrowFunction(n) || ts.isFunctionExpression(n) || ts.isFunctionDeclaration(n) || ts.isMethodDeclaration(n) || ts.isClassLike(n)

/** A block is a Scope region when it declares `using` or forks (D14, D19). */
export const needsScope = (stmts: readonly ts.Statement[]) => stmts.some((st) => isUsingStatement(st) || containsFork(st))

const skipParens = (e: ts.Expression): ts.Expression => (ts.isParenthesizedExpression(e) ? skipParens(e.expression) : e)

const isUsingStatementNode = (n: ts.Node) => ts.isVariableStatement(n) && isUsingStatement(n)

/** Does the function use `this` (looking through arrows, which share it)? */
const containsShallowThis = (fn: ts.Node): boolean => {
  let found = false
  const visit = (n: ts.Node) => {
    if (found) return
    if (n.kind === ts.SyntaxKind.ThisKeyword) {
      found = true
      return
    }
    if (ts.isFunctionExpression(n) || ts.isFunctionDeclaration(n) || ts.isClassLike(n) || ts.isMethodDeclaration(n)) return
    ts.forEachChild(n, visit)
  }
  const body = (fn as ts.FunctionLikeDeclaration).body
  if (body) visit(body)
  return found
}
