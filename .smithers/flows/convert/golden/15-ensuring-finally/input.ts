import { Effect } from "effect"

export const tracked = Effect.succeed(1).pipe(
  Effect.ensuring(Effect.log("cleanup"))
)
