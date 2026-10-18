// expect: EK4801 EK4819 EK4820
// The rules of Effect's language service that VibeLang reports in .ts files
// (D242): a discarded Effect, yield without *, and yield* without return.
import { Data, Effect } from "effect"

class Boom extends Data.TaggedError("Boom")<{}> {}

export const program = Effect.gen(function* () {
  Effect.log("lost")
  yield Effect.succeed(1)
  yield* Effect.fail(new Boom())
})

export const traced = Effect.fn("traced")(function* (n: number) {
  Effect.log("lost")
  const m = yield Effect.succeed(n)
  return m
})

// Outside an Effect generator, these are a generator's own and an assignment.
export function* numbers() {
  yield 1
}

let later = Effect.succeed(1)
later = Effect.succeed(2)
export const kept = later
