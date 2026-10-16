// Edge cases of the continuation kernel: state at the resume site, pending
// interruption, masking, cross-fiber abandonment, and opaque foreign frames.
import { describe, expect, test } from "bun:test"
import { Cause, Context, Effect, Exit, Fiber, Scope } from "effect"
import { abandon, type Continuation, handle, operation, resume, Suspended, transfer } from "../kernel/kernel.ts"

const run = <A, E>(effect: Effect.Effect<A, E, never>) => Effect.runPromiseExit(effect)

const defectTag = (exit: Exit.Exit<unknown, unknown>) => {
  if (Exit.isSuccess(exit)) return undefined
  return (exit.cause.reasons.find(Cause.isDieReason)?.defect as any)?._tag
}

class Logger extends Context.Service<Logger, string>()("edges/Logger") {}

describe("state at the resume site", () => {
  test("T13 the body keeps its snapshot; the reinstalled prompt restores the resume site's Context", async () => {
    const Ask = operation<string, string>("Ask")
    const body = Effect.gen(function*() {
      const first = yield* Ask("first")
      const seenAfterFirst = yield* Logger
      const second = yield* Ask("second")
      return [first, seenAfterFirst, second]
    }).pipe(Effect.provideService(Logger, "body"))
    const program = handle("h", [Ask], body, {
      Ask: (label: string, k: Continuation) =>
        Effect.gen(function*() {
          const clauseSees = yield* Logger
          if (label === "first") {
            // Resume from inside a different Context; after the handled
            // computation returns, this clause must see "resume-site" again.
            const rest = yield* resume(k, `${label}@${clauseSees}`).pipe(
              Effect.andThen((result) => Effect.map(Effect.service(Logger), (after) => [...(result as Array<string>), `after-resume:${after}`])),
              Effect.provideService(Logger, "resume-site")
            )
            return rest
          }
          return yield* resume(k, `${label}@${clauseSees}`)
        })
    }).pipe(Effect.provideService(Logger, "handle-site"))
    const exit = await run(program)
    expect(Exit.isSuccess(exit) && exit.value).toEqual([
      "first@handle-site",
      "body",
      "second@resume-site",
      "after-resume:resume-site"
    ])
  })

  test("T12 an interruption that arrives during an uninterruptible clause is delivered when resuming into an interruptible body", async () => {
    const Ask = operation<void, string>("Ask")
    const events: Array<string> = []
    const note = (line: string) => Effect.sync(() => void events.push(line))
    const body = Effect.ensuring(Effect.andThen(Ask(undefined), note("body continued (must not)")), note("body finalizer"))
    const program = handle("h", [Ask], body, {
      Ask: (_: void, k: Continuation) =>
        Effect.uninterruptible(Effect.gen(function*() {
          yield* note("clause sleeping uninterruptibly")
          yield* Effect.sleep("10 millis")
          yield* note("clause resumes")
          return yield* resume(k, "value")
        }))
    })
    const exit = await run(Effect.gen(function*() {
      const fiber = yield* Effect.forkChild(program)
      yield* Effect.sleep("2 millis")
      yield* Fiber.interrupt(fiber)
      return yield* Fiber.await(fiber)
    }))
    expect(Exit.isSuccess(exit) && Exit.hasInterrupts(exit.value)).toBe(true)
    expect(events).toEqual(["clause sleeping uninterruptibly", "clause resumes", "body finalizer"])
  })
})

describe("abandonment under masking", () => {
  test("a recovery handler inside a masked region may observe abandonment, but the abort is re-raised when the mask ends", async () => {
    const Ask = operation<void, string>("Ask")
    const events: Array<string> = []
    const note = (line: string) => Effect.sync(() => void events.push(line))
    const body = Effect.gen(function*() {
      const observed = yield* Effect.uninterruptible(
        Effect.catchCause(Ask(undefined), (cause) => Effect.as(note(`observed interrupt=${Cause.hasInterrupts(cause)}`), "recovered"))
      )
      yield* note(`continued with ${observed} (must not)`)
      return observed
    }).pipe(Effect.ensuring(note("body finalizer")))
    const exit = await run(handle("h", [Ask], body, { Ask: () => Effect.succeed("clause result") }))
    expect(Exit.isSuccess(exit) && exit.value).toBe("clause result")
    expect(events).toEqual(["observed interrupt=true", "body finalizer"])
  })

  test("a segment captured entirely inside a mask that swallows the abort is a defect", async () => {
    const Ask = operation<void, string>("Ask")
    const body = Effect.catchCause(Ask(undefined), () => Effect.succeed("swallowed"))
    const exit = await run(Effect.uninterruptible(handle("h", [Ask], body, { Ask: () => Effect.succeed("clause result") })))
    expect(defectTag(exit)).toBe("AbandonedContinuationContinued")
  })
})

describe("ownership across fibers", () => {
  test("T14 a Scope closed on another fiber cannot abandon; the capturing fiber still can", async () => {
    const Ask = operation<void, string>("Ask")
    const events: Array<string> = []
    const note = (line: string) => Effect.sync(() => void events.push(line))
    const exit = await run(Effect.gen(function*() {
      const scope = yield* Scope.make()
      let parked: Continuation | undefined
      const body = Effect.gen(function*() {
        yield* Effect.addFinalizer(() => Effect.flatMap(Effect.service(Logger), (l) => note(`segment finalizer sees Logger=${l}`)))
        return yield* Ask(undefined)
      }).pipe(Effect.scoped, Effect.provideService(Logger, "segment"))
      yield* handle("h", [Ask], body, {
        Ask: (_: void, k: Continuation) => {
          parked = k
          return Effect.as(transfer(k), "parked")
        }
      }).pipe(Effect.provideService(Scope.Scope, scope))
      const closer = yield* Effect.forkChild(Scope.close(scope, Exit.void))
      const closerExit = yield* Fiber.await(closer)
      yield* note(`closer: ${defectTag(closerExit) ?? "ok"}`)
      // The owning fiber abandons it itself: finalizers run under k's Context.
      yield* abandon(parked!)
      const after = yield* Effect.serviceOption(Logger)
      yield* note(`capturing fiber afterwards sees Logger=${after._tag}`)
    }))
    expect(Exit.isSuccess(exit)).toBe(true)
    expect(events).toEqual([
      "closer: ContinuationWrongFiber",
      "segment finalizer sees Logger=segment",
      "capturing fiber afterwards sees Logger=None"
    ])
  })
})

describe("opaque foreign frames", () => {
  test("an operation performed under a synchronous foreign callback cannot reach the handler", async () => {
    const Ask = operation<number, number>("Ask")
    const body = Effect.sync(() => [1, 2].map((i) => Effect.runSync(Ask(i))))
    const exit = await run(handle("h", [Ask], body, { Ask: (i: number, k: Continuation) => resume(k, i) }))
    // runSync starts a new root fiber with an empty Context: the evidence is
    // gone, so the operation is unhandled there. runSync rethrows that defect
    // inside Effect.sync, which reports it as a defect of the outer fiber.
    expect(defectTag(exit)).toBe("UnhandledOperation")
  })
})

describe("hazard: Effect frames close over the fiber that pushed them", () => {
  test("T14b abandonment on another fiber must not rewrite the capturing fiber's live Context", async () => {
    const Ask = operation<void, string>("Ask")
    const exit = await run(Effect.gen(function*() {
      const scope = yield* Scope.make()
      const body = Effect.as(Ask(undefined), "x").pipe(Effect.provideService(Logger, "segment"))
      yield* handle("h", [Ask], body, {
        Ask: (_: void, k: Continuation) => Effect.as(transfer(k), "parked")
      }).pipe(Effect.provideService(Scope.Scope, scope))
      // The capturing fiber moves on under a different provider, and another
      // fiber closes the Scope that owns the parked continuation.
      return yield* Effect.gen(function*() {
        const closer = yield* Effect.forkChild(Scope.close(scope, Exit.void))
        yield* Fiber.await(closer) // the closer gets ContinuationWrongFiber (T14)
        return yield* Effect.serviceOption(Logger)
      }).pipe(Effect.provideService(Logger, "main-later"))
    }))
    const seen = Exit.isSuccess(exit) ? exit.value : undefined
    console.log(`\n--- T14b capturing fiber sees Logger = ${JSON.stringify(seen)}`)
    expect(seen && (seen as any)._tag === "Some" && (seen as any).value).toBe("main-later")
  })
})

describe("finalizer failure during abandonment", () => {
  test("a finalizer defect while abandoning turns the handle's success into that defect", async () => {
    const Ask = operation<void, string>("Ask")
    const body = Effect.scoped(Effect.gen(function*() {
      yield* Effect.addFinalizer(() => Effect.die(new Error("release failed")))
      return yield* Ask(undefined)
    }))
    const exit = await run(handle("h", [Ask], body, { Ask: () => Effect.succeed("clause result") }))
    expect(Exit.isFailure(exit)).toBe(true)
    const die = Exit.isFailure(exit) ? exit.cause.reasons.find(Cause.isDieReason) : undefined
    expect((die?.defect as Error)?.message).toBe("release failed")
    expect(Exit.isFailure(exit) && exit.cause.reasons.some(Cause.isInterruptReason)).toBe(false)
  })
})

describe("snapshot evidence versus delimited dynamic binding", () => {
  test("T15 a continuation resumed outside its outer handler does not rebind to the handler at the resume site; it reports HandlerPromptExited", async () => {
    const Ask = operation<void, string>("Ask")
    const Yield = operation<void, void>("Yield")
    const exit = await run(Effect.gen(function*() {
      const scope = yield* Scope.make()
      // Capture inside outer handler H1 for Ask, transfer out of H1's region.
      const parked: Suspended = yield* handle("H1", [Ask], handle("G", [Yield],
        Effect.andThen(Yield(undefined), Ask(undefined)),
        { Yield: (_: void, k: Continuation) => transfer(k) }
      ), { Ask: (_: void, k: Continuation) => resume(k, "from H1") }).pipe(Effect.provideService(Scope.Scope, scope))
      // Resume under a different handler H2 for Ask. Under DDB the body's Ask
      // would reach H2; under snapshot evidence it still names H1, which is gone.
      return yield* handle("H2", [Ask], resume(parked, undefined), {
        Ask: (_: void, k: Continuation) => resume(k, "from H2")
      })
    }))
    expect(defectTag(exit)).toBe("HandlerPromptExited")
  })
})
