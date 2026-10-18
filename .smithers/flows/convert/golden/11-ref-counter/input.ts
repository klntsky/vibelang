import { Effect, Ref } from "effect"

export const counted = Effect.gen(function* () {
  const ref = yield* Ref.make(0)
  yield* Ref.update(ref, (n) => n + 1)
  yield* Ref.update(ref, (n) => n + 1)
  return yield* Ref.get(ref)
})
