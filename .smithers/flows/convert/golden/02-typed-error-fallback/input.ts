import { Data, Effect } from "effect"

class NotFound extends Data.TaggedError("NotFound")<{ readonly id: string }> {}

const find = (id: string): Effect.Effect<string, NotFound> =>
  id === "1" ? Effect.succeed("Ada") : Effect.fail(new NotFound({ id }))

export const greeting = (id: string) =>
  find(id).pipe(
    Effect.map((name) => `Hello, ${name}`),
    Effect.catchTag("NotFound", (e) => Effect.succeed(`No user ${e.id}`))
  )
