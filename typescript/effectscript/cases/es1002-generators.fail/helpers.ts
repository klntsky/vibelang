import { Effect } from "effect"

export const fromTypeScript = Effect.gen(function* () {
  return yield* Effect.succeed(1)
})
