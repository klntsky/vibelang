// When does Effect.fnUntracedEager run the generator body?
import { Effect } from "effect"
const log: Array<string> = []
const eager = Effect.fnUntracedEager(function*(tag: string) {
  log.push(`${tag}: body started`)
  yield* Effect.void // an Exit: consumed eagerly
  log.push(`${tag}: after sync yield`)
  yield* Effect.sleep(1) // not an Exit: stops eager evaluation
  log.push(`${tag}: after async yield`)
  return tag
})
const lazy = Effect.fnUntraced(function*(tag: string) {
  log.push(`${tag}: body started`)
  return tag
})
const e1 = eager("eager")
const l1 = lazy("lazy")
log.push("-- effects constructed, nothing run yet --")
await Effect.runPromise(e1)
await Effect.runPromise(e1)
await Effect.runPromise(l1)
console.log(log.join("\n"))
