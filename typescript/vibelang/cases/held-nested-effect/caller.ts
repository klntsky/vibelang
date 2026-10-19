import { Effect } from "effect"
import { pending } from "./main.js"

export function check(): string {
  let runs = 0
  const migration = Effect.sync(() => { runs++ })
  const [, load] = pending(migration)
  if (Effect.runSync(load) !== migration || runs !== 0) {
    throw new Error("the loader must return its migration unrun")
  }
  Effect.runSync(Effect.runSync(load))
  if (Number(runs) !== 1) throw new Error("the migration runs only when explicitly run")
  return "loader returns migration unrun"
}
