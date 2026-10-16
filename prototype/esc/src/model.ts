// Shared data model for the EffectScript prototype compiler.
import ts from "typescript"

export interface Span {
  start: number
  end: number
}

/** One `.es` source file in the compile set. */
export interface EsFile {
  path: string // absolute .es path
  outPath: string // absolute generated .ts path
  text: string
  sf: ts.SourceFile
  /** Top-level function declarations by name. */
  functions: Map<string, ts.FunctionDeclaration>
  /** Service and control classes declared in this file (D1, D2). */
  services: Map<string, ServiceDecl>
  /** Names declared or imported at top level (prelude names are skipped when shadowed). */
  topNames: Set<string>
  /** Top-level functions that reach themselves through calls in this file. */
  recursive: Set<string>
  /** Imported .es bindings: local name -> { file, name }. */
  esImports: Map<string, { file: string; name: string }>
}

export interface ServiceDecl {
  name: string
  node: ts.ClassDeclaration
  kind: "service" | "control"
  key: string
}

export type CallType = "effect" | "plain" | "unknown"

export type Origin = "lib" | "platform" | "clock" | "effect" | "es" | "ts" | "unknown"

export interface CallFact {
  type: CallType
  /** The callee is a value of Effect type that is not callable: `p()` runs `p`. */
  calleeEffectValue: boolean
  /** `S.m(...)` where `S` is a Context key and `m` is a member of its service shape. */
  serviceReceiver: boolean
  /** Resolved top-level `.es` function the call targets, if any. */
  esCallee?: { file: string; name: string }
  origin: Origin
  calleeText: string
  /** D23: exception classes a trusted TypeScript function declares with `@throws {T}`; they join E. */
  throws?: Array<{ name: string; file: string }>
  /** The callee's type is a control-operation-requiring TypeScript function (D20); unused yet. */
  effectE?: ts.Type
  effectR?: ts.Type
  /** True when the call is an ordinary TS call whose type is an Effect we did not produce. */
}

export type LambdaContext = "program" | "effectFn" | "plainFn" | "none"

export interface LambdaFact {
  ctx: LambdaContext
  /** The body (not nested functions) performs an Effect operation. */
  needsEffect: boolean
  /** The first operation that can pause, for messages. */
  pauses?: string
  /** For effectFn contexts: the contextual Effect return type (for contract checks of implementations). */
  ctxReturn?: ts.Type
  ctxText?: string
}

export interface ConvFact {
  /** Wrap in `$program(...)`: a zero-argument function value where an Effect is expected (D7). */
  program: boolean
  /**
   * Prototype extension (not in the proposal): an array of programs where an
   * iterable of Effects is expected, as in `Effect.all(stores.map(makeSync))`.
   */
  programs?: boolean
  /** D26: a producer where a Stream is expected. */
  stream?: boolean
  /** Pass the `$direct` variant: an `.es` function value where a plain function is expected (D9). */
  direct: boolean
  /** An `.es` function that cannot compile to plain JavaScript passed where a plain function is expected (ES4301). */
  rejectPlain: boolean
}

export interface Facts {
  calls: Map<ts.CallExpression, CallFact>
  lambdas: Map<ts.Node, LambdaFact>
  convs: Map<ts.Expression, ConvFact>
  /** `for...of` statements whose iterable is a producer. */
  producerLoops: Set<ts.ForOfStatement>
  /** `for...of` statements whose iterable is an Effect Stream (D26). */
  streamLoops: Set<ts.ForOfStatement>
  /** `.es` functions that compile to plain JavaScript (D10), keyed `${file}#${name}`. */
  direct: Set<string>
  /** Pipe calls whose first argument is a program (D8). */
  pipePrograms: Set<ts.Expression>
  /** Recursive functions: the contract inferred in the previous pass, as type text (fixpoint over E and R). */
  recTypes: Map<string, { A: string; E: string; R: string }>
  /** D27: `wire(...)` argument order derived from Layer types (consumers first), as source indexes. */
  wireOrders: Map<ts.CallExpression, number[]>
}

export const emptyFacts = (): Facts => ({
  calls: new Map(),
  lambdas: new Map(),
  convs: new Map(),
  producerLoops: new Set(),
  streamLoops: new Set(),
  direct: new Set(),
  pipePrograms: new Set(),
  recTypes: new Map(),
  wireOrders: new Map()
})

export interface Diagnostic {
  code: string
  severity: "error" | "warning"
  file: string
  start: number
  end: number
  message: string
  notes?: string[]
  /** TypeScript errors inside this node restate this diagnostic; they are not reported. */
  covers?: ts.Node
}

export const PRELUDE_NAMES = [
  "Effect",
  "Schema",
  "Layer",
  "TaggedError",
  "Control",
  "pipe",
  "wire",
  "fork",
  "handle",
  "resume",
  "resumeWith",
  "abandon"
] as const

export const HELPER_NAMES = [
  "$bind",
  "$catch",
  "$finally",
  "$region",
  "$using",
  "$normal",
  "$program",
  "$trusted",
  "$programs",
  "$Context",
  "$service",
  "$control",
  "$produce",
  "$yield",
  "$forOf",
  "$forStream",
  "$toStream",
  "$tailResume",
  "$tailResumeWith",
  "$fork"
] as const
