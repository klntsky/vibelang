// Q1 follow-up: where does the synchronous resumption chain break?
// Prints, at process exit, how far the chain got and the first fiber failure.
import { Deferred, Effect, Exit, Fiber } from "effect"

const n = Number(process.argv[2] ?? "100")
let maxResumed = -1
let firstFailure: string | undefined
let failedCount = 0
let settled = false
process.on("exit", (code) => {
  console.log(JSON.stringify({ n, code, settled, maxResumedIndex: maxResumed, failedCount, firstFailure }))
})

const program = Effect.gen(function*() {
  const ds = yield* Effect.forEach(Array.from({ length: n + 1 }), () => Deferred.make<void>())
  const fibers = []
  for (let i = 0; i < n; i++) {
    const f = yield* Effect.forkChild(
      Effect.gen(function*() {
        yield* Deferred.await(ds[i])
        maxResumed = Math.max(maxResumed, i)
        yield* Deferred.succeed(ds[i + 1], undefined)
      })
    )
    f.addObserver((exit) => {
      if (Exit.isFailure(exit)) {
        failedCount++
        try { const r: any = exit.cause.reasons[0]; firstFailure ??= `fiber#${i}: ${r?._tag} ${r?.defect?.name ?? ""}: ${String(r?.defect?.message ?? r?.error ?? "").slice(0, 80)}` } catch { firstFailure ??= `fiber#${i}: (unreadable)` }
      }
    })
    fibers.push(f)
  }
  yield* Effect.sleep(5)
  yield* Deferred.succeed(ds[0], undefined)
  yield* Fiber.awaitAll(fibers)
})

await Effect.runPromise(program)
settled = true
