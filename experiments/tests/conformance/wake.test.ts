import { afterEach, expect, test } from "bun:test"
import { Effect, Exit, Scheduler } from "effect"
import { suspend, wakeConfig, type Resumer } from "../../../packages/runtime/src/kernel/kernel.ts"
import { note } from "./helpers.ts"

const originalBudget = wakeConfig.budget
afterEach(() => { wakeConfig.budget = originalBudget })

// Explicit queue: no timing assumptions and no timers left alive by failed tests.
function manualScheduler() {
  const tasks: Array<() => void> = []
  const dispatcher = {
    scheduleTask(task: () => void, _priority: number) { tasks.push(task) },
    flush() { while (tasks.length) tasks.shift()!() }
  }
  const scheduler: Scheduler.Scheduler = { executionMode: "async", shouldYield: () => false, makeDispatcher: () => dispatcher }
  return { scheduler, tasks, flush: () => dispatcher.flush() }
}

test("W01 §1.3 wake delivers inline below budget and preserves waker/receiver order", () => {
  wakeConfig.budget = 1
  let r!: Resumer<number>
  const seen: string[] = []
  const fiber = Effect.runFork(Effect.andThen(suspend<number>(x => { r = x }), note(seen, "receiver")))
  seen.push("before")
  expect(r.resume(42)).toBe(true)
  seen.push("after")
  expect(seen).toEqual(["before", "receiver", "after"])
  expect(Exit.isSuccess(fiber.pollUnsafe()!)).toBe(true)
  expect(r.resume(43)).toBe(false)
})

test("W02 §1.3 budget=2 bounds nesting and defers before the third receiver", () => {
  wakeConfig.budget = 2
  const queue = manualScheduler()
  const seen: string[] = []
  const rs: Resumer<number>[] = []
  const fibers = Array.from({ length: 5 }, (_, i) => Effect.runFork(Effect.gen(function*() {
    const n = yield* suspend<number>(r => { rs[i] = r })
    seen.push(`start:${i}`)
    if (i < 4) rs[i + 1]!.resume(n + 1)
    seen.push(`end:${i}`)
    return n
  }).pipe(Effect.provideService(Scheduler.Scheduler, queue.scheduler))))
  expect(rs[0]!.resume(0)).toBe(true)
  expect(seen).toEqual(["start:0", "start:1", "end:1", "end:0"])
  expect(queue.tasks.length).toBe(1)
  queue.flush()
  expect(seen).toEqual(["start:0", "start:1", "end:1", "end:0", "start:2", "start:3", "end:3", "end:2", "start:4", "end:4"])
  expect(fibers.map(f => f.pollUnsafe()).map(e => Exit.isSuccess(e!) ? e.value : "failure")).toEqual([0, 1, 2, 3, 4])
})

test("W03 §1.3 default wake budget defers at 64; a 130-fiber chain completes", () => {
  wakeConfig.budget = 64
  const queue = manualScheduler()
  const rs: Resumer<number>[] = []
  let reached = 0
  const fibers = Array.from({ length: 130 }, (_, i) => Effect.runFork(Effect.gen(function*() {
    const n = yield* suspend<number>(r => { rs[i] = r })
    reached++
    if (i + 1 < 130) rs[i + 1]!.resume(n + 1)
    return n
  }).pipe(Effect.provideService(Scheduler.Scheduler, queue.scheduler))))
  rs[0]!.resume(0)
  expect(reached).toBe(64)
  expect(queue.tasks.length).toBe(1)
  queue.flush()
  expect(reached).toBe(130)
  expect(fibers.every(f => Exit.isSuccess(f.pollUnsafe()!))).toBe(true)
})

test("W04 §1.4 cancellation before deferred delivery releases once and late resume returns false", () => {
  wakeConfig.budget = 0
  const queue = manualScheduler()
  let r!: Resumer<string>
  const released: string[] = []
  const received: string[] = []
  const fiber = Effect.runFork(Effect.flatMap(suspend<string>(x => { r = x }, { release: x => { released.push(x) } }),
    x => note(received, x)).pipe(Effect.provideService(Scheduler.Scheduler, queue.scheduler)))
  expect(r.resume("socket")).toBe(true)
  fiber.interruptUnsafe(9010)
  queue.flush()
  expect(Exit.hasInterrupts(fiber.pollUnsafe()!)).toBe(true)
  expect(released).toEqual(["socket"])
  expect(received).toEqual([])
  expect(r.resume("late")).toBe(false)
})

test("W05 §1.4 an accepted deferred value must be released if register then defects", () => {
  wakeConfig.budget = 0
  const queue = manualScheduler()
  const released: string[] = []
  let accepted: boolean | undefined
  const fiber = Effect.runFork(suspend<string>(r => {
    accepted = r.resume("socket")
    throw new Error("register failed after accepting socket")
  }, { release: x => { released.push(x) } }).pipe(Effect.provideService(Scheduler.Scheduler, queue.scheduler)))
  queue.flush()
  expect(accepted).toBe(true)
  expect(Exit.isFailure(fiber.pollUnsafe()!)).toBe(true)
  expect(released).toEqual(["socket"])
})
