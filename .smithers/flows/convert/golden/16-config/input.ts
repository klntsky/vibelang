import { Config, Effect } from "effect"

export const port = Effect.gen(function* () {
  const value = yield* Config.Int("PORT")
  return value + 1
})
