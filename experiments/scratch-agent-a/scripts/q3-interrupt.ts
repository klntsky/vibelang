// Q3: interruption delivery points, what Fiber.interrupt waits for, cause shapes.
import { Cause, Effect, Exit, Fiber } from "effect"

const show = (exit: Exit.Exit<unknown, unknown>) =>
  Exit.isSuccess(exit)
    ? `Success(${JSON.stringify(exit.value)})`
    : `Failure[${exit.cause.reasons.map((r) =>
      r._tag === "Fail" ? `Fail(${JSON.stringify(r.error)})` : r._tag === "Die" ? `Die(${String((r as any).defect)})` : `Interrupt(#${(r as any).fiberId})`
    ).join(", ")}]`

const main = Effect.gen(function*() {
  // (a) interrupting a suspended fiber runs its finalizers synchronously inside interruptUnsafe
  {
    const log: Array<string> = []
    const f = yield* Effect.forkChild(Effect.never.pipe(Effect.onInterrupt(() => Effect.sync(() => log.push("onInterrupt ran")))))
    yield* Effect.yieldNow
    yield* Effect.sync(() => {
      log.push("before interruptUnsafe")
      f.interruptUnsafe()
      log.push("after interruptUnsafe returned")
    })
    console.log("(a) suspended target:", log.join(" -> "), "| target exit:", show(f.pollUnsafe()!))
  }

  // (b) a fiber interrupting itself while running: delivered before the next primitive
  {
    const log: Array<string> = []
    const exit = yield* Effect.exit(Effect.gen(function*() {
      const self = yield* Effect.fiber
      yield* Effect.sync(() => {
        self.interruptUnsafe()
        log.push("same step continues after interruptUnsafe")
      })
      log.push("next primitive ran (should not)")
    }).pipe(Effect.forkChild, Effect.flatMap(Fiber.await)))
    console.log("(b) running self-interrupt:", log.join(" -> "), "| exit:", show(exit))
  }

  // (c) Fiber.interrupt waits for the target's uninterruptible region and finalizers
  {
    const log: Array<string> = []
    const f = yield* Effect.forkChild(
      Effect.uninterruptible(Effect.andThen(Effect.sleep(50), Effect.sync(() => log.push("uninterruptible region done")))).pipe(
        Effect.andThen(Effect.sync(() => log.push("code after region (should not run)"))),
        Effect.ensuring(Effect.andThen(Effect.sleep(20), Effect.sync(() => log.push("finalizer done"))))
      )
    )
    yield* Effect.yieldNow
    const t0 = Date.now()
    yield* Fiber.interrupt(f)
    log.push(`Fiber.interrupt returned after ~${Math.round((Date.now() - t0) / 10) * 10}ms`)
    console.log("(c) interrupt waits:", log.join(" -> "))
  }

  // (d) cause shapes
  const interruptAndAwait = <A, E>(eff: Effect.Effect<A, E>) =>
    Effect.gen(function*() {
      const f = yield* Effect.forkChild(eff)
      yield* Effect.yieldNow
      yield* Fiber.interrupt(f)
      return f.pollUnsafe()!
    })
  console.log("(d1) interrupt + failing finalizer:", show(yield* interruptAndAwait(Effect.never.pipe(Effect.ensuring(Effect.fail("fin"))))))
  console.log("(d2) interrupt + dying finalizer:  ", show(yield* interruptAndAwait(Effect.never.pipe(Effect.ensuring(Effect.die("boom"))))))
  console.log("(d3) fail + failing finalizer:     ", show(yield* Effect.exit(Effect.fail("orig").pipe(Effect.ensuring(Effect.fail("fin"))))))
  console.log("(d4) fail + same-value finalizer:  ", show(yield* Effect.exit(Effect.fail("x").pipe(Effect.ensuring(Effect.fail("x"))))))
  console.log("(d5) success + failing finalizer:  ", show(yield* Effect.exit(Effect.succeed(1).pipe(Effect.ensuring(Effect.fail("fin"))))))
  console.log("(d6) Cause.combine(fail a, fail a):", Cause.combine(Cause.fail("a"), Cause.fail("a")).reasons.length, "reason(s)")
  console.log("(d7) Cause.combine(fail a, die b): ", Cause.combine(Cause.fail("a"), Cause.die("b")).reasons.map((r) => r._tag).join(","))
  // (d8) a typed failure raised while an interrupt is pending inside a recovery region
  console.log(
    "(d8) catch skipped by interrupt:   ",
    show(
      yield* interruptAndAwait(
        Effect.uninterruptible(Effect.andThen(Effect.sleep(10), Effect.fail("late"))).pipe(
          Effect.catch(() => Effect.succeed("recovered"))
        )
      )
    )
  )

  // (e) onInterrupt does not fire on plain failure; receives interruptor ids
  {
    const log: Array<string> = []
    yield* Effect.exit(Effect.fail("e").pipe(Effect.onInterrupt(() => Effect.sync(() => log.push("fired on fail")))))
    const parent = yield* Effect.fiberId
    yield* interruptAndAwait(Effect.never.pipe(Effect.onInterrupt((ids) => Effect.sync(() => log.push(`fired, interruptors=[${[...ids]}]`)))))
    console.log(`(e) onInterrupt (parent fiber #${parent}):`, log.join(" | "))
  }

  // (f) Effect.interruptible inside uninterruptible: pending interrupt fires on entry
  {
    const log: Array<string> = []
    const f = yield* Effect.forkChild(
      Effect.uninterruptible(Effect.gen(function*() {
        yield* Effect.sleep(20)
        log.push("uninterruptible work done")
        yield* Effect.interruptible(Effect.sync(() => log.push("interruptible body ran (should not)")))
        log.push("after interruptible (should not)")
      }))
    )
    yield* Effect.yieldNow
    yield* Fiber.interrupt(f)
    console.log("(f) interruptible() inside uninterruptible:", log.join(" -> "), "| exit:", show(f.pollUnsafe()!))
  }

  // (g) Effect.callback canceller: runs on interrupt, and late resumes are ignored
  {
    const log: Array<string> = []
    let resume!: (e: Effect.Effect<string>) => void
    const f = yield* Effect.forkChild(Effect.callback<string>((r) => {
      resume = r
      return Effect.sync(() => log.push("canceller ran"))
    }))
    yield* Effect.yieldNow
    yield* Fiber.interrupt(f)
    resume(Effect.succeed("late"))
    console.log("(g) callback cancel:", log.join(" -> "), "| exit:", show(f.pollUnsafe()!))
  }
})

await Effect.runPromise(main)
