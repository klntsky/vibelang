import { Effect } from "effect"

// A synchronous boundary latch: write arms it at its final continuation.
// The tracer consumes it only once the write's two frames have unwound.
export const latch = { depth: undefined as number | undefined, published: false, interrupted: false }
export const write = (_operand?: number): Effect.Effect<void> =>
  Effect.void.pipe(Effect.tap(() => Effect.withFiber((fiber) => Effect.sync(() => {
    latch.depth = (fiber as unknown as { _stack: unknown[] })._stack.length
  }))))
export const publish = (): Effect.Effect<void> => Effect.sync(() => { latch.published = true })

export const operand = (): Effect.Effect<number> => Effect.succeed(42)
export const transaction = (_strings: TemplateStringsArray): Effect.Effect<void> => write()
export const writeValue = Effect.suspend(write)
let evaluations = 0
export const make = (): Effect.Effect<number> => { evaluations++; return Effect.succeed(evaluations) }
export const twice = (program: Effect.Effect<number>): Effect.Effect<number[]> =>
  Effect.all([program, program], { concurrency: 1 })
