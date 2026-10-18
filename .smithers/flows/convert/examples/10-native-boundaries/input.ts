import { Data, Effect } from "effect"

class InvalidAddress extends Data.TaggedError("InvalidAddress")<{ readonly reason: unknown }> {}

export const hostname = (address: string) => Effect.try({
  try: () => new URL(address).hostname,
  catch: (reason) => new InvalidAddress({ reason })
})

export const encoded = (text: string) => Effect.sync(() => new TextEncoder().encode(text))
