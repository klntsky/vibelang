// EffectScript prototype runtime: the prelude (DECISIONS D4) and the helpers
// generated code calls. Generated files import this module by relative path.
// Handlers and producers run on the continuation kernel in experiments/kernel
// (imported, never edited).
import { Context, Data, Effect, Exit, Fiber, Layer, Queue, Schema, Scope, Stream } from "effect"
import { pipe } from "effect/Function"
import * as K from "../../../packages/runtime/src/kernel/kernel.ts"

export { Effect, Layer, Schema, pipe }
export const $Context = Context

/** D3: `TaggedError` is Effect's `Data.TaggedError`. */
export const TaggedError = Data.TaggedError

// ---------------------------------------------------------------------------
// Calls and programs
// ---------------------------------------------------------------------------

/** The optimistic first-pass call: run it when it is an Effect, else succeed with it. */
export type Bind<T> = [T] extends [Effect.Effect<infer A, infer E, infer R>] ? Effect.Effect<A, E, R> : Effect.Effect<T, never, never>
export const $bind = <T>(value: T): Bind<T> => (Effect.isEffect(value) ? value : Effect.succeed(value)) as any

/** A program value: a function with no arguments where an Effect is expected (D7). */
export const $program = <A, E, R>(thunk: () => Effect.Effect<A, E, R>): Effect.Effect<A, E, R> => Effect.suspend(thunk)

// ---------------------------------------------------------------------------
// Regions (Chapter 24) and completion records
// ---------------------------------------------------------------------------

export const $normal = { kind: "normal" } as const

/** D13: `catch` recovers a cause made only of expected failures (the kernel's `catchExpected`). */
export const $catch: <A, E, R, B, E2, R2>(
  self: Effect.Effect<A, E, R>,
  handler: (error: E) => Effect.Effect<B, E2, R2>
) => Effect.Effect<A | B, E2, R | R2> = K.catchExpected

/** `finally`: runs on every exit; a failure inside it is never dropped (D15; onExit merges causes). */
export const $finally = <A, E, R, X, E2, R2>(
  self: Effect.Effect<A, E, R>,
  finalizer: Effect.Effect<X, E2, R2>
): Effect.Effect<A, E | E2, R | R2> => Effect.onExit(self, () => Effect.asVoid(finalizer))

/** A block with `using` or `fork`: a Scope region whose Scope is not put in the Context (D14). */
export const $region = <A, E, R>(body: (scope: Scope.Scope) => Effect.Effect<A, E, R>): Effect.Effect<A, E, R> =>
  Effect.scopedWith(body)

/**
 * `using x = acquire()`: only this acquisition sees the block's Scope (D14,
 * Codex R8). A TypeScript `Disposable` is released with `Symbol.dispose`.
 */
export const $using = <A, E, R>(scope: Scope.Scope, acquire: Effect.Effect<A, E, R>): Effect.Effect<A, E, Exclude<R, Scope.Scope>> =>
  Effect.tap(Scope.provide(acquire, scope), (value: any) =>
    value !== null && typeof value === "object" && typeof value[Symbol.dispose] === "function"
      ? Scope.addFinalizer(scope, Effect.sync(() => value[Symbol.dispose]()))
      : Effect.void)

/**
 * D19: `fork` is block-owned. Inside a Scope region the child is interrupted
 * when the block exits; outside one, `Scope` joins the caller's R.
 */
export function $fork<A, E, R>(program: Effect.Effect<A, E, R>, scope: Scope.Scope): Effect.Effect<Fiber.Fiber<A, E>, never, R>
export function $fork<A, E, R>(program: Effect.Effect<A, E, R>): Effect.Effect<Fiber.Fiber<A, E>, never, R | Scope.Scope>
export function $fork(program: Effect.Effect<any, any, any>, scope?: Scope.Scope): Effect.Effect<any, any, any> {
  return scope ? Effect.forkIn(program, scope) : Effect.forkScoped(program)
}

/** The prelude's `fork`, for TypeScript callers and type positions. */
export const fork = Effect.forkScoped

// ---------------------------------------------------------------------------
// wire (D27): prototype runtime. Arguments must be listed dependents first;
// the compiler intrinsic that orders them from Layer types is not built.
// ---------------------------------------------------------------------------

type LOut<L> = L extends Layer.Layer<infer O, any, any> ? O : never
type LErr<L> = L extends Layer.Layer<any, infer E, any> ? E : never
type LIn<L> = L extends Layer.Layer<any, any, infer I> ? I : never

export const wire = <const L extends readonly [Layer.Any, ...Array<Layer.Any>]>(
  ...layers: L
): Layer.Layer<LOut<L[number]>, LErr<L[number]>, Exclude<LIn<L[number]>, LOut<L[number]>>> => {
  let acc = layers[layers.length - 1] as Layer.Layer<any, any, any>
  for (let i = layers.length - 2; i >= 0; i--) acc = Layer.provideMerge(layers[i] as Layer.Layer<any, any, any>, acc)
  return acc as any
}

// ---------------------------------------------------------------------------
// Control operations and handlers (D2, D16-D18), on the continuation kernel
// ---------------------------------------------------------------------------

/** D2: control operations are abstract classes that `extend Control`; the compiler rewrites them. */
export abstract class Control {}

interface ControlInfo {
  readonly ops: Record<string, K.Operation<any, any, any>>
  readonly impl: Record<string, (...args: Array<any>) => Effect.Effect<any, any, any>>
}
const controls = new WeakMap<object, ControlInfo>()

/** Registers a generated control class: one kernel operation per abstract method. */
export const $control = (cls: Context.Key<any, any>, methods: ReadonlyArray<string>) => {
  const ops: Record<string, K.Operation<any, any, any>> = {}
  const impl: Record<string, (...args: Array<any>) => Effect.Effect<any, any, any>> = {}
  for (const m of methods) {
    const op = K.operation<Array<any>, any, any>(`${cls.key}.${m}`)
    ops[m] = op
    impl[m] = (...args) => op(args)
  }
  controls.set(cls, { ops, impl })
}

declare const ContinuationBrand: unique symbol
/** The rest of a handled program, paused at an operation that returns X. Usable only inside its clause. */
export interface Continuation<X, A, E> {
  readonly [ContinuationBrand]: { readonly x: (_: X) => void; readonly a: A; readonly e: E }
}

type Clauses<S, A, E> = {
  readonly [M in keyof S]?: S[M] extends (...args: infer P) => Effect.Effect<infer X, any, any>
    ? (...args: [...P, k: Continuation<X, A, E>]) => Effect.Effect<any, any, any>
    : never
}
type ClauseOut<C, A> = {
  [M in keyof C]: C[M] extends (...args: any) => Effect.Effect<infer B, infer E2, infer R2>
    ? { a: B extends K.TailResume ? A : B; e: E2; r: R2 }
    : never
}[keyof C]
type Out<O> = O extends { a: infer A } ? A : never
type OutE<O> = O extends { e: infer E } ? E : never
type OutR<O> = O extends { r: infer R } ? R : never

/**
 * `handle(program, Control, clauses, onReturn?)`: runs `program` with a deep
 * handler for `Control`'s operations. Each clause receives the operation's
 * arguments and `k`, the rest of the program.
 */
export function handle<A, E, R, I, S, C extends Clauses<S, A, E>>(
  program: Effect.Effect<A, E, R>,
  control: Context.Key<I, S>,
  clauses: C
): Effect.Effect<A | Out<ClauseOut<C, A>>, E | OutE<ClauseOut<C, A>>, Exclude<R, I> | OutR<ClauseOut<C, A>>>
export function handle<A, E, R, I, S, C extends Clauses<S, A, E>, B, EB, RB>(
  program: Effect.Effect<A, E, R>,
  control: Context.Key<I, S>,
  clauses: C,
  onReturn: (value: A) => Effect.Effect<B, EB, RB>
): Effect.Effect<B | Out<ClauseOut<C, A>>, E | EB | OutE<ClauseOut<C, A>>, Exclude<R, I> | RB | OutR<ClauseOut<C, A>>>
export function handle(
  program: Effect.Effect<any, any, any>,
  control: Context.Key<any, any>,
  clauses: Record<string, (...args: Array<any>) => Effect.Effect<any, any, any>>,
  onReturn?: (value: any) => Effect.Effect<any, any, any>
): Effect.Effect<any, any, any> {
  const info = controls.get(control)
  if (!info) return Effect.die(new Error(`${String((control as any).key)} is not a control operation`))
  const ops: Array<K.Operation<any, any, any>> = []
  const kclauses: Record<string, (payload: any, k: K.Continuation) => Effect.Effect<any, any, any>> = {}
  for (const [m, op] of Object.entries(info.ops)) {
    const clause = clauses[m]
    if (!clause) continue
    ops.push(op)
    kclauses[op.name] = (payload: Array<any>, k) => clause(...payload, k)
  }
  return K.handle(String((control as any).key), ops, Effect.provideService(program, control, info.impl as any), kclauses, onReturn)
}

export const resume: <X, A, E>(k: Continuation<X, A, E>, value: X) => Effect.Effect<A, E, never> = K.resume as any

/** D17: the injected program's R is `never`. */
export const resumeWith: <X, A, E, E2>(k: Continuation<X, A, E>, program: Effect.Effect<X, E2, never>) => Effect.Effect<A, E | E2, never> =
  K.resumeWith as any

export const abandon: <X, A, E>(k: Continuation<X, A, E>) => Effect.Effect<void> = K.abandon as any

/** D18: a clause's tail `resume` is returned as a request, never yielded (KERNEL-CHANGES.md section 1). */
export const $tailResume: <X, A, E>(k: Continuation<X, A, E>, value: X) => K.TailResume = K.tailResume as any
export const $tailResumeWith: <X, A, E, E2>(k: Continuation<X, A, E>, program: Effect.Effect<X, E2, never>) => K.TailResume =
  K.tailResumeWith as any

// ---------------------------------------------------------------------------
// Producers (Chapter 8, D26): function* performs Yield; for...of handles it
// ---------------------------------------------------------------------------

/** The phantom requirement a producer body has until a loop handles its Yield. */
export interface YieldOf<T> {
  readonly "~esc/Yield": T
}
const YieldOp = K.operation<unknown, void>("esc/Yield")

export const $yield = <T>(value: T): Effect.Effect<void, never, YieldOf<T>> => YieldOp(value) as any

export class Producer<T, E, R> {
  declare readonly "~esc/Producer": { readonly t: T; readonly e: E; readonly r: R }
  constructor(readonly body: () => Effect.Effect<unknown, E, any>) {}
}

type ElementOf<R> = R extends YieldOf<infer T> ? T : never

/** A `function*` body becomes a cold producer: calling it runs none of the body. */
export const $produce = <E, R>(body: () => Effect.Effect<unknown, E, R>): Producer<ElementOf<R>, E, Exclude<R, YieldOf<any>>> =>
  new Producer(body)

type Completion = { readonly kind: string; readonly label?: string }

/**
 * `for (const x of producer)`: internal iteration. The loop body is a region
 * returning a completion record; `normal` and this loop's `continue` resume
 * the producer in tail position (D18); `break` and anything that leaves the
 * loop return from the clause, so the producer is abandoned and its cleanup
 * runs before the loop exits.
 */
export const $forOf = <T, E, R, C extends Completion, E2, R2>(
  producer: Producer<T, E, R>,
  body: (value: T) => Effect.Effect<C, E2, R2>,
  label?: string
): Effect.Effect<Exclude<C, { readonly kind: "break" | "continue"; readonly label: "" }> | typeof $normal, E | E2, R | R2> => {
  const own = (c: Completion) => c.label === "" || c.label === label
  return K.handle("for-of", [YieldOp], producer.body(), {
    [YieldOp.name]: (value: any, k: K.Continuation) =>
      Effect.map(body(value), (c: Completion) =>
        c.kind === "normal" || (c.kind === "continue" && own(c))
          ? K.tailResume(k, undefined)
          : c.kind === "break" && own(c)
          ? $normal
          : c)
  }, () => Effect.succeed($normal)) as any
}

// ---------------------------------------------------------------------------
// Running examples
// ---------------------------------------------------------------------------

export const runMain = <A, E>(program: Effect.Effect<A, E, never>) => Effect.runPromiseExit(program)
export { Exit }

/** Prototype extension: an array of programs where an iterable of Effects is expected. */
export const $programs = <P extends () => Effect.Effect<any, any, any>>(programs: Iterable<P>): Array<ReturnType<P>> =>
  Array.from(programs, (p) => Effect.suspend(p)) as Array<ReturnType<P>>

/**
 * D1: the base of a generated service class. Its static side is an Effect 4
 * Context key whose identifier and shape are both the service's interface;
 * its instance side is empty, so the merged interface is the class's type.
 */
export const $service = <I>(key: string): (abstract new () => {}) & Context.Key<I, I> =>
  Context.Service<I, I>()(key) as any

/**
 * D23: a call to a trusted TypeScript function whose JSDoc names `@throws {T}`.
 * An exception whose tag or class name is listed becomes a typed failure;
 * any other exception is a defect.
 */
export const $trusted = <E>() => <A>(thunk: () => A, names: ReadonlyArray<string>): Effect.Effect<A, E> =>
  Effect.suspend(() => {
    try {
      return Effect.succeed(thunk())
    } catch (e: any) {
      return names.some((n) => e?._tag === n || e?.constructor?.name === n) ? Effect.fail(e as E) : Effect.die(e)
    }
  })

// ---------------------------------------------------------------------------
// Streams (D26)
// ---------------------------------------------------------------------------

/**
 * `for (const x of stream)`: pulls the Stream; `break`, `return`, or an error
 * in the body stops pulling, which closes the stream.
 */
export const $forStream = <T, E, R, C extends Completion, E2, R2>(
  stream: Stream.Stream<T, E, R>,
  body: (value: T) => Effect.Effect<C, E2, R2>,
  label?: string
): Effect.Effect<Exclude<C, { readonly kind: "break" | "continue"; readonly label: "" }> | typeof $normal, E | E2, R | R2> =>
  Effect.suspend(() => {
    const own = (c: Completion) => c.label === "" || c.label === label
    let result: Completion = $normal
    return Effect.as(
      Stream.runForEachWhile(stream, (value: T) =>
        Effect.map(body(value), (c: Completion) => {
          if (c.kind === "normal" || (c.kind === "continue" && own(c))) return true
          result = c.kind === "break" && own(c) ? $normal : c
          return false
        })),
      undefined
    ).pipe(Effect.map(() => result as any))
  })

/**
 * D26: a producer where a Stream is expected. The producer runs on the
 * stream's own fiber and blocks on a one-element buffer until the stream
 * pulls; closing the stream interrupts that fiber, so the producer's cleanup runs.
 */
export const $toStream = <T, E, R>(producer: Producer<T, E, R>): Stream.Stream<T, E, R> =>
  Stream.callback<T, E, R>((queue) =>
    Effect.andThen(
      $forOf(producer, (value: T) => Effect.as(Queue.offer(queue, value), $normal)),
      Queue.end(queue)
    ), { bufferSize: 1 })
