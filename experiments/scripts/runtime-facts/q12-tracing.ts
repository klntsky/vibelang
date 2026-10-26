// Q12: where the current span and stack frame live, and what Effect.fn adds per call.
import { Context, Effect, References, Tracer } from "effect"

const inspect = Effect.withFiber((fiber) => {
  const ctx = fiber.context
  const keys = [...ctx.mapUnsafe.keys()]
  const frame = Context.get(ctx, References.CurrentStackFrame)
  const frames: Array<string> = []
  for (let f = frame; f; f = f.parent) frames.push(f.name)
  const span = Context.getOrUndefined(ctx, Tracer.ParentSpan)
  return Effect.succeed({
    contextKeys: keys,
    stackFrames: frames,
    parentSpan: span ? `${span._tag}:${(span as any).name}` : "none"
  })
})

const traced = Effect.fn("outerFn")(function*() {
  const inner = yield* Effect.fn("innerFn")(function*() {
    return yield* inspect
  })()
  return inner
})
const untraced = Effect.fnUntraced(function*() {
  return yield* inspect
})
const anon = Effect.fn(function*() {
  return yield* inspect
})

console.log("Effect.fn nested:", JSON.stringify(await Effect.runPromise(traced())))
console.log("Effect.fnUntraced:", JSON.stringify(await Effect.runPromise(untraced())))
console.log("Effect.fn(no name):", JSON.stringify(await Effect.runPromise(anon())))
console.log("withSpan('s'):", JSON.stringify(await Effect.runPromise(Effect.withSpan(inspect, "s"))))

// rough per-call cost of the wrappers (same body)
const body = function*() {
  return 1
}
const variants = {
  "Effect.gen": () => Effect.gen(body),
  "Effect.fnUntraced": Effect.fnUntraced(body),
  "Effect.fnUntracedEager": Effect.fnUntracedEager(body),
  "Effect.fn(name)": Effect.fn("bench")(body)
}
const N = 100_000
for (const [name, mk] of Object.entries(variants)) {
  const prog = Effect.forEach(Array.from({ length: N }), () => mk(), { discard: true })
  await Effect.runPromise(prog) // warm
  const t0 = performance.now()
  await Effect.runPromise(prog)
  console.log(`${name.padEnd(24)} ${((performance.now() - t0) * 1000 / N).toFixed(2)} us/call (N=${N})`)
}
