// Single-pass experiment: the intrinsics a neutral emission calls. Each is a choice point the
// patched checker (prototype/single-pass/patch/es-checker.js) resolves while it checks the
// emission once. The declared return types are placeholders: the checker computes each call's
// type from its arguments and from the position the call sits in.
//
// This is a global script (no top-level import or export), so the aliases below are globals the
// checker can instantiate by name.

/** A lambda: the plain form, the Effect-function form, and (zero parameters) the program form. */
declare function $esDual<P, Q, G>(plain: P, effect: Q, program: G, meta: string, id: number): unknown
/** A method or nested function body: the plain thunk and the Effect thunk. */
declare function $esBody<P, Q>(plain: P, effect: Q, meta: string, id: number): unknown
/** A call in an Effect function: the Effect to run (the call itself, or the call's value as a success). */
declare function $esBind<T>(value: T, id: number): unknown
/** A zero-argument call in an Effect function: the callee when it is an Effect value, else $esBind(call). */
declare function $esBind0<F, T>(callee: F, call: T, id: number): unknown
/** A zero-argument call in plain code: the callee when it is an Effect value, else the call. */
declare function $esVal0<F, T>(callee: F, call: T, id: number): unknown
/** A call in plain code to an .es function: the plain variant's value when the function compiles to plain JavaScript (D10). */
declare function $esPlain<T>(call: T, id: number): unknown
/** A receiver in an Effect function: the service (when it is a key and the member is a service member), else the receiver. */
declare function $esSvc<K>(receiver: K, member: string, id: number): unknown
/** A value in an argument, element, or property position: program, programs, Stream, or plain-variant conversion. */
declare function $esConv<T>(value: T, meta: string, id: number): unknown
/** A top-level function; meta lists its body's intrinsics for D10 (plain JavaScript) decisions. */
declare function $esFn<F>(fn: F, meta: string, id: number): F
/** A for...of iterable in an Effect function: producers and Streams add their E and R. */
declare function $esIter<I>(iterable: I, id: number): import("effect").Effect.Effect<Iterable<$EsElem<I>>, $EsErr<I>, $EsReq<I>>

type $EsSucceed<T> = import("effect").Effect.Effect<T, never, never>
type $EsTrusted<A, E> = import("effect").Effect.Effect<A, E, never>
type $EsBind<T> = import("../../../packages/runtime/src/index.ts").Bind<T>
type $EsStreamOf<P> = P extends import("../../../packages/runtime/src/index.ts").Producer<infer T, infer E, infer R>
  ? import("effect").Stream.Stream<T, E, R>
  : never
type $EsDirectOf<F> = F extends (...args: infer P) => import("effect").Effect.Effect<infer A, any, any> ? (...args: P) => A : F
type $EsElem<I> = I extends import("../../../packages/runtime/src/index.ts").Producer<infer T, any, any> ? T
  : I extends import("effect").Stream.Stream<infer T, any, any> ? T
  : I extends Iterable<infer T> ? T
  : never
type $EsErr<I> = I extends import("../../../packages/runtime/src/index.ts").Producer<any, infer E, any> ? E
  : I extends import("effect").Stream.Stream<any, infer E, any> ? E
  : never
type $EsReq<I> = I extends import("../../../packages/runtime/src/index.ts").Producer<any, any, infer R> ? R
  : I extends import("effect").Stream.Stream<any, any, infer R> ? R
  : never
