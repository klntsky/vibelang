import { Effect } from "effect"

const slow = Effect.sleep("5 seconds").pipe(Effect.as("done"))

export const quick = slow.pipe(
  Effect.timeout("1 second"),
  Effect.orElseSucceed(() => "too slow")
)
