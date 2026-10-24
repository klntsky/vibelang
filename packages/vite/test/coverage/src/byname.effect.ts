import { Data, Effect } from "effect"

class Oops extends Data.TaggedError("Oops")<{ readonly n: number }> {}

const step = Effect.fnUntraced(function* (payload: number) {
  if (payload < 0) return yield* Effect.fail(new Oops({ n: payload }))
  return payload
})

export const timed = (n: number) => Effect.timeout(step(n), "1 second")

export const pair = Effect.fnUntraced(function* (n: number) {
  const [x, y] = yield* Effect.all([step(n), step(n + 1)])
  return x + y
})

export const retried = (n: number) => Effect.retry(step(n), { times: 2 })

export const neverRetried = (n: number) => Effect.retry(step(n), { times: 2 })
