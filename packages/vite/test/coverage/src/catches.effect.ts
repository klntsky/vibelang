import { Data, Effect } from "effect"

class Oops extends Data.TaggedError("Oops")<{ readonly n: number }> {}
class Bad extends Data.TaggedError("Bad")<{ readonly code: string }> {}

const step = Effect.fnUntraced(function* (payload: number) {
  if (payload < 0) return yield* Effect.fail(new Oops({ n: payload }))
  return payload
})

const mapped = (cause: Oops): Bad => new Bad({ code: String(cause.n) })

export const load = Effect.fnUntraced(function* (n: number) {
  const a = yield* Effect.catch(step(n), (cause) => Effect.fail(mapped(cause)))
  return a + 1
})

export const loadDirect = (n: number) => Effect.catch(step(n), (cause) => Effect.fail(mapped(cause)))

export const orZero = (n: number) => Effect.orElseSucceed(step(n), () => 0)

export const neverLoaded = (n: number) => Effect.catch(step(n), (cause) => Effect.fail(mapped(cause)))
