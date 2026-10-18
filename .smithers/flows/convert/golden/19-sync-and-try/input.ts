import { Data, Effect } from "effect"

class BadJson extends Data.TaggedError("BadJson")<{ readonly error: unknown }> {}

export const parsed = (text: string) =>
  Effect.try({ try: () => JSON.parse(text) as unknown, catch: (error) => new BadJson({ error }) })

export const id = Effect.sync(() => crypto.randomUUID())
