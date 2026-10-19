import type { Effect } from "effect"
import type { B, Out, viaLet, viaDefault, partial } from "./main.vibe"

type Errors<P> = P extends Effect.Effect<infer A, infer E, infer R> ? E : never
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false
type Assert<T extends true> = T

// Both unannotated functions infer only Out, including the default rethrow.
type ExtractorErrors = Assert<Equal<Errors<ReturnType<typeof viaLet>>, Out>>
type DefaultErrors = Assert<Equal<Errors<ReturnType<typeof viaDefault>>, Out>>
// A refutable payload still permits B to escape through default.
type PartialErrors = Assert<Equal<Errors<ReturnType<typeof partial>>, Out | B>>
