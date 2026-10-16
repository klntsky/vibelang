import { Console, Data, Effect } from "effect"

class NotFound extends Data.TaggedError("NotFound")<{ readonly id: number }> {}

const names = ["ada", "grace", "barbara"]

// No annotations: lookup is Effect<string, NotFound, never>.
function lookup(id: number) {
  if (id >= names.length) Effect.fail(new NotFound({ id }))
  return names[id]
}

// main is Effect<void, NotFound, never>: lookup's error flows into it.
export function main() {
  Console.log(`found ${lookup(1)}`)
  const missing = lookup(7) // fails, so main stops here
  Console.log(`not printed: ${missing}`)
}

// The module top level is not inside a program, so main() is an Effect here.
Effect.runPromise(Effect.catch(main(), (e) => Console.log(`caught ${e._tag} ${e.id}`)))
