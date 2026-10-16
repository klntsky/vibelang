// Q6 follow-up: do two concurrent, overlapping sibling Effect.provide calls share a MemoMap?
import { Context, Effect, Layer } from "effect"

const log: Array<string> = []
let builds = 0
class Svc extends Context.Service<Svc, { readonly id: number }>()("q6b/Svc") {}
const SvcLive = Layer.effect(
  Svc,
  Effect.acquireRelease(
    Effect.sync(() => {
      builds++
      log.push(`build#${builds}`)
      return Svc.of({ id: builds })
    }),
    (s) => Effect.sync(() => log.push(`release#${s.id}`))
  )
)
const hold = Effect.flatMap(Svc, (s) => Effect.as(Effect.sleep(30), s.id))

const overlapping = await Effect.runPromise(
  Effect.all([hold.pipe(Effect.provide(SvcLive)), hold.pipe(Effect.provide(SvcLive))], { concurrency: 2 })
)
console.log("concurrent sibling provides (root context) -> ids", JSON.stringify(overlapping), "| log:", log.join(","))

log.length = 0
builds = 0
// same, but under an outer provide that put CurrentMemoMap into the context
const underOuter = await Effect.runPromise(
  Effect.all([hold.pipe(Effect.provide(SvcLive)), hold.pipe(Effect.provide(SvcLive))], { concurrency: 2 }).pipe(
    Effect.provide(Layer.empty)
  )
)
console.log("concurrent sibling provides under outer provide(Layer.empty) -> ids", JSON.stringify(underOuter), "| log:", log.join(","))
