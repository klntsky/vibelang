// Declarations name prelude types, so the emitted .d.ts imports them.
export class NotFound extends TaggedError("NotFound")<{ readonly id: number }> {}

export const one: Effect<number> = Effect.succeed(1)
export const scoped: Effect<void, never, Scope> = Effect.void

export function lookup(id: number) {
  if (id > 1) Effect.fail(new NotFound({ id }))
  return "ada"
}

// expect-result: "ada"
export function main() {
  return lookup(0)
}
