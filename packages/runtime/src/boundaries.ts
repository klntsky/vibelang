import { Effect } from "effect"

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
