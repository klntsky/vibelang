import { Console, Effect } from "effect"

export const hello = Effect.gen(function* () {
  yield* Console.log("Hello")
  yield* Effect.log("logged")
  return "ok"
})
