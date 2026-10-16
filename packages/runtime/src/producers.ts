import { Cause, Channel, Deferred, Effect, Exit, Pull, Stream } from "effect"
import type { NonEmptyReadonlyArray } from "effect/Array"
import * as K from "./kernel/kernel.js"
import { $normal } from "./regions.js"

// ---------------------------------------------------------------------------
// Producers (D26, design chapters 8 and 26)
//
// `function*` in an .es file declares a producer, and `yield v` performs the
// built-in Yield operation. Calling the producer runs none of its body: it
// returns a Stream that carries the body. `for...of` over that Stream runs the
// body under a Yield handler on the loop's own fiber (internal iteration);
// anything else that consumes it as a Stream runs the body on a fiber of its
// own and hands over one element per pull.
// ---------------------------------------------------------------------------

/** The built-in Yield operation that `yield` performs. */
const YieldOp = K.operation<unknown, void>("esc/Yield")

/** The requirement a producer body has until a loop or a Stream handles its yields. */
export interface YieldOf<T> {
  readonly "~esc/Yield": T
}

type ElementOf<R> = R extends YieldOf<infer T> ? T : never

/** `yield value` in a producer. */
export const $yield = <T>(value: T): Effect.Effect<void, never, YieldOf<T>> => YieldOp(value) as any

const producerBody: unique symbol = Symbol.for("@effectscript/runtime/producer") as any

/** A producer: a Stream that also carries its body for internal iteration. */
export type Producer<T, E = never, R = never> = Stream.Stream<T, E, R>

type Body = () => Effect.Effect<unknown, any, any>

const bodyOf = (u: unknown): Body | undefined =>
  typeof u === "object" && u !== null ? (u as { [producerBody]?: Body })[producerBody] : undefined

/**
 * A `function*` body becomes a producer: `function lines(path) { return
 * $produce(Effect.fnUntraced(function* lines() { … })) }`. Each run of the
 * producer calls `body` again, so iterating it twice runs it twice.
 */
export const $produce = <E, R>(
  body: () => Effect.Effect<unknown, E, R>
): Producer<ElementOf<R>, E, Exclude<R, YieldOf<any>>> => {
  const stream = handoff<ElementOf<R>, E, Exclude<R, YieldOf<any>>>(body as any)
  Object.defineProperty(stream, producerBody, { value: body })
  return stream
}

/** A completion record of a loop body, as the compiler's regions produce. */
type Completion = { readonly kind: string; readonly label?: string; readonly value?: unknown }

/**
 * `for (const x of iterable)` where `iterable` is a producer or a Stream. The
 * loop body is a region that returns a completion record, or nothing when no
 * jump leaves it. Nothing, `normal`, and a
 * `continue` that targets this loop, go on to the next element; a `break`
 * that targets this loop ends it normally; anything else ends the loop and is
 * returned for the enclosing code to dispatch.
 *
 * Over a producer, the body runs as the clause of a Yield handler on this
 * fiber, and resumes the producer in tail position, so a long producer runs in
 * constant stack (D18). A loop that ends early returns from the clause
 * without resuming, so the producer's cleanup runs before the loop exits.
 * Over any other Stream, the loop pulls it with Stream.toPull and closes it
 * when the loop exits.
 */
export function $forOf<T, E, R, E2, R2>(
  iterable: Stream.Stream<T, E, R>,
  body: (value: T) => Effect.Effect<Completion | void, E2, R2>,
  label?: string
): Effect.Effect<Completion, E | E2, R | R2>
export function $forOf<T, E2, R2>(
  iterable: Iterable<T>,
  body: (value: T) => Effect.Effect<Completion | void, E2, R2>,
  label?: string
): Effect.Effect<Completion, E2, R2>
export function $forOf(
  iterable: unknown,
  body: (value: any) => Effect.Effect<Completion | void, any, any>,
  label?: string
): Effect.Effect<Completion, any, any> {
  // A body that no jump leaves returns nothing: normal completion.
  const own = (c: Completion) => c.label === "" || c.label === label
  const next = (c: Completion | void) => c === undefined || c.kind === "normal" || (c.kind === "continue" && own(c))
  const exit = (c: Completion | void): Completion => (c === undefined || c.kind === "break" && own(c) ? $normal : c)
  const producer = bodyOf(iterable)
  if (producer) {
    return K.handle("for-of", [YieldOp], producer(), {
      [YieldOp.name]: (value: unknown, k: K.Continuation) =>
        Effect.map(body(value), (c) => (next(c) ? K.tailResume(k, undefined) : exit(c)))
    }, () => Effect.succeed($normal))
  }
  if (Stream.isStream(iterable)) {
    return Effect.scoped(Effect.gen(function*() {
      const pull = yield* Stream.toPull(iterable)
      while (true) {
        const chunk = yield* Effect.exit(pull)
        if (Exit.isFailure(chunk)) {
          if (Pull.isDoneCause(chunk.cause)) return $normal
          return yield* Effect.failCause(chunk.cause)
        }
        for (const value of chunk.value) {
          const c = yield* body(value)
          if (!next(c)) return exit(c)
        }
      }
    }))
  }
  return Effect.gen(function*() {
    for (const value of iterable as Iterable<unknown>) {
      const c = yield* body(value)
      if (!next(c)) return exit(c)
    }
    return $normal
  })
}

/**
 * `yield* iterable` in a producer: yields each element of another producer,
 * a Stream, or an iterable. Another producer's body runs in place, so its
 * yields reach the same handler.
 */
export const $yieldAll = <T, E = never, R = never>(
  iterable: Stream.Stream<T, E, R> | Iterable<T>
): Effect.Effect<void, E, R | YieldOf<T>> => {
  const producer = bodyOf(iterable)
  if (producer) return Effect.asVoid(producer())
  return Effect.asVoid($forOf(iterable as Iterable<unknown>, (value) => Effect.as($yield(value), $normal))) as any
}

/**
 * The Stream a producer is. Each run starts the producer on a fiber of its
 * own, in the stream's Scope, when the stream is first pulled. The producer's
 * Yield clause completes the pending pull and waits for the next one before it
 * resumes the producer: a handoff with no buffer, so the producer never runs
 * ahead of its consumer, and a pull from any fiber works (D26). Closing the
 * stream interrupts the producer's fiber, which abandons the producer, so its
 * cleanup runs before the close completes.
 */
const handoff = <T, E, R>(body: () => Effect.Effect<unknown, E, R>): Stream.Stream<T, E, R> =>
  Stream.fromChannel(Channel.fromTransform((_upstream, scope) =>
    Effect.sync(() => {
      let wanted = Deferred.makeUnsafe<void>() // completed by a pull: produce the next element
      let pending: Deferred.Deferred<T, E | Cause.Done> | undefined // the pull waiting for an element
      let started = false
      let finished = false
      const deliver = (effect: Effect.Effect<T, E | Cause.Done>) => {
        const response = pending
        pending = undefined
        if (response !== undefined) Deferred.doneUnsafe(response, effect)
      }
      const produce = Effect.andThen(
        Deferred.await(wanted),
        K.handle("stream", [YieldOp], Effect.suspend(body), {
          [YieldOp.name]: (value: unknown, k: K.Continuation) =>
            Effect.suspend(() => {
              wanted = Deferred.makeUnsafe<void>()
              deliver(Effect.succeed(value as T))
              return Effect.as(Deferred.await(wanted), K.tailResume(k, undefined))
            })
        })
      )
      const run = Effect.onExit(produce, (exit) =>
        Effect.sync(() => {
          finished = true
          deliver(Exit.isSuccess(exit) ? Cause.done() : Effect.failCause(exit.cause as Cause.Cause<E>))
        }))
      const pull: Pull.Pull<NonEmptyReadonlyArray<T>, E> = Effect.suspend(() => {
        if (finished) return Cause.done()
        const response = Deferred.makeUnsafe<T, E | Cause.Done>()
        pending = response
        const start = started ? Effect.void : Effect.asVoid(Effect.forkIn(run as Effect.Effect<void>, scope))
        started = true
        return Effect.andThen(start, () => {
          Deferred.doneUnsafe(wanted, Effect.void)
          return Effect.map(Deferred.await(response), (value): NonEmptyReadonlyArray<T> => [value])
        })
      })
      return pull
    })
  )) as Stream.Stream<T, E, R>

/** Kept for code generated by the prototype compiler: a producer is a Stream already. */
export const $toStream = <T, E, R>(producer: Producer<T, E, R>): Stream.Stream<T, E, R> => producer

/** Kept for code generated by the prototype compiler: `$forOf` iterates Streams too. */
export const $forStream = $forOf
