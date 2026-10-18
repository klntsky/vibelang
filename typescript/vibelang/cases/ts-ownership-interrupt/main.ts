import { Effect } from "effect"

const onLapse = (_unconfirmedMs: number) => Effect.void
export const expire = Effect.gen(function* () {
  yield* Effect.logWarning("run lease lapsed; interrupting owned work").pipe(
    Effect.annotateLogs({ runId: "run", unconfirmedMs: 100 }),
    Effect.andThen(onLapse(100)),
    Effect.andThen(Effect.interrupt)
  )
})
