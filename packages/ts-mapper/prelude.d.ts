// The prelude of Effeck's TypeScript 7 content mapper. Every file the mapper
// projects starts with one import from this module:
//
//   import { Effect, Layer, $gen, $run, ... } from "effeck-ts-mapper/prelude"; true satisfies $PreludeLoaded;
//
// It imports the prelude names the file uses and does not declare itself
// (D4, D48, D146, D151), as the fork resolves them, and the helpers the
// projection writes, under names that no identifier of the file uses.
//
// These declarations are types only: tsc emits no JavaScript for a mapped
// file, so nothing here runs. The projector that writes the calls lives in
// typescript/tsc/internal/effeckmapper. Namespaces are re-exported straight
// from effect, so a declaration file that names Effect.Effect, Layer.Layer,
// or Option.Option names effect's own modules.

import type * as EffectModule from "effect/Effect"
import type * as ScopeModule from "effect/Scope"
import type * as StreamModule from "effect/Stream"
import type { Cause, Config, Context, Exit } from "effect"
import type * as Runtime from "effeck-runtime"

// ---------------------------------------------------------------- prelude names

// D4, D146: effect's root modules, every one in scope in a .effeck file.
export {
  Arbitrary, Array, BigDecimal, BigInt, Boolean, Brand, ByteSize, Cache, Cause, Channel, ChannelSchema, Chunk,
  Clock, Combiner, Config, ConfigProvider, Console, Context, Cron, Crypto, Data, DateTime, Deferred, Differ,
  Duration, Effect, Effectable, Equal, Equivalence, ErrorReporter, ExecutionPlan, Exit, Fiber, FiberHandle,
  FiberMap, FiberSet, FileSystem, Filter, Formatter, Function, Graph, Hash, HashMap, HashRing, HashSet, HKT,
  Inspectable, Iterable, JsonPatch, JsonPointer, JsonSchema, Latch, Layer, LayerMap, LayerRef, Logger, LogLevel,
  ManagedRuntime, Match, Metric, MutableHashMap, MutableHashSet, MutableList, MutableRef, Newtype,
  NonEmptyIterable, Number, Optic, Option, Order, Ordering, PartitionedSemaphore, Path, Pipeable, PlatformError,
  Pool, Predicate, PrimaryKey, PubSub, Pull, Queue, Random, RcMap, RcRef, Record, Redactable, Redacted, Reducer,
  Ref, References, RegExp, Request, RequestResolver, Resource, Result, Runtime, Schedule, Scheduler, Schema,
  SchemaAST, SchemaGetter, SchemaIssue, SchemaParser, SchemaRepresentation, SchemaTransformation, Scope,
  ScopedCache, ScopedRef, Semaphore, Sink, StandardSchema, Stdio, Stream, String, Struct, SubscriptionRef, Symbol,
  SynchronizedRef, Take, Terminal, Tracer, Trie, Tuple, TxChunk, TxDeferred, TxHashMap, TxHashSet,
  TxPriorityQueue, TxPubSub, TxQueue, TxReentrantLock, TxRef, TxSemaphore, TxSubscriptionRef, Types, UndefinedOr,
  Unify, Utils, Version,
  absurd, cast, flow, hole, identity
} from "effect"

/** D4: in a .effeck file, `Effect<A, E, R>` is `Effect.Effect<A, E, R>`. */
export type Effect<A, E = never, R = never> = EffectModule.Effect<A, E, R>;

/** D48: in a .effeck file, the type `Scope` is `Scope.Scope`. */
export type Scope = ScopeModule.Scope;

// D151: the Effect functions a .effeck file calls without the Effect. prefix.
export {
  all, die, exit, forEach, log, logDebug, logError, logInfo, logWarning, race, repeat, retry, sleep, timeout, withSpan
} from "effect/Effect"

// The runtime's prelude names (the fork's prelude.go).
export {
  TaggedError, Control, pipe, wire, fork, handle, resume, resumeWith, abandon, sync, callback, Resume, runPromise,
  runFork, runSync, runCallback, detach, forkIn, interrupt, yieldNow, uninterruptible, restore, interruptible,
  customMatcher
} from "effeck-runtime"

// ---------------------------------------------------------------- helpers

// The runtime's region helpers (D13, D15, D132, D173): regions, handle
// statements, and provide blocks lower onto these.
export { $catch, $finally, $tailResume, $tailResumeWith, $provideService } from "effeck-runtime"

/**
 * `using x = acquire()` in an Effect body (D47, D94, D14): the block's Scope
 * holds an acquisition, which removes Scope from the acquisition's R; any
 * other value is a Disposable the block releases.
 */
export declare function $acquire<A, E, R>(acquisition: EffectModule.Effect<A, E, R>): EffectModule.Effect<A, E, Exclude<R, ScopeModule.Scope>>
export declare function $acquire<T>(value: T): EffectModule.Effect<T, never, never>

/** A fork in a block (D19, D96): the block's Scope owns the fiber, so Scope leaves R. */
export declare function $scoped<A, E, R>(fork: EffectModule.Effect<A, E, R>): EffectModule.Effect<A, E, Exclude<R, ScopeModule.Scope>>

// finally (exit) { ... } (D178), and the interruption blocks (D179, D180).
export { $finallyExit, uninterruptible, restore, interruptible } from "effeck-runtime"

// The pattern matching helpers the parser writes for match and is (D206-D225).
export { $is, $extract, $rest, $noMatch, $patternError } from "effeck-runtime"

/*
 * Completion records, as the checker sees them. A return in a region's
 * block returns $box(value): the value itself, joined with a brand, so the
 * block's generator never infers a single literal type and widens it on its
 * own; the enclosing function's own return, after $returned removes the
 * brand, sees every return site at once and widens as TypeScript would for
 * the source. A break or continue that leaves the region returns
 * $jump("break:label"), and a block that ends normally returns $end. (The
 * runtime's records are { kind, value } objects; their property would widen
 * an inferred literal union.)
 */
declare const endBrand: unique symbol
declare const jumpBrand: unique symbol
declare const boxBrand: unique symbol
/** What a region's return adds to its value. */
export interface $Boxed {
  readonly [boxBrand]: true
}
/** A return in a region's block: its value, unwidened. */
export declare function $box<T>(value: T): T | $Boxed
/** The normal end of a region's block. */
export declare const $end: { readonly [endBrand]: true }
/** A break or continue that leaves a region: "break:label" or "continue:label". */
export interface $Jump<K extends string> {
  readonly [jumpBrand]: K
}
export declare function $jump<const K extends string>(kind: K): $Jump<K>
/** A completion that can only be a return's value: the value, unboxed. */
export declare function $unboxed<T>(completion: T): Exclude<T, $Jump<string> | typeof $end | $Boxed>
/** The completion is a return's value. */
export declare function $returned<T>(completion: T): completion is Exclude<T, $Jump<string> | typeof $end | $Boxed>
/** The completion is the jump kind. */
export declare function $jumped<T, const K extends string>(completion: T, kind: K): completion is Extract<T, $Jump<K>>

type AnyEffect = EffectModule.Effect<any, any, any>

/**
 * The E and R of what a generator yields, member by member: a union of the
 * yielded Effects' channels without subtype reduction, as the fork joins
 * them. (Effect.gen's own [Eff] extends [Effect<…, infer E, …>] merges the
 * candidates with subtype reduction, so an error that structurally extends
 * another, such as a TaggedError with a name field, would vanish from E.)
 */
type YieldE<Eff> = Eff extends EffectModule.Effect<infer _A, infer E, infer _R> ? E : never
type YieldR<Eff> = Eff extends EffectModule.Effect<infer _A, infer _E, infer R> ? R : never

/** A function body: `function f() { body }` projects to `function f() { return $gen(function* () { body }) }`. */
export declare function $gen<Eff extends AnyEffect, AEff>(f: () => Generator<Eff, AEff, never>): EffectModule.Effect<AEff, YieldE<Eff>, YieldR<Eff>>
export declare function $gen<Self, Eff extends AnyEffect, AEff>(
  options: { readonly self: Self },
  f: (this: Self) => Generator<Eff, AEff, never>
): EffectModule.Effect<AEff, YieldE<Eff>, YieldR<Eff>>

/**
 * Effect subtypes that a call gives as data, not runs (D24): a Config, an
 * Exit, a Context key, and a yieldable error are Effects to TypeScript.
 */
type DataEffect = Config.Config<any> | Exit.Exit<any, any> | Context.Key<any, any> | Cause.YieldableError

/** The members of T that run: Effects that are not data subtypes. */
type Running<T> = Extract<Exclude<T, DataEffect>, AnyEffect>

/** Accepts T only when some member of T runs, so a union without an Effect falls through. */
type HasRunning<T> = [Running<T>] extends [never] ? never : unknown

/**
 * Runs a call (L3): a call `c` in an Effect body projects to
 * `(yield* $run(c))`. An Effect runs and gives its success type; a union
 * runs its Effect members, so `Effect<A, E, R> | U` gives `A | U`. Data
 * subtypes, values of other types, and a type parameter's values are data,
 * lifted to an Effect that succeeds with them. Overloads, not a conditional
 * type, so a generic body's `T | undefined` resolves to data instead of a
 * deferred type (research/typedirected.md section 6.5).
 */
export declare function $run<T extends DataEffect>(value: T): EffectModule.Effect<T, never, never>
export declare function $run<A, E, R>(value: EffectModule.Effect<A, E, R>): EffectModule.Effect<A, E, R>
export declare function $run<T>(value: T & HasRunning<T>): EffectModule.Effect<
  EffectModule.Success<Running<T>> | Exclude<T, Running<T>>,
  EffectModule.Error<Running<T>>,
  EffectModule.Services<Running<T>>
>
export declare function $run<T>(value: T): EffectModule.Effect<T, never, never>

// ---------------------------------------------------------------- producers

declare const emitBrand: unique symbol
/** What a producer's `yield x` yields: x, as an element of its Stream (D26). */
export interface $Emit<A> {
  readonly [emitBrand]: A
}
/** `yield x` in a producer projects to `yield $emit(x)`. */
export declare function $emit<A>(value: A): $Emit<A>
type Emitted<Y> = Y extends $Emit<infer A> ? A : never

/**
 * A producer, a generator function in an .effeck file (D26):
 * `function* f() { body }` projects to `function f() { return $produce(function* () { body }) }`.
 * Its Stream's A is what it emits; E and R are what it runs.
 */
export declare function $produce<Y>(
  body: () => Generator<Y, unknown, any>
): StreamModule.Stream<Emitted<Y>, YieldE<Exclude<Y, $Emit<any>>>, YieldR<Exclude<Y, $Emit<any>>>>
export declare function $produce<Self, Y>(
  options: { readonly self: Self },
  body: (this: Self) => Generator<Y, unknown, any>
): StreamModule.Stream<Emitted<Y>, YieldE<Exclude<Y, $Emit<any>>>, YieldR<Exclude<Y, $Emit<any>>>>

/** `yield* p` in a producer emits each element of the producer or Stream p. */
export declare function $delegate<A, E, R>(
  producer: StreamModule.Stream<A, E, R>
): Generator<$Emit<A> | EffectModule.Effect<never, E, R>, void, unknown>
export declare function $delegate<T>(iterable: Iterable<T>): Generator<$Emit<T>, void, unknown>

/**
 * A for...of in an Effect body or a producer iterates `(yield* $each(xs))`:
 * a Stream's elements, with its E and R, or an iterable as itself (D26).
 * The iterable overload comes first, so a value of type any iterates as
 * any, adding nothing to E and R; a Stream is not iterable.
 */
export declare function $each<T>(iterable: Iterable<T>): EffectModule.Effect<Iterable<T>, never, never>
export declare function $each<A, E, R>(stream: StreamModule.Stream<A, E, R>): EffectModule.Effect<Iterable<A>, E, R>

/**
 * L1's dual form: a parameterless lambda that is both a program and a
 * function that returns it, `() => body` at a position that accepts either.
 */
export declare function $dual<A, E, R>(program: EffectModule.Effect<A, E, R>): EffectModule.Effect<A, E, R> & (() => EffectModule.Effect<A, E, R>)

/**
 * A function value at a position that takes a program (D7): the function
 * runs as its program, `Effect.suspend(f)`. Any other value is itself.
 */
export declare function $asProgram<A, E, R>(f: () => EffectModule.Effect<A, E, R>): EffectModule.Effect<A, E, R>
export declare function $asProgram<T>(value: T): T

/** `throw x` in an Effect body projects to `return yield* $fail(x)`. */
export declare const $fail: typeof EffectModule.fail

/**
 * The base of a service, a control, or a D1 abstract class (D1, D45):
 * `abstract class Users extends $service<Users>("pkg/users#Users") {}`.
 */
export declare const $service: typeof Runtime.$service

/**
 * The base of a service that declares `static readonly default` (D40): a
 * Context.Reference, which needs no provider and joins no R. Types only;
 * the executable rendering passes the default.
 */
export declare function $reference<I>(key: string): Runtime.ReferenceKey<I>

declare const inferred: unique symbol
/** A contract channel written `infer` (D57), or dependencies left out of a function's contract. */
export type $Infer = typeof inferred
type Pin<Written, Inferred> = [Written] extends [$Infer] ? Inferred : Written

/**
 * A written contract with an `infer` channel (D57):
 * `function f(): A!infer E!R { body }` projects to
 * `function f() { return $contract<A, $Infer, R>()($gen(function* () { body })) }`.
 * The written channels are checked and pinned; the inferred ones come from the body.
 */
export declare function $contract<A, E, R>(): <A2 extends Pin<A, unknown>, E2 extends Pin<E, unknown>, R2 extends Pin<R, unknown>>(
  effect: EffectModule.Effect<A2, E2, R2>
) => EffectModule.Effect<Pin<A, A2>, Pin<E, E2>, Pin<R, R2>>

/** What a service member call reads: each member, with the service added to R (D41). */
type Accessors<I, S> = {
  readonly [M in keyof S]: S[M] extends (...args: infer P) => EffectModule.Effect<infer A, infer E, infer R>
    ? (...args: P) => EffectModule.Effect<A, E, R | I>
    : S[M] extends (...args: infer P) => infer B
      ? (...args: P) => EffectModule.Effect<B, never, I>
      : EffectModule.Effect<S[M], never, I>
}

/**
 * A member call on a possible service key (D41): `Users.find(id)` projects to
 * `$use(Users).find(id)`. A Context key's members are its service's, with the
 * service added to R, and the key's own members remain where the service
 * declares no member of that name. Any other value is itself.
 */
export declare function $use<K extends Context.Key<any, any>>(
  key: K
): K extends Context.Key<infer I, infer S> ? Accessors<I, S> & Omit<K, keyof S> : K
export declare function $use<T>(value: T): T

/**
 * A call without arguments of a value that has no call signature (D90):
 * `Users()` projects to `$value(Users)()`. A function is itself; an Effect
 * value, such as a Context key or a Config, becomes a function that runs it.
 */
export declare function $value<F extends (...args: never[]) => unknown>(f: F): F
export declare function $value<A, E, R>(effect: EffectModule.Effect<A, E, R>): () => EffectModule.Effect<A, E, R>
export declare function $value<T>(value: T): T

// ---------------------------------------------------------------- sentinel

/**
 * Whether T is the type of a name whose module did not resolve. Such a type
 * is TypeScript's error type, which passes for any other type, so the usual
 * `0 extends 1 & T` test gives the error type too; only an unwrapped
 * comparison against a type no real value has gives an answer.
 */
type Unresolved<T> = [T] extends [{ readonly "~effeck-ts-mapper/unresolved": true }] ? true : false

/**
 * The first line of every projected file states `true satisfies $PreludeLoaded`.
 * When this module cannot resolve effect or effeck-runtime from its own
 * location, every helper above is the error type, and skipLibCheck hides the
 * failed import; the statement then fails at line 1 of the .effeck file with
 * the fix (TS1360).
 */
export type $PreludeLoaded =
  Unresolved<typeof EffectModule.gen> extends true
    ? "effeck-ts-mapper: the prelude cannot resolve effect from its own location; install effect 4.0.5 beside effeck-ts-mapper"
    : Unresolved<typeof Runtime.$service> extends true
      ? "effeck-ts-mapper: the prelude cannot resolve effeck-runtime from its own location; install effeck-runtime beside effeck-ts-mapper"
      : true
