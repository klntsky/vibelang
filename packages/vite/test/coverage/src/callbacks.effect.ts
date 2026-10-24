import { Data, Effect } from "effect"

export class Oops extends Data.TaggedError("Oops")<{ readonly n: number }> {}

export const step = Effect.fnUntraced(function* (base: number, payload: number) {
  if (payload < 0) return yield* Effect.fail(new Oops({ n: payload }))
  return base + payload
})

export const handler = (base: number) => (payload: number) => step(base, payload)

export const unusedHandler = (base: number) => (payload: number) => step(base, payload)

export const lazyHandler = (base: number) => (payload: number) => step(base, payload)
