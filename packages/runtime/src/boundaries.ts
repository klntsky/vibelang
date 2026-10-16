import { Effect } from "effect"
import { $expected } from "./regions.js"

/** A class whose instances a trusted call throws as expected failures. */
export type FailureClass<E> = abstract new (...args: any) => E

/**
 * D5, D23: a call whose contract lists expected failures, such as a function
 * tagged `@effectscript trusted` with `@throws {PortError}`, or `JSON.parse`,
 * whose shipped contract lists `SyntaxError`. The compiler passes the listed
 * classes. An exception that is an instance of one of them fails the Effect;
 * any other exception is a defect.
 */
export const $trusted = <A, E>(thunk: () => A, classes: ReadonlyArray<FailureClass<E>>): Effect.Effect<A, E> =>
  Effect.suspend(() => {
    try {
      return Effect.succeed(thunk())
    } catch (e) {
      return classes.some((c) => e instanceof c) ? Effect.fail(e as E) : Effect.die(e)
    }
  })

/**
 * D54: the same call inside a `try` with a `catch` in a function that
 * compiles to plain JavaScript. A listed exception is thrown as an expected
 * failure, which the function's `catch` receives; any other propagates as a
 * defect, which it does not.
 */
export const $trustedPlain = <A>(thunk: () => A, classes: ReadonlyArray<FailureClass<unknown>>): A => {
  try {
    return thunk()
  } catch (e) {
    throw classes.some((c) => e instanceof c) ? $expected(e) : e
  }
}
