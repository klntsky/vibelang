// The prelude of Effeck's TypeScript 7 content mapper. Every file the mapper
// projects starts with
//
//   import { $gen, $, $fail, $TaggedError, Effect } from "effeck-ts-mapper/prelude";
//
// leaving out Effect when the file binds that name itself.
//
// These declarations are types only: tsc emits no JavaScript for a mapped
// file, so nothing here runs. The projector that writes the calls lives in
// typescript/tsc/internal/effeckmapper.

import type { Data, Effect } from "effect"

/**
 * The effect module, a prelude name in Effeck (D4): `A!E!R` projects to
 * `Effect.Effect<A, E, R>`, and `Effect.gen` needs no import.
 */
export { Effect } from "effect"

/** A function body: `function f() { body }` projects to `function f() { return $gen(function* () { body }) }`. */
export declare const $gen: typeof Effect.gen

/**
 * Runs a call: a call `c` in a function body projects to `(yield* $(c))`. An
 * Effect runs and gives its success type; any other value is lifted to an
 * Effect that succeeds with it and fails with nothing.
 */
export declare function $<T>(value: T): T extends Effect.Effect<any, any, any> ? T : Effect.Effect<T, never, never>

/** `throw x` in a function body projects to `return yield* $fail(x)`. */
export declare const $fail: typeof Effect.fail

/** `error Name { fields }` projects to `class Name extends $TaggedError("Name")<{ fields }> {}`. */
export declare const $TaggedError: typeof Data.TaggedError
