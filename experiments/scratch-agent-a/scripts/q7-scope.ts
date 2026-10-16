// Q7: finalizer ordering, strategies, closed scopes, failure combination, forks.
import { Effect, Exit, Scope } from "effect"

const show = (exit: Exit.Exit<unknown, unknown>) =>
  Exit.isSuccess(exit) ? "Success" : `Failure[${exit.cause.reasons.map((r) => r._tag === "Fail" ? `Fail(${r.error})` : r._tag === "Die" ? `Die(${(r as any).defect})` : "Interrupt").join(", ")}]`

const main = Effect.gen(function*() {
  // (a) sequential: reverse registration order
  {
    const log: Array<string> = []
    const scope = yield* Scope.make()
    for (const n of [1, 2, 3]) yield* Scope.addFinalizer(scope, Effect.sync(() => log.push(`f${n}`)))
    yield* Scope.close(scope, Exit.void)
    console.log("(a) sequential close order:", log.join(","))
  }
  // (b) parallel: all start before any finishes
  {
    const log: Array<string> = []
    const scope = yield* Scope.make("parallel")
    for (const n of [1, 2, 3]) {
      yield* Scope.addFinalizer(scope, Effect.gen(function*() {
        log.push(`start f${n}`)
        yield* Effect.sleep(10 * n)
        log.push(`end f${n}`)
      }))
    }
    const t0 = Date.now()
    yield* Scope.close(scope, Exit.void)
    console.log("(b) parallel close:", log.join(","), `(~${Math.round((Date.now() - t0) / 10) * 10}ms)`)
  }
  // (c) adding a finalizer to a closed scope runs it immediately with the scope's exit
  {
    const log: Array<string> = []
    const scope = yield* Scope.make()
    yield* Scope.close(scope, Exit.fail("closed-with"))
    yield* Scope.addFinalizerExit(scope, (exit) => Effect.sync(() => log.push(`ran immediately with ${show(exit)}`)))
    log.push("after addFinalizerExit")
    console.log("(c) closed scope:", log.join(" -> "))
  }
  // (d) finalizer failures: all finalizers run; reasons combined
  {
    const log: Array<string> = []
    const scope = yield* Scope.make()
    yield* Scope.addFinalizer(scope, Effect.sync(() => log.push("f1 ran")))
    yield* Scope.addFinalizer(scope, Effect.fail("f2-failed"))
    yield* Scope.addFinalizer(scope, Effect.die("f3-died"))
    yield* Scope.addFinalizer(scope, Effect.fail("f2-failed"))
    const exit = yield* Effect.exit(Scope.close(scope, Exit.void))
    console.log("(d) close with failing finalizers:", show(exit), "|", log.join(","))
  }
  // (e) Effect.scoped: body failure + finalizer failure
  {
    const exit = yield* Effect.exit(Effect.scoped(Effect.gen(function*() {
      yield* Effect.addFinalizer(() => Effect.fail("fin-failed"))
      return yield* Effect.fail("body-failed")
    })))
    console.log("(e) scoped body fail + finalizer fail:", show(exit))
  }
  // (f) Scope.fork: child closed when parent closes, in reverse position
  {
    const log: Array<string> = []
    const parent = yield* Scope.make()
    yield* Scope.addFinalizer(parent, Effect.sync(() => log.push("parent f1")))
    const child = yield* Scope.fork(parent)
    yield* Scope.addFinalizer(child, Effect.sync(() => log.push("child f1")))
    yield* Scope.addFinalizer(parent, Effect.sync(() => log.push("parent f2")))
    yield* Scope.close(parent, Exit.void)
    const child2 = yield* Scope.fork(parent)
    console.log("(f) fork:", log.join(","), "| fork of closed parent is born:", child2.state._tag)
  }
  // (g) closing a child early removes it from the parent
  {
    const log: Array<string> = []
    const parent = yield* Scope.make()
    const child = yield* Scope.fork(parent)
    yield* Scope.addFinalizer(child, Effect.sync(() => log.push("child f")))
    yield* Scope.close(child, Exit.void)
    yield* Scope.close(parent, Exit.void)
    console.log("(g) child closed first, then parent:", log.join(","))
  }
  // (h) finalizer run in uninterruptible mode?
  {
    const scope = yield* Scope.make()
    let interruptible: boolean | undefined
    yield* Scope.addFinalizer(scope, Effect.withFiber((f) => Effect.sync(() => {
      interruptible = (f as any).interruptible
    })))
    yield* Scope.close(scope, Exit.void)
    console.log("(h) fiber interruptible while finalizer runs:", interruptible)
  }
})

await Effect.runPromise(main)
