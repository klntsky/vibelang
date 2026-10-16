// G4: handoff for producers that cannot act on a lost race.
import { expect, test } from "bun:test"
import { Effect, Exit, Fiber } from "effect"
import { handoff, HandoffProtocolViolation, wakeConfig } from "../../packages/runtime/src/kernel/kernel.ts"

const recorder = () => {
  const events: Array<string> = []
  return { events, note: (line: string) => Effect.sync(() => void events.push(line)) }
}

test("H1 delivery first: the waiter receives the value once and nothing is released", async () => {
  const { events, note } = recorder()
  const waiter = handoff<string>(
    (deliver) => {
      setTimeout(() => deliver("conn-1"), 1)
      return note("cancelAndJoin (must not run)")
    },
    (value) => note(`released ${value} (must not run)`)
  )
  const exit = await Effect.runPromiseExit(waiter)
  expect(Exit.isSuccess(exit) && exit.value).toBe("conn-1")
  expect(events).toEqual([])
})

test("H2 cancellation first: a value delivered while the producer shuts down is released before cancellation completes", async () => {
  const { events, note } = recorder()
  const exit = await Effect.runPromiseExit(Effect.gen(function*() {
    let deliver: ((value: string) => void) | undefined
    const fiber = yield* Effect.forkChild(handoff<string>(
      (d) => {
        deliver = d
        // Asynchronous shutdown: the producer delivers once more before it is quiet.
        return Effect.andThen(note("shutdown started"), Effect.andThen(
          Effect.sync(() => deliver!("conn-late")),
          note("producer quiet")
        ))
      },
      (value) => note(`released ${value}`)
    ))
    yield* Effect.sleep("1 millis")
    yield* Fiber.interrupt(fiber)
    yield* note("interrupt returned")
    return yield* Fiber.await(fiber)
  }))
  expect(Exit.isSuccess(exit) && Exit.hasInterrupts(exit.value)).toBe(true)
  expect(events).toEqual(["shutdown started", "producer quiet", "released conn-late", "interrupt returned"])
})

test("H3 a deferred delivery that loses to cancellation is released", async () => {
  const { events, note } = recorder()
  const previous = wakeConfig.budget
  wakeConfig.budget = 0
  try {
    const exit = await Effect.runPromiseExit(Effect.gen(function*() {
      let deliver: ((value: string) => void) | undefined
      const fiber = yield* Effect.forkChild(handoff<string>(
        (d) => {
          deliver = d
          return Effect.void
        },
        (value) => note(`released ${value}`)
      ))
      yield* Effect.sleep("1 millis")
      deliver!("conn-2")                 // accepted, deferred by the wake budget
      yield* Fiber.interrupt(fiber)      // cancellation wins before the task runs
      yield* Effect.sleep("5 millis")
      return yield* Fiber.await(fiber)
    }))
    expect(Exit.isSuccess(exit) && Exit.hasInterrupts(exit.value)).toBe(true)
    expect(events).toEqual(["released conn-2"])
  } finally {
    wakeConfig.budget = previous
  }
})

test("H4 a delivery after quiescence is a protocol violation raised to the producer", async () => {
  let deliver: ((value: string) => void) | undefined
  const exit = await Effect.runPromiseExit(Effect.gen(function*() {
    const fiber = yield* Effect.forkChild(handoff<string>((d) => {
      deliver = d
      return Effect.void
    }, () => Effect.void))
    yield* Effect.sleep("1 millis")
    yield* Fiber.interrupt(fiber)
  }))
  expect(Exit.isSuccess(exit)).toBe(true)
  expect(() => deliver!("too-late")).toThrow(HandoffProtocolViolation)
})
