### 16. Callbacks and higher-order functions

#### A callback type without a contract accepts any contract

```ts es
function mapAll<A, B>(items: ReadonlyArray<A>, f: (a: A) => B) {
  const out: B[] = []
  for (const item of items) out.push(f(item))
  return out
}
// <A, B, E, R>(items: ReadonlyArray<A>, f: (a: A) => B!E!R) => B[]!E!R

const names = mapAll(ids, (id) => Users.find(id).name)   // string[]!UserNotFound!Users
const sizes = mapAll(words, (w) => w.length)             // number[]!!
```

`f`'s type says nothing about errors or services, so `mapAll` passes through whatever its callback does: a missing contract on a callback parameter means "inferred from each call's argument", never "cannot fail, needs nothing". To restrict a callback, write its contract: `f: (a: A) => B!!`. D12 stands alongside D57: `infer` selects inferred channels in the return annotation of a function with a body. It does not introduce inference in parameter types or aliases. In a callback parameter, `(a: A) => Effect<B>` keeps TypeScript's defaults and means `(a: A) => Effect<B, never, never>`, while `(a: A) => B` accepts any contract.

This rule also covers declarations, methods, and function expressions that are not call arguments. Added channel parameters are named E and R, then E2 and R2 for another callback (D68). A contract-less callback cannot be called inside a synchronous JavaScript callback such as a `sort` comparator: it can pause, so that call is EK4301.

The declaration file writes the polymorphism out, as the comment shows. A TypeScript caller therefore passes a callback that returns an `Effect`, and wraps a plain one: `mapAll(words, (w) => Effect.succeed(w.length))`.

`mapAll` compiles to frames in v0 whatever its callback does (Chapter 15). The same rule makes wrappers short:

```ts es
function withRetry<A>(times: number, program: () => A) {
  return retry(program, Schedule.recurs(times))
}
// <A, E, R>(times: number, program: () => A!E!R) => A!E!R
```

#### Callbacks into JavaScript

A JavaScript function such as `Array.prototype.map` calls its callback synchronously, on a stack frame the runtime cannot pause or capture. A callback passed where a plain function type is expected must therefore compile to plain JavaScript (Chapter 15): no Effect operation, and E = `never`. Anything else is EK4301 (Chapter 10). The same holds for a named Effeck function passed as a value: `stores.map(makeSync)` passes `makeSync`'s plain form, and a function without one is rejected.

#### Which Effect combinators run their argument on another fiber

A control operation must be handled on the fiber that performs it (Chapter 9). The compiler therefore needs to know, for each Effect function that takes a program, whether it runs that program on the same fiber. These were measured by performing a control operation inside each combinator, with the handler outside it (`tests/fiber-behavior.test.ts`):

```ts es
try {
  retry(Approval.approve("a"), Schedule.recurs(2))     // same fiber: reaches the handler
  timeout(Approval.approve("b"), "1 second")           // other fiber: EK4201
} handle Approval.approve(_request) {
  resume(true)
}
```

| Same fiber: a handler outside still works | Other fibers: handle control operations inside |
| --- | --- |
| `flatMap`, `map`, `catch`, `catchCause`, `ensuring`, `onExit`, `provideService`, `scoped`, `acquireRelease` (acquire), `acquireUseRelease` (use), `retry`, `repeat`, `uninterruptible`, sequential `forEach` and `all`, `zip` by default | Measured: `timeout`, `race`, `raceFirst`, `forEach` with concurrency 2, unbounded `all`, `zip` with `concurrent: true`, `forkChild`. From source: `forkIn`, `forkScoped`, `forkDetach`, `Layer.merge` builds, parallel Scope finalizers |

`timeout` is on the right because it runs its program on a child fiber to race it against the clock. For an Effect function outside this table, the compiler assumes the argument may run on another fiber.

The runtime is the guarantee and the table is the diagnostic. A control operation that reaches another fiber fails loudly at run time with `HandlerNotOnThisFiber` (T7b, T8), never silently. The table turns that defect into a compile-time error where it can, and a combinator missing from it costs a conservative error, not a wrong program.

TypeScript functions are judged by their signatures, because a TypeScript function can fork inside its body without taking a program argument: a TypeScript function whose R contains a control operation is EK4202 (Chapter 9). Services need no such check: their implementations live in the Context, which every forked fiber inherits.

The rule is conservative in one case. TypeScript code can supply an implementation of a control operation that always continues exactly once, and that implementation runs in place like a service and would work on a forked fiber. The compiler cannot see that from Effeck, so it treats every control operation as bound to its fiber.
