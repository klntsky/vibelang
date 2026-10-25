// A .ts file keeps TypeScript's verdict (D266, which supersedes D242): the
// yield without * below draws no EK4819, and TS2339 on Effect.catchAll keeps
// EK4813's rename to Effect.catch.
import { Effect } from "effect"

export const counted = Effect.gen(function* () {
  const n = yield Effect.succeed(1)
  return n
})

export const recover = Effect.catchAll
