// Rules from Effect's language service that also apply to .ts files (D242).
import { Effect } from "effect"

export const counted = Effect.gen(function* () {
  const n = yield Effect.succeed(1)
  return n
})

export const recover = Effect.catchAll
