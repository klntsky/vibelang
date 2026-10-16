// Plain TypeScript: an Effect-returning function, as Effect code writes it today.
import { Effect } from "effect"

export const names = ["ada", "grace", "barbara"]

export function findName(id: number): Effect.Effect<string> {
  return Effect.succeed(names[id] ?? "nobody")
}
