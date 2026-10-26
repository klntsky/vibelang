// Q1: does a waiter resume synchronously on the completer's JS stack?
// For each primitive: a waiter fiber suspends, then a completer fiber runs
// [set flag] -> [complete] -> [clear flag]. The waiter records the flag when it
// resumes. flag=true means the waiter ran nested inside the completer's step.
import { Deferred, Effect, Exit, Fiber, Latch, PubSub, Queue, Semaphore } from "effect"

let inCompleter = false
const results: Array<string> = []

const probe = (name: string, setup: Effect.Effect<{
  wait: Effect.Effect<unknown>
  complete: Effect.Effect<unknown>
}, never, any>) =>
  Effect.scoped(Effect.gen(function*() {
    const { wait, complete } = yield* setup
    let seen: string = "not-resumed"
    const waiter = yield* Effect.forkChild(
      Effect.andThen(wait, Effect.sync(() => {
        seen = inCompleter ? "SYNC (nested in completer step)" : "DEFERRED (dispatcher/timer task)"
      }))
    )
    // let the waiter start and suspend
    yield* Effect.sleep(5)
    yield* Effect.sync(() => {
      inCompleter = true
    })
    yield* complete
    yield* Effect.sync(() => {
      inCompleter = false
    })
    const exit = yield* Fiber.await(waiter)
    results.push(`${name.padEnd(42)} ${seen}${exit._tag === "Failure" ? "  (waiter exit: Failure)" : ""}`)
  }))

const program = Effect.gen(function*() {
  yield* probe("Deferred.succeed", Effect.map(Deferred.make<number>(), (d) => ({
    wait: Deferred.await(d),
    complete: Deferred.succeed(d, 1)
  })))
  yield* probe("Deferred.done", Effect.map(Deferred.make<number>(), (d) => ({
    wait: Deferred.await(d),
    complete: Deferred.done(d, Exit.succeed(1))
  })))
  yield* probe("Queue.offer (unbounded) -> take", Effect.map(Queue.unbounded<number>(), (q) => ({
    wait: Queue.take(q),
    complete: Queue.offer(q, 1)
  })))
  yield* probe("Queue.offerUnsafe + flushUnsafe -> take", Effect.map(Queue.unbounded<number>(), (q) => ({
    wait: Queue.take(q),
    complete: Effect.sync(() => {
      Queue.offerUnsafe(q, 1)
      Queue.flushUnsafe(q)
    })
  })))
  yield* probe("Queue.take -> blocked offer (bounded 1)", Effect.gen(function*() {
    const q = yield* Queue.bounded<number>(1)
    yield* Queue.offer(q, 0)
    return { wait: Queue.offer(q, 1), complete: Queue.take(q) }
  }))
  yield* probe("Queue.offer (rendezvous cap 0) -> take", Effect.map(Queue.bounded<number>(0), (q) => ({
    wait: Queue.take(q),
    complete: Queue.offer(q, 1)
  })))
  yield* probe("Queue.shutdown -> take", Effect.map(Queue.unbounded<number>(), (q) => ({
    wait: Effect.exit(Queue.take(q)),
    complete: Queue.shutdown(q)
  })))
  yield* probe("Latch.open", Effect.map(Latch.make(false), (l) => ({
    wait: l.await,
    complete: l.open
  })))
  yield* probe("Latch.openUnsafe", Effect.map(Latch.make(false), (l) => ({
    wait: l.await,
    complete: Effect.sync(() => l.openUnsafe())
  })))
  yield* probe("Latch.release", Effect.map(Latch.make(false), (l) => ({
    wait: l.await,
    complete: l.release
  })))
  yield* probe("Semaphore.release", Effect.gen(function*() {
    const s = yield* Semaphore.make(1)
    yield* Semaphore.take(s, 1)
    return { wait: Semaphore.take(s, 1), complete: Semaphore.release(s, 1) }
  }))
  yield* probe("PubSub.publish -> subscription take", Effect.gen(function*() {
    const ps = yield* PubSub.unbounded<number>()
    const sub = yield* PubSub.subscribe(ps)
    return { wait: PubSub.take(sub), complete: PubSub.publish(ps, 1) }
  }))
  yield* probe("Fiber completion -> Fiber.join", Effect.gen(function*() {
    const d = yield* Deferred.make<void>()
    const target = yield* Effect.forkChild(Deferred.await(d))
    // completer completes `target` by releasing its deferred; target then exits and
    // notifies its observers (the joiner) from inside its own evaluate()
    return { wait: Fiber.join(target), complete: Deferred.succeed(d, undefined) }
  }))
  yield* probe("Fiber.interrupt target -> Fiber.await", Effect.gen(function*() {
    const target = yield* Effect.forkChild(Effect.never)
    return { wait: Fiber.await(target), complete: Fiber.interrupt(target) }
  }))
  yield* probe("Effect.callback resume called later", Effect.sync(() => {
    let resume!: (e: Effect.Effect<void>) => void
    return {
      wait: Effect.callback<void>((r) => {
        resume = r
      }),
      complete: Effect.sync(() => resume(Effect.void))
    }
  }))
})

await Effect.runPromise(program)
for (const r of results) console.log(r)
