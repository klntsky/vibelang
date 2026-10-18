import { Data, Effect } from "effect"

class MissingParcel extends Data.TaggedError("MissingParcel")<{ readonly tracking: string }> {}

const destination = (tracking: string): Effect.Effect<string, MissingParcel> =>
  tracking === "PK-9" ? Effect.succeed("Oslo") : Effect.fail(new MissingParcel({ tracking }))

export const route = (tracking: string) => destination(tracking).pipe(
  Effect.map((city) => `Ship to ${city}`),
  Effect.catchTag("MissingParcel", (e) => Effect.succeed(`Investigate ${e.tracking}`))
)

export const destinationOrDepot = (tracking: string) =>
  destination(tracking).pipe(Effect.orElseSucceed(() => "depot"))
