// TypeScript code that reads main.effeck through its declaration file.
import { Effect } from "effect"
import { fallback, greet, NamesLive } from "../../out/main.js"

export const greeting: Effect.Effect<string, unknown, never> = Effect.provide(greet(1), NamesLive)
export const missing = Effect.provide(Effect.flip(greet(2)), NamesLive)
export const nobody = fallback()
