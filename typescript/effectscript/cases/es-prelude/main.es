// Every .es file sees the prelude without an import (D4, D48): Effect is the
// effect module and the type Effect<A, E, R>, Scope the module and the type
// Scope.Scope. The emitted JavaScript imports exactly the names the file uses
// as values: effect modules from effect, the rest from @effectscript/runtime.
class NotFound extends TaggedError("NotFound")<{ readonly id: number }> {}

const names = ["ada", "grace"]

function lookup(id: number) {
  if (id >= names.length) Effect.fail(new NotFound({ id }))
  return names[id]
}

const one: Effect<number> = Effect.succeed(1)
const done: Effect<void, never, Scope> = Effect.void
const both: Effect.Effect<number, never, Scope.Scope> = Effect.as(done, 2)
const double = (n: number) => n * 2

// expect-result: {"found":"ada","tag":"NotFound","piped":8,"one":1}
export function main() {
  const found = lookup(0)
  const tag = new NotFound({ id: 9 })._tag
  const piped = pipe(2, double, double)
  const n = Effect.map(one, (x) => x)
  return { found, tag, piped, one: n }
}

export const unused = both
