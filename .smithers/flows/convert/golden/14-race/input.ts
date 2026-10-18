import { Effect } from "effect"

const cache = Effect.sleep("50 millis").pipe(Effect.as("cache"))
const network = Effect.sleep("10 millis").pipe(Effect.as("network"))

export const fastest = Effect.race(cache, network)
