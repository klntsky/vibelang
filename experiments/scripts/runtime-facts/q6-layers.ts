// Q6: layer memoization, resource scope, failures, missing and cyclic deps.
import { Context, Effect, Exit, Layer, ManagedRuntime } from "effect"

const log: Array<string> = []
let builds = 0
class Svc extends Context.Service<Svc, { readonly id: number }>()("q6/Svc") {}
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
const useSvc = Effect.map(Svc, (s) => s.id)

const reset = () => {
  builds = 0
  log.length = 0
}

const main = Effect.gen(function*() {
  // (a) nested Effect.provide of the same layer
  reset()
  const nested = yield* useSvc.pipe(Effect.provide(SvcLive), Effect.provide(SvcLive))
  console.log("(a) nested provide x2 -> id", nested, "| log:", log.join(","))

  // (b) sibling Effect.provide calls with no shared context
  reset()
  const siblings = yield* Effect.all([useSvc.pipe(Effect.provide(SvcLive)), useSvc.pipe(Effect.provide(SvcLive))])
  console.log("(b) sibling provides -> ids", JSON.stringify(siblings), "| log:", log.join(","))

  // (c) provide { local: true } and Layer.fresh inside an outer provide
  reset()
  const local = yield* Effect.all([useSvc, useSvc.pipe(Effect.provide(SvcLive, { local: true })), useSvc.pipe(Effect.provide(Layer.fresh(SvcLive)))]).pipe(Effect.provide(SvcLive))
  console.log("(c) outer + local + fresh -> ids", JSON.stringify(local), "| log:", log.join(","))

  // (d) when are layer resources released with Effect.provide?
  reset()
  yield* Effect.andThen(useSvc.pipe(Effect.provide(SvcLive)), Effect.sync(() => log.push("after provided effect")))
  console.log("(d) resource lifetime:", log.join(" -> "))

  // (e) construction failure: partially-built layers are finalized; error is typed
  reset()
  class Bad extends Context.Service<Bad, {}>()("q6/Bad") {}
  const BadLive = Layer.effect(Bad, Effect.andThen(Svc, Effect.fail("bad-layer-error" as const))).pipe(Layer.provide(SvcLive))
  const failed = yield* Effect.exit(Effect.void.pipe(Effect.provide(BadLive)))
  console.log("(e) failed layer exit:", Exit.isFailure(failed) ? failed.cause.reasons.map((r) => `${r._tag}(${String((r as any).error ?? (r as any).defect)})`).join(",") : "success", "| log:", log.join(","))

  // (f) Layer.merge builds concurrently
  class S1 extends Context.Service<S1, {}>()("q6/S1") {}
  class S2 extends Context.Service<S2, {}>()("q6/S2") {}
  const slow = <T extends Context.Service<any, {}>>(tag: T) => Layer.effect(tag as any, Effect.as(Effect.sleep(100), {}))
  const t0 = Date.now()
  yield* Effect.void.pipe(Effect.provide(Layer.merge(slow(S1), slow(S2)) as Layer.Layer<never>))
  console.log("(f) merge of two 100ms layers took ~", Math.round((Date.now() - t0) / 10) * 10, "ms")

  // (g) missing dependency at runtime (type check bypassed)
  class Needs extends Context.Service<Needs, {}>()("q6/Needs") {}
  const NeedsLive = Layer.effect(Needs, Effect.as(Svc, {}))
  const missing = yield* Effect.exit(Effect.void.pipe(Effect.provide(NeedsLive as unknown as Layer.Layer<Needs>)))
  console.log("(g) missing dependency:", Exit.isFailure(missing) ? missing.cause.reasons.map((r) => `${r._tag}(${String((r as any).defect ?? (r as any).error)})`).join(",") : "success")

  // (h) a cyclic layer (via Layer.suspend) — does it error or hang?
  class C extends Context.Service<C, {}>()("q6/C") {}
  const Cyclic: Layer.Layer<C> = Layer.suspend(() => Layer.effect(C, Effect.as(C, {})).pipe(Layer.provide(Cyclic)))
  const cyc = yield* Effect.exit(Effect.timeout(Effect.void.pipe(Effect.provide(Cyclic)), 300))
  console.log("(h) cyclic layer:", Exit.isFailure(cyc) ? cyc.cause.reasons.map((r) => `${r._tag}(${String((r as any).error?._tag ?? (r as any).error ?? (r as any).defect)})`).join(",") : "success")
})

await Effect.runPromise(main)

// (i) ManagedRuntime: built once, memo shared with Effect.provide inside it
reset()
const rt = ManagedRuntime.make(SvcLive)
const r1 = await rt.runPromise(useSvc)
const r2 = await rt.runPromise(useSvc.pipe(Effect.provide(SvcLive)))
await rt.dispose()
console.log("(i) ManagedRuntime ids", JSON.stringify([r1, r2]), "| log:", log.join(","))
