import { describe, expect, test } from "bun:test"
import { Cause, Deferred, Effect, Exit, Fiber } from "effect"
import { callback, sync, try as attempt, type Resume } from "vibelang-runtime"

test("sync is lazy and rerunnable, passes inputs, and leaves returned Effects inert", async () => {
  let runs = 0
  const program = sync((a: number, b: number) => (++runs, a + b), 20, 22)
  expect(runs).toBe(0)
  expect(Effect.runSync(program)).toBe(42)
  expect(Effect.runSync(program)).toBe(42)
  expect(runs).toBe(2)
  const value = Effect.succeed("inert")
  expect(Effect.runSync(sync(() => value))).toBe(value)
  expect(Effect.runSync(sync(42))).toBe(42)
  expect(await Effect.runPromise(sync(Promise.resolve(43)))).toBe(43)
  expect(await Effect.runPromise(sync(async (n: number) => n * 2, 21))).toBe(42)
})

for (const asynchronous of [false, true]) {
  test(`sync ${asynchronous ? "rejection" : "throw"} is a defect; try maps it to an expected failure`, async () => {
    const error = new Error("foreign failure")
    const evaluate = asynchronous ? async () => { throw error } : () => { throw error }
    expect(await Effect.runPromiseExit(sync(evaluate))).toEqual(Exit.die(error))
    const unmapped = await Effect.runPromiseExit(attempt(evaluate))
    expect(Exit.isFailure(unmapped)).toBe(true)
    if (Exit.isFailure(unmapped)) {
      const failure = unmapped.cause.reasons.find(Cause.isFailReason)?.error
      expect(failure).toBeInstanceOf(Cause.UnknownError)
      expect((failure as Cause.UnknownError).cause).toBe(error)
    }
    expect(await Effect.runPromiseExit(attempt(evaluate, cause => ({ _tag: "Mapped", cause })))).toEqual(
      Exit.fail({ _tag: "Mapped", cause: error })
    )
  })
}

test("try keeps the mapper second, passes inputs, and calls the mapper only on failure", async () => {
  const errors: unknown[] = []
  const mapper = (error: unknown) => { errors.push(error); return "invalid" }
  expect(Effect.runSync(attempt((a: number, b: number) => a + b, mapper, 20, 22))).toBe(42)
  expect(await Effect.runPromise(attempt(async (n: number) => n + 1, 41))).toBe(42)
  expect(Effect.runSync(attempt(42, mapper))).toBe(42)
  expect(await Effect.runPromise(attempt(Promise.resolve(42), mapper))).toBe(42)
  expect(errors).toEqual([])
  expect(Effect.runSyncExit(attempt(JSON.parse, mapper, "{"))).toEqual(Exit.fail("invalid"))
  expect(errors).toHaveLength(1)
  expect(errors[0]).toBeInstanceOf(SyntaxError)
})

test("a try mapper that throws produces a defect", async () => {
  const defect = new Error("bad mapper")
  for (const evaluate of [() => { throw "foreign" }, async () => { throw "foreign" }]) {
    expect(await Effect.runPromiseExit(attempt(evaluate, () => { throw defect }))).toEqual(Exit.die(defect))
  }
})

describe("callback", () => {
  test("passes inputs before resume and signal, supports synchronous and delayed completion, and reruns registration", async () => {
    let runs = 0
    const program = callback((a: number, b: number, resume: Resume<number, never>, signal: AbortSignal) => {
      runs++
      expect(signal).toBeInstanceOf(AbortSignal)
      expect(signal.aborted).toBe(false)
      resume(a + b)
    }, 20, 22)
    expect(runs).toBe(0)
    expect(await Effect.runPromise(program)).toBe(42)
    expect(await Effect.runPromise(program)).toBe(42)
    expect(runs).toBe(2)
    expect(await Effect.runPromise(callback((resume: Resume<number, never>) => {
      queueMicrotask(() => resume(42))
    }))).toBe(42)
  })

  test("fail and exit retain expected failures, defects, and interruption", async () => {
    expect(await Effect.runPromiseExit(callback((resume: Resume<never, string>) => resume.fail("bad")))).toEqual(Exit.fail("bad"))
    for (const exit of [Exit.succeed(42), Exit.fail("bad"), Exit.die("defect"), Exit.interrupt(123)]) {
      expect(await Effect.runPromiseExit(callback((resume: Resume<number, string>) => resume.exit(exit)))).toEqual(exit)
    }
    const defect = new Error("registration failed")
    expect(await Effect.runPromiseExit(callback(() => { throw defect }))).toEqual(Exit.die(defect))
  })

  test("the first completion wins and successful completion does not run cancellation", async () => {
    let canceled = 0
    const value = await Effect.runPromise(callback((resume: Resume<number, string>) => {
      resume(1)
      resume(2)
      resume.fail("late")
      return () => { canceled++ }
    }))
    expect(value).toBe(1)
    expect(canceled).toBe(0)
  })

  test("interruption aborts the signal, runs cancellation once, and ignores late delivery", async () => {
    const events: string[] = []
    let deliver!: Resume<number, never>
    await Effect.runPromise(Effect.gen(function*() {
      const started = yield* Deferred.make<void>()
      const child = yield* Effect.forkChild(callback((resume: Resume<number, never>, signal: AbortSignal) => {
        deliver = resume
        signal.addEventListener("abort", () => { events.push("abort") })
        Deferred.doneUnsafe(started, Effect.void)
        return () => { events.push("cancel") }
      }))
      yield* Deferred.await(started)
      yield* Fiber.interrupt(child)
      const exit = child.pollUnsafe()
      expect(Exit.isFailure(exit!)).toBe(true)
      expect(events).toEqual(["abort", "cancel"])
      deliver(42)
      yield* Fiber.interrupt(child)
      expect(child.pollUnsafe()).toBe(exit)
      expect(events).toEqual(["abort", "cancel"])
    }))
  })

  test("a cancellation defect is retained alongside interruption", async () => {
    const defect = new Error("cancel failed")
    await Effect.runPromise(Effect.gen(function*() {
      const child = yield* Effect.forkChild(callback(() => () => { throw defect }), { startImmediately: true })
      yield* Fiber.interrupt(child)
      const exit = child.pollUnsafe()!
      expect(Exit.isFailure(exit)).toBe(true)
      if (Exit.isFailure(exit)) {
        expect(exit.cause.reasons.some(Cause.isInterruptReason)).toBe(true)
        expect(exit.cause.reasons.find(Cause.isDieReason)?.defect).toBe(defect)
      }
    }))
  })
})
