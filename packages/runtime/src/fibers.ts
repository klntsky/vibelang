import { Deferred, Effect, Fiber, Scope } from "effect"

// The fiber forms of an .effeck file (D166-D170). The parser writes `await x`
// as $await(x), `select { ... }` as a call of $select, and `for concurrent`
// as a call of $forConcurrent; `fork x` is the prelude's fork.

/** What `await` waits for: a Fiber or a Deferred, or an array or object of them. */
export type Awaitable = Fiber.Fiber<any, any> | Deferred.Deferred<any, any>

type NotAwaitable = Effect.Effect<any, any, any> | PromiseLike<any> | ((...args: never) => unknown)

// Each type parameter is inferred rather than matched against any: a
// Deferred is invariant in its error type, and any is not assignable to never.
type AwaitedLeaf<T> = T extends Fiber.Fiber<infer A, infer _E> ? A : T extends Deferred.Deferred<infer A, infer _E> ? A : never
type AwaitedLeafError<T> = T extends Fiber.Fiber<infer _A, infer E> ? E : T extends Deferred.Deferred<infer _A, infer E> ? E : never
type IsAwaitable<T> = T extends Fiber.Fiber<infer _A, infer _E> ? true : T extends Deferred.Deferred<infer _A, infer _E> ? true : false

/**
 * The value `await x` returns: a Fiber's or a Deferred's value, or an array
 * or object of them with the same shape. never for an operand await cannot
 * wait for (EK4603). Elements must be Fibers or Deferreds themselves.
 */
export type AwaitedValue<T> =
  IsAwaitable<T> extends true ? AwaitedLeaf<T>
  : T extends NotAwaitable ? never
  : T extends readonly unknown[] ? { -readonly [K in keyof T]: AwaitedLeaf<T[K]> }
  : T extends object ? { -readonly [K in keyof T]: AwaitedLeaf<T[K]> }
  : never

/** The failures `await x` can propagate. */
export type AwaitedError<T> =
  IsAwaitable<T> extends true ? AwaitedLeafError<T>
  : T extends NotAwaitable ? never
  : T extends readonly unknown[] ? AwaitedLeafError<T[number]>
  : T extends object ? AwaitedLeafError<T[keyof T]>
  : never

/**
 * D167: `await x` waits for a Fiber (joining it), a Deferred, or every one of
 * an array or object of them, keeping the shape. When one fails, the fibers
 * still running are interrupted and the failure propagates, as Effect.all
 * fails fast.
 */
export function $await<const T>(value: T): Effect.Effect<AwaitedValue<T>, AwaitedError<T>>
// The implementation's return type is any: comparing it with the overload's
// recursive types would instantiate them without bound.
export function $await(value: unknown): any {
  return awaitValue(value)
}

function awaitValue(value: unknown): Effect.Effect<unknown, unknown> {
  if (Fiber.isFiber(value)) return Fiber.join(value)
  if (Deferred.isDeferred(value)) return Deferred.await(value)
  if (typeof value !== "object" || value === null) return Effect.die(new TypeError("await expects a Fiber, a Deferred, or an array or object of them"))
  const fibers: Array<Fiber.Fiber<unknown, unknown>> = []
  collectFibers(value, fibers)
  const all: Effect.Effect<unknown, unknown> = Array.isArray(value)
    ? Effect.all(value.map(awaitValue), { concurrency: "unbounded" })
    : Effect.map(
      Effect.all(Object.fromEntries(Object.entries(value).map(([key, item]) => [key, awaitValue(item)])), { concurrency: "unbounded" }),
      (result: Record<string, unknown>) => Object.fromEntries(Object.keys(value).map((key) => [key, result[key]])) // the order written
    )
  return Effect.onError(all, () => Effect.forEach(fibers, Fiber.interrupt, { discard: true }))
}

function collectFibers(value: unknown, fibers: Array<Fiber.Fiber<unknown, unknown>>): void {
  if (Fiber.isFiber(value)) {
    fibers.push(value)
  } else if (Array.isArray(value)) {
    for (const item of value) collectFibers(item, fibers)
  } else if (typeof value === "object" && value !== null && !Deferred.isDeferred(value)) {
    for (const item of Object.values(value)) collectFibers(item, fibers)
  }
}

type Program = () => Effect.Effect<any, any, any>

type Index<K> = K extends `${infer N extends number}` ? N : never

/** The winner of a select: the index of its case, in the order written, and its value. */
export type Selected<P extends ReadonlyArray<Program>, D extends boolean> =
  | { [K in keyof P]: { readonly index: Index<K>; readonly value: Effect.Success<ReturnType<P[K]>> } }[number]
  | (D extends true ? { readonly index: -1; readonly value: undefined } : never)

/**
 * D168: `select` starts each case in its own fiber, in the order written,
 * and the first to finish wins: its value is the result, or its failure
 * propagates. The other cases are interrupted. A case that finishes while
 * the cases are starting stops the rest from starting, so of cases that are
 * ready at once, the first written wins. With a default clause, when no case
 * finishes while they start, every case is interrupted and the result's index
 * is -1.
 */
export const $select = <const D extends boolean, const P extends ReadonlyArray<Program>>(
  hasDefault: D,
  ...programs: P
): Effect.Effect<Selected<P, D>, Effect.Error<ReturnType<P[number]>>, Effect.Services<ReturnType<P[number]>>> =>
  Effect.uninterruptibleMask((restore) =>
    Effect.gen(function*() {
      const fibers: Array<Fiber.Fiber<unknown, unknown>> = []
      const interruptOthers = (winner: number) =>
        Effect.forEach(fibers.filter((_, index) => index !== winner), Fiber.interrupt, { discard: true })
      for (let index = 0; index < programs.length; index++) {
        const fiber = yield* Effect.forkChild(Effect.suspend(programs[index]), { startImmediately: true })
        fibers.push(fiber)
        const exit = fiber.pollUnsafe()
        if (exit !== undefined) {
          yield* interruptOthers(index)
          return yield* Effect.map(exit, (value) => ({ index, value }))
        }
      }
      if (hasDefault) {
        yield* interruptOthers(-1)
        return { index: -1, value: undefined }
      }
      const [index, exit] = yield* Effect.onExit(restore(firstExit(fibers)), () => interruptOthers(-1))
      yield* interruptOthers(index)
      return yield* Effect.map(exit, (value) => ({ index, value }))
    })
  ) as any

const firstExit = (fibers: ReadonlyArray<Fiber.Fiber<unknown, unknown>>) =>
  Effect.callback<readonly [number, import("effect").Exit.Exit<unknown, unknown>]>((resume) => {
    let done = false
    const removers = fibers.map((fiber, index) =>
      fiber.addObserver((exit) => {
        if (done) return
        done = true
        resume(Effect.succeed([index, exit] as const))
      })
    )
    return Effect.sync(() => {
      for (const remove of removers) remove()
    })
  })

/**
 * D169: `for concurrent (const x of xs) body` runs the body for each item in
 * its own fiber, at most `concurrency` at once (no limit when undefined), and
 * joins them all before the loop ends. A failure interrupts the rest.
 */
export const $forConcurrent = <X, E, R>(
  items: Iterable<X>,
  concurrency: number | undefined,
  body: (item: X) => Effect.Effect<unknown, E, R>
): Effect.Effect<void, E, R> =>
  Effect.forEach(items, body, { concurrency: concurrency ?? "unbounded", discard: true })

/** D170: starts a program in a fiber that nothing owns (Effect.forkDetach). */
export const detach = <A, E, R>(program: Effect.Effect<A, E, R>): Effect.Effect<Fiber.Fiber<A, E>, never, R> =>
  Effect.forkDetach(program)

/** D170: starts a program in a fiber that `scope` owns (Effect.forkIn). */
export const forkIn = <A, E, R>(scope: Scope.Scope, program: Effect.Effect<A, E, R>): Effect.Effect<Fiber.Fiber<A, E>, never, R> =>
  Effect.forkIn(program, scope)
