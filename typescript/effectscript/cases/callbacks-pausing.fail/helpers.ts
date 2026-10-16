import { Effect } from "effect"

export const find = (id: string): Effect.Effect<{ readonly id: string }> => Effect.succeed({ id })
