import { describe, expect, test } from "bun:test"
import { Cause, Deferred, Effect, Exit, Fiber, Result, Scope } from "effect"
import {
  $await, $awaitAll, $awaitAllSettled, $awaitAny, $awaitRace, $forConcurrent,
  $join, $select, detach, forkIn
} from "vibelang-runtime"

// D167, D196, D225: these operands may contain already-running fibers.
// Joining a wrapper fiber alone does not stop those fibers on failure.
for (const [name, join] of [["join", $join], ["legacy await", $await]] as const) {
  describe(name, () => {
    test("keeps tuple and record shape, values, and written key order", async () => {
      await Effect.runPromise(Effect.gen(function*() {
        const ready = yield* Deferred.make<number>()
        const first = yield* Effect.forkChild(Deferred.await(ready))
        const second = yield* Effect.forkChild(Effect.succeed("second"))
        const waiting = yield* Effect.forkChild(join({ first, second }))
        yield* Deferred.succeed(ready, 42)
        const value = yield* Fiber.join(waiting)
        expect(value).toEqual({ first: 42, second: "second" })
        expect(Object.keys(value)).toEqual(["first", "second"])
        expect(yield* join([first, second] as const)).toEqual([42, "second"])
        expect(yield* join(first)).toBe(42)
        expect(yield* join([])).toEqual([])
        expect(yield* join({})).toEqual({})
      }))
    })

    test("failure interrupts already-running siblings and waits for cleanup", async () => {
      const events: string[] = []
      await Effect.runPromise(Effect.gen(function*() {
        const started = yield* Deferred.make<void>()
        const child = yield* Effect.forkChild(Effect.ensuring(
          Effect.andThen(Deferred.succeed(started, undefined), Effect.never),
          Effect.andThen(Effect.yieldNow, Effect.sync(() => { events.push("closed") }))
        ))
        yield* Deferred.await(started)
        const failed = yield* Effect.forkChild(Effect.fail("broken"))
        const exit = yield* Effect.exit(join({ child, failed }))
        expect(exit).toEqual(Exit.fail("broken"))
        expect(child.pollUnsafe()).toBeDefined()
        expect(events).toEqual(["closed"])
      }))
    })
  })
}

test("legacy await accepts Deferred leaves, including nested runtime collections", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const deferred = yield* Deferred.make<number>()
    yield* Deferred.succeed(deferred, 7)
    expect(yield* $await(deferred)).toBe(7)
    expect(yield* $await({ nested: [deferred] } as any)).toEqual({ nested: [7] })
    for (const value of [null, undefined, 1, "bad", () => 1]) {
      const exit = yield* Effect.exit($await(value))
      expect(Exit.isFailure(exit) && exit.cause.reasons.some(r => Cause.isDieReason(r) && r.defect instanceof TypeError)).toBe(true)
    }
  }))
})

test("await.all starts programs concurrently and preserves mixed operand order", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const started = yield* Deferred.make<void>()
    const later = yield* Deferred.make<number>()
    const fiber = yield* Effect.forkChild(Effect.succeed("fiber"))
    const all = yield* Effect.forkChild($awaitAll({
      slow: Deferred.await(later),
      signal: Effect.andThen(Deferred.succeed(started, undefined), Effect.succeed("effect")),
      fiber,
      deferred: started
    }))
    yield* Deferred.await(started)
    yield* Deferred.succeed(later, 42)
    const values = yield* Fiber.join(all)
    expect(values).toEqual({ slow: 42, signal: "effect", fiber: "fiber", deferred: undefined })
    expect(Object.keys(values)).toEqual(["slow", "signal", "fiber", "deferred"])
  }))
})

test("await.all reads a single-use iterable once and remains rerunnable", async () => {
  let iterations = 0
  let runs = 0
  const values = {
    *[Symbol.iterator]() {
      expect(++iterations).toBe(1)
      yield Effect.sync(() => ++runs)
      yield Effect.succeed("second")
    }
  }
  const all = $awaitAll(values)
  expect(runs).toBe(0)
  expect(await Effect.runPromise(all)).toEqual([1, "second"])
  expect(await Effect.runPromise(all)).toEqual([2, "second"])
})

test("await.all failure closes held fibers before propagating", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const started = yield* Deferred.make<void>()
    let closed = false
    const held = yield* Effect.forkChild(Effect.ensuring(
      Effect.andThen(Deferred.succeed(started, undefined), Effect.never),
      Effect.sync(() => { closed = true })
    ))
    yield* Deferred.await(started)
    expect(yield* Effect.exit($awaitAll([held, Effect.fail("failed")]))).toEqual(Exit.fail("failed"))
    expect(closed).toBe(true)
  }))
})

test("await.allSettled keeps success and expected failure Results in written order", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const ready = yield* Deferred.make<number>()
    const held = yield* Effect.forkChild(Effect.fail("held failure"))
    yield* Deferred.succeed(ready, 3)
    const values = yield* $awaitAllSettled({ ready, failed: Effect.fail("bad"), held, ok: Effect.succeed(4) })
    expect(values).toEqual({ ready: Result.succeed(3), failed: Result.fail("bad"), held: Result.fail("held failure"), ok: Result.succeed(4) })
    expect(Object.keys(values)).toEqual(["ready", "failed", "held", "ok"])
    expect(yield* $awaitAllSettled([])).toEqual([])
  }))
})

test("await.race picks the first ready operand without starting later programs", async () => {
  let started = 0
  const later = Effect.sync(() => ++started)
  expect(await Effect.runPromise($awaitRace([Effect.succeed("first"), later]))).toBe("first")
  expect(await Effect.runPromiseExit($awaitRace([Effect.fail("first failure"), later]))).toEqual(Exit.fail("first failure"))
  expect(started).toBe(0)
})

for (const success of [true, false]) {
  test(`await.race waits for a delayed ${success ? "success" : "failure"} and closes held losers`, async () => {
    await Effect.runPromise(Effect.gen(function*() {
      const started = yield* Deferred.make<void>()
      let closed = false
      const held = yield* Effect.forkChild(Effect.ensuring(
        Effect.andThen(Deferred.succeed(started, undefined), Effect.never),
        Effect.sync(() => { closed = true })
      ))
      yield* Deferred.await(started)
      const winner = Effect.andThen(Effect.yieldNow, success ? Effect.succeed(42) : Effect.fail("bad"))
      expect(yield* Effect.exit($awaitRace([held, winner]))).toEqual(success ? Exit.succeed(42) : Exit.fail("bad"))
      expect(closed).toBe(true)
    }))
  })
}

test("an empty await.race stays pending until interrupted", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const race = yield* Effect.forkChild($awaitRace([]), { startImmediately: true })
    expect(race.pollUnsafe()).toBeUndefined()
    yield* Fiber.interrupt(race)
    expect(Exit.isFailure(race.pollUnsafe()!)).toBe(true)
  }))
})

test("await.any ignores early failures, keeps the winning fiber, and closes held losers", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const started = yield* Deferred.make<void>()
    let closed = false
    const loser = yield* Effect.forkChild(Effect.ensuring(
      Effect.andThen(Deferred.succeed(started, undefined), Effect.never),
      Effect.sync(() => { closed = true })
    ))
    yield* Deferred.await(started)
    const winner = yield* Effect.forkChild(Effect.succeed(42))
    expect(yield* $awaitAny([Effect.fail("early"), loser, winner])).toBe(42)
    expect(winner.pollUnsafe()).toEqual(Exit.succeed(42))
    expect(closed).toBe(true)
    const exit = yield* Effect.exit($awaitAny([Effect.fail("a"), Effect.fail("b")]))
    expect(Exit.isFailure(exit) && exit.cause.reasons.filter(Cause.isFailReason).map(r => r.error).sort()).toEqual(["a", "b"])
  }))
})

test("select's default cancels pending cases and synchronous completion takes precedence", async () => {
  let closed = 0
  const pending = () => Effect.ensuring(Effect.never, Effect.sync(() => { closed++ }))
  expect(await Effect.runPromise($select(true, pending, pending))).toEqual({ index: -1, value: undefined })
  expect(closed).toBe(2)
  expect(await Effect.runPromise($select(true, () => Effect.succeed(7), pending))).toEqual({ index: 0, value: 7 })
  expect(closed).toBe(2)
})

test("for concurrent enforces its bound, starts distinct fibers, and joins every item", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const atCapacity = yield* Deferred.make<void>()
    const gate = yield* Deferred.make<void>()
    let active = 0
    let maximum = 0
    const completed: number[] = []
    const ids = new Set<number>()
    const loop = yield* Effect.forkChild($forConcurrent([0, 1, 2, 3, 4, 5], 2, item => Effect.gen(function*() {
      ids.add((yield* Effect.fiber).id)
      active++
      maximum = Math.max(maximum, active)
      if (active === 2) yield* Deferred.succeed(atCapacity, undefined)
      yield* Deferred.await(gate)
      active--
      completed.push(item)
    })))
    yield* Deferred.await(atCapacity)
    expect(active).toBe(2)
    expect(completed).toEqual([])
    yield* Deferred.succeed(gate, undefined)
    expect(yield* Fiber.join(loop)).toBeUndefined()
    expect(maximum).toBe(2)
    expect(completed.sort()).toEqual([0, 1, 2, 3, 4, 5])
    expect(ids.size).toBe(6)
  }))
})

test("for concurrent failure interrupts active items and does not start queued items", async () => {
  const started: number[] = []
  const closed: number[] = []
  const exit = await Effect.runPromiseExit(Effect.gen(function*() {
    const ready = yield* Deferred.make<void>()
    return yield* $forConcurrent([0, 1, 2, 3], 2, item => Effect.ensuring(Effect.gen(function*() {
      started.push(item)
      if (item === 0) return yield* Effect.andThen(Deferred.succeed(ready, undefined), Effect.never)
      return yield* Effect.andThen(Deferred.await(ready), Effect.fail("item failed"))
    }), Effect.sync(() => { closed.push(item) })))
  }))
  expect(exit).toEqual(Exit.fail("item failed"))
  expect(started).toEqual([0, 1])
  expect(closed.sort()).toEqual([0, 1])
  expect(await Effect.runPromise($forConcurrent([], undefined, () => Effect.die("unreachable")))).toBeUndefined()
})

test("forkIn follows its supplied scope; detach survives the creating fiber", async () => {
  await Effect.runPromise(Effect.gen(function*() {
    const gate = yield* Deferred.make<number>()
    const creator = yield* Effect.forkChild(detach(Deferred.await(gate)))
    const detached = yield* Fiber.join(creator)
    expect(detached.pollUnsafe()).toBeUndefined()
    yield* Deferred.succeed(gate, 42)
    expect(yield* Fiber.join(detached)).toBe(42)
    const scope = yield* Scope.make()
    const scoped = yield* forkIn(scope, Effect.never)
    yield* Scope.close(scope, Exit.void)
    expect(Exit.isFailure(scoped.pollUnsafe()!)).toBe(true)
  }))
})
