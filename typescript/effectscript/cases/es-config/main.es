// compilerOptions.effectscript (D34) accepts per-code categories for ES2004
// and ES4403 and a tracing mode; a valid block compiles without diagnostics.
import { Effect } from "effect"

// expect-result: 3
export function main() {
  return Effect.succeed(1 + 2)
}
