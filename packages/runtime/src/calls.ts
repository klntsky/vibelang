import { Effect } from "effect"

/** The optimistic first-pass call: run it when it is an Effect, else succeed with it. */
export type Bind<T> = [T] extends [Effect.Effect<infer A, infer E, infer R>] ? Effect.Effect<A, E, R> : Effect.Effect<T, never, never>
export const $bind = <T>(value: T): Bind<T> => (Effect.isEffect(value) ? value : Effect.succeed(value)) as any

/** A program value: a function with no arguments where an Effect is expected (D7). */
export const $program = <A, E, R>(thunk: () => Effect.Effect<A, E, R>): Effect.Effect<A, E, R> => Effect.suspend(thunk)

/** Prototype extension: an array of programs where an iterable of Effects is expected. */
export const $programs = <P extends () => Effect.Effect<any, any, any>>(programs: Iterable<P>): Array<ReturnType<P>> =>
  Array.from(programs, (p) => Effect.suspend(p)) as Array<ReturnType<P>>

export const runMain = <A, E>(program: Effect.Effect<A, E, never>) => Effect.runPromiseExit(program)
