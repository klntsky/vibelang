// The for...of lowering of §5.9: internal iteration. The loop installs a Yield
// handler whose clause runs the loop body and then resumes the producer;
// `break` returns from the clause without resuming, so the Owner frame
// abandons the producer and its finalizers run before the loop exits.
//
//   function* naturals() {
//     using _ = Log.span("naturals")          // here: a finalizer
//     for (let i = 0;; i++) yield i
//   }
//   function firstThree() {
//     const out: number[] = []
//     for (const n of naturals()) { out.push(n); if (out.length === 3) break }
//     return out
//   }
import { expect, test } from "bun:test"
import { Effect, Exit } from "effect"
import { type Continuation, handle, hooks, operation, resume } from "../../packages/runtime/src/kernel/kernel.ts"

const Yield = operation<number, void>("Yield")

// Generated shape of `for (const x of producer) body`.
const forOf = (
  producer: Effect.Effect<unknown, unknown, never>,
  body: (value: number) => Effect.Effect<"continue" | "break">
) =>
  handle("for-of", [Yield], Effect.asVoid(producer), {
    Yield: (value: number, k: Continuation) =>
      Effect.flatMap(body(value), (signal) => (signal === "break" ? Effect.void : resume(k, undefined)))
  })

test("T16 for...of over a producer: break abandons it and its finalizer runs before the loop exits", async () => {
  const events: Array<string> = []
  const note = (line: string) => Effect.sync(() => void events.push(line))
  const naturals = Effect.scoped(Effect.gen(function*() {
    yield* Effect.addFinalizer(() => note("producer finalizer"))
    for (let i = 0;; i++) yield* Yield(i)
  }))
  const firstThree = Effect.gen(function*() {
    const out: Array<number> = []
    yield* forOf(naturals, (n) => Effect.sync(() => {
      out.push(n)
      return out.length === 3 ? "break" : "continue"
    }))
    yield* note(`loop exited with ${out.join(",")}`)
    return out
  })
  const exit = await Effect.runPromiseExit(firstThree)
  expect(Exit.isSuccess(exit) && exit.value).toEqual([0, 1, 2])
  expect(events).toEqual(["producer finalizer", "loop exited with 0,1,2"])
})

test("T17 iterating 100,000 elements with tail resumption runs in constant stack space", async () => {
  let maxDepth = 0
  hooks.onResume = (fiber) => {
    maxDepth = Math.max(maxDepth, fiber._stack.length)
  }
  try {
    const N = 100_000
    let sum = 0
    const counter = Effect.gen(function*() {
      for (let i = 0; i < N; i++) yield* Yield(i)
    })
    const exit = await Effect.runPromiseExit(forOf(counter, (n) => Effect.sync(() => {
      sum += n
      return "continue" as const
    })))
    expect(Exit.isSuccess(exit)).toBe(true)
    expect(sum).toBe((N * (N - 1)) / 2)
    expect(maxDepth).toBeLessThan(20)
  } finally {
    hooks.onResume = undefined
  }
})
