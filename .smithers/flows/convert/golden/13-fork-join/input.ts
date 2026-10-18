import { Effect, Fiber } from "effect"

const work = (n: number) => Effect.sleep("10 millis").pipe(Effect.as(n))

export const both = Effect.gen(function* () {
  const a = yield* Effect.forkChild(work(1))
  const b = yield* Effect.forkChild(work(2))
  return [yield* Fiber.join(a), yield* Fiber.join(b)]
})
