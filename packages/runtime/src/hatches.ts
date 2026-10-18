import { Cause, Effect, Exit } from "effect"

// The boundary hatches (D148): `sync`, `try`, and `callback` run plain
// TypeScript inside VibeLang. In a .vibe file a hatch's first argument is not
// VibeLang: calls do not run, `throw`, `async`, and `await` keep their
// JavaScript meaning, and platform APIs are allowed. That TypeScript is
// self-contained (D229): values reach it as inputs, the arguments after the
// function, which fill its first parameters (D230); an async function then
// receives the AbortSignal, and a callback `resume` and the signal. The
// compiler lowers `sync` and `try` to Effect's own functions and reads the
// overload it chose by the first parameter's name (evaluate or value) and
// whether the second is named mapError (D231); the function itself decides
// between sync and async (D232). These implementations serve TypeScript
// callers and a hatch passed as a value.

/** The inputs a hatch passes to its function's first parameters (D230). */
type Inputs = ReadonlyArray<unknown>

const isPromiseLike = (value: unknown): value is PromiseLike<unknown> =>
  typeof value === "object" && value !== null && typeof (value as { readonly then?: unknown }).then === "function"

const call = (evaluate: unknown, inputs: Inputs): unknown => (evaluate as (...args: Array<unknown>) => unknown)(...inputs)

/**
 * D148: `sync((n) => n * 2, count)` is Effect.sync, so a throw is a defect; an
 * async function is Effect.promise, so a rejection is a defect; and
 * `sync(expression)` evaluates the expression when the Effect runs. Inputs
 * after the function fill its first parameters (D230).
 */
export function sync<R, Args extends Inputs = []>(evaluate: (...args: [...Args, AbortSignal]) => PromiseLike<R>, ...args: Args): Effect.Effect<R>
export function sync<R, Args extends Inputs = []>(evaluate: (...args: Args) => R, ...args: Args): Effect.Effect<Awaited<R>>
export function sync<A>(value: A): Effect.Effect<Awaited<A>>
export function sync(evaluate: unknown, ...inputs: Array<unknown>): Effect.Effect<unknown> {
  if (typeof evaluate !== "function") {
    return isPromiseLike(evaluate) ? Effect.promise(() => evaluate) : Effect.succeed(evaluate)
  }
  return Effect.suspend(() => {
    const result = call(evaluate, inputs)
    return isPromiseLike(result) ? Effect.promise(() => result) : Effect.succeed(result)
  })
}

/**
 * D148: `try((text) => JSON.parse(text), text)` is Effect.try, so a throw is an
 * expected Cause.UnknownError, or the error that `mapError` makes of it; an
 * async function is Effect.tryPromise; and `try(expression)` evaluates the
 * expression when the Effect runs. The mapper stays the second argument, and
 * inputs follow it (D231). Exported as `try`.
 */
export function attempt<R, E, Args extends Inputs = []>(
  evaluate: (...args: [...Args, AbortSignal]) => PromiseLike<R>,
  mapError: (error: unknown) => E,
  ...args: Args
): Effect.Effect<R, E>
export function attempt<R, E, Args extends Inputs = []>(evaluate: (...args: Args) => R, mapError: (error: unknown) => E, ...args: Args): Effect.Effect<Awaited<R>, E>
export function attempt<R, Args extends Inputs = []>(evaluate: (...args: [...Args, AbortSignal]) => PromiseLike<R>, ...args: Args): Effect.Effect<R, Cause.UnknownError>
export function attempt<R, Args extends Inputs = []>(evaluate: (...args: Args) => R, ...args: Args): Effect.Effect<Awaited<R>, Cause.UnknownError>
export function attempt<A, E>(value: A, mapError: (error: unknown) => E): Effect.Effect<Awaited<A>, E>
export function attempt<A>(value: A): Effect.Effect<Awaited<A>, Cause.UnknownError>
export function attempt(evaluate: unknown, ...rest: Array<unknown>): Effect.Effect<unknown, unknown> {
  // A function in second place is the mapper, as the overloads read it.
  const mapError = typeof rest[0] === "function" ? (rest[0] as (error: unknown) => unknown) : undefined
  const inputs = mapError ? rest.slice(1) : rest
  const run = typeof evaluate === "function" ? () => call(evaluate, inputs) : () => evaluate
  const failure = (error: unknown) => (mapError ? mapError(error) : new Cause.UnknownError(error))
  return Effect.suspend(() => {
    let result: unknown
    try {
      result = run()
    } catch (error) {
      return Effect.fail(failure(error))
    }
    if (!isPromiseLike(result)) return Effect.succeed(result)
    const pending = result
    return Effect.tryPromise({ try: () => pending, catch: failure })
  })
}

export { attempt as try }

/**
 * The `resume` a `callback` hatch receives. A .vibe file names it without an
 * import, to annotate `resume` where inputs keep callback's type arguments
 * from being written: `callback((source, resume: Resume<number, Error>) => …, source)`.
 */
export interface Resume<A, E> {
  /** Succeeds with value. */
  (value: A): void
  /** Fails with error, an expected failure. */
  readonly fail: (error: E) => void
  /** Ends as exit says. */
  readonly exit: (exit: Exit.Exit<A, E>) => void
}

/**
 * D148: `callback((resume, signal) => …)` is Effect.callback. `resume(value)`
 * succeeds, `resume.fail(error)` fails, and `resume.exit(exit)` adopts an Exit.
 * A function that the body returns runs when the fiber is interrupted. Inputs
 * after the function fill its first parameters, before `resume` (D230).
 */
export function callback<A, E = never, Args extends Inputs = []>(
  register: (...args: [...Args, Resume<A, E>, AbortSignal]) => void | (() => void),
  ...args: Args
): Effect.Effect<A, E>
export function callback<A, E = never, Args extends Inputs = []>(register: (...args: [...Args, Resume<A, E>]) => void | (() => void), ...args: Args): Effect.Effect<A, E>
export function callback(register: (...args: Array<any>) => void | (() => void), ...args: Inputs): Effect.Effect<unknown, unknown> {
  return Effect.callback<unknown, unknown>((resumeEffect, signal) => {
    const resume = ((value: unknown) => resumeEffect(Effect.succeed(value))) as Resume<unknown, unknown> & { fail: unknown; exit: unknown }
    resume.fail = (error: unknown) => resumeEffect(Effect.fail(error))
    resume.exit = (exit: Exit.Exit<unknown, unknown>) => resumeEffect(exit)
    const cleanup = call(register, [...args, resume, signal]) as void | (() => void)
    return typeof cleanup === "function" ? Effect.sync(cleanup) : undefined
  })
}

// The way back in (D148): plain TypeScript, in a .ts file or a hatch's body,
// starts an Effect with these runners.
export { runCallback, runFork, runPromise, runSync } from "effect/Effect"
