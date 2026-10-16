// Q5: service key identity, Reference defaults, missing services, fiber cache.
import { Context, Effect, Exit } from "effect"

// two distinct service objects sharing one key string
const A = Context.Service<{ readonly who: string }>("q5/Shared")
const B = Context.Service<{ readonly who: string; readonly extra: number }>("q5/Shared")

let defaultCalls = 0
const Ref = Context.Reference<{ readonly n: number }>("q5/Ref", {
  defaultValue: () => {
    defaultCalls++
    return { n: defaultCalls }
  }
})

const Missing = Context.Service<{ readonly x: number }>("q5/Missing")

const main = Effect.gen(function*() {
  // (a) key collision: providing A satisfies B at runtime
  const bValue = yield* Effect.provideService(B as any, { who: "provided-as-A" } as any)(
    Effect.provideService(A, { who: "A" })(Effect.service(B as any))
  )
  const collide = yield* Effect.provideService(A, { who: "provided via A" })(Effect.service(B as any) as Effect.Effect<any>)
  console.log("(a) A === B:", (A as any) === (B as any), "| A.key === B.key:", A.key === B.key, "| yield B after providing only A ->", JSON.stringify(collide), "| inner-most wins ->", JSON.stringify(bValue))

  // (b) Reference default computed once and shared across fibers / runs
  const v1 = yield* Ref
  const v2 = yield* Effect.forkChild(Ref).pipe(Effect.flatMap((f) => Effect.flatMap(Effect.yieldNow, () => (f as any).pollUnsafe() ? Effect.succeed((f as any).pollUnsafe().value) : Effect.never)))
  const v3 = Effect.runSync(Ref)
  console.log("(b) default values:", JSON.stringify([v1, v2, v3]), "| defaultValue() calls:", defaultCalls, "| same object:", v1 === v3)

  // (c) a missing service at runtime (type check bypassed)
  const exit = yield* Effect.exit(Effect.service(Missing) as unknown as Effect.Effect<unknown>)
  console.log("(c) missing service:", Exit.isFailure(exit) ? exit.cause.reasons.map((r) => `${r._tag}(${String((r as any).defect ?? (r as any).error).split("\n")[0]})`).join(",") : "success?!")

  // (d) Context data structure: add is O(1) overlay up to depth 8, merge copies
  let ctx = Context.empty() as Context.Context<any>
  const keys = Array.from({ length: 12 }, (_, i) => Context.Service<number>(`q5/k${i}`))
  const depths: Array<number> = []
  for (const k of keys) {
    ctx = Context.add(ctx, k, 1)
    depths.push((ctx as any).depth)
  }
  console.log("(d) overlay depth after each add:", depths.join(","), "| base size now:", (ctx as any).base.size)

  // (e) fiber cache reuse: adding a non-cached key keeps the cacheRoot
  const sameCache = yield* Effect.withFiber((fiber) => {
    const before = fiber.context
    const afterPlain = Context.add(before, keys[0], 2)
    const afterCached = Context.add(before, (Context as any).Reference("effect/Scheduler/MaxOpsBeforeYield", { defaultValue: () => 1 }), 10)
    return Effect.succeed({
      plainAddKeepsCacheRoot: (afterPlain as any).cacheRoot === (before as any).cacheRoot,
      fiberCachedKeyAddResetsCacheRoot: (afterCached as any).cacheRoot !== (before as any).cacheRoot
    })
  })
  console.log("(e) fiber cache:", JSON.stringify(sameCache))
})

await Effect.runPromise(main)
