// G7: cost of capturing at every level of a deep recursion. Run: bun bench/deep-capture.ts
// Each level performs once before descending, so capture n copies about n frames.
import { Effect } from "effect"
import { type Continuation, handle, operation, resume } from "../kernel/kernel.ts"

const Tick = operation<number, void>("Tick")

const descend: (n: number) => Effect.Effect<number> = Effect.fnUntraced(function*(n: number) {
  yield* Tick(n)
  if (n === 0) return 0
  return 1 + (yield* descend(n - 1))
})

for (const depth of [1_000, 2_000, 4_000, 8_000]) {
  let copied = 0
  const program = handle("bench", [Tick], descend(depth), {
    Tick: (_: number, k: Continuation) => {
      copied += k.frameCount
      return resume(k, undefined)
    }
  })
  const start = performance.now()
  await Effect.runPromise(program)
  const ms = performance.now() - start
  console.log(`depth ${String(depth).padStart(5)}: ${ms.toFixed(1).padStart(7)} ms, ${copied.toLocaleString().padStart(11)} frames copied`)
}
