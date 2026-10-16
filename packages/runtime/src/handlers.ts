import { Context, Effect } from "effect"
import * as K from "./kernel/kernel.js"
import { controls } from "./services.js"

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
