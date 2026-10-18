import { Effect } from "effect"
import { greet } from "./greet.effeck"

// greet's type comes from greet.effeck through the mapper: Effect<string, EmptyName, never>.
export const ok: Effect.Effect<string, { readonly _tag: "EmptyName" }> = greet("Ada")

// Misuse: greet succeeds with a string, not a number.
export const bad: Effect.Effect<number, unknown> = greet("Ada")
