// A local declaration or an import shadows a prelude name (D4). This file
// declares its own handle and imports Effect from effect, so the emitted
// JavaScript imports neither from the prelude.
import { Effect } from "effect"

function handle(n: number) {
  return n * 10
}

const program: Effect.Effect<number> = Effect.succeed(handle(4))

// expect-result: 40
export function main() {
  return program
}
