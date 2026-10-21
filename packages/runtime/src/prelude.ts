import { Data } from "effect"
import type * as EffectModule from "effect/Effect"
import type * as ScopeModule from "effect/Scope"
import { customMatcher, type MatchHint } from "./match.js"

// The prelude (D4, D48): every .vibe file sees these exports without an import.
// Effect and Scope are both the effect module and a type, so `Effect<A, E, R>`
// means `Effect.Effect<A, E, R>` and `Scope` means `Scope.Scope`, while
// `Effect.gen` and `Scope.make` still reach the module.
export { Effect, Exit, Layer, Schema, Scope } from "effect"
export { pipe } from "effect/Function"

/** D4: in a .vibe file, `Effect<A, E, R>` is `Effect.Effect<A, E, R>`. */
export type Effect<A, E = never, R = never> = EffectModule.Effect<A, E, R>

/** D48: in a .vibe file, the type `Scope` is `Scope.Scope`. */
export type Scope = ScopeModule.Scope

/**
 * D3: `TaggedError` is Effect's `Data.TaggedError`, with the pattern matching
 * proposal's custom matcher (D215): `when NotFound` tests that the subject is
 * an instance, and the extractor `NotFound(let error)` binds it.
 */
export const TaggedError: typeof Data.TaggedError = ((tag: string) => {
  const Base: any = Data.TaggedError(tag)
  Base[customMatcher] = function(this: abstract new(...args: any) => any, subject: unknown, hint?: MatchHint) {
    if (!(subject instanceof this)) return false
    return hint?.matchType === "extractor" ? [subject] : true
  }
  return Base
}) as any

/**
 * D278: the base of `error Name extends Base`, a subclass of Base whose
 * instances carry their own tag and report it as their name, as
 * `Data.TaggedError` does for its own. The declaration's members declare the
 * tag's type, so the base keeps Base's type, its constructor and its statics.
 */
export const $errorSubclass = <B extends abstract new(...args: any) => { readonly _tag: string }>(
  base: B,
  tag: string
): B => {
  const Base = base as unknown as abstract new(...args: Array<any>) => { _tag: string }
  abstract class Tagged extends Base {
    constructor(...args: Array<any>) {
      super(...args)
      this._tag = tag
    }
  }
  ;(Tagged.prototype as unknown as { name: string }).name = tag
  return Tagged as unknown as B
}

/**
 * D278: what the constructor of `error Name extends Base { fields }` takes
 * when it writes none: what Base's constructor takes, plus the fields. Base's
 * constructor must take a single object of fields, as an error declared
 * without a constructor does; otherwise this is never (EK2118).
 */
export type $ErrorArgs<B, A> = B extends abstract new(args: infer P) => any ? ([P] extends [void] ? {} : P) & A : never
