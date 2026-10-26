// Q1: is there any recursion-depth guard for chains of synchronous resumptions?
// Fiber i awaits d[i], then completes d[i+1]. Completing d[0] resumes fiber 0
// on the completer's stack, which resumes fiber 1 on its stack, and so on.
// Usage: <runtime> q1-chain-depth.ts <n>
import { Deferred, Effect, Exit, Fiber } from "effect"

;(Error as any).stackTraceLimit = Infinity
const n = Number(process.argv[2] ?? "100")

const program = Effect.gen(function*() {
  const ds = yield* Effect.forEach(Array.from({ length: n + 1 }), () => Deferred.make<void>())
  let lastDepth = -1
  let maxResumed = -1
  const fibers = []
  for (let i = 0; i < n; i++) {
    fibers.push(
      yield* Effect.forkChild(
        Effect.gen(function*() {
          yield* Deferred.await(ds[i])
          maxResumed = Math.max(maxResumed, i)
          if (i === n - 1) lastDepth = new Error().stack!.split("\n").length
          yield* Deferred.succeed(ds[i + 1], undefined)
        })
      )
    )
  }
  yield* Effect.sleep(5) // all waiters suspended
  const baseDepth = yield* Effect.sync(() => new Error().stack!.split("\n").length)
  yield* Deferred.succeed(ds[0], undefined)
  const exits = yield* Fiber.awaitAll(fibers)
  const failures = exits.filter(Exit.isFailure)
  return {
    n,
    baseDepth,
    lastDepth,
    framesPerHop: lastDepth > 0 ? ((lastDepth - baseDepth) / n).toFixed(1) : "n/a",
    maxResumedIndex: maxResumed,
    failedFibers: failures.length,
    firstFailure: failures[0] ? String(failures[0].cause).slice(0, 160) : "none"
  }
})

const exit = await Effect.runPromiseExit(program)
console.log(Exit.isSuccess(exit) ? JSON.stringify(exit.value) : `n=${n} program failed: ${String(exit.cause).slice(0, 300)}`)
