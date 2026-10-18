// A callback declared to return a plain value or an Effect, as
// Arbitrary.checkEffect's property is; the function decides at run time
// with Effect.isEffect.
import { Effect } from "effect"

export function probe<E = never, R = never>(
  property: (value: number) => boolean | Effect.Effect<boolean, E, R>
): Effect.Effect<string, E, R> {
  const result = property(7)
  return Effect.isEffect(result) ? Effect.map(result, (ok) => `effect: ${ok}`) : Effect.succeed(`plain: ${result}`)
}
