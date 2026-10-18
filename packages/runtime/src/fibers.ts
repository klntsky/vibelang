import { Deferred, Effect, Fiber, Scope } from "effect"

// The fiber forms of a .vibe file (D166-D170). The parser writes `await x`
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

type IsFiber<T> = T extends Fiber.Fiber<infer _A, infer _E> ? true : false
type JoinedLeaf<T> = T extends Fiber.Fiber<infer A, infer _E> ? A : never
type JoinedLeafError<T> = T extends Fiber.Fiber<infer _A, infer E> ? E : never
type NotJoinable = Effect.Effect<any, any, any> | Deferred.Deferred<any, any> | PromiseLike<any> | ((...args: never) => unknown)

/**
 * The value `join x` returns: a Fiber's value, or an array or object of them
 * with the same shape. never for an operand join cannot join (EK4611).
 * Elements must be Fibers themselves.
 */
export type JoinedValue<T> =
  IsFiber<T> extends true ? JoinedLeaf<T>
  : T extends NotJoinable ? never
  : T extends readonly unknown[] ? { -readonly [K in keyof T]: JoinedLeaf<T[K]> }
  : T extends object ? { -readonly [K in keyof T]: JoinedLeaf<T[K]> }
  : never

/** The failures `join x` can propagate. */
export type JoinedError<T> =
  IsFiber<T> extends true ? JoinedLeafError<T>
  : T extends NotJoinable ? never
  : T extends readonly unknown[] ? JoinedLeafError<T[number]>
  : T extends object ? JoinedLeafError<T[keyof T]>
  : never

/**
 * D225: `join x` joins a Fiber (Fiber.join), or every fiber of an array or
 * object of them, keeping the shape. When one fails, the fibers still running
 * are interrupted and the failure propagates, as Effect.all fails fast.
 */
export function $join<const T>(value: T): Effect.Effect<JoinedValue<T>, JoinedError<T>>
// The implementation's return type is any for the same reason as $await's.
export function $join(value: unknown): any {
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

// ---------------------------------------------------------------------------
// D196: the await operations. The parser writes `await.all x` as
// $awaitAll(x), and likewise allSettled, race, and any. Each element is a
// program passed by name (D147), started in its own fiber, or a Fiber or a
// Deferred already running, which is waited for, not restarted. Each behaves
// as the Promise method of the same name.

/** An element of an await operation's operand. */
// A Deferred is invariant in its error type, and any is not assignable to
// never, so a Deferred that cannot fail needs its own member.
export type AwaitElement =
  | Effect.Effect<any, any, any>
  | Fiber.Fiber<any, any>
  | Deferred.Deferred<any, any>
  | Deferred.Deferred<any, never>

// The Effect that waits for an element.
type ElementEffect<T> =
  T extends Fiber.Fiber<infer A, infer E> ? Effect.Effect<A, E>
  : T extends Deferred.Deferred<infer A, infer E> ? Effect.Effect<A, E>
  : T

type OperandEffects<Arg> =
  Arg extends ReadonlyArray<unknown> ? { -readonly [K in keyof Arg]: ElementEffect<Arg[K]> }
  : Arg extends Iterable<infer X> ? Iterable<ElementEffect<X>>
  : { -readonly [K in keyof Arg]: ElementEffect<Arg[K]> }

type AllOptions = {
  readonly concurrency?: number | "unbounded" | undefined
  readonly discard?: boolean | undefined
  readonly mode?: "default" | "result" | undefined
}

type AllReturn<Arg, O extends AllOptions> = OperandEffects<Arg> extends infer X extends
  | Iterable<Effect.Effect<any, any, any>>
  | Record<string, Effect.Effect<any, any, any>> ? Effect.All.Return<X, O>
  : never

type RaceElement<Arg> = Arg extends Iterable<infer X> ? ElementEffect<X> : never

type RaceReturn<Arg> = Effect.Effect<
  Effect.Success<RaceElement<Arg>>,
  Effect.Error<RaceElement<Arg>>,
  Effect.Services<RaceElement<Arg>>
>

const isIterable = (u: unknown): u is Iterable<unknown> =>
  typeof u === "object" && u !== null && Symbol.iterator in u && !Effect.isEffect(u)

const elementEffect = (element: unknown): Effect.Effect<unknown, unknown, unknown> =>
  Fiber.isFiber(element)
    ? Fiber.join(element)
    : Deferred.isDeferred(element)
    ? Deferred.await(element)
    : element as Effect.Effect<unknown, unknown, unknown>

// The operand's elements, read once: an iterable may not be read twice.
const operandOf = (operand: unknown): {
  readonly effects: ReadonlyArray<Effect.Effect<unknown, unknown, unknown>> | Record<string, Effect.Effect<unknown, unknown, unknown>>
  readonly held: ReadonlyArray<Fiber.Fiber<unknown, unknown> | undefined>
} => {
  if (Array.isArray(operand) || isIterable(operand)) {
    const elements = Array.from(operand as Iterable<unknown>)
    return { effects: elements.map(elementEffect), held: elements.map((e) => Fiber.isFiber(e) ? e : undefined) }
  }
  const entries = Object.entries(operand as Record<string, unknown>)
  return {
    effects: Object.fromEntries(entries.map(([key, e]) => [key, elementEffect(e)])),
    held: entries.map(([, e]) => Fiber.isFiber(e) ? e : undefined)
  }
}

// Effect.all returns a record's keys in the order its effects finish; an
// await operation keeps the order written, as await does (D167).
const inOrderWritten = (
  effects: ReadonlyArray<unknown> | Record<string, unknown>,
  all: Effect.Effect<any, any, any>
): Effect.Effect<any, any, any> =>
  Array.isArray(effects)
    ? all
    : Effect.map(all, (result: Record<string, unknown>) => Object.fromEntries(Object.keys(effects).map((key) => [key, result[key]])))

// Interrupts the fibers the operand held, except the one at index keep.
const interruptHeld = (held: ReadonlyArray<Fiber.Fiber<unknown, unknown> | undefined>, keep = -1) =>
  Effect.forEach(held.filter((f, index): f is Fiber.Fiber<unknown, unknown> => f !== undefined && index !== keep), Fiber.interrupt, {
    discard: true
  })

/**
 * `await.all x`: waits for every element, keeping the shape of an array,
 * tuple, or object (Effect.all with unbounded concurrency; for an object,
 * the Await Dictionary proposal's Promise.allKeyed). When one fails, the
 * rest are interrupted, fibers the operand held included, and the failure
 * propagates.
 */
// Not a const type parameter: an element passed by name, such as a template
// literal, would infer its literal type, which its program's type does not
// match. The [] in the constraint keeps an array literal a tuple.
export function $awaitAll<Arg extends Iterable<AwaitElement> | readonly [] | Record<string, AwaitElement>>(
  operand: Arg
): AllReturn<Arg, { readonly concurrency: "unbounded" }>
export function $awaitAll(operand: unknown): any {
  const { effects, held } = operandOf(operand)
  return Effect.onError(inOrderWritten(effects, Effect.all(effects as any, { concurrency: "unbounded" })), () => interruptHeld(held))
}

/**
 * `await.allSettled x`: waits for every element and returns each outcome as
 * a Result, keeping the shape (Effect.all with mode "result"). It never
 * fails.
 */
export function $awaitAllSettled<Arg extends Iterable<AwaitElement> | readonly [] | Record<string, AwaitElement>>(
  operand: Arg
): AllReturn<Arg, { readonly concurrency: "unbounded"; readonly mode: "result" }>
export function $awaitAllSettled(operand: unknown): any {
  const { effects, held } = operandOf(operand)
  return Effect.onError(
    inOrderWritten(effects, Effect.all(effects as any, { concurrency: "unbounded", mode: "result" })),
    () => interruptHeld(held)
  )
}

/**
 * `await.race x`: the first element to finish wins, success or failure
 * (Effect.raceFirst over any number). The others are interrupted, fibers the
 * operand held included. Of elements that are ready at once, the first
 * written wins. An empty race never finishes, as Promise.race([]) does not.
 */
export function $awaitRace<Arg extends Iterable<AwaitElement>>(operand: Arg): RaceReturn<Arg>
export function $awaitRace(operand: unknown): any {
  const { effects, held } = operandOf(operand)
  const programs = (effects as ReadonlyArray<Effect.Effect<unknown, unknown, unknown>>).map((effect) => () => effect)
  if (programs.length === 0) return Effect.never
  return Effect.flatMap(
    Effect.onExit($select(false, ...programs), (exit) => exit._tag === "Success" ? interruptHeld(held, exit.value.index) : interruptHeld(held)),
    (selected) => Effect.succeed(selected.value)
  )
}

/**
 * `await.any x`: the first element to succeed wins (Effect.raceAll); early
 * failures are ignored until one succeeds or every element has failed. The
 * others are interrupted, fibers the operand held included.
 */
export function $awaitAny<Arg extends Iterable<AwaitElement>>(operand: Arg): RaceReturn<Arg>
export function $awaitAny(operand: unknown): any {
  const { effects, held } = operandOf(operand)
  let winner = -1
  return Effect.onExit(
    Effect.raceAll(effects as ReadonlyArray<Effect.Effect<unknown, unknown, unknown>>, {
      onWinner: ({ index }) => {
        winner = index
      }
    }),
    (exit) => interruptHeld(held, exit._tag === "Success" ? winner : -1)
  )
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
