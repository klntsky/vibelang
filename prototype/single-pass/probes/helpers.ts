// Probe helpers: TypeScript functions whose overload candidates classify the same lambda
// differently, and one whose callback type is a bare type parameter. Trusted (D23) so that
// .es code may call them bare.
import { Effect } from "effect"

/**
 * A plain callback first, an Effect callback second.
 * @effectscript trusted
 */
export function register(f: (x: number) => number): string
export function register<E, R>(f: (x: number) => Effect.Effect<number, E, R>): Effect.Effect<string, E, R>
export function register(f: (x: number) => unknown): unknown {
  const r = f(1)
  return Effect.isEffect(r) ? Effect.map(r as Effect.Effect<number>, String) : String(r)
}

/**
 * The reverse order: an Effect callback first, a plain callback second.
 * @effectscript trusted
 */
export function attach<E, R>(f: (x: number) => Effect.Effect<number, E, R>): Effect.Effect<string, E, R>
export function attach(f: (x: number) => number): string
export function attach(f: (x: number) => unknown): unknown {
  const r = f(1)
  return Effect.isEffect(r) ? Effect.map(r as Effect.Effect<number>, String) : String(r)
}

/**
 * A program first, a thunk second.
 * @effectscript trusted
 */
export function start<A, E, R>(program: Effect.Effect<A, E, R>): Effect.Effect<A, E, R>
export function start<A>(thunk: () => A): A
export function start(x: unknown): unknown {
  return typeof x === "function" ? (x as () => unknown)() : x
}

/**
 * A bare type parameter: the callback's type is whatever the argument's is.
 * @effectscript trusted
 */
export function apply<F>(f: F): F {
  return f
}
