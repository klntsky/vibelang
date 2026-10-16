// A parameter that accepts a function or an Effect, as HttpRouter.add's
// handler does; the function decides at run time with Effect.isEffect.
import { Effect } from "effect"

export function route(handler: Effect.Effect<string> | ((request: string) => Effect.Effect<string>)): Effect.Effect<string> {
  return Effect.isEffect(handler) ? Effect.map(handler, (s) => `program: ${s}`) : Effect.map(handler("req"), (s) => `function: ${s}`)
}
