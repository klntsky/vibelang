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

for (const outcome of ["normal", "break", "return", "failure", "defect", "interrupt"] as const) {
  test(`iterable ${outcome}: loop closes a JavaScript generator exactly once`, async () => {
    const events: string[] = []
    const values = {
      *[Symbol.iterator]() {
        try {
          yield 1
          yield 2
          events.push("exhausted")
        } finally {
          events.push("closed")
        }
      }
    }
    const body = () => outcome === "failure" ? Effect.fail("body")
      : outcome === "defect" ? Effect.die("body")
      : outcome === "interrupt" ? Effect.interrupt
      : Effect.succeed(outcome === "normal" ? $normal : { kind: outcome, label: "", value: 42 })
    const exit = await Effect.runPromiseExit($forOf(values, body))
    if (outcome === "normal") expect(exit).toEqual(Exit.succeed($normal))
    else if (outcome === "break") expect(exit).toEqual(Exit.succeed($normal))
    else if (outcome === "return") expect(exit).toEqual(Exit.succeed({ kind: "return", label: "", value: 42 }))
    else expect(Exit.isFailure(exit)).toBe(true)
    expect(events).toEqual(outcome === "normal" ? ["exhausted", "closed"] : ["closed"])
  })
}

test("abandoning yield delegation closes the delegated JavaScript generator", async () => {
  let closed = 0
  const values = {
    *[Symbol.iterator]() {
      try { yield 1; yield 2 } finally { closed++ }
    }
  }
  const input = $produce(() => $yieldAll(values))
  await Effect.runPromise($forOf(input, () => Effect.succeed({ kind: "break", label: "" })))
  expect(closed).toBe(1)
})

test("iterable cleanup failures accompany body failures, and exhausted iterators are not closed twice", async () => {
  let returned = 0
  const values = {
    [Symbol.iterator]() {
      return {
        next: () => ({ done: false as const, value: 1 }),
        return() { returned++; throw new Error("close failed") }
      }
    }
  }
  const exit = await Effect.runPromiseExit($forOf(values, () => Effect.fail("body failed")))
  expect(returned).toBe(1)
  expect(Exit.isFailure(exit) && exit.cause.reasons.filter(Cause.isFailReason).map(r => r.error)).toEqual(["body failed"])
  expect(Exit.isFailure(exit) && String(exit.cause.reasons.find(Cause.isDieReason)?.defect)).toContain("close failed")
  const exhausted = {
    [Symbol.iterator]() {
      return {
        next: () => ({ done: true as const, value: undefined }),
        return() { returned++; return { done: true as const, value: undefined } }
      }
    }
  }
  await Effect.runPromise($forOf(exhausted, () => Effect.die("unreachable")))
  expect(returned).toBe(1)
})

test("iterable next failures and invalid results match JavaScript iterator validation", async () => {
  let returned = 0
  const invalid = (next: () => unknown, close?: () => unknown) => ({
    [Symbol.iterator]() { return { next, return: close ?? (() => { returned++; return { done: true } }) } }
  }) as Iterable<number>
  const defect = new Error("next failed")
  expect(await Effect.runPromiseExit($forOf(invalid(() => { throw defect }), () => Effect.void))).toEqual(Exit.die(defect))
  expect(returned).toBe(0)
  for (const source of [invalid(() => 1), invalid(() => ({ done: false, value: 1 }), () => 42), invalid(() => ({ done: false, value: 1 }), () => undefined)]) {
    const exit = await Effect.runPromiseExit($forOf(source, () => Effect.succeed({ kind: "break", label: "" })))
    expect(Exit.isFailure(exit) && exit.cause.reasons.find(Cause.isDieReason)?.defect).toBeInstanceOf(TypeError)
  }
})

test("iterable loops cache next, preserve its receiver, and skip return when a value getter throws", async () => {
  const events: string[] = []
  let index = 0
  const iterator = {
    get next() {
      events.push("get next")
      return function(this: unknown) {
        expect(this).toBe(iterator)
        return Object.assign(() => {}, { done: false, value: ++index })
      }
    },
    return() { events.push("return"); return Object.assign(() => {}, { done: true }) }
  }
  await Effect.runPromise($forOf({ [Symbol.iterator]: () => iterator }, value => Effect.succeed(value === 2 ? { kind: "break", label: "" } : $normal)))
  expect(events).toEqual(["get next", "return"])
  expect(index).toBe(2)
  const defect = new Error("value getter failed")
  const broken = {
    [Symbol.iterator]() {
      return {
        next: () => ({ done: false, get value(): number { throw defect } }),
        return() { events.push("unexpected return"); return { done: true, value: undefined } }
      }
    }
  }
  expect(await Effect.runPromiseExit($forOf(broken, () => Effect.void))).toEqual(Exit.die(defect))
  expect(events).toEqual(["get next", "return"])
})
