import { Data, Effect } from "effect"

class Timeout extends Data.TaggedError("Timeout") {}
class Refused extends Data.TaggedError("Refused")<{ readonly code: number }> {}

declare const call: Effect.Effect<string, Timeout | Refused>

export const safe = call.pipe(
  Effect.catch((e) => Effect.succeed(e._tag === "Timeout" ? "timed out" : `refused ${e.code}`))
)
