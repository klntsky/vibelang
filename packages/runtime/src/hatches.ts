import { Cause, Effect, Exit } from "effect"

// The boundary hatches (D148): `sync`, `try`, and `callback` run plain
// TypeScript inside Effeck. In a .effeck file their bodies are not Effeck:
// calls do not run, `throw`, `async`, and `await` keep their JavaScript
// meaning, and platform APIs are allowed. The compiler lowers `sync` and
// `try` to Effect's own functions and names the overload it chose by the
// first parameter's name: evaluateAsync, evaluate, promise, or value. These
// implementations serve TypeScript callers and a hatch passed as a value.

const isPromiseLike = (value: unknown): value is PromiseLike<unknown> =>
  typeof value === "object" && value !== null && typeof (value as { readonly then?: unknown }).then === "function"

/**
 * D148: `sync(() => …)` is Effect.sync, so a throw is a defect;
 * `sync(async () => …)` is Effect.promise, so a rejection is a defect; and
 * `sync(expression)` evaluates the expression when the Effect runs.
 */
export function sync<A>(evaluateAsync: (signal: AbortSignal) => PromiseLike<A>): Effect.Effect<A>
export function sync<A>(evaluate: () => A): Effect.Effect<A>
export function sync<A>(promise: PromiseLike<A>): Effect.Effect<A>
export function sync<A>(value: A): Effect.Effect<A>
export function sync(evaluate: unknown): Effect.Effect<unknown> {
  if (typeof evaluate !== "function") {
    return isPromiseLike(evaluate) ? Effect.promise(() => evaluate) : Effect.succeed(evaluate)
  }
  return Effect.suspend(() => {
    const result = (evaluate as () => unknown)()
    return isPromiseLike(result) ? Effect.promise(() => result) : Effect.succeed(result)
  })
}

/**
 * D148: `try(() => …)` is Effect.try, so a throw is an expected
 * Cause.UnknownError, or the error that `mapError` makes of it;
 * `try(async (signal) => …)` is Effect.tryPromise; and `try(expression)`
 * evaluates the expression when the Effect runs. Exported as `try`.
 */
export function attempt<A>(evaluateAsync: (signal: AbortSignal) => PromiseLike<A>): Effect.Effect<A, Cause.UnknownError>
export function attempt<A, E>(evaluateAsync: (signal: AbortSignal) => PromiseLike<A>, mapError: (error: unknown) => E): Effect.Effect<A, E>
export function attempt<A>(evaluate: () => A): Effect.Effect<A, Cause.UnknownError>
export function attempt<A, E>(evaluate: () => A, mapError: (error: unknown) => E): Effect.Effect<A, E>
export function attempt<A>(promise: PromiseLike<A>): Effect.Effect<A, Cause.UnknownError>
export function attempt<A, E>(promise: PromiseLike<A>, mapError: (error: unknown) => E): Effect.Effect<A, E>
export function attempt<A>(value: A): Effect.Effect<A, Cause.UnknownError>
export function attempt<A, E>(value: A, mapError: (error: unknown) => E): Effect.Effect<A, E>
export function attempt(evaluate: unknown, mapError?: (error: unknown) => unknown): Effect.Effect<unknown, unknown> {
  const run = typeof evaluate === "function" ? (evaluate as () => unknown) : () => evaluate
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

/** The `resume` a `callback` hatch receives. */
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
 * A function that the body returns runs when the fiber is interrupted.
 */
export function callback<A, E = never>(register: (resume: Resume<A, E>, signal: AbortSignal) => void | (() => void)): Effect.Effect<A, E> {
  return Effect.callback<A, E>((resumeEffect, signal) => {
    const resume = ((value: A) => resumeEffect(Effect.succeed(value))) as Resume<A, E> & { fail: unknown; exit: unknown }
    resume.fail = (error: E) => resumeEffect(Effect.fail(error))
    resume.exit = (exit: Exit.Exit<A, E>) => resumeEffect(exit)
    const cleanup = register(resume, signal)
    return typeof cleanup === "function" ? Effect.sync(cleanup) : undefined
  })
}

// The way back in (D148): plain TypeScript, in a .ts file or a hatch's body,
// starts an Effect with these runners.
export { runCallback, runFork, runPromise, runSync } from "effect/Effect"
