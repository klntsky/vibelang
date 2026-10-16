// The declaration file of a .es file imports the prelude types its
// declarations name, and names its imports by output (./users.js).
import { lookup } from "./users.es"
export { NotFound } from "./users.es"

export const one: Effect<number> = Effect.succeed(1)
export const scoped: Effect<void, never, Scope> = Effect.void

// expect-result: "ada"
export function main() {
  return lookup(0)
}
