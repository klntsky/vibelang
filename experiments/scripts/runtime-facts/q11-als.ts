// Q11: do AsyncLocalStorage values survive fiber suspension and resumption?
import { AsyncLocalStorage } from "node:async_hooks"
import { Deferred, Effect, Fiber } from "effect"

const als = new AsyncLocalStorage<string>()
const read = (label: string, out: Array<string>) => Effect.sync(() => out.push(`${label}=${als.getStore() ?? "undefined"}`))

const scenario = Effect.gen(function*() {
  const out: Array<string> = []
  yield* read("start", out)
  yield* Effect.sleep(1)
  yield* read("after sleep(setTimeout)", out)
  yield* Effect.promise(() => Promise.resolve(1))
  yield* read("after promise", out)
  yield* Effect.yieldNow
  yield* read("after yieldNow(setImmediate)", out)
  // resumed synchronously by a fiber that runs under a DIFFERENT store
  const d = yield* Deferred.make<void>()
  als.run("other-store", () => {
    Effect.runFork(Effect.andThen(Effect.sleep(5), Deferred.succeed(d, undefined)))
  })
  yield* Deferred.await(d)
  yield* read("after resume by fiber in other-store", out)
  // resumed by a raw callback invoked from another store's timer
  yield* Effect.callback<void>((resume) => {
    als.run("timer-store", () => setTimeout(() => resume(Effect.void), 1))
  })
  yield* read("after callback resumed from timer-store", out)
  // forked child (scheduled start) and immediate child
  const child = yield* Effect.forkChild(Effect.andThen(Effect.sleep(1), Effect.sync(() => als.getStore())))
  const childImmediate = yield* Effect.forkChild(Effect.sync(() => als.getStore()), { startImmediately: true })
  out.push(`forkChild child=${yield* Fiber.join(child)}`, `startImmediately child=${yield* Fiber.join(childImmediate)}`)
  // effect run while Effect itself changes the store via als.run in a sync step
  return out
})

const result = await als.run("request-A", () => Effect.runPromise(scenario))
console.log(result.join("\n"))
