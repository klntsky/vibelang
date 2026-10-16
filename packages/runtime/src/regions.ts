import { Effect, Scope } from "effect"
import * as K from "./kernel/kernel.js"

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
 * D54: an expected failure thrown in a function that compiles to plain
 * JavaScript. The function's own catch tells it from a defect.
 */
class ExpectedFailure {
  constructor(readonly error: unknown) {}
}

/** D54: `throw e` in a function that compiles to plain JavaScript throws `$expected(e)`. */
export const $expected = (error: unknown): unknown => new ExpectedFailure(error)

/**
 * D54, D13: a catch in a function that compiles to plain JavaScript starts
 * with `e = $caught(e)`. It returns the expected failure, and rethrows
 * anything else, a defect, so the catch never sees it.
 */
export const $caught = (thrown: unknown): unknown => {
  if (thrown instanceof ExpectedFailure) return thrown.error
  throw thrown
}
