// G6: tracing spans and AsyncLocalStorage across a prompt capture.
import { expect, test } from "bun:test"
import { AsyncLocalStorage } from "node:async_hooks"
import { Effect, Exit, Tracer } from "effect"
import { type Continuation, handle, operation, resume } from "../../packages/runtime/src/kernel/kernel.ts"

test("G6a spans: the clause sees the outer span, the body sees its own after resume, each span ends once", async () => {
  const Ask = operation<void, string>("Ask")
  const seen: Array<string> = []
  const ended: Array<string> = []
  const recording = Tracer.make({
    span(options) {
      const span = Tracer.nativeTracer.span(options)
      const end = span.end.bind(span)
      Object.defineProperty(span, "end", {
        value: (endTime: bigint, exit: Exit.Exit<unknown, unknown>) => {
          ended.push(options.name)
          end(endTime, exit)
        }
      })
      return span
    }
  })
  const spanName = Effect.map(Effect.currentSpan, (span) => span.name)
  const body = Effect.gen(function*() {
    const value = yield* Ask(undefined)
    seen.push(`body after resume: ${yield* spanName}`)
    return value
  }).pipe(Effect.withSpan("inner"))
  const program = handle("h", [Ask], body, {
    Ask: (_: void, k: Continuation) =>
      Effect.gen(function*() {
        seen.push(`clause: ${yield* spanName}`)
        yield* Effect.sleep("1 millis") // a root wait while the continuation is held
        return yield* resume(k, "x")
      })
  }).pipe(Effect.withSpan("outer"), Effect.withTracer(recording))
  const exit = await Effect.runPromiseExit(program)
  expect(Exit.isSuccess(exit) && exit.value).toBe("x")
  expect(seen).toEqual(["clause: outer", "body after resume: inner"])
  expect(ended).toEqual(["inner", "outer"])
})

test("G6b AsyncLocalStorage is fiber-wide: a prompt capture does not split it", async () => {
  const Ask = operation<void, string>("Ask")
  const als = new AsyncLocalStorage<string>()
  const seen: Array<string> = []
  const body = Effect.gen(function*() {
    yield* Effect.sync(() => als.enterWith("body"))
    yield* Ask(undefined)
    seen.push(`body after resume: ${als.getStore()}`)
  })
  const program = handle("h", [Ask], body, {
    Ask: (_: void, k: Continuation) =>
      Effect.gen(function*() {
        seen.push(`clause: ${als.getStore()}`)
        yield* Effect.sleep("1 millis")
        seen.push(`clause after a root wait: ${als.getStore()}`)
        return yield* resume(k, undefined)
      })
  })
  const exit = await als.run("outer", () => Effect.runPromiseExit(program))
  console.log(["", "--- G6b AsyncLocalStorage across a prompt capture", ...seen].join("\n"))
  expect(Exit.isSuccess(exit)).toBe(true)
  // The store set inside the handled region is visible in the clause: the
  // kernel splits Effect's Context at the prompt, but not the JS async context.
  expect(seen).toEqual(["clause: body", "clause after a root wait: body", "body after resume: body"])
})
