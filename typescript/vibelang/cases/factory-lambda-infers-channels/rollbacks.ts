import { Effect } from "effect"

/** Runs effect, then rollback with its value, as a flow's compensation does. */
export const withRollback = <A, E, R, R2>(
  effect: Effect.Effect<A, E, R>,
  rollback: (a: A) => Effect.Effect<void, never, R2>
): Effect.Effect<A, E, R | R2> => Effect.tap(effect, (a) => rollback(a))
