// Q2: scheduler behaviour. (1) Is a yield a microtask or a macrotask?
// (2) How many ops before an automatic yield? (3) Do yieldNowWith priorities
// order tasks across fibers? (4) Does Effect.sleep use setTimeout/clearTimeout?
import { Effect, Fiber, References, Scheduler } from "effect"

const log: Array<string> = []

// (1) ordering of Effect.yieldNow vs microtask / setTimeout(0) / setImmediate
const ordering = Effect.gen(function*() {
  yield* Effect.sync(() => {
    Promise.resolve().then(() => log.push("microtask"))
    setTimeout(() => log.push("setTimeout(0)"), 0)
    if (typeof setImmediate === "function") setImmediate(() => log.push("setImmediate (registered before yield)"))
  })
  yield* Effect.yieldNow
  log.push("fiber resumed after Effect.yieldNow")
})

// (2) count ops between automatic yields: a fiber running an uninterrupted
// chain of Effect.sync and another fiber that records when it gets to run.
const autoYield = (maxOps: number | undefined) =>
  Effect.gen(function*() {
    let counter = 0
    let observedAt = -1
    const spinner = Effect.forEach(Array.from({ length: 10_000 }), () => Effect.sync(() => counter++), {
      discard: true
    })
    const other = Effect.sync(() => {
      if (observedAt === -1) observedAt = counter
    })
    const a = yield* Effect.forkChild(spinner)
    const b = yield* Effect.forkChild(other)
    yield* Fiber.join(a)
    yield* Fiber.join(b)
    return observedAt
  }).pipe(maxOps === undefined ? (x) => x : Effect.provideService(References.MaxOpsBeforeYield, maxOps))

// (3) priority across fibers vs within one fiber
const priorities = Effect.gen(function*() {
  const order: Array<string> = []
  const f1 = yield* Effect.forkChild(Effect.andThen(Effect.yieldNowWith(5), Effect.sync(() => order.push("fiber1 prio 5"))))
  const f2 = yield* Effect.forkChild(Effect.andThen(Effect.yieldNowWith(-5), Effect.sync(() => order.push("fiber2 prio -5"))))
  yield* Fiber.join(f1)
  yield* Fiber.join(f2)
  return order
})

// (4) Effect.sleep timer calls
const timers = Effect.gen(function*() {
  const calls: Array<string> = []
  const origSet = globalThis.setTimeout
  const origClear = globalThis.clearTimeout
  ;(globalThis as any).setTimeout = (fn: any, ms: any, ...rest: any[]) => {
    calls.push(`setTimeout(${ms})`)
    return origSet(fn, ms, ...rest)
  }
  ;(globalThis as any).clearTimeout = (h: any) => {
    calls.push("clearTimeout")
    return origClear(h)
  }
  const f = yield* Effect.forkChild(Effect.sleep("10 seconds"))
  yield* Effect.yieldNow
  yield* Fiber.interrupt(f)
  yield* Effect.sleep(0)
  ;(globalThis as any).setTimeout = origSet
  ;(globalThis as any).clearTimeout = origClear
  return calls
})

const sameDispatcher = Effect.gen(function*() {
  const a = yield* Effect.forkChild(Effect.never)
  const b = yield* Effect.forkChild(Effect.never)
  yield* Effect.yieldNow
  const self = yield* Effect.fiber
  const same = (a as any).currentDispatcher === (b as any).currentDispatcher
  const parentSame = (self as any).currentDispatcher === (a as any).currentDispatcher
  yield* Fiber.interrupt(a)
  yield* Fiber.interrupt(b)
  return { siblingDispatchersIdentical: same, parentChildDispatcherIdentical: parentSame }
})

const main = Effect.gen(function*() {
  yield* ordering
  console.log("(1) order:", log.join(" -> "))
  console.log("(2) default MaxOpsBeforeYield ref value:", yield* References.MaxOpsBeforeYield)
  console.log("(2) other fiber first ran after counter =", yield* autoYield(undefined), "(default)")
  console.log("(2) other fiber first ran after counter =", yield* autoYield(100), "(MaxOpsBeforeYield=100)")
  console.log(
    "(2) other fiber first ran after counter =",
    yield* autoYield(undefined).pipe(Effect.provideService(References.PreventSchedulerYield, true)),
    "(PreventSchedulerYield=true)"
  )
  console.log("(3) cross-fiber priority order:", (yield* priorities).join(", "))
  console.log("(4) timer calls for sleep(10s)+interrupt:", (yield* timers).join(", "))
  console.log("(5) dispatchers:", JSON.stringify(yield* sameDispatcher))
  const sched = yield* Effect.withFiber((f) => Effect.succeed((f as any).cache.scheduler))
  console.log(
    "(6) default scheduler:",
    sched.constructor.name,
    "executionMode=" + sched.executionMode,
    "isMixed=" + (sched instanceof Scheduler.MixedScheduler)
  )
})

await Effect.runPromise(main)
