import { Console, Effect, Ref } from "effect"

export const inspect = Effect.scoped(Effect.gen(function* () {
  const lease = yield* Effect.acquireRelease(
    Effect.succeed("batch-17"),
    (name) => Effect.log(`Released ${name}`)
  )
  const visits = yield* Ref.make(10)
  yield* Ref.update(visits, (n) => n + 3)
  return yield* Ref.get(visits).pipe(
    Effect.tap((n) => Console.log(lease, n)),
    Effect.ensuring(Effect.log("inspection finished"))
  )
}))
