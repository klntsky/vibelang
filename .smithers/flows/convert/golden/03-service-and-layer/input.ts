import { Context, Effect, Layer } from "effect"

class Clock2 extends Context.Service<Clock2, {
  readonly now: () => Effect.Effect<number>
}>()("app/Clock2") {}

const ClockLive = Layer.succeed(Clock2, { now: () => Effect.succeed(42) })

export const program = Effect.gen(function* () {
  const clock = yield* Clock2
  const t = yield* clock.now()
  return t * 2
}).pipe(Effect.provide(ClockLive))
