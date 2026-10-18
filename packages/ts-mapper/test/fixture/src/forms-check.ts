// The types stock TypeScript gives each form's virtual TypeScript
// (forms.effeck, and the prelude's own $dual), as exact equalities: a form
// that projects to the wrong type fails here with TS2322.
import type { Cause, Effect } from "effect"
import { $dual, $gen } from "effeck-ts-mapper/prelude"
import { type NotFound, each, retried, spanned, timed } from "./forms.effeck"

type Equals<X, Y> = (<T>() => T extends X ? 1 : 2) extends (<T>() => T extends Y ? 1 : 2) ? true : false

// An Effect function: (id) => Effect<string, NotFound, never>, so forEach collects strings.
export const effectFunction: Equals<ReturnType<typeof each>, Effect.Effect<Array<string>, NotFound, never>> = true
// A program: the lambda is an Effect<string, NotFound, never> itself.
export const program: Equals<ReturnType<typeof retried>, Effect.Effect<string, NotFound, never>> = true
// By name: timeout receives the call as a program and adds its TimeoutError.
export const byName: Equals<ReturnType<typeof timed>, Effect.Effect<string, NotFound | Cause.TimeoutError, never>> = true
// A block param: the block is the program withSpan wraps.
export const block: Equals<ReturnType<typeof spanned>, Effect.Effect<string, NotFound, never>> = true

// The dual: one value that is both the program and a function returning it.
const twice = $dual($gen(function* () { return 2 }))
export const dual: Equals<typeof twice, Effect.Effect<number, never, never> & (() => Effect.Effect<number, never, never>)> = true
