import { expect, test } from "bun:test"
import { Context, Effect, Exit, References, Scheduler, Tracer } from "effect"
import { handle, operation, resume } from "../../../packages/runtime/src/kernel/kernel.ts"
import { frameName } from "../../../packages/runtime/src/kernel/internals.ts"
import { note, sync, value } from "./helpers.ts"

for (const consume of ["resume", "abandon"] as const) {
  for (const shape of ["fn", "withSpan"] as const) {
    test(`T01-${shape}-${consume} §1.5/1.7 tracing frames park spans and end each once with the segment Exit`, () => {
      const Ask = operation<void, number>(`T01${shape}${consume}`)
      const spans: any[] = []
      const seen: string[] = []
      let frames: string[] = []
      const tracer = Tracer.make({
        span(options) {
          const span = Tracer.nativeTracer.span(options)
          const end = span.end
          ;(span as any).end = function(...args: any[]) {
            seen.push(`end:${span.name}`)
            return (end as any).apply(span, args)
          }
          spans.push(span)
          return span
        }
      })
      const middle = Effect.gen(function*() {
        const n = yield* Ask()
        yield* Effect.withFiber(f => note(seen, `body:${(f as any).cache.span?.name}:${(f as any).cache.stackFrame?.name}`))
        return n + 1
      })
      const body = shape === "fn" ? Effect.fn("captured")(function*() { return yield* middle })()
        : middle.pipe(Effect.withSpan("captured"))
      const exit = sync(handle("h", [Ask], body, {
        [Ask.name]: (_, k) => Effect.gen(function*() {
          frames = k.segment!.map(frameName)
          yield* Effect.withFiber(f => note(seen, `clause:${(f as any).cache.span?.name}`))
          expect(spans.find(s => s.name === "captured").status._tag).toBe("Started")
          return yield* (consume === "resume" ? resume(k, 41) : Effect.succeed(42))
        })
      }).pipe(Effect.withSpan("outer"), Effect.withTracer(tracer)))
      expect(value(exit)).toBe(42)
      expect(frames).toContain("OnExit")
      expect(frames).toContain("Iterator")
      expect(seen).toEqual(consume === "resume"
        ? ["clause:outer", "body:captured:captured", "end:captured", "end:outer"]
        : ["clause:outer", "end:captured", "end:outer"])
      const captured = spans.find(s => s.name === "captured")
      expect(captured.status._tag).toBe("Ended")
      expect(captured.status.exit._tag).toBe(consume === "resume" ? "Success" : "Failure")
      if (consume === "abandon") expect(Exit.hasInterrupts(captured.status.exit)).toBe(true)
    })
  }
}

test("T02 §1.5 Context.Reference and fiber caches refresh at capture, resume, restore closures and prompt exit", () => {
  const Ask = operation<void, void>("T02")
  const Ref = Context.Reference("conformance/ref", { defaultValue: () => "default" })
  const seen: any[] = []
  const log = (phase: string) => Effect.withFiber(f => note(seen,
    [phase, f.getRef(Ref), f.getRef(Scheduler.MaxOpsBeforeYield), (f as any).cache.maxOpsBeforeYield,
      f.getRef(References.CurrentLogLevel), (f as any).cache.logLevel]))
  const body = Effect.gen(function*() {
    yield* log("before")
    yield* Ask()
    yield* log("after")
  }).pipe(Effect.provideService(Ref, "body"), Effect.provideService(Scheduler.MaxOpsBeforeYield, 8192),
    Effect.provideService(References.CurrentLogLevel, "Debug"))
  const exit = sync(handle("h", [Ask], body, {
    T02: (_, k) => Effect.gen(function*() {
      yield* log("clause")
      yield* Effect.andThen(resume(k, undefined), log("resumer")).pipe(
        Effect.provideService(Ref, "resumer"), Effect.provideService(Scheduler.MaxOpsBeforeYield, 4096),
        Effect.provideService(References.CurrentLogLevel, "Warning"))
      yield* log("clause-after")
    })
  }).pipe(Effect.provideService(Ref, "outer"), Effect.provideService(Scheduler.MaxOpsBeforeYield, 16384),
    Effect.provideService(References.CurrentLogLevel, "Info")))
  expect(Exit.isSuccess(exit)).toBe(true)
  expect(seen).toEqual([
    ["before", "body", 8192, 8192, "Debug", "Debug"],
    ["clause", "outer", 16384, 16384, "Info", "Info"],
    ["after", "body", 8192, 8192, "Debug", "Debug"],
    ["resumer", "resumer", 4096, 4096, "Warning", "Warning"],
    ["clause-after", "outer", 16384, 16384, "Info", "Info"]
  ])
})
