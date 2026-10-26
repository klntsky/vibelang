// Q4: structured concurrency — children, combinators, inheritance.
import { Context, Effect, Exit, Fiber, Scope } from "effect"

const Tag = Context.Reference<string>("q4/Tag", { defaultValue: () => "default" })
const elapsed = (t0: number) => `${Math.round((Date.now() - t0) / 10) * 10}ms`

const main = Effect.gen(function*() {
  // (a) forkChild: parent exit awaits interruption of children (incl. slow finalizers)
  {
    const log: Array<string> = []
    const t0 = Date.now()
    const parent = yield* Effect.forkDetach(Effect.gen(function*() {
      yield* Effect.forkChild(Effect.never.pipe(Effect.ensuring(Effect.andThen(Effect.sleep(50), Effect.sync(() => log.push(`child finalizer done @${elapsed(t0)}`))))))
      yield* Effect.yieldNow
      log.push("parent body done")
      return "parent-value"
    }))
    const exit = yield* Fiber.await(parent)
    log.push(`parent observers notified @${elapsed(t0)} exit=${exit._tag}`)
    console.log("(a) forkChild cleanup:", log.join(" -> "))
  }

  // (b) forkDetach: child survives parent
  {
    let child: Fiber.Fiber<void> | undefined
    const parent = yield* Effect.forkDetach(Effect.gen(function*() {
      child = yield* Effect.forkDetach(Effect.sleep(30))
    }))
    yield* Fiber.await(parent)
    console.log("(b) forkDetach child still running after parent exit:", child!.pollUnsafe() === undefined)
    yield* Fiber.interrupt(child!)
  }

  // (c) forkIn / forkScoped: tied to a scope, not to the parent fiber
  {
    const scope = yield* Scope.make()
    let child: Fiber.Fiber<never> | undefined
    const parent = yield* Effect.forkDetach(Effect.gen(function*() {
      child = yield* Effect.forkIn(Effect.never, scope)
    }))
    yield* Fiber.await(parent)
    const aliveAfterParent = child!.pollUnsafe() === undefined
    yield* Scope.close(scope, Exit.void)
    console.log("(c) forkIn: alive after parent exit:", aliveAfterParent, "| interrupted when scope closed:", Exit.isFailure(child!.pollUnsafe()!))
  }

  // (d) which combinators run work on other fibers?
  const id = Effect.fiberId
  const outer = yield* id
  const allIds = yield* Effect.all([id, id], { concurrency: 2 })
  const allSeq = yield* Effect.all([id, id])
  const raceIds = yield* Effect.race(Effect.andThen(Effect.sleep(5), id), Effect.andThen(Effect.sleep(50), id))
  const timeoutId = yield* Effect.timeout(id, "1 second")
  const zipConc = yield* Effect.zip(id, id, { concurrent: true })
  console.log(`(d) outer fiber #${outer}; all(concurrency 2) -> [${allIds}]; all(sequential) -> [${allSeq}]; race winner -> #${raceIds}; timeout(self) -> #${timeoutId}; zip(concurrent) -> [${zipConc}]`)

  // (e) loser cleanup: does race / all wait for losers' finalizers?
  {
    const log: Array<string> = []
    const t0 = Date.now()
    const slowLoser = Effect.never.pipe(Effect.ensuring(Effect.andThen(Effect.sleep(50), Effect.sync(() => log.push(`loser finalizer done @${elapsed(t0)}`)))))
    yield* Effect.race(Effect.andThen(Effect.sleep(5), Effect.succeed("win")), slowLoser)
    log.push(`race returned @${elapsed(t0)}`)
    console.log("(e1) race:", log.join(" -> "))
  }
  {
    const log: Array<string> = []
    const t0 = Date.now()
    const slowSibling = Effect.never.pipe(Effect.ensuring(Effect.andThen(Effect.sleep(50), Effect.sync(() => log.push(`sibling finalizer done @${elapsed(t0)}`)))))
    const exit = yield* Effect.exit(Effect.all([Effect.andThen(Effect.sleep(5), Effect.fail("boom")), slowSibling], { concurrency: "unbounded" }))
    log.push(`all returned ${exit._tag} @${elapsed(t0)}`)
    console.log("(e2) all(unbounded) first failure:", log.join(" -> "))
  }
  {
    const log: Array<string> = []
    const t0 = Date.now()
    const slow = Effect.never.pipe(Effect.ensuring(Effect.andThen(Effect.sleep(50), Effect.sync(() => log.push(`timed-out effect finalizer done @${elapsed(t0)}`)))))
    const exit = yield* Effect.exit(Effect.timeout(slow, 10))
    log.push(`timeout returned ${exit._tag} @${elapsed(t0)}`)
    console.log("(e3) timeout:", log.join(" -> "))
  }

  // (f) inheritance: context snapshot at fork, interruptibility, nothing merged back on join
  {
    const before = yield* Effect.provideService(
      Effect.gen(function*() {
        const f = yield* Effect.forkChild(Effect.andThen(Effect.sleep(5), Tag))
        return f
      }),
      Tag,
      "parent-local"
    )
    const childSaw = yield* Fiber.join(before)
    console.log("(f1) child forked inside provideService(Tag) saw:", childSaw, "| parent now sees:", yield* Tag)

    const childChanges = yield* Effect.forkChild(
      Effect.scoped(Effect.andThen(Effect.updateServiceScoped(Tag, () => "child-set"), Tag))
    )
    const v = yield* Fiber.join(childChanges)
    console.log("(f2) child updated Tag to:", v, "| parent after join sees:", yield* Tag)

    const interruptibility = yield* Effect.uninterruptible(Effect.gen(function*() {
      const plain = yield* Effect.forkChild(Effect.withFiber((f) => Effect.succeed((f as any).interruptible)))
      const inherit = yield* Effect.forkChild(Effect.withFiber((f) => Effect.succeed((f as any).interruptible)), { uninterruptible: "inherit" })
      return { forkChildDefault: yield* Fiber.join(plain), forkChildInherit: yield* Fiber.join(inherit) }
    }))
    console.log("(f3) child interruptible when forked from uninterruptible parent:", JSON.stringify(interruptibility))

    const ctxIdentity = yield* Effect.gen(function*() {
      const parentCtx = yield* Effect.context()
      const f = yield* Effect.forkChild(Effect.context(), { startImmediately: true })
      const childCtx = yield* Fiber.join(f)
      return parentCtx === childCtx
    })
    console.log("(f4) child's initial Context object === parent's Context object:", ctxIdentity)
  }
})

await Effect.runPromise(main)

// (g) race with a synchronously-completing first effect never forks the second
{
  let secondStarted = false
  await Effect.runPromise(Effect.race(Effect.succeed("sync-win"), Effect.sync(() => {
    secondStarted = true
  })))
  console.log("(g) race(sync winner, other): second effect started =", secondStarted)
}
