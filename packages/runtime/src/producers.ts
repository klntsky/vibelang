import { Effect, Queue, Stream } from "effect"
import * as K from "./kernel/kernel.js"
import { $normal } from "./regions.js"

/** The phantom requirement a producer body has until a loop handles its Yield. */
export interface YieldOf<T> {
  readonly "~esc/Yield": T
}
const YieldOp = K.operation<unknown, void>("esc/Yield")

export const $yield = <T>(value: T): Effect.Effect<void, never, YieldOf<T>> => YieldOp(value) as any

export class Producer<T, E, R> {
  declare readonly "~esc/Producer": { readonly t: T; readonly e: E; readonly r: R }
  constructor(readonly body: () => Effect.Effect<unknown, E, any>) {}
}

type ElementOf<R> = R extends YieldOf<infer T> ? T : never

/** A `function*` body becomes a cold producer: calling it runs none of the body. */
export const $produce = <E, R>(body: () => Effect.Effect<unknown, E, R>): Producer<ElementOf<R>, E, Exclude<R, YieldOf<any>>> =>
  new Producer(body)

type Completion = { readonly kind: string; readonly label?: string }

/**
 * `for (const x of producer)`: internal iteration. The loop body is a region
 * returning a completion record; `normal` and this loop's `continue` resume
 * the producer in tail position (D18); `break` and anything that leaves the
 * loop return from the clause, so the producer is abandoned and its cleanup
 * runs before the loop exits.
 */
export const $forOf = <T, E, R, C extends Completion, E2, R2>(
  producer: Producer<T, E, R>,
  body: (value: T) => Effect.Effect<C, E2, R2>,
  label?: string
): Effect.Effect<Exclude<C, { readonly kind: "break" | "continue"; readonly label: "" }> | typeof $normal, E | E2, R | R2> => {
  const own = (c: Completion) => c.label === "" || c.label === label
  return K.handle("for-of", [YieldOp], producer.body(), {
    [YieldOp.name]: (value: any, k: K.Continuation) =>
      Effect.map(body(value), (c: Completion) =>
        c.kind === "normal" || (c.kind === "continue" && own(c))
          ? K.tailResume(k, undefined)
          : c.kind === "break" && own(c)
          ? $normal
          : c)
  }, () => Effect.succeed($normal)) as any
}

/**
 * `for (const x of stream)`: pulls the Stream; `break`, `return`, or an error
 * in the body stops pulling, which closes the stream.
 */
export const $forStream = <T, E, R, C extends Completion, E2, R2>(
  stream: Stream.Stream<T, E, R>,
  body: (value: T) => Effect.Effect<C, E2, R2>,
  label?: string
): Effect.Effect<Exclude<C, { readonly kind: "break" | "continue"; readonly label: "" }> | typeof $normal, E | E2, R | R2> =>
  Effect.suspend(() => {
    const own = (c: Completion) => c.label === "" || c.label === label
    let result: Completion = $normal
    return Effect.as(
      Stream.runForEachWhile(stream, (value: T) =>
        Effect.map(body(value), (c: Completion) => {
          if (c.kind === "normal" || (c.kind === "continue" && own(c))) return true
          result = c.kind === "break" && own(c) ? $normal : c
          return false
        })),
      undefined
    ).pipe(Effect.map(() => result as any))
  })

/**
 * D26: a producer where a Stream is expected. The producer runs on the
 * stream's own fiber and blocks on a one-element buffer until the stream
 * pulls; closing the stream interrupts that fiber, so the producer's cleanup runs.
 */
export const $toStream = <T, E, R>(producer: Producer<T, E, R>): Stream.Stream<T, E, R> =>
  Stream.callback<T, E, R>((queue) =>
    Effect.andThen(
      $forOf(producer, (value: T) => Effect.as(Queue.offer(queue, value), $normal)),
      Queue.end(queue)
    ), { bufferSize: 1 })
