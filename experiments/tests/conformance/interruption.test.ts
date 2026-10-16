import { afterEach, expect, test } from "bun:test"
import { Effect, Exit, Fiber } from "effect"
import { abandonNow, handle, hooks, operation, resume } from "../../kernel/kernel.ts"
import { defect, note, run, summary, sync, value } from "./helpers.ts"

afterEach(() => { delete hooks.onCapture; delete hooks.onResume })

test("I01 §1.3/1.4 interrupt at capture retains an owner and releases the segment", () => {
  const Ask = operation<void, void>("I01")
  const seen: string[] = []
  hooks.onCapture = f => (f as any).interruptUnsafe(9001)
  const exit = sync(handle("h", [Ask], Ask().pipe(Effect.ensuring(note(seen, "segment"))), {
    I01: () => note(seen, "clause")
  }).pipe(Effect.ensuring(note(seen, "outside"))))
  expect(Exit.hasInterrupts(exit)).toBe(true)
  expect(seen).toEqual(["segment", "outside"])
})

test("I02 §1.3/1.5 interrupt at resume never executes the next body statement", () => {
  const Ask = operation<void, void>("I02")
  const seen: string[] = []
  hooks.onResume = f => (f as any).interruptUnsafe(9002)
  const body = Effect.andThen(Ask(), note(seen, "continued")).pipe(Effect.ensuring(note(seen, "segment")))
  const exit = sync(handle("h", [Ask], body, { I02: (_, k) => resume(k, undefined) }))
  expect(Exit.hasInterrupts(exit)).toBe(true)
  expect(seen).toEqual(["segment"])
})

test("I03 §1.4/1.7 real interruption during abandonment must not report the synthetic abort", () => {
  const Ask = operation<void, void>("I03")
  const body = Ask().pipe(Effect.ensuring(Effect.withFiber(f => Effect.sync(() => f.interruptUnsafe(9003)))))
  const exit = sync(handle("h", [Ask], body, { I03: () => Effect.succeed("stop") }))
  const ids = Exit.isFailure(exit) ? exit.cause.reasons.filter(r => r._tag === "Interrupt").map(r => r.fiberId) : []
  console.log("I03 observed", JSON.stringify(summary(exit)))
  expect(ids).toEqual([9003])
})

test("I04 §1.3/1.7 abandonment waits for a masked asynchronous finalizer even if interrupted", async () => {
  const Ask = operation<void, void>("I04")
  const seen: string[] = []
  let subject: Fiber.Fiber<any, any>
  let signal!: () => void
  const started = new Promise<void>(r => { signal = r })
  const body = Ask().pipe(Effect.ensuring(Effect.gen(function*() {
    yield* note(seen, "release-start")
    signal()
    yield* Effect.sleep("5 millis")
    yield* note(seen, "release-end")
  })))
  subject = Effect.runFork(handle("h", [Ask], body, { I04: () => Effect.succeed("stop") }))
  await started
  subject.interruptUnsafe(9004)
  const exit = await run(Fiber.await(subject))
  expect(Exit.hasInterrupts(value(exit))).toBe(true)
  expect(seen).toEqual(["release-start", "release-end"])
})

test("I05 §1.3/1.5 a capture-hook deferred interrupt must honor the restored clause mask", () => {
  const Ask = operation<void, void>("I05")
  const seen: string[] = []
  hooks.onCapture = f => (f as any).interruptUnsafe(9005)
  const body = Effect.interruptible(Ask())
  const clause = Effect.catchCause(note(seen, "clause-start"), () => note(seen, "clause-caught"))
  const exit = sync(Effect.uninterruptible(handle("h", [Ask], body, { I05: () => clause })))
  console.log("I05 observed", JSON.stringify({ exit: summary(exit), seen }))
  // This is deliberately a reentrant instrumentation test, using the exposed
  // capture hook. Ordinary other-fiber steps cannot interleave inside capture.
  // Capture restores the prompt's mask; delivery should honor the new state.
  expect(seen).toEqual(["clause-start"])
  expect(Exit.hasInterrupts(exit)).toBe(true)
})

test("I06 §1.7 a finalizer may perform an operation under its own nested handler during abandonment", () => {
  const Ask = operation<void, void>("I06Ask")
  const Fin = operation<void, void>("I06Fin")
  const seen: string[] = []
  const finalizer = handle("fin", [Fin], Effect.andThen(Fin(), note(seen, "fin-end")), {
    I06Fin: (_, k) => Effect.andThen(note(seen, "fin-clause"), resume(k, undefined))
  })
  const exit = sync(handle("h", [Ask], Ask().pipe(Effect.ensuring(finalizer)), { I06Ask: () => Effect.succeed("stop") }))
  expect(value(exit)).toBe("stop")
  expect(seen).toEqual(["fin-clause", "fin-end"])
})

test("I07 §1.7 masked abandon recovery cannot continue past the end of the mask", () => {
  const Ask = operation<void, void>("I07")
  const seen: string[] = []
  const body = Effect.gen(function*() {
    yield* Effect.uninterruptible(Effect.catchCause(Ask(), () => note(seen, "caught-masked")))
    yield* note(seen, "continued")
  }).pipe(Effect.ensuring(note(seen, "released")))
  const exit = sync(handle("h", [Ask], body, { I07: () => Effect.succeed("stop") }))
  expect(value(exit)).toBe("stop")
  expect(seen).toEqual(["caught-masked", "released"])
})

test("I08 §1.7 a wholly masked segment swallowing abandonment yields AbandonedContinuationContinued", () => {
  const Ask = operation<void, void>("I08")
  const body = Effect.catchCause(Ask(), () => Effect.void)
  const exit = sync(Effect.uninterruptible(handle("h", [Ask], body, { I08: () => Effect.succeed("stop") })))
  expect(defect(exit)).toBe("AbandonedContinuationContinued")
})

test("I09 §1.7 pending interruption cannot skip a nested Owner during abandonment", () => {
  const A = operation<void, void>("I09A")
  const B = operation<void, void>("I09B")
  const seen: string[] = []
  const body = handle("B", [B], B().pipe(Effect.ensuring(note(seen, "B-body"))), {
    I09B: () => A().pipe(Effect.ensuring(note(seen, "B-clause")))
  })
  hooks.onCapture = (f, k) => { if (k.op === "I09A") (f as any).interruptUnsafe(9009) }
  const exit = sync(handle("A", [A], body, { I09A: () => Effect.void }))
  expect(Exit.hasInterrupts(exit)).toBe(true)
  expect(seen).toEqual(["B-clause", "B-body"])
})

test("I10 §1.3/1.5 abandonment must preserve both earlier and newly arriving real interrupts", () => {
  const Ask = operation<void, void>("I10")
  const body = Ask().pipe(Effect.ensuring(Effect.withFiber(f => Effect.sync(() => f.interruptUnsafe(9102)))))
  const exit = sync(handle("h", [Ask], body, {
    I10: (_, k) => Effect.uninterruptible(Effect.withFiber(f => {
      f.interruptUnsafe(9101)
      return abandonNow(k)
    }))
  }))
  const ids = Exit.isFailure(exit) ? exit.cause.reasons.filter(r => r._tag === "Interrupt").map(r => r.fiberId) : []
  console.log("I10 observed", JSON.stringify(summary(exit)))
  expect(ids).toEqual([9101, 9102])
})

test("I11 §1.7 a finalizer that performs to an outer handler during abandonment completes; the abort stays inside", () => {
  const Ask = operation<void, void>("I11Ask")
  const Fin = operation<void, void>("I11Fin")
  const seen: string[] = []
  const finalizer = Effect.gen(function*() {
    yield* note(seen, "fin-start")
    yield* Fin()
    yield* note(seen, "fin-end")
  })
  const exit = sync(handle("fin", [Fin], handle("ask", [Ask], Ask().pipe(Effect.ensuring(finalizer)), {
    I11Ask: () => Effect.succeed("stop")
  }), {
    I11Fin: (_, k) => Effect.andThen(note(seen, "fin-clause"), resume(k, undefined))
  }))
  // Fixed: capture carries the in-progress abort with the continuation, so the
  // outer clause runs and the finalizer finishes; no interrupt escapes.
  expect(value(exit)).toBe("stop")
  expect(seen).toEqual(["fin-start", "fin-clause", "fin-end"])
})
