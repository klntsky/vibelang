import { Data } from "effect"
import type * as EffectModule from "effect/Effect"
import type * as ScopeModule from "effect/Scope"

// The prelude (D4, D48): every .es file sees these exports without an import.
// Effect and Scope are both the effect module and a type, so `Effect<A, E, R>`
// means `Effect.Effect<A, E, R>` and `Scope` means `Scope.Scope`, while
// `Effect.gen` and `Scope.make` still reach the module.
export { Effect, Exit, Layer, Schema, Scope } from "effect"
export { pipe } from "effect/Function"

/** D4: in a .es file, `Effect<A, E, R>` is `Effect.Effect<A, E, R>`. */
export type Effect<A, E = never, R = never> = EffectModule.Effect<A, E, R>

/** D48: in a .es file, the type `Scope` is `Scope.Scope`. */
export type Scope = ScopeModule.Scope

/** D3: `TaggedError` is Effect's `Data.TaggedError`. */
export const TaggedError = Data.TaggedError
