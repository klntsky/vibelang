import { Console, Data, Effect } from "effect"
import { findName, names } from "./users.js"

class NotFound extends Data.TaggedError("NotFound")<{ readonly id: number }> {}

// No annotations: the compiler infers Effect<string, NotFound, never>.
function lookup(id: number) {
  if (id >= names.length) Effect.fail(new NotFound({ id }))
  return findName(id)
}

export function main() {
  const name = lookup(1)
  Console.log(`found ${name.toUpperCase()}`)
}
