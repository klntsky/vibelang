import { expect, test } from "bun:test"
import { Effect, Exit, Fiber } from "effect"
import { handle, operation, resume } from "../../kernel/kernel.ts"
import { frameName } from "../../kernel/internals.ts"
import { note, run, value } from "./helpers.ts"

for (const consume of ["resume", "abandon"] as const) {
  test(`X01-${consume} §1.3/1.7 capture moves asyncFinalizer; only abandonment runs cancellation cleanup`, async () => {
    const Ask = operation<void, number>(`X01${consume}`)
    let signal!: (effect: Effect.Effect<number>) => void
    const seen: string[] = []
    let frames: string[] = []
    const waiting = Effect.callback<number>(r => { signal = r; return note(seen, "cancel") })
    const body = Effect.andThen(waiting, Effect.withFiber(f => Effect.as(note(seen, `body:${f.interruptible}`), 42)))
    const fiber = Effect.runFork(handle("h", [Ask], body, {
      [Ask.name]: (_, k) => {
        frames = k.segment!.map(frameName)
        return consume === "resume" ? resume(k, 1) : Effect.succeed(42)
      }
    }))
    signal(Ask())
    const exit = await run(Fiber.await(fiber))
    expect(value(value(exit))).toBe(42)
    expect(frames).toContain("AsyncFinalizer")
    expect(seen).toEqual(consume === "resume" ? ["body:true"] : ["cancel"])
  })
}

test("X02 §1.7 asyncFinalizer failure during abandonment is combined into the boundary Exit", async () => {
  const Ask = operation<void, number>("X02")
  let signal!: (effect: Effect.Effect<number>) => void
  const waiting = Effect.callback<number>(r => { signal = r; return Effect.die("cancel-defect") })
  const fiber = Effect.runFork(handle("h", [Ask], waiting, { X02: () => Effect.succeed(42) }))
  signal(Ask())
  const exit = value(await run(Fiber.await(fiber)))
  expect(Exit.isFailure(exit) && exit.cause.reasons.map(r => r._tag === "Die" ? r.defect : r._tag)).toEqual(["cancel-defect"])
})
