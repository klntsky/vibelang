import { Effect, Fiber } from "effect"

const probe = (region: string, delay: number) => Effect.sleep(delay).pipe(Effect.as(region))

export const closest = Effect.gen(function* () {
  const audit = yield* Effect.forkChild(Effect.log("probe started"))
  const winner = yield* Effect.race(probe("west", 7), probe("east", 12))
  yield* Fiber.join(audit)
  return winner
})
