// Runtime regression probe: Chapter 6's callback adapter (design.md lines 845-856)
// wraps kernel `suspend` in Effect.acquireRelease and annotates two paths:
//   if (!waiter.resume(socket)) socket.destroy()   // "nobody is waiting anymore"
//   return () => socket.destroy()                  // "the waiter was cancelled first"
// Effect.acquireRelease masks its acquire (internal/effect.ts:4143-4159, the
// acquire is NOT wrapped in restore unless { interruptible: true }). So while
// the fiber is parked in `suspend`, an interruption cannot reach it. Claim:
// neither annotated path can ever run in that example; interruption waits for
// the connect, then the release runs. With { interruptible: true } the paths
// become live.
import { expect, test } from "bun:test"
import { Effect, Exit, Fiber } from "effect"
import { suspend } from "../../packages/runtime/src/kernel/kernel.ts"

type Sock = { id: number; destroyed: boolean }

const run = async (interruptible: boolean) => {
  const events: Array<string> = []
  let connectNow: (() => void) | undefined
  const connect = Effect.acquireRelease(
    suspend<Sock>((waiter) => {
      const socket: Sock = { id: 1, destroyed: false }
      connectNow = () => {
        if (!waiter.resume(socket)) {
          socket.destroyed = true
          events.push("late: producer destroyed the socket")
        } else {
          events.push("delivered")
        }
      }
      return () => {
        events.push("registration cleanup ran (waiter cancelled first)")
      }
    }),
    (socket) => Effect.sync(() => {
      socket.destroyed = true
      events.push("release ran")
    }),
    interruptible ? { interruptible: true } : undefined
  )
  const session = Effect.scoped(Effect.flatMap(connect, () => Effect.sync(() => void events.push("handshake (must not run after interrupt)"))))
  const exit = await Effect.runPromiseExit(Effect.gen(function*() {
    const fiber = yield* Effect.forkChild(session)
    yield* Effect.sleep("2 millis")
    yield* Effect.forkChild(Fiber.interrupt(fiber))
    yield* Effect.sleep("2 millis")
    events.push(`fiber exited before connect: ${fiber.pollUnsafe() !== undefined}`)
    connectNow!()
    return yield* Fiber.await(fiber)
  }))
  return { events, interrupted: Exit.isSuccess(exit) && Exit.hasInterrupts(exit.value) }
}

// 2026-10-16: kernel regressions K15a-K15c (tests/review-fixes.test.ts) now
// cover both policies. The expected order below was corrected: the wake-up
// runs the fiber synchronously inside waiter.resume, so the release runs
// before resume returns and "delivered" is logged; the review text already
// recorded this order.
test("default acquireRelease: the suspend is masked; interruption waits for connect; the adapter's race paths never run", async () => {
  const { events, interrupted } = await run(false)
  console.log(["", "--- masked acquire", ...events].join("\n"))
  expect(interrupted).toBe(true)
  expect(events).toEqual([
    "fiber exited before connect: false",
    "release ran",
    "delivered"
  ])
})

test("acquireRelease({ interruptible: true }): interruption cancels the wait; the late value is the producer's to destroy", async () => {
  const { events, interrupted } = await run(true)
  console.log(["", "--- interruptible acquire", ...events].join("\n"))
  expect(interrupted).toBe(true)
  expect(events).toEqual([
    "registration cleanup ran (waiter cancelled first)",
    "fiber exited before connect: true",
    "late: producer destroyed the socket"
  ])
})
