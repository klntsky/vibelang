import * as Effect from "effect/Effect"
export const implement = (execute: (n: number) => Effect.Effect<number>): Effect.Effect<number> =>
  Effect.suspend(() => execute(1))
import * as Context from "effect/Context"
import * as Effectable from "effect/Effectable"
export class Needed extends Context.Service<Needed, { readonly value: number }>()("Needed") {}
export class NeedsAction extends Effectable.Class<number, never, Needed> {
  asEffect(): Effect.Effect<number, never, Needed> { return Effect.map(Needed, (s) => s.value) }
}
/** @vibelang trusted */
export const needsAction = (): NeedsAction => new NeedsAction()
