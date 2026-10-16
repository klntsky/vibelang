// Plain TypeScript: an Effect-returning function, as Effect code writes it today.
import { Effect } from "effect"

export const findName = (id: number): Effect.Effect<string> => Effect.succeed(["ada", "grace", "barbara"][id] ?? "nobody")
