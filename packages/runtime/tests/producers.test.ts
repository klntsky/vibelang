import { expect, test } from "bun:test"
import { Cause, Deferred, Effect, Exit, Fiber, Stream } from "effect"
import { $finally, $forOf, $normal, $produce, $yield, $yieldAll } from "vibelang-runtime"
import { hooks } from "vibelang-runtime/kernel"

const producer = (values: Iterable<number>, events: string[]) => $produce(() => $finally(
  Effect.gen(function*() {
    events.push("open")
    for (const value of values) yield* $yield(value)
  }),
  Effect.sync(() => { events.push("close") })
))

// D26: all three iteration paths dispatch the same completion records.
// The values and target labels vary independently of the implementation.
for (const source of ["iterable", "stream", "producer"] as const) {
  for (const jump of ["normal", "continue", "break", "return", "outer break", "outer continue"] as const) {
    test(`${source}: ${jump} completion preserves values, target labels, and cleanup`, async () => {
      const events: string[] = []
      const values = [3, 7, 11, 19, 23]
      const input = source === "producer" ? producer(values, events)
        : source === "stream" ? Stream.ensuring(Stream.fromIterable(values), Effect.sync(() => { events.push("close") }))
        : values
      const seen: number[] = []
      const completion = jump === "outer break" ? { kind: "break", label: "outer" }
        : jump === "outer continue" ? { kind: "continue", label: "outer" }
        : jump === "return" ? { kind: "return", value: 42 }
        : { kind: jump, label: "inner" }
      const result = await Effect.runPromise($forOf(input as any, (value: number) => Effect.sync(() => {
        seen.push(value)
        return value === 7 ? completion : undefined
      }), "inner"))
      const continues = jump === "normal" || jump === "continue"
      expect(seen).toEqual(continues ? values : [3, 7])
      expect(result).toEqual(continues || jump === "break" ? $normal : completion)
      expect(events).toEqual(source === "producer" ? ["open", "close"] : source === "stream" ? ["close"] : [])
    })
  }
}

test("producer collection is cold, rerunnable, and preserves generated finite sequences", async () => {
  // A deterministic property sweep catches duplication, loss, reordering,
  // premature termination, and shared execution state across both adapters.
  let seed = 0x18c0ffee
  for (let trial = 0; trial < 40; trial++) {
    const values = Array.from({ length: trial * 7 % 53 }, () => {
      seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0
      return seed % 2001 - 1000
    })
    const events: string[] = []
    const input = producer(values, events)
    expect(events).toEqual([])
    const seen: number[] = []
    const loop = $forOf(input, value => Effect.sync(() => { seen.push(value) }))
    await Effect.runPromise(loop)
    await Effect.runPromise(loop)
    expect(seen).toEqual([...values, ...values])
    expect(await Effect.runPromise(Stream.runCollect(input))).toEqual(values)
    expect(await Effect.runPromise(Stream.runCollect(input))).toEqual(values)
    expect(events).toEqual(["open", "close", "open", "close", "open", "close", "open", "close"])
  }
})

for (const source of ["producer", "stream"] as const) {
  for (const failure of ["expected", "defect", "interruption"] as const) {
    test(`${source}: loop body ${failure} closes the source before propagating`, async () => {
      const events: string[] = []
      const input = source === "producer" ? producer([1, 2, 3], events)
        : Stream.ensuring(Stream.make(1, 2, 3), Effect.sync(() => { events.push("close") }))
      const cause = failure === "expected" ? Cause.fail("body") : failure === "defect" ? Cause.die("body") : Cause.interrupt(123)
      const seen: number[] = []
      const exit = await Effect.runPromiseExit($forOf(input, value => Effect.gen(function*() {
        seen.push(value)
        if (value === 2) return yield* Effect.failCause(cause)
      })))
      expect(exit).toEqual(Exit.failCause(cause))
      expect(seen).toEqual([1, 2])
      expect(events).toEqual(source === "producer" ? ["open", "close"] : ["close"])
    })
  }
}

test("early exit retains a producer cleanup failure and a loop failure together", async () => {
  const input = $produce(() => $finally($yield(1), Effect.fail("cleanup")))
  const exit = await Effect.runPromiseExit($forOf(input, () => Effect.fail("body")))
  expect(Exit.isFailure(exit) && exit.cause.reasons.filter(Cause.isFailReason).map(r => r.error)).toEqual(["body", "cleanup"])
})

test("a Stream pull can move between fibers without running the producer ahead", async () => {
  const events: string[] = []
  const input = $produce(() => $finally(Effect.gen(function*() {
    for (let value = 0; value < 3; value++) {
      events.push(`before:${value}`)
      yield* $yield(value)
      events.push(`after:${value}`)
    }
  }), Effect.sync(() => { events.push("closed") })))
  await Effect.runPromise(Effect.scoped(Effect.gen(function*() {
    const pull = yield* Stream.toPull(input)
    expect(events).toEqual([])
    for (let value = 0; value < 3; value++) {
      const reader = yield* Effect.forkChild(pull)
      expect(yield* Fiber.join(reader)).toEqual([value])
      expect(events.at(-1)).toBe(`before:${value}`)
    }
  })))
  expect(events).toEqual(["before:0", "after:0", "before:1", "after:1", "before:2", "closed"])
})

test("interrupting an external pull waits for asynchronous producer cleanup", async () => {
  const events: string[] = []
  await Effect.runPromise(Effect.gen(function*() {
    const entered = yield* Deferred.make<void>()
    const cleaning = yield* Deferred.make<void>()
    const release = yield* Deferred.make<void>()
    const input = $produce(() => $finally(
      Effect.andThen(Deferred.succeed(entered, undefined), Effect.never),
      Effect.gen(function*() {
        yield* Deferred.succeed(cleaning, undefined)
        yield* Deferred.await(release)
        events.push("closed")
      })
    ))
    const consumer = yield* Effect.forkChild(Stream.runCollect(input))
    yield* Deferred.await(entered)
    consumer.interruptUnsafe(123)
    yield* Deferred.await(cleaning)
    expect(consumer.pollUnsafe()).toBeUndefined()
    expect(events).toEqual([])
    yield* Deferred.succeed(release, undefined)
    const exit = yield* Fiber.await(consumer)
    expect(Exit.isFailure(exit) && exit.cause.reasons.some(Cause.isInterruptReason)).toBe(true)
    expect(events).toEqual(["closed"])
  }))
})

test("yield delegation preserves nested cleanup on break", async () => {
  const events: string[] = []
  const outer = $produce(() => $finally(Effect.gen(function*() {
    yield* $yieldAll([1, 2])
    yield* $yieldAll(producer([3, 4, 5], events))
    events.push("too late")
  }), Effect.sync(() => { events.push("outer closed") })))
  const seen: number[] = []
  await Effect.runPromise($forOf(outer, value => Effect.sync(() => {
    seen.push(value)
    return value === 4 ? { kind: "break", label: "" } : $normal
  })))
  expect(seen).toEqual([1, 2, 3, 4])
  expect(events).toEqual(["open", "close", "outer closed"])
})

test("a 100,000-element delegated producer keeps a bounded continuation stack", async () => {
  const previous = hooks.onCapture
  let maximum = 0
  let captures = 0
  hooks.onCapture = (fiber, k) => {
    maximum = Math.max(maximum, fiber._stack.length + k.segment.length)
    captures++
  }
  try {
    const inner = $produce(() => Effect.gen(function*() {
      for (let n = 0; n < 100_000; n++) yield* $yield(n)
    }))
    const input = $produce(() => $yieldAll(inner))
    let count = 0
    let sum = 0
    expect(await Effect.runPromise($forOf(input, n => Effect.sync(() => { count++; sum += n })))).toEqual($normal)
    expect(count).toBe(100_000)
    expect(sum).toBe(4_999_950_000)
    expect(captures).toBe(100_000)
    expect(maximum).toBeLessThan(40)
  } finally {
    hooks.onCapture = previous
  }
})
