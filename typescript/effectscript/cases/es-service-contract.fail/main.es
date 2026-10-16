// expect: ES2101
// A service member has no body to infer from, so it states its full
// contract, Effect<A, E, R> (Chapter 14). A bare return type, a partial
// Effect, and a missing annotation are each ES2101; value members take any
// type.
import { Effect } from "effect"

abstract class Rates {
  abstract lookup(currency: string): number
  abstract convert(amount: number): Effect.Effect<number>
  abstract latest()
  abstract readonly base: string
  abstract refresh(): Effect.Effect<void, never, never>
}
