import { Effect } from "effect"
import { capture, declared, generic, bound, parameter, parameterGeneric, inferred, refuse } from "./main.js"

// This consumer is TypeScript: calling an export builds its Effect value.
export function check(): string {
  const input = { value: 1 }
  const programs = [capture(input), declared(input), generic(input), bound(input), parameter(input), parameterGeneric(input), inferred(input, null)]
  input.value = 2
  for (const program of programs) {
    for (let run = 0; run < 2; run++) {
      const actual = Effect.runSync(program)
      if (actual !== 1) throw new Error(`expected snapshot 1, got ${actual}`)
    }
  }

  const refusal = refuse("late") // must not throw at this call
  const outcome = Effect.runSync(Effect.result(refusal))
  if (outcome._tag !== "Failure" || outcome.failure !== "late") {
    throw new Error("refusal must fail when run")
  }
  let threw = false
  try { refuse("") } catch (e) { threw = e instanceof Error && e.message === "missing reason" }
  if (!threw) throw new Error("factory validation must throw at the call")
  return "snapshot: 1 (seven forms, twice)"
}
