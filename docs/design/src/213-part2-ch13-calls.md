## Part II. The rules

Part I showed the language by example. This part states the rules behind those examples precisely, still from the programmer's side. Part III explains how the runtime carries them out.

### 13. What a call means

#### One syntax, several meanings

A call is always written `f(x)`. What it compiles to depends on what `f` is, and the compiler decides from the types:

```ts es
const n = double(4)                    // Effeck function that compiles to plain JavaScript
const user = Users.find(id)            // service method
const row = findUser(id)               // TypeScript function returning an Effect
const label = formatName(user)         // trusted TypeScript function
const ok = Approval.approve(request)   // control operation
const port = Port()                    // a Config value: data until called
```

```ts gen
const n = double$direct(4)                     // a plain JavaScript call (Chapter 15)
const user = yield* (yield* Users).find(id)    // the implementation in the current Context, then the method
const row = yield* findUser(id)                // runs the returned program
const label = formatName(user)                 // an ordinary call
const ok = yield* (yield* Approval).approve(request)    // asks the nearest handler (Chapter 9)
const port = yield* Port                       // runs the Config
```

The full rule:

| The callee is | The call | Generated |
| --- | --- | --- |
| An Effeck function that can pause | Runs it on the current fiber | `yield* f(a)` |
| An Effeck function that compiles to plain JavaScript (Chapter 15) | Runs it inline | `f$direct(a)`, or `f(a)` with a plain return annotation (D67) |
| A TypeScript function whose call has the type `Effect<A, E, R>` | Runs the returned program | `yield* f(a)` |
| A TypeScript function whose R contains a control operation | Rejected in v0: EK4202 (Chapter 9) | |
| A value of an `Effect` type, called as `p()` | Runs it | `yield* p` |
| A service member, `Users.find(id)` or `Path.join(a, b)` | Looks the service up in the current Context (it joins R), then applies these rules to the member: an `Effect` runs, a plain value is a trusted call (Chapter 4) | `yield* (yield* Users).find(id)`; `(yield* Path).join(a, b)` |
| A service member that is an `Effect` value, `Terminal.readLine()` | Looks the service up, then runs the member | `yield* (yield* Terminal).readLine` |
| A service, `Users()` | Returns the implementation | `yield* Users` |
| A control operation, `Approval.approve(x)` | Asks the nearest handler on this fiber | `yield* (yield* Approval).approve(x)` |
| A JavaScript built-in, or an export of `effect` or `@effect/*` that does not return an `Effect` | A trusted call: listed exceptions join E, others are defects | `f(a)`, with listed exceptions converted |
| A TypeScript function tagged `@effeck trusted` | The same | `f(a)`, with listed exceptions converted |
| Any other plain TypeScript or JavaScript function | Rejected: EK4401 (Chapter 10) | |
| A platform API, or a clock or randomness read | Rejected: EK4402, EK4403 (Chapters 10, 11) | |
| `new C(…)` | Constructs; never runs a program | `new C(…)` |

Because the meaning depends on types, the compiler needs TypeScript's type checker, not a source transform. That is why Effeck is a fork of the TypeScript compiler (Chapter 27).

#### Calls run `Effect` itself, not everything that extends it

A call runs its result only when the call's type, after TypeScript instantiates it, is `Effect<A, E, R>` itself. Several Effect types extend `Effect` and carry structure of their own: `Config` (`Config.ts:113`), `Exit` (`Exit.ts:89`), service keys (`Context.ts:68`), and `TaggedError` instances, which are yieldable (`Cause.ts:1312`). A call that returns one of them returns data, and calling that data runs it:

```ts es
const Port = Config.withDefault(Config.Port("PORT"), 3000)    // data: composed before anything reads it

function makeError(raw: string) {
  return new InvalidId({ raw })                                // data: returned, not thrown
}

function serverPort() {
  return Port()                                                // runs the Config
}
```

Without this rule, `Config.Port("PORT")` would read the variable before `Config.withDefault` saw it, and `makeError` would fail instead of returning an error value.

#### Function values

A function used as a value is passed in the form its destination expects:

| Expected type | What is passed |
| --- | --- |
| A function type that returns an `Effect`, as Effect's combinators expect | The function's Effect form |
| A plain function type, as `Array.prototype.map` expects | The function's plain-JavaScript form, if it has one (Chapter 15); otherwise EK4301 |
| `Effect<A, E, R>`, an iterable or record of them | A function with no parameters, converted to a program (Chapter 5); or an expression that calls an Effect, passed by name (D147) |
| Either a function or an `Effect`, as `HttpRouter.add` and `Effect.withErrorReporting` accept | A function with parameters: the function. A function without parameters: the function if it compiles to plain JavaScript, a program if it can pause |

The last row matters because several Effect APIs choose at run time with `Effect.isEffect`. In `HttpRouter.add("GET", "/users/:id", (request) => showUser(request))`, the handler has a parameter and is passed as a function. In `HttpRouter.add("GET", "/health", () => checkHealth())` and `Effect.withErrorReporting(() => serve())`, the argument can pause and is passed as a program.

An Effect callback or program destination means Effect's own interface, or a type parameter constrained to it; structured subtypes such as `Config` remain data (D109). A destination that allows both plain and Effect returns selects plain when D10 permits and Effect otherwise (D112). A contract-annotated lambda selected as plain has type `(…) => A`, and plain-callback inference starts from A (D113). These rules apply to named function values as well as lambdas (D66).

#### Programs and continuations

Two kinds of "rest of the work" appear in this document, and they behave differently. A **program** is the reusable kind from Chapter 5. A **continuation** is the `k` a handler receives in Chapter 9.

| | Program | Continuation |
| --- | --- | --- |
| What it is | A description of work that has not started | The rest of one particular run, paused at a call |
| Created by | Writing a function or thunk; importing an `Effect` | A control operation, when a handler receives it |
| How many times it runs | Every time it is called | At most once |
| Retry | Runs it again from scratch, with fresh local variables | Impossible: it has been used |
| Other fibers | Can run on any fiber (`fork`) | Only on the fiber that created it |
| Type | `() => A` with contract `Effect<A, E, R>`; `Effect<A, E, R>` at the TypeScript boundary | `Continuation<V, E, H>`, usable only inside its clause (Chapter 9) |

#### How program values are spelled

A call runs where it stands, except where the expected type is `Effect`: there an expression that calls an `.effeck` function or a function that returns an `Effect`, or looks up a service, is passed by name (D147). The compiler passes a program that evaluates the expression each time it runs, as if the source had written `() =>` before it, so `Effect.retry(syncInventory(store), Schedule.recurs(3))` retries the call and `fork(fetchReport())` runs nothing at the call site. This holds for arguments, array elements, and record fields, in function bodies and at a module's top level. A position that accepts any value, such as `Console.log`'s parameters or `Effect.succeed`'s, still runs the call where it stands, and an expression whose value is already an `Effect` keeps its meaning. Each overload candidate decides by its own declared parameter, as it does for a lambda (D56). A hover on the callee of a call passed by name says so.

A program of several statements needs a form that does not run: a function with no parameters. The compiler converts such a function to an `Effect` only where the expected type is `Effect`, including the elements of an array and the fields of a record, such as `Effect.all`'s argument, and an array or record of programs built at run time, such as `stores.map(makeSync)` (D49). Other iterables, including `Set` and `Map` values, convert by their iterated type and never as records (D111). It never converts a value because it merely looks like a function:

```ts gen
// Effect.retry(syncInventory(store), Schedule.recurs(3))
yield* Effect.retry(Effect.gen(function* () { return yield* syncInventory(store); }), Schedule.recurs(3))
// Effect.retry(retrySync, Schedule.recurs(3)), with const retrySync = () => syncInventory(store)
yield* Effect.retry(Effect.suspend(retrySync), Schedule.recurs(3))
```

"The expected type" has one precise meaning: the declared parameter type of the overload candidate TypeScript is trying, before instantiation (D44). TypeScript's own contextual type is the wrong one, because it is inferred from the argument itself. In `ids.map((id) => Users.find(id))`, `map`'s declared parameter is `(value: T, index: number) => U`, a plain function type, so the lambda is a callback that cannot pause (EK4301); instantiating `U` from the lambda would have made it look like an Effect position. When resolution fails, the candidates whose arity fits decide. The instantiated type is used only when the declared type is a bare type parameter fixed by another argument, such as `Layer.succeed`'s `NoInfer<S>`.

With overloads, the first candidate that accepts the lambda in the form that candidate expects wins (D56). TypeScript tries candidates in declaration order, and against each one the lambda takes the form the candidate's parameter declares: a plain function, an Effect function, or a program.

```ts dts
// helpers.d.ts: a plain callback first, an Effect callback second
import { Effect } from "effect"

/** @effeck trusted */
export declare function register(f: (x: number) => number): string
export declare function register<E, R>(f: (x: number) => number!E!R): string!E!R
```

```ts es
const a = register((x) => Math.max(x, 0))                    // plain: overload 1 accepts it
const b = register((x) => Users.find(String(x)).id.length)   // Effect function: overload 2
```

Against overload 1, `b`'s lambda is a plain function, and a plain function cannot call a service, so overload 1 fails and overload 2 accepts the Effect form. Swap the two overloads and `a`'s lambda becomes an Effect function: declaration order decides, as it does in TypeScript. The lambda's own inferred type never picks its form, so the result does not depend on which form the compiler tries first. A lambda with no expected type, such as one a function returns, is plain when Chapter 15's rule allows it and an Effect function otherwise.

#### Where programs start

A call never starts a fiber. A program starts only when a **runner** receives it. The compiler recognizes runners by their resolved symbols: `Effect.runPromise`, `runPromiseExit`, `runSync`, `runSyncExit`, `runFork`, `runCallback`, and their `*With` forms; `ManagedRuntime`'s `run*` methods; `NodeRuntime.runMain` and `BunRuntime.runMain`; and `@effect/vitest`'s `it.effect`, `it.live`, their `.each` and `.prop` forms, and `layer(…)`.

```ts es
// main.effeck
export function main() {
  const name = Effect.provide(() => welcome("42"), AppLive)
  Effect.log(`welcomed ${name}`)
}

NodeRuntime.runMain(() => main())
```

A TypeScript entry file works too, because `main()` returns an ordinary cold `Effect`:

```ts ts
// run.ts
import { NodeRuntime } from "@effect/platform-node"
import { main } from "./main.js"

NodeRuntime.runMain(main())
```

A module's top level is not inside any program. A call there may build values, such as `Layer.effect(…)`, `wire(…)`, `Config.Port(…)`, or `Schedule.exponential(…)`, but it may not run a program:

```ts es
// config.effeck
export const config = loadConfig()
```

```text
error EK1001: loadConfig() runs a program, but a module's top level is not inside one.
  config.effeck:2:23   export const config = loadConfig()
  Export a function instead, or provide the value as a service with a Layer.
```

A runner is called only where no fiber runs the code: at a module's top level, and in a plain callback that such a place passes to a TypeScript function, such as a `describe` body or a web framework's route handler. Inside code that a fiber runs, a runner would start a new root with an empty Context and no handlers, outside the caller's structured concurrency:

```ts es
function refreshAll(ids: ReadonlyArray<string>) {
  for (const id of ids) Effect.runFork(refresh(id))
}
```

```text
error EK1005: Effect.runFork starts a new root fiber, but refreshAll already runs on one.
  refresh.effeck:2:25   for (const id of ids) Effect.runFork(() => refresh(id))
  Call the program, or fork it so the block owns it:   fork(() => refresh(id))
```

Tests in `.effeck` use `it.effect`. A test that must `await` a Promise stays in a `.ts` file.

#### Evaluation order

Evaluation order is JavaScript's, and generated code keeps it. Arguments are evaluated left to right, each once. A receiver is evaluated once and keeps its `this`. `a() && b()` runs `b` only if `a()` is truthy. A getter runs once per access. In `double(w) * double(h) * Settings.scale()`, both `double` calls run before the service call, even though only the service call can pause (Chapter 15).
