// Q2: does a generator that only yields already-completed effects (Exit values)
// ever hit the automatic yield? Compare with Effect.sync steps.
import { Effect, Fiber } from "effect"

const measure = (label: string, spinner: (bump: () => void) => Effect.Effect<unknown>) =>
  Effect.gen(function*() {
    let counter = 0
    let observedAt = -1
    const a = yield* Effect.forkChild(spinner(() => counter++))
    const b = yield* Effect.forkChild(Effect.sync(() => { if (observedAt === -1) observedAt = counter }))
    yield* Fiber.join(a)
    yield* Fiber.join(b)
    console.log(`${label.padEnd(52)} other fiber first ran at counter=${observedAt}`)
  })

const N = 200_000
await Effect.runPromise(Effect.gen(function*() {
  yield* measure("gen: for { bump; yield* Effect.void } (Exit)", (bump) => Effect.gen(function*() {
    for (let i = 0; i < N; i++) { bump(); yield* Effect.void }
  }))
  yield* measure("gen: for { yield* Effect.sync(bump) } (primitive)", (bump) => Effect.gen(function*() {
    for (let i = 0; i < N; i++) yield* Effect.sync(bump)
  }))
}))
