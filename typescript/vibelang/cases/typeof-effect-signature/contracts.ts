import type { Effect } from "effect"
import type * as Chain from "./chain.vibe"
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false
export const effect: Equal<ReturnType<typeof Chain.run>, Effect.Effect<number, Chain.RunError, never>> = true
export const errors: Equal<Effect.Error<ReturnType<typeof Chain.run>>, Chain.RunError> = true
