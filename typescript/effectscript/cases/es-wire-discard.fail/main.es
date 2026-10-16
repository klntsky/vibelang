// expect: ES2001 ES3201
// A layer that provides nothing, such as one that only runs a startup
// program, keeps its errors and requirements through wire (D27): the
// contract of launch must name Offline, and the runner reports Mailer.
import { Effect, Layer } from "effect"
import { wire } from "@effectscript/runtime"

abstract class Mailer {
  abstract send(to: string): Effect.Effect<void, never, never>
}

class Offline extends TaggedError("Offline")<{}> {}

const Announce = Layer.effectDiscard(() => {
  const sent = Mailer.send("team@example.com")
  if (sent === undefined) throw new Offline()
})

const AppLive = wire(Announce)

export function launch(): Effect<never, never, infer R> {
  return Layer.launch(AppLive)
}

Effect.runFork(() => launch())
