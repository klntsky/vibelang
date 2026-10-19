import { Effect } from "effect"
export const runAsync = <A, E>(program: () => Effect.Effect<A, E>): Promise<A> => Effect.runPromise(program())
export const awaited = <A>(value: () => Promise<A>): Effect.Effect<A, unknown> =>
  Effect.tryPromise({ try: value, catch: (cause) => cause })
