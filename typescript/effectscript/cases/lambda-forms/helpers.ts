// TypeScript functions whose overload candidates ask a lambda argument for
// different forms (design D56), and one whose parameter is a bare type
// parameter. Each reports which form it received.
import { Effect } from "effect"

export const find = (id: string): Effect.Effect<{ readonly id: string }> => Effect.succeed({ id })

const describe = (r: unknown): unknown =>
  Effect.isEffect(r) ? Effect.map(r as Effect.Effect<number>, (n) => `effect ${n}`) : `plain ${r}`

/** A plain callback first, an Effect callback second. */
export function register(f: (x: number) => number): string
export function register<E, R>(f: (x: number) => Effect.Effect<number, E, R>): Effect.Effect<string, E, R>
export function register(f: (x: number) => unknown): unknown {
  return describe(f(1))
}

/** The reverse order: an Effect callback first, a plain callback second. */
export function attach<E, R>(f: (x: number) => Effect.Effect<number, E, R>): Effect.Effect<string, E, R>
export function attach(f: (x: number) => number): string
export function attach(f: (x: number) => unknown): unknown {
  return describe(f(1))
}

/** A program first, a thunk second. */
export function start<A, E, R>(program: Effect.Effect<A, E, R>): Effect.Effect<string, E, R>
export function start<A>(thunk: () => A): string
export function start(x: unknown): unknown {
  return typeof x === "function" ? `thunk ${(x as () => unknown)()}` : Effect.map(x as Effect.Effect<unknown>, (a) => `program ${a}`)
}

/** A bare type parameter: nothing about the parameter's type says what form to take. */
export function apply<F>(f: F): F {
  return f
}
