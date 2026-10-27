// The part of vibelang-runtime/experimental/Process (D313) that the case
// uses: a service that Effect's platform bundles do not provide.
import type * as Context from "effect/Context"
import type * as Effect from "effect/Effect"

export interface Process {
  readonly pid: Effect.Effect<number>
}

export declare const Process: Context.Service<Process, Process>
