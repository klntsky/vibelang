// An Effect user imports Effect from effect and writes Effect<…> types,
// which name the prelude's type. The declaration file keeps the import and
// imports the prelude's type under a free name, so TypeScript reads it
// without a duplicate identifier.
import { Effect } from "effect"

export class Missing extends TaggedError("Missing")<{ readonly id: number }> {}

export abstract class Names {
  abstract lookup(id: number): Effect<string, Missing, never>
}

export const NamesLive = Layer.succeed(Names, {
  lookup(id) {
    if (id !== 1) throw new Missing({ id })
    return "Ada"
  }
})

export function fallback(): Effect<string, never, never> {
  return "nobody"
}

export function greet(id: number) {
  return `Hello, ${Names.lookup(id)}`
}
