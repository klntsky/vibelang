// Why does an interrupted Effect.fn report several Interrupt reasons?
import { Effect, Fiber } from "effect"

const probe = async (label: string, eff: Effect.Effect<unknown>) => {
  const exit = await Effect.runPromise(Effect.gen(function*() {
    const f = yield* Effect.forkChild(eff)
    yield* Effect.yieldNow
    yield* Fiber.interrupt(f)
    return f.pollUnsafe()!
  }))
  if (exit._tag !== "Failure") return console.log(label, "success?")
  console.log(label, exit.cause.reasons.map((r: any) => `${r._tag}(#${r.fiberId}; annotations=[${[...r.annotations.keys()].map((k: string) => k.replace("effect/Cause/", "")).join(",")}])`).join("  "))
}

await probe("Effect.gen                :", Effect.gen(function*() { yield* Effect.never }))
await probe("Effect.fn('a')            :", Effect.fn("a")(function*() { yield* Effect.never })())
await probe("Effect.fn('a') > fn('b')  :", Effect.fn("a")(function*() { yield* Effect.fn("b")(function*() { yield* Effect.never })() })())
await probe("withSpan('s')(never)      :", Effect.withSpan(Effect.never, "s"))
