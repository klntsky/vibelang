# Effeck

A language for writing Effect programs. Specification, 2026-10-16. Runtime: `effect@4.0.5`.

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

Effeck is TypeScript syntax in which every function is an Effect. You call functions normally, `throw` and `catch` typed errors, and ask for services by name. The compiler infers each function's `Effect<A, E, R>` and emits the Effect code you would have written by hand, so TypeScript code uses it like any other Effect library.

```ts effect
const welcome = Effect.fn("welcome")(function* (id: string) {
  const users = yield* Users
  const mailer = yield* Mailer
  const user = yield* users.find(id)
  yield* mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
})
```

```ts es
function welcome(id: string) {
  const user = Users.find(id)
  Mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
}
// string!UserNotFound | MailError!Users | Mailer
```

The comment under the Effeck version is not something you write. It is the contract the compiler infers, and what your editor shows when you hover over `welcome`.

The two functions do the same work, with two differences. `Effect.fn("welcome")` opens a tracing span on every call; Effeck adds a span only where you ask for one (Chapter 2). And the generated code looks each service up at the call that uses it, so a missing `Mailer` fails after `Users.find` has run instead of before.

## How to read this document

| Part | What it covers | Read it if |
| --- | --- | --- |
| [I. The language by example](#part-i-the-language-by-example) | Functions, errors, dependency injection, pipelines, resources, concurrency, producers, handlers, TypeScript interop, the standard library, and the questions Effect users ask, each shown next to the Effect code you write today | You use Effect and want to know what Effeck is like. Start here. |
| [II. The rules](#part-ii-the-rules) | The precise rules behind Part I: what a call means, how contracts are inferred and checked, which functions compile to plain JavaScript | You are reviewing the language design |
| [III. The runtime](#part-iii-the-runtime) | How Effeck runs on Effect's fiber runtime: one-shot delimited continuations, ownership, cancellation, and the tests that check them | You care how it executes, or you are reviewing the runtime |
| [IV. The compiler](#part-iv-the-compiler) | The fork of the TypeScript compiler, the pipeline, declarations, and editor support | You will build it |
| [V. Decisions](#part-v-decisions) | Every decision with its reason and evidence, the closed end-to-end build calls, and the experiments that confirm the implementation | You want to check or change a decision |
| [Appendices](#appendices) | The evidence index, prior art, and every Effect feature mapped to Effeck | You want to check a claim |

**Status.** This is the specification. Every rule in it is decided: the owner's decisions and the defaults made to complete the design are logged in `docs/design/DECISIONS.md` (D1-D60) and collected in [Part V](#29-decisions), which records the closed end-to-end build calls. D57 and D58 are controversial defaults. The runtime in Part III is a working prototype: 148 tests pass in `experiments/`. The compiler exists as two prototypes: `prototype/esc`, on TypeScript's API, compiles 46 example files written from this document, and the Go fork of TypeScript in `typescript/` lowers calls by type and infers contracts (Chapter 27). The compiler, spec, runtime, and tooling share one private repository (D60).

## Effect to Effeck at a glance

| In Effect today | In Effeck | Chapter |
| --- | --- | --- |
| `Effect.gen(function* () { … })`, `Effect.fn(…)` | an ordinary `function` | [2](#2-functions-and-calls) |
| `yield* effect` | just call it: `f(x)` | [2](#2-functions-and-calls) |
| `Effect.map`, `Effect.flatMap`, `Effect.andThen`, `Effect.tap` | plain statements: `const x = f(); g(x)` | [2](#2-functions-and-calls) |
| `Effect.fail(e)` or `return yield* new E()` | `throw new E()` | [3](#3-errors-the-e-channel) |
| `class E extends Data.TaggedError("E")<{ ... }> {}` | `error E { ... }` | [3](#3-errors-the-e-channel) |
| `Effect.catchTag("E", h)` | `try { … } catch (e) { … }` | [3](#3-errors-the-e-channel) |
| `Effect.ensuring(cleanup)` | `finally { … }` | [3](#3-errors-the-e-channel) |
| `class S extends Context.Service<…>()("key") {}` | `service S { method(): A!E }` | [4](#4-dependency-injection-the-r-channel) |
| `const s = yield* S` then `yield* s.method()` | `S.method()` | [4](#4-dependency-injection-the-r-channel) |
| `Layer.effect(S, Effect.gen(…))` | `Layer.effect(S, () => { … })` | [4](#4-dependency-injection-the-r-channel) |
| `program.pipe(Effect.provide(layer))` | `Effect.provide(() => program(), layer)` | [4](#4-dependency-injection-the-r-channel) |
| `effect.pipe(Effect.retry(…), Effect.timeout(…))` | `pipe(() => f(), Effect.retry(…), Effect.timeout(…))` | [5](#5-programs-as-values-retry-timeout-and-pipe) |
| `Effect.scoped` with `Effect.acquireRelease` | `using x = acquire()` | [6](#6-resources) |
| `Effect.all([a, b], { concurrency: 2 })` | `Effect.all([() => a(), () => b()], { concurrency: 2 })` | [7](#7-concurrency) |
| `Effect.forkScoped(effect)` | `fork(() => f())`, owned by the enclosing block | [7](#7-concurrency) |
| a `Stream` you build by hand | a `function*` producer and `for...of` | [8](#8-producers-streams-of-values) |
| nothing comparable | `control Op { ... }` and `handle` | [9](#9-handlers-defining-your-own-effects) |
| `node:fs`, `fetch`, `process.env`, `child_process` | `FileSystem`, `HttpClient`, `Config`, `ChildProcessSpawner` | [11](#11-the-standard-library) |

Where `async`, `Option`, `Schema`, `Stream`, testing, and the rest of Effect fit is answered in [Chapter 12](#12-questions-effect-users-ask) and, feature by feature, in [Appendix C](#c-every-effect-feature-in-effeck).

## Part I. The language by example

Each chapter shows something you do in Effect today and the same thing in Effeck. Effeck code uses Effect's own modules unchanged; what changes is that you stop writing generators, `yield*`, and most combinators.

Three conventions for the snippets. A snippet shown without an enclosing function sits inside one, because a module's top level cannot run a program (Chapter 13). Names such as `User`, `guest`, and `MailerTest` are declared nearby, and helpers such as `toUser` are Effeck functions. Imports of Effect modules outside the prelude (`Schedule`, `Stream`, `Fiber`, and so on) are left out.

### 1. A first look

Here is a small but complete program: two services, two typed errors, a function that uses them, and a `main` that runs it.

#### The program

```ts effect
import { Context, Data, Effect } from "effect"

class UserNotFound extends Data.TaggedError("UserNotFound")<{
  readonly id: string
}> {}

class MailError extends Data.TaggedError("MailError")<{
  readonly reason: string
}> {}

class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, UserNotFound>
}>()("app/Users") {}

class Mailer extends Context.Service<Mailer, {
  send(to: string, body: string): Effect.Effect<void, MailError>
}>()("app/Mailer") {}

export const welcome = Effect.fn("welcome")(function* (id: string) {
  const users = yield* Users
  const mailer = yield* Mailer
  const user = yield* users.find(id)
  yield* mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
})
```

```ts es
error UserNotFound {
  id: string
}

error MailError {
  reason: string
}

service Users {
  find(id: string): User!UserNotFound
}

service Mailer {
  send(to: string, body: string): void!MailError
}

export function welcome(id: string) {
  const user = Users.find(id)
  Mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
}
```

Four things changed:

- **`welcome` is a plain function.** No `Effect.fn`, no generator, no `yield*`. Calling `Users.find(id)` runs it and gives you the `User`; if it fails, `welcome` fails with the same error.
- **A service is a `service` declaration.** `service Users`, an abstract class, replaces the `Context.Service` class: no key string and no `Context` import. You call `Users.find(id)` directly instead of first getting the service with `yield* Users` (Chapter 4).
- **The contract is inferred.** `welcome` is `string!UserNotFound | MailError!Users | Mailer`: it succeeds with a `string`, can fail with either error, and needs both services. You write the contract only when you want the compiler to check it (Chapter 2).
- **Errors are the same classes.** `error UserNotFound { id: string }` declares a class that extends `TaggedError`, Effect's `Data.TaggedError`, in scope without an import.

#### What is in scope

Every `.effeck` file starts with a small prelude in scope, so the program above has no imports:

| Name | What it is |
| --- | --- |
| `Effect` | Effect's `Effect` module, and the type `Effect<A, E, R>` |
| `Schema`, `Layer` | Effect's modules, unchanged |
| `Scope` | Effect's `Scope` module, and the type `Scope`, which is Effect's `Scope.Scope` |
| `TaggedError` | Effect's `Data.TaggedError` |
| `Control` | The base class of control operations (Chapter 9) |
| `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, `abandon` | Effeck's built-ins, each introduced where it is first used |
| `sync`, `try`, `callback` | Boundary hatches: their arguments are plain TypeScript (D148) |
| `runPromise`, `runFork`, `runSync`, `runCallback` | Effect's runners, the way back in from TypeScript (D150) |
| `sleep`, `log`, `logDebug`, `logInfo`, `logWarning`, `logError`, `retry`, `repeat`, `timeout`, `all`, `forEach`, `race`, `withSpan`, `die` | The Effect module's functions without the `Effect.` prefix (D151) |
| Every other export of `effect`'s root | `Console`, `Option`, `Stream`, `Schedule`, `Duration`, and the rest, plus `flow` and `identity` (D146) |

Modules from `effect`'s subpath packages, such as `effect/http`, are imported as usual. An import or a local declaration with the same name shadows a prelude name, as it would shadow any global. The prelude is what most Effect files import anyway: `Effect` appears in 77-81% of the Effect files in two large codebases (Smithers, 2,149 files; opencode, 1,031), `Schema` in 39-44%, `Layer` in 34-44%, and no other module in more than 22%. `Scope` is there because contracts name it whenever a resource outlives a function (Chapter 6). JavaScript's own globals keep their names: `Array`, `String`, `Number`, `Boolean`, `Symbol`, `BigInt`, `Function`, and `RegExp` mean JavaScript's, so Effect's modules with those names need an import under another name, as in `import { Array as Arr } from "effect"`. A host global does not shadow Effect: in a project whose `lib` includes the DOM, `Option` is still Effect's.

#### Running it

```ts es
// main.effeck
import { AppLive } from "./app.effeck"
import { welcome } from "./welcome.effeck"

export function main() {
  const name = Effect.provide(() => welcome("42"), AppLive)
  log(`sent a welcome email to ${name}`)
}
```

`Effect.provide` supplies the services `welcome` needs from an application Layer (Chapter 4 builds `AppLive`). The `() =>` in `() => welcome("42")` is how you hand a program to Effect without running it yet (Chapter 5). The module exports `main` without parameters, so its JavaScript ends with `if (import.meta.main) NodeRuntime.runMain(main())`: `node main.js` hands `main` to Node's runner, the only place a program starts, and importing the module starts nothing (D135).

#### What the compiler produces

Effeck compiles to TypeScript. This is the output for `welcome`:

```ts gen
export const welcome = Effect.fnUntraced(function* (id: string) {
  const users = yield* Users
  const user = yield* users.find(id)
  const mailer = yield* Mailer
  yield* mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
})
```

```ts dts
export declare const welcome: (id: string) =>
  string!UserNotFound | MailError!Users | Mailer
```

TypeScript code that imports `welcome` sees an ordinary function returning an `Effect`. Each service class compiles to an interface for its methods and a `Context.Key` class (Chapter 4), so a TypeScript program can provide `Users` with an ordinary Layer, and Effeck can use services written in TypeScript.

### 2. Functions and calls

#### Every function is an Effect

There is one kind of function. Some can fail, some need services, some pause; the compiler works out which from the body, and records it in the function's contract `Effect<A, E, R>`.

```ts es
function add(a: number, b: number) {
  return a + b
}
// number!!

function greeting(id: string) {
  const user = Users.find(id)
  return `Hello, ${user.name}`
}
// string!UserNotFound!Users
```

`add` performs no Effect operation and cannot fail, so the compiler emits a plain JavaScript function for it: calling `add(1, 2)` costs what any JavaScript call costs (Chapter 15). `greeting` can fail and needs `Users`, so it compiles to an Effect.

#### Calling runs the function

A call runs the function and gives you its success value. If the callee fails, the caller fails with the same error, unless it catches it (Chapter 3). There is no `yield*` and no `await`.

```ts effect
const userName = Effect.fn("userName")(function* (id: string) {
  const users = yield* Users
  const user = yield* users.find(id)
  const name = user.name.trim()
  yield* Effect.log(`found ${name}`)
  return name
})
```

```ts es
function userName(id: string) {
  const name = Users.find(id).name.trim()
  log(`found ${name}`)
  return name
}
```

Where Effect code sequences with `Effect.map`, `flatMap`, `andThen`, or `tap`, Effeck uses the value and then the next statement.

#### Control flow is ordinary JavaScript

Loops, conditions, early returns, and local variables work as they do in any function, with effectful calls anywhere inside them.

```ts effect
const notifyAll = Effect.fn("notifyAll")(function* (ids: ReadonlyArray<string>) {
  const users = yield* Users
  let sent = 0
  for (const id of ids) {
    const user = yield* users.find(id)
    if (!user.subscribed) continue
    yield* welcome(id)
    sent++
  }
  return sent
})
```

```ts es
function notifyAll(ids: ReadonlyArray<string>) {
  let sent = 0
  for (const id of ids) {
    const user = Users.find(id)
    if (!user.subscribed) continue
    welcome(id)
    sent++
  }
  return sent
}
// number!UserNotFound | MailError!Users | Mailer
```

The loop runs one `welcome` at a time, in order. Running several at once is always explicit (Chapter 7).

#### Tracing

`Effect.fn("welcome")` gives every call a span. Effeck functions compile to untraced frames by default, because a traced frame costs 10 to 20 times more (Chapter 24). Add a span where you want one, with Effect's own `withSpan`:

```ts es
function welcomeTraced(id: string) {
  return withSpan(welcome(id), "welcome")
}
```

#### Writing the contract yourself

Most functions leave the contract to inference. Write it when you want the compiler to check a promise, typically on exported functions. The annotation is the whole computation, the way `Promise<string>` is on an `async` function: the `return` is checked against `string`.

```ts es
export function greeting(id: string): string!UserNotFound!Users {
  const user = Users.find(id)
  return `Hello, ${user.name}`
}
```

A contract may be wider than what the body does, never narrower:

```ts es
export function greeting(id: string): string!!Users {
  return `Hello, ${Users.find(id).name}`
}
```

```text
error EK2001: UserNotFound can escape greeting, but its contract says E = never.
  greeting.effeck:2:22   return `Hello, ${Users.find(id).name}`
                                      ^^^^^^^^^^^^^ Users.find can fail with UserNotFound
  Catch it inside greeting, or change the contract to Effect<string, UserNotFound, Users>.
```

To check selected channels, write `infer` followed by a name in each channel the compiler should infer (D57):

```ts es
function greeting(id: string): Effect<string, infer E, infer R> {
  return `Hello, ${Users.find(id).name}`
}
// string!UserNotFound!Users
```

Here A is checked, and E and R come from the body. `Effect<string, never, infer R>` instead promises no expected failures while inferring services. All three slots must be present: `Effect<string>` in a return annotation on a function with a body is EK2003, with fixes for explicit `never` or explicit inference ([Chapter 14](#14-contracts-and-inference)). The name after `infer` is documentation only; it introduces no type variable.

#### Calling TypeScript

Effeck calls TypeScript functions directly. What a call does depends on what the function returns:

```ts es
import { loadAvatar } from "./avatars.ts"    // (id: string) => Avatar!AvatarError!Storage
import { formatName } from "./format.ts"     // (user: User) => string, declared trusted

function profileCard(id: string) {
  const user = Users.find(id)
  return {
    title: formatName(user),                 // a trusted plain function: an ordinary call
    avatar: loadAvatar(user.id)              // returns an Effect, so the call runs it
  }
}
// { title: string; avatar: Avatar }!UserNotFound | AvatarError!Users | Storage
```

A TypeScript function that returns an `Effect` is treated exactly like an Effeck function: calling it runs it. A TypeScript function that returns anything else is foreign code, and nothing says what its exceptions mean, so the compiler asks you to say it once. `formatName` carries a trusted contract, a one-line tag on its declaration (Chapter 10). Without one, the call is an error that offers the fixes:

```text
error EK4401: formatName is a plain TypeScript function, and nothing says what its exceptions mean.
  profile-card.effeck:7:12   title: formatName(user),
  Declare it trusted once:  add /** @effeck trusted */ to formatName in format.ts
  Or bound this call:       try(formatName(user), (cause) => new FormatError({ cause }))
  Or move it into a .effeck file, where its contract is inferred.
```

Effect's own modules (`Option`, `Array`, `Schedule`, and the rest) and JavaScript's built-ins (`String`, `Math`, `JSON`, and the rest) need none of this: their contracts ship with the compiler. Node.js and browser APIs are replaced by Effect's modules (Chapter 11).

### 3. Errors: the E channel

#### Throwing a typed error

Errors are the same tagged error classes you use today. `error InvalidId { raw: string }` declares one. It is short for `class InvalidId extends TaggedError("InvalidId")<{ readonly raw: string }> {}`, where `TaggedError` is Effect's `Data.TaggedError`, in scope in every `.effeck` file. The tag is the class name, every field is readonly, and `error Timeout` declares an error without fields (D126). Errors that must be serialized use `Schema.TaggedError`. `throw` adds the error to the function's E.

```ts effect
class InvalidId extends Data.TaggedError("InvalidId")<{ readonly raw: string }> {}

const parseId = Effect.fn("parseId")(function* (raw: string) {
  if (!/^\d+$/.test(raw)) return yield* new InvalidId({ raw })
  return Number(raw)
})
```

```ts es
error InvalidId { raw: string }

function parseId(raw: string) {
  if (!/^\d+$/.test(raw)) throw new InvalidId({ raw })
  return Number(raw)
}
// number!InvalidId!
```

Errors from callees flow upward the same way. A function's E is everything it throws plus everything its callees can fail with, minus what it catches.

Any value can be thrown, and it joins E with its static type: `throw new Error("bad")` adds `Error`, and `throw "bad"` adds `string`, as `Effect.fail` would. Tagged error classes are the convention because `catch` can tell them apart with `instanceof` or `_tag`.

#### Catching

`try`/`catch` handles typed errors. The caught value is typed as exactly the errors the `try` block can produce.

```ts effect
const lookup = Effect.fn("lookup")(function* (raw: string) {
  const id = yield* parseId(raw)
  const users = yield* Users
  return yield* users.find(String(id)).pipe(
    Effect.catchTag("UserNotFound", () => Effect.succeed(guest))
  )
})
```

```ts es
function lookup(raw: string) {
  const id = parseId(raw)
  try {
    return Users.find(String(id))
  } catch (e) {                // e: UserNotFound
    return guest
  }
}
// User!InvalidId!Users
```

To handle some errors and pass the rest on, name them in the clause (D131). `catch (e: UserNotFound)` handles only failures with that type's `_tag`, and the others propagate with their types:

```ts es
function lookupOrGuest(raw: string) {
  try {
    return Users.find(String(parseId(raw)))
  } catch (e: UserNotFound) {
    return guest
  }
}
// User!InvalidId!Users
```

Several clauses can follow one `try`, each naming a type or a union of types, and a final untyped clause catches the rest. A failure raised inside a clause is not tested by the clauses after it. The test compares `_tag`, so it also matches tagged errors from other packages, such as `Schema.SchemaError`. A clause type without a literal `_tag` is EK2105, and a clause after an untyped one is EK2106. Narrowing and rethrowing inside an untyped clause still works, since TypeScript's narrowing applies to the caught value.

Effect's combinators still work when an expression reads better than a block. Pass the program as a function:

```ts es
const user = Effect.catchTag(Users.find(id), "UserNotFound", () => guest)
```

#### Promising that nothing escapes

A contract with `never` in E says every error is handled inside. The compiler checks it.

```ts es
export function safeLookup(raw: string): User!!Users {
  try {
    return lookupOrGuest(raw)
  } catch (e) {                // e: InvalidId
    return guest
  }
}
```

Remove the `try` and you get `EK2001: InvalidId can escape safeLookup` (the same error as in Chapter 2).

#### Cleanup with `finally`

`finally` runs when the block exits in any way: success, failure, or interruption.

```ts effect
const report = Effect.fn("report")(
  function* (id: string) {
    const reports = yield* Reports
    return yield* reports.build(id)
  },
  Effect.ensuring(Metrics.use((metrics) => metrics.increment("reports")))
)
```

```ts es
function report(id: string) {
  try {
    return Reports.build(id)
  } finally {
    Metrics.increment("reports")
  }
}
```

A cleanup that acquires and releases a resource is better written with `using` (Chapter 6).

#### Failures, defects, and interruption

Effect distinguishes three ways a computation can stop, and Effeck keeps them apart:

```ts es
function lookupAudited(id: string) {
  try {
    try {
      return Users.find(id)             // fails with UserNotFound
    } finally {
      Audit.flush()                     // and this crashes with a TypeError: a defect
    }
  } catch (e) {                         // does not run: the exit also carries a defect
    return guest
  }
}
```

`catch` handles an expected failure only when nothing else went wrong. Here the exit carries both the `UserNotFound` and the crash, so the `catch` block does not run and the whole failure propagates, crash included. Effect's own `Effect.catch` would recover from this exit and drop the crash.

| What happened | Example | In E? | Caught by `catch`? | Runs `finally`? |
| --- | --- | --- | --- | --- |
| Expected failure | `throw new InvalidId(…)`, a callee's E | Yes | Yes | Yes |
| Defect: a bug | `(undefined as any).name` throws a `TypeError` | No | No | Yes |
| Interruption | a timeout, a cancelled request, a parent shutting down | No | No | Yes |
| An expected failure plus a defect or an interruption | `lookupAudited` above | No: E describes causes made only of expected failures, so this exit can carry a failure outside E (Chapter 14) | No: the whole exit propagates | Yes |
| Several expected failures | both branches of `Effect.race` failed | Yes | Yes: `e` is the first | Yes |

A `catch` that caught bugs or cancellation would hide them, so it does not. To deal with defects deliberately, use Effect's tools at a boundary: `Effect.catchDefect(() => risky(), …)`. To declare that an expected failure cannot happen here, turn it into a defect: `const id = Effect.orDie(() => parseId(raw))`. To see every failure in an exit, use `Effect.catchCause` or `Effect.exit`. Chapter 17 states the full rules.

#### Coming from `Effect.gen`

Two things that look the same mean different things, on purpose.

**`throw` is `Effect.fail`.** Inside `Effect.gen`, `throw new Error("unreachable")` is a defect. In Effeck, every `throw` is a typed failure: `Error` joins E, and a `catch` can handle it. To signal a bug, die explicitly: `Effect.die(new Error("unreachable"))`.

**`catch` sees expected failures only.** Inside `Effect.gen`, a JavaScript `catch` sees any synchronous exception from the code in its `try`. In Effeck, an exception that no contract lists is a defect, and `catch` does not see it. That covers Effect's "cannot fail here" functions, such as `Option.getOrThrow` and `Schema.decodeUnknownSync`, whose exceptions are defects (Chapter 10). Each has a form for code that can fail: `Option.getOrElse`, `Effect.fromOption`, `Schema.decodeUnknownEffect`. A `catch` that therefore can never run is reported, as a warning by default:

```ts es
function firstAdmin(users: ReadonlyArray<User>) {
  try {
    return Option.fromNullishOr(users.find(isAdmin)).getOrThrow()
  } catch (e) {
    return guest
  }
}
```

```text
warning EK2004: this catch can never run: nothing in its try block fails with an expected error.
  admins.effeck:3:12   return Option.getOrThrow(Option.fromNullishOr(users.find(isAdmin)))
                          ^^^^^^^^^^^^^^^^^ its exceptions are defects
  Use a form that can fail:   return users.find(isAdmin) ?? guest
  or bound the call:          try(…, (cause) => new NoAdmin({ cause }))
```

### 4. Dependency injection: the R channel

Dependency injection is Effect's: services are `Context` keys, implementations are `Layer`s, and `Effect.provide` supplies them. Effeck removes the ceremony and adds compile-time checks of the dependency graph.

#### Declaring a service

```ts effect
class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, UserNotFound>
  rename(id: string, name: string): Effect.Effect<void, UserNotFound>
}>()("app/Users") {}
```

```ts es
service Users {
  find(id: string): User!UserNotFound
  rename(id: string, name: string): void!UserNotFound
}
```

`service Users { ... }` declares a service, and it does the job of the class on the left. It is short for `abstract class Users { ... }` in which every member without a body is abstract, and every value member is also readonly; static members, such as a layer, stay as written (D128). An abstract class whose instance members are all abstract is a service whichever way it is written. The rules, each checked by the compiler:

- **What makes a class a service.** In an `.effeck` file, a class is a service when it is `abstract`, has no `extends` clause and no type parameters, and every instance member is abstract: methods with full contracts, and `abstract readonly` values. Static members are allowed and stay on the class. An abstract class with a concrete instance member is an ordinary class.
- **Identity.** Effect identifies a service by its key string, and two services with the same string silently share a slot. The compiler derives the key from the package, module path, and class name (`@acme/app/src/users#Users`) and rejects a duplicate anywhere in the program (EK3001).
- **Each service is its own type.** TypeScript compares classes by shape, so two abstract classes with the same members would be one type: providing one would type-check as providing the other, and the program would die at run time with `Service not found`. The compiler gives every service class its own identity, as Effect's `Context.Service` classes have.
- **Contracts are written out.** A service has no body to infer from, so each method states its full `Effect<A, E, R>` (EK2101, Chapter 14).
- **A service is a key, not a base class.** `extends Users` (EK3002), `new Users()` (EK3003), `instanceof Users` (EK3004), and a generic service class (EK3005) are errors. Implement a service with an object literal or with a class that `implements Users`.

```text
error EK3002: a class cannot extend a service.
  users-memory.effeck:1:28   class UsersInMemory extends Users {
  Implement it instead:  class UsersInMemory implements Users { … }
```

What the class compiles to:

```ts gen
export interface Users {
  find(id: string): Effect.Effect<User, UserNotFound>
  rename(id: string, name: string): Effect.Effect<void, UserNotFound>
  readonly "~effeck/service"?: "@acme/app/src/users#Users"
}
export class Users extends serviceKey<Users>("@acme/app/src/users#Users") {}
```

TypeScript sees `Users` two ways. As a type, it is the interface, so `implements Users` and a parameter typed `db: Users` mean the methods. As a value, it is a `Context.Key`, so `Layer.succeed(Users, …)`, `Effect.provideService`, and `yield* Users` work as they do with a `Context.Service` class. The optional brand keeps two services with the same methods distinct in R. Effect's own class form cannot be the output: it makes the class's instance type the service identifier, so `implements Users` would fail with TS2720 (`prototype/esc/scratch/probe2.ts`).

The syntax takes nothing away from TypeScript. Calling an abstract member through its class (`Users.find(id)`) is error TS2339 today, and calling a class without `new` (`Users()`) is TS2348, so Effeck gives both a meaning. Fully abstract classes are also rare in ordinary code: Smithers has none in 4,461 files, and opencode has five in 3,235, whose only fully abstract one extends another class.

A `Context.Service` written in TypeScript, as a class or as a value like Effect's own `FileSystem`, is a service too, and Effeck uses it the same way.

#### Using a service

Call its methods. Using a service adds it to R.

```ts effect
const users = yield* Users
const user = yield* users.find(id)
```

```ts es
const user = Users.find(id)
```

Calling the service itself with no arguments, `Users()`, gives you its implementation object, the equivalent of `yield* Users`. You need it inside providers, and to read a service's value members:

```ts es
service Settings {
  baseUrl: string
  scale(): number!!
}

function apiUrl(path: string) {
  return `${Settings().baseUrl}/${path}`
}
// string!!Settings
```

Calling any member through the service looks the service up, so the service joins R whatever the member returns. `Path.join(a, b)`, from Effect's `Path` service, returns a string and still adds `Path`. A member that is itself an `Effect` value runs when called: `Terminal.readLine()`. A member the service declares wins over the key's own members, such as `pipe`, `use`, and `key`, so `Rpc.call(…)` reaches the service even though the key, a class, has a `call` of its own. A static member with the same name as a service member is EK3006.

#### Writing a provider

A provider is a `Layer`. Build one with Effect's own `Layer.effect`, passing a function that runs when the layer is built and returns the implementation. `layer UsersLive: Users { ... }` writes the same provider with less: the statements before the members run once, when the layer is built, the members form the implementation, and a value member is `readonly name = value` (D129). It is short for `Layer.effect(Users, () => { ...; return { ... } })`, or `Layer.succeed(Users, { ... })` when there are no statements. A statement after the members is EK2104.

```ts effect
const UsersLive = Layer.effect(Users, Effect.gen(function* () {
  const db = yield* Database
  return Users.of({
    find: Effect.fn("Users.find")(function* (id) {
      const rows = yield* db.query("select * from users where id = $1", [id]).pipe(Effect.orDie)
      if (rows.length === 0) return yield* new UserNotFound({ id })
      return toUser(rows[0])
    }),
    rename: Effect.fn("Users.rename")(function* (id, name) {
      yield* db.execute("update users set name = $1 where id = $2", [name, id]).pipe(Effect.orDie)
    })
  })
}))
// Layer<Users, never, Database>
```

```ts es
layer UsersLive: Users {
  const db = Database()              // runs once, when the layer is built
  find(id) {
    const rows = Effect.orDie(db.query("select * from users where id = $1", [id]))
    if (rows.length === 0) throw new UserNotFound({ id })
    return toUser(rows[0])
  }
  rename(id, name) {
    Effect.orDie(db.execute("update users set name = $1 where id = $2", [name, id]))
  }
}
// Layer<Users, never, Database>
```

`Database()` runs when the layer is built and captures the implementation in `db`. `Database` is therefore a requirement of *building* `UsersLive`, not of calling `Users.find`. Code that calls `Users.find` needs only `Users`.

The compiler checks each method against the service's contract. Without the `Effect.orDie`, the database's `DbError` would escape:

```text
error EK2001: DbError can escape UsersLive.find, but Users.find's contract allows only UserNotFound.
  users-live.effeck:5:20   const rows = db.query("select * from users where id = $1", [id])
                                    ^^^^^^^^ can fail with DbError
  Handle it, add DbError to Users.find's contract, or make it a defect with Effect.orDie.
```

Effect 4 code often keeps the default Layer on the service class as `static readonly layer`. A service class can do the same, because static members stay on the class:

```ts es
service Users {
  find(id: string): User!UserNotFound
  rename(id: string, name: string): void!UserNotFound

  static readonly layer = Layer.effect(Users, () => {
    const db = Database()
    return {
      find(id) { … },
      rename(id, name) { … }
    }
  })
}
```

An implementation can also be a class that `implements` the service. Its methods are Effeck methods, checked against the service's contracts:

```ts es
class UsersInMemory implements Users {
  readonly #rows = new Map<string, User>()

  find(id: string) {
    const user = this.#rows.get(id)
    if (user === undefined) throw new UserNotFound({ id })
    return user
  }

  rename(id: string, name: string) {
    this.#rows.set(id, { ...this.find(id), name })
  }
}

const UsersInMemoryLive = Layer.sync(Users, () => new UsersInMemory())
// Layer<Users, never, never>
```

#### Constructors and defaults

Effect 4's `make` option becomes a static method, and a service that should work without any provider declares a default:

```ts es
service Sessions {
  get(token: string): Session | undefined!!
  set(token: string, session: Session): void!!

  static make() {
    const entries = new Map<string, Session>()
    return {
      get: (token: string) => entries.get(token),
      set: (token: string, session: Session) => { entries.set(token, session) }
    }
  }

  static readonly layer = Layer.effect(Sessions, Sessions.make())
}

service FeatureFlags {
  newCheckout: boolean
  static readonly default: FeatureFlags = { newCheckout: false }
}

function checkoutPage(cartId: string) {
  return FeatureFlags().newCheckout ? newCheckout(cartId) : classicCheckout(cartId)
}
// FeatureFlags is not in R: it always has a value
```

A service with `static readonly default` compiles to Effect's `Context.Reference`. Code that uses it never needs a provider, and `Effect.provideService(() => …, FeatureFlags, { newCheckout: true })` overrides it for a region. Effect's own `Context.Reference("app/Flag", { defaultValue: () => false })` works unchanged too, because a call that returns a service key is data (Chapter 13).

#### Providers that hold resources

A provider can acquire a resource that lives as long as the layer, such as a connection pool. Acquire it without `using`:

```ts effect
const DatabaseLive = Layer.effect(Database, Effect.gen(function* () {
  const config = yield* DatabaseConfig
  const pool = yield* Postgres.connect(config.url)
  return Database.of({
    query: (sql, params) => pool.query(sql, params),
    execute: (sql, params) => pool.execute(sql, params)
  })
}))
// Layer<Database, ConnectError, DatabaseConfig>
```

```ts es
layer DatabaseLive: Database {
  const pool = Postgres.connect(DatabaseConfig().url)    // no `using`: the pool lives as long as the layer
  query(sql, params) { return pool.query(sql, params) }
  execute(sql, params) { return pool.execute(sql, params) }
}
// Layer<Database, ConnectError, DatabaseConfig>
```

`Postgres.connect` returns `Pool!ConnectError!Scope`. Without `using`, the pool belongs to whoever supplies the `Scope` (Chapter 6), and `Layer.effect` supplies the layer's own Scope, as it does in Effect today. The pool therefore closes when the program that provided `DatabaseLive` finishes. Writing `using pool = …` would close it as soon as the provider function returned, before any query ran.

#### Requirements that would leak

Sometimes a method looks up another service each time it is called, instead of capturing it when the layer is built. That lookup happens in the *caller's* context, so its requirement belongs to the caller, and the service contract has to say so. The compiler catches the mismatch:

```ts es
layer UsersAudited: Users {
  const db = Database()
  find(id) {
    Audit.record(`users.find ${id}`)    // looked up on every call
    return findIn(db, id)
  }
  rename(id, name) { renameIn(db, id, name) }
}
```

```text
error EK3101: UsersAudited.find looks up Audit each time it is called, but
  Users.find(id: string): Effect<User, UserNotFound, never>
  allows no call-time requirements.
  users-audited.effeck:5:7   Audit.record(`users.find ${id}`)
  Capture it once:      const audit = Audit()   …   audit.record(…)
  or declare it:        abstract find(id: string): Effect<User, UserNotFound, Audit>
```

Either fix is honest. Capturing makes `Audit` a requirement of building the layer. Declaring it makes every caller of `Users.find` provide `Audit`.

#### Composing an application

```ts effect
const AppLive = Layer.mergeAll(UsersLive, MailerLive).pipe(
  Layer.provideMerge(DatabaseLive),
  Layer.provideMerge(DatabaseConfigLive)
)
```

```ts es
const AppLive = wire(UsersLive, MailerLive, DatabaseLive, DatabaseConfigLive)
// Layer<Users | Mailer | Database | DatabaseConfig, ConfigError | ConnectError, never>
```

`wire` reads each provider's inputs and outputs from its `Layer<Out, E, In>` type, orders them, and generates the composition on the left. Its value is the errors. Today Effect reports a missing provider only as a leftover requirement where you run the program, silently picks one of two providers for the same service, and hangs on some cycles. `wire` reports all three at compile time:

```text
error EK3201: no provider for Smtp.
  welcome → Mailer.send → MailerLive needs Smtp
error EK3202: two providers for Users: UsersLive (src/users.effeck:12) and UsersTest (test/fakes.effeck:3). Pass one.
error EK3203: providers form a cycle: SessionsLive → Database → DatabaseLive → Sessions → SessionsLive
```

The contract is bounded by what Layer types say:

- Inputs that no argument provides stay in the result's `In`, as with `Layer.provide`, so you can wire part of a graph. EK3201 is reported where the program's requirements must be empty: at a runner, or at a contract whose R excludes them.
- An argument whose type does not name its outputs, such as a value typed `Layer<any>`, is EK3204: compose it with `Layer.provide`. A Layer chosen at run time with `Layer.unwrap` has a proper type and wires normally.
- `Layer.provide`, `Layer.merge`, and the rest keep working; `wire` is optional.

#### Running with providers

```ts es
function main() {
  const name = Effect.provide(() => welcome("42"), AppLive)
  log(`welcomed ${name}`)
}
// void!UserNotFound | MailError | ConfigError | ConnectError!
```

`main`'s contract shows what providing costs: `ConfigError` and `ConnectError` come from building the layers. The layers are built when `Effect.provide` runs, shared by everything inside it, and released in reverse order when it finishes. If building a layer fails, the program fails with that error, and anything already acquired is released.

`with AppLive { ... }` provides a layer to a block (D130). It is short for `Effect.provide(() => { ... }, AppLive)`, except that its statements are code of the enclosing function: a `return` returns from the function, and `break` and `continue` leave its loops. Its errors are the block's and the layer's, and its requirements are the block's that the layer does not provide plus the layer's own. Like `Effect.provide`, it accepts a layer, an array of layers, or a `Context`. A `with` block at a module's top level would run a program there, so it is EK1001.

```ts es
function main() {
  with AppLive {
    log(`welcomed ${welcome("42")}`)
  }
}
// void!UserNotFound | MailError | ConfigError | ConnectError!
```

Each `Effect.provide` builds its layers once, so two sibling `provide` calls build them twice:

```ts es
function counts() {
  const users = Effect.provide(() => countUsers(), AppLive)      // opens a database pool
  const orders = Effect.provide(() => countOrders(), AppLive)    // opens a second pool
  return users + orders
}
```

Provide the application layer once, at `main`. A program with several entry points, such as a web server whose framework calls each handler, builds one `ManagedRuntime` from the layer and runs every entry point through it (Chapter 12).

#### Testing with fake services

```ts effect
const UsersTest = Layer.succeed(Users, Users.of({
  find: (id) => Effect.succeed({ id, name: "Ada", email: "ada@example.com" }),
  rename: () => Effect.void
}))

it.effect("welcome returns the user's name", () =>
  Effect.gen(function* () {
    const name = yield* welcome("42").pipe(Effect.provide(Layer.mergeAll(UsersTest, MailerTest)))
    assert.strictEqual(name, "Ada")
  }))
```

```ts es
layer UsersTest: Users {
  find(id) { return ({ id, name: "Ada", email: "ada@example.com" }) }
  rename() {}
}

it.effect("welcome returns the user's name", () => {
  const name = Effect.provide(() => welcome("42"), wire(UsersTest, MailerTest))
  assert.strictEqual(name, "Ada")
})
```

To replace one service for part of a program, provide it around that part. The override applies only inside:

```ts es
function welcomeQuietly(id: string) {
  return Effect.provideService(welcome(id), Mailer, { send: () => {} })
}
```

#### What a function sees, and when

- A service call uses the providers in effect where the call runs. Code inside `welcomeQuietly` sees the quiet `Mailer`; code outside does not.
- After a pause (`Effect.sleep`, a network call), a function sees the same providers it saw before.
- A forked fiber starts with the providers of the code that forked it (Chapter 7).
- An implementation you captured, `const db = Database()`, stays that implementation even if `Database` is overridden later.

### 5. Programs as values: retry, timeout, and pipe

#### The one rule

A call runs where it stands, except where a parameter expects a program: there it is passed by name, and runs when the program runs. A function value does not run.

```ts es
syncInventory(store)                    // runs now
fork(syncInventory(store))              // passed by name: the forked fiber runs it

const job = () => syncInventory(store)  // a program: nothing has run
job()                                   // runs it
job()                                   // runs it again, from scratch
```

That is the whole story of programs as values. An `Effect` value you get from TypeScript works the same way: it is a program, and calling it runs it (`Clock.currentTimeMillis()`). A value whose type extends `Effect` with structure of its own, such as a `Config` or an `Exit`, is data until you call it (Chapter 13).

Where a parameter's type says it expects an `Effect`, the call itself is the program (D147): the parameter already says "not yet", so the source needs no marker. Elsewhere a program is written as a function with no arguments, because that needs no new syntax: `() =>` says "not yet". A program of several statements is always `() => { … }`. A quotation operator would be new syntax with the same meaning, and Effect's constructors (`Effect.suspend(() => …)`) are the boilerplate the language exists to remove.

Effect's combinators take programs, so you pass the call itself wherever Effect expects an Effect, as in Effect's own code. The compiler passes it by name.

```ts effect
const user = yield* fetchUser(id).pipe(Effect.retry(Schedule.exponential("100 millis")))
const fresh = yield* fetchUser(id).pipe(Effect.timeout("1 second"))
const fastest = yield* Effect.race(fromCache(id), fromNetwork(id))
```

```ts es
const user = retry(fetchUser(id), Schedule.exponential("100 millis"))
const fresh = timeout(fetchUser(id), "1 second")
const fastest = race(fromCache(id), fromNetwork(id))   // the first to succeed
```

Each line is a call, so it runs: `user` is a `User`, not an `Effect`. The inner calls are passed by name: `Effect.retry` runs `fetchUser(id)` again on each attempt, with fresh local variables each time.

#### The pipe operator

Stacking several combinators reads best as a pipeline. Effeck has TC39's proposed pipe operator, Hack style (D154): in `x |> f(%, a)`, the topic `%` stands for `x`.

```ts effect
const user = yield* fetchUser(id).pipe(
  Effect.retry({ times: 3 }),
  Effect.timeout("2 seconds"),
  Effect.withSpan("load-user")
)
```

```ts es
const user = fetchUser(id)
  |> retry(%, { times: 3 })
  |> timeout(%, "2 seconds")
  |> withSpan(%, "load-user")
// user: User;  E gains Cause.TimeoutError from Effect.timeout
```

The topic follows the one rule (D155). Each `%` above stands where a parameter expects a program, so the expression before its `|>` is passed by name: `retry` receives `fetchUser(id)` as a program and runs it again on each attempt, `timeout` receives the retried program, and `withSpan` the timed one. Where `%` stands for a value, the expression before `|>` runs once, before the body, and `%` is its result: `9 |> addMargin(%) |> double(%)` is `double(addMargin(9))`. A pipe does one or the other (EK1008), and its body must use `%` (EK1006). The types flow exactly as in Effect: `Effect.timeout` adds `Cause.TimeoutError` to E, and `Effect.retry` keeps E and R. The pipe is a call, so it runs; to keep it as a program, wrap it: `const loadUser = () => fetchUser(id) |> retry(%, { times: 3 })`.

The built-in `pipe` function still works: applied to plain data it is Effect's `pipe`, and with a function with no parameters first, that function is the program. It suits data-last functions that have no data-first form, such as `Match.when`.

Most pipes you write today disappear:

| You pipe today | In Effeck |
| --- | --- |
| `Effect.map`, `Effect.flatMap`, `Effect.andThen`, `Effect.tap` | statements |
| `Effect.catchTag`, `Effect.catch`, `Effect.mapError` | `try`/`catch` |
| `Effect.ensuring` | `finally` |
| `Effect.provide`, `Effect.provideService` | the same functions, with a program argument |
| `Effect.retry`, `Effect.timeout`, `Effect.withSpan`, `Effect.cached`, `Effect.uninterruptible` | the same functions, called or piped with `\|>` as above |

On a cause that also holds a defect, Effect's recovery operators and `try`/`catch` differ; Chapter 17 lists how.

Measured on two large Effect codebases, once statements replace sequencing, error handling, and cleanup, only 1.1% (Smithers) and 2.6% (opencode) of the Effect pipes outside tests keep two or more stages (`docs/research/pipe-census.md`). Pipelines over plain data stay plain: `items |> Arr.filter(%, isActive) |> Arr.map(%, toRow)` is `Arr.map(Arr.filter(items, isActive), toRow)`.

#### Methods on Effect's data types

A value of one of Effect's data types takes its module's functions as methods (D152). `x.name(args)` calls `Module.name(x, args)`:

```ts effect
const alerts = pipe(readings, Stream.filter((c) => c > 25), Stream.map((c) => `${c} C`), Stream.take(3))
const label = Option.getOrElse(Option.map(user.nickname, (n) => n.toUpperCase()), () => "anonymous")
```

```ts es
const alerts = readings.filter((c) => c > 25).map((c) => `${c} C`).take(3)
const label = user.nickname.map((n) => n.toUpperCase()).getOrElse(() => "anonymous")
```

The module is the one that declares the value's type: a `Stream` takes `Stream`'s functions, an `Option` takes `Option`'s. Eighteen modules qualify: `BigDecimal`, `Cause`, `Chunk`, `DateTime`, `Deferred`, `Duration`, `Effect`, `Exit`, `Fiber`, `HashMap`, `HashSet`, `Option`, `Queue`, `Ref`, `Result`, `Schedule`, `Sink`, and `Stream`. A type's own members come first, so an `Effect`'s `pipe` is its own, and arrays, strings, and objects keep JavaScript's methods: `[1, 2].map(f)` is `Array.prototype.map`. A method that returns an Effect is a call like any other, so it runs where it stands: `fiber.join()` waits for the fiber, and `counter.update((n) => n + 1)` updates the `Ref`. Chapter 13 states the rule.

The compiled JavaScript calls the module's function, `Stream.take(Stream.map(Stream.filter(readings, …), …), 3)`, so a bundler drops the functions a program never calls, as it does for Effect code.

#### Why `|>` passes by name

D8 kept the operator out of v0 because calls run: in `fetchUser(id) |> Effect.retry(%, policy)`, a topic that always held a value would run `fetchUser(id)` once, before `Effect.retry` saw it, and retry would have nothing to repeat. The by-name rule (D147) removes the conflict: where `%` stands for a program, the expression before `|>` is passed as one, exactly as `retry(fetchUser(id), policy)` passes it. The proposal itself is still at Stage 2 and its topic token is undecided (`docs/research/tc39-syntax.md` §1); Effeck uses `%`, the token in the proposal's README, and will follow the token TC39 chooses.

#### Storing, returning, and composing programs

Programs are values, so you can put them in variables, return them, and pass them around. Nothing runs until something calls them.

```ts es
function makeSync(store: Store) {
  return () => syncInventory(store)            // returns a program
}
// (() => SyncReport!SyncError!Inventory)!!

function syncEverything(stores: ReadonlyArray<Store>) {
  const syncAll = stores.map(makeSync)                    // an array of programs; nothing ran
  return all(syncAll, { concurrency: 4 })          // now they run
}
```

`stores.map(makeSync)` passes `makeSync` to JavaScript's `Array.prototype.map`, which expects a plain function. `makeSync` performs no Effect operation, so it compiles to plain JavaScript (Chapter 15), and `map` receives that form: `syncAll` holds programs. A function that can pause cannot be passed to `map` (EK4301, Chapter 10). Where `Effect.all` expects Effects, an array or record of programs converts element by element.

```ts es
function startServer() {
  const cachedConfig = Effect.cached(() => loadConfig())  // Effect.cached returns a program
  const config = cachedConfig()                           // loads once
  const again = cachedConfig()                            // returns the cached value
  return Http.listen(config.port)
}
```

### 6. Resources

#### `using`

`using` acquires a resource and releases it when the enclosing block exits: on success, on failure, and on interruption. It is standard JavaScript syntax (Explicit Resource Management reached Stage 4 on 2026-05-19, for ES2027) with Effect's guarantees added.

```ts effect
const readReport = Effect.fn("readReport")(
  function* (path: string) {
    const files = yield* Files
    const file = yield* files.open(path)
    const text = yield* file.readAll()
    return yield* parseReport(text)
  },
  Effect.scoped
)
```

```ts es
function readReport(path: string) {
  using file = Files.open(path)
  return parseReport(file.readAll())
}
// Report!IOError | ParseError!Files
```

`Files.open` returns a scoped resource: `FileHandle!IOError!Scope`. The block that declares `using` supplies the `Scope`, which is why `Scope` does not appear in `readReport`'s requirements. Several `using` declarations release in reverse order.

A call that runs an Effect is an acquisition even when its result has no `Symbol.dispose`; it receives the block Scope. Other initializers must be Disposable, `null`, or `undefined`. A Disposable result also runs `[Symbol.dispose]()` at block exit. If its Effect release already disposes it, this disposes it twice (D94).

In `for (using item of values)`, each iteration owns a Scope region, including `using` and `fork` inside its body (D95). C-style `for` heads still use TypeScript disposal and do not yet release on Effect failure or interruption; an Effect acquisition there reports TS2850.

#### Who owns a resource

Only `using` acquisitions belong to the block. A resource acquired without `using` belongs to the caller, even when the same block also has a `using`:

```ts es
function openAndLog(url: string) {
  const connection = Database.connect(url)      // no `using`: the caller owns it
  using log = Logs.open("connect.log")          // `using`: this block owns it
  log.write(`connected to ${url}`)
  return connection                             // still open when it reaches the caller
}
// Connection!ConnectError | IOError!Database | Logs | Scope
```

`log` closes when `openAndLog` returns. `connection` stays open, and `Scope` in the contract says the caller supplies the scope that will close it:

```ts es
function openLog(day: string) {
  return Files.open(`/var/log/app-${day}.log`)    // no `using`: the caller owns the file
}
// FileHandle!IOError!Files | Scope

function compareLogs(a: string, b: string) {
  using left = openLog(a)
  using right = openLog(b)
  return diffLines(left.readAll(), right.readAll())
}
// Diff!IOError!Files; both files close when compareLogs returns
```

#### Defining a resource

Write the acquire and release steps with Effect's `acquireRelease`:

```ts es
function lease(name: string) {
  return Effect.acquireRelease(
    () => Locks.acquire(name),        // acquire
    (lock) => Locks.release(lock)     // release: always runs, exactly once
  )
}
// Lock!LockError!Locks | Scope
```

The release is registered before anything else can happen, so a cancelled request never leaks a lock. By default the acquire step is also masked: an interruption that arrives while it runs waits for it to finish, and then the release runs. That suits a quick step such as taking a lock or opening a file. An acquisition that waits on the outside world, such as a connection that may never complete, passes `{ interruptible: true }` so that cancellation reaches the wait, as the adapter below does.

`using` adds no policy of its own: it supplies the block's Scope, and the resource's `acquireRelease` decides whether acquisition can be interrupted.

#### Resources and handlers

A resource held by code that a handler stops (Chapter 9) is released when it stops. If a handler decides not to continue your function, every `using` and `finally` between the handler and your code runs before the handler returns.

#### Adapting a callback API

Wrapping a callback-based API is boundary code. It belongs in TypeScript, using the runtime's `suspend`, which, unlike the `callback` hatch, tells the producer when a value arrived too late so the producer can release it:

```ts ts
// net-adapter.ts
import { Effect } from "effect"
import { suspend } from "effeck-runtime"
import * as net from "node:net"

export const connect = (host: string, port: number) =>
  Effect.acquireRelease(
    suspend<net.Socket>((waiter) => {
      const socket = net.connect(port, host)
      socket.once("connect", () => {
        if (!waiter.resume(socket)) socket.destroy()   // the waiter was cancelled first: still ours
      })
      return () => socket.destroy()                    // cancelled while connecting
    }, { release: (socket) => socket.destroy() }),     // accepted, but cancelled before delivery
    (socket) => Effect.sync(() => socket.destroy()),
    { interruptible: true }                            // cancellation reaches the wait
  )
```

```ts es
import { connect } from "./net-adapter.ts"

function session(host: string) {
  using socket = connect(host, 5432)
  return handshake(socket)
}

function sessionWithin(host: string) {
  return timeout(session(host), "2 seconds")    // cancels a connect that hangs
}
```

The two acquisition policies behave differently when cancellation arrives during the wait:

| Acquisition | Cancellation during the wait | Who releases a value that arrives late |
| --- | --- | --- |
| Masked (the default) | Waits for the value, registers its release, then releases it | Effect, through the registered release |
| `{ interruptible: true }` | Wins at once: the registration's cleanup runs and the fiber moves on | The producer, when `resume` returns `false`; the runtime, through `release`, for a value accepted but not yet delivered |

Without `{ interruptible: true }`, `sessionWithin` would wait for a connection that never completes, and the race branches in the adapter would never run (Chapter 21).

A callback API that hands over a plain value, not a resource, can be adapted in `.effeck` with the `callback` hatch (D148). Its register function is plain TypeScript, so it may call the SDK directly; `resume(value)` succeeds, `resume.fail(error)` fails, `resume.exit(exit)` adopts an `Exit`, and a function it returns runs on interruption:

```ts es
import { onceReady } from "./sdk.ts"     // (callback: (sdk: Sdk) => void) => void

function ready() {
  return callback<Sdk>((resume) => {
    onceReady((sdk) => resume(sdk))
  })
}
// Sdk!!
```

Only `suspend` reports a lost race, so an adapter for a value that needs releasing stays in TypeScript, as above.

### 7. Concurrency

Ordinary calls run one after another. Running things at the same time is always written with an explicit combinator, and the concurrent parts are programs, so they start only when the combinator starts them.

#### Running several things at once

```ts effect
const dashboard = Effect.fn("dashboard")(function* (id: string) {
  const [profile, orders] = yield* Effect.all([loadProfile(id), recentOrders(id)], {
    concurrency: 2
  })
  return { profile, orders }
})
```

```ts es
function dashboard(id: string) {
  const [profile, orders] = all([loadProfile(id), recentOrders(id)], {
    concurrency: 2
  })
  return { profile, orders }
}
```

```ts es
const thumbnails = forEach(images, (image) => resize(image), { concurrency: 8 })
const fastest = race(fromCache(id), fromNetwork(id))
const first = Effect.raceFirst(fromCache(id), fromNetwork(id))
const quick = timeout(slowQuery(id), "500 millis")
```

Each combinator has its own rule for a failing branch, and Effeck adds nothing to Effect's semantics here:

| Combinator | Succeeds with | When a branch fails |
| --- | --- | --- |
| `Effect.all`, `Effect.forEach` | Every result, in order | Interrupts the other branches, waits for their cleanup, and fails with that failure |
| `Effect.race` | The first success | Keeps waiting for the other branch; fails only if both fail, with both failures |
| `Effect.raceFirst` | Whichever branch finishes first, success or failure | Fails if the first branch to finish failed; the other is interrupted |
| `Effect.timeout` | The program's result | At the deadline, interrupts the program and fails with `Cause.TimeoutError` |

A `catch` around any of these sees the failure as usual. When `Effect.race` fails because both branches failed, `catch` receives the first failure (Chapter 3).

#### Background work with a lifetime

`fork` starts a program on a new fiber owned by the enclosing block. When the block exits, by any path, the fiber is interrupted, and the block waits for its cleanup before releasing anything acquired earlier in the block. A function body is a block, so `fork` needs no `using` nearby.

```ts effect
const serve = Effect.fn("serve")(
  function* (port: number) {
    const http = yield* Http
    const server = yield* http.listen(port)
    yield* Effect.forkScoped(heartbeat())
    return yield* server.awaitShutdown()
  },
  Effect.scoped
)
```

```ts es
function serve(port: number) {
  using server = Http.listen(port)
  fork(heartbeat())          // interrupted when serve exits, before the server closes
  return server.awaitShutdown()
}
```

`fork` returns a `Fiber`, which you can join or interrupt with Effect's `Fiber` module:

```ts es
function withProgress(id: string) {
  const pending = fork(buildReport(id))
  sendProgressEmail(id)
  return pending.join()          // waits for it; fails if it failed
}
// Report!ReportError | MailError!Reports | Mailer
```

`fork` adds the child's requirements to R and nothing else: the fiber cannot outlive `withProgress`, so no `Scope` appears in the contract, and its failures reach you only through `Fiber.join`. For a longer lifetime, say so with Effect's combinators: `Effect.forkScoped` ties the fiber to the caller's scope and adds `Scope` to R, `Effect.forkChild` ties it to the current fiber, and `Effect.forkDetach` ties it to nothing.

An expression-bodied arrow has no block to own its child: `() => fork(p)` uses `Effect.forkScoped` and keeps `Scope` in R, unlike `() => { return fork(p) }` (D96). Returning a block-owned child currently returns a fiber already interrupted when that block exited; the compiler does not warn.

#### What a new fiber sees

A forked fiber starts with the services of the code that forked it, as in Effect. Later changes in either fiber do not flow to the other. Handlers, introduced in Chapter 9, are the one thing that does not cross to a new fiber; Chapter 9 shows the compiler's check.

### 8. Producers: streams of values

A `function*` in Effeck is a **producer**: a function that yields values one at a time and can do anything an Effeck function does between yields, including calling services and waiting. `for...of` consumes it. The comparison below builds the same reusable, lazy line source over the same `Files` service on both sides.

```ts effect
const lines = (path: string) =>
  Stream.unwrap(Effect.gen(function* () {
    const files = yield* Files
    const file = yield* files.open(path)                // closes when the stream ends
    return Stream.fromEffectRepeat(file.readLine()).pipe(
      Stream.takeWhile((line): line is string => line !== undefined)
    )
  }))

const firstErrors = Effect.fn("firstErrors")(function* (path: string) {
  return yield* lines(path).pipe(
    Stream.filter((line) => line.startsWith("ERROR")),
    Stream.take(3),
    Stream.runCollect
  )
})
```

```ts es
function* lines(path: string) {
  using file = Files.open(path)
  while (true) {
    const line = file.readLine()        // a service call: may wait on I/O
    if (line === undefined) return
    yield line
  }
}

function firstErrors(path: string) {
  const found: string[] = []
  for (const line of lines(path)) {
    if (line.startsWith("ERROR")) found.push(line)
    if (found.length === 3) break       // the file closes here
  }
  return found
}
// string[]!IOError!Files
```

How a producer behaves:

- **Lazy.** Calling `lines(path)` runs none of its body. The file opens when the loop asks for the first line.
- **One value at a time.** Each `yield` hands one value to the loop body. When the body finishes, the producer continues to the next `yield`.
- **Cleanup on early exit.** `break`, `return`, or an error in the loop body stops the producer, and its `using` and `finally` run before the loop exits. Here the file is closed after the third error line.
- **One pass per run.** Iterating the same producer again starts it again from the beginning.
- **Constant stack.** A loop over a million elements uses the same stack depth as a loop over three (Chapter 26).

A JavaScript generator could not do this: its body runs synchronously inside `next()`, so it cannot wait for a service between values. An Effeck producer can, because each `yield` is a control operation that the loop handles (Chapter 9 shows the machinery, and Part III how it runs). For the same reason, `Effect.gen`, `Option.gen`, and other generator-based APIs cannot be written in `.effeck` files: there, `function*` declares a producer (Chapter 10).

#### What `for...of` does depends on what it iterates

| Iterated value | What `for...of` does |
| --- | --- |
| A producer | Runs it under a `Yield` handler: the loop body runs once per `yield` |
| An Effect `Stream` | Pulls the stream chunk by chunk; leaving the loop closes the stream |
| Any other iterable: an array, a `Map`, a `Set`, a string | JavaScript's loop, unchanged |

The choice is made from the static type, like every other call rule (Chapter 13).

#### Producers and Effect streams

To TypeScript, a producer is a `Stream`: the declaration file types `lines` as `(path: string) => Stream.Stream<string, IOError, Files>`. Inside Effeck, a producer converts to a `Stream` wherever a `Stream` is expected, so Stream's operators apply directly:

```ts es
function errorCount(path: string) {
  return lines(path).filter((line) => line.startsWith("ERROR")).runCount()
}
// number!IOError!Files
```

The conversion runs the producer on a fiber of its own and hands over each element only when the stream pulls it, through a handoff with no buffer, so a pull from any fiber works and the producer never runs ahead of its consumer. Closing the stream interrupts that fiber, which runs the producer's cleanup.

The other direction needs no adapter. Stream's own functions take callbacks that return Effects, so Effeck callbacks work there, and `for...of` iterates a `Stream` directly:

```ts es
function auditAll(events: Stream.Stream<Event, EventError>) {
  for (const event of events) Audit.record(event)
}
// void!EventError!Audit
```

### 9. Handlers: defining your own effects

#### What a service cannot do

A service call always comes back to the caller exactly once, with a value or an error. That covers most dependencies. It does not cover cases where whoever handles the request should decide *whether and when the caller continues*:

- pause a checkout until a person approves it, and let the approver's code decide what happens next;
- stop a search as soon as one result is found, cleaning up everything the search had opened;
- let a test script the answers to a sequence of requests, one at a time;
- run a producer one value at a time (Chapter 8).

A **control operation** is a request whose handler receives the rest of the caller's computation and decides what to do with it.

#### Declaring and performing an operation

A control operation is declared like a service, with `control`, which is short for an abstract class that extends the global `Control` (D128):

```ts es
control Approval {
  approve(request: string): boolean!ReviewBusy
}

function checkout(cartId: string) {
  using hold = Inventory.reserve(cartId)
  const ok = Approval.approve(`charge cart ${cartId}`)
  if (!ok) throw new Declined({ cartId })
  return Payments.charge(cartId)
}
// Receipt!ReviewBusy | Declined | PaymentError!Approval | Inventory | Payments
```

To the code that performs it, `Approval.approve` looks like any call: it returns a `boolean` or fails with `ReviewBusy`. Like a service, `Approval` appears in R until something handles it. The service rules apply: its members are abstract with full contracts, its key is derived from its module, and it cannot be extended or instantiated.

#### Handling it

A `handle` clause after a try block answers an operation that the block performs. It names the operation and its parameters, which take the operation's parameter types. `resume(value)` continues the block from the call with that value.

```ts es
function reviewedCheckout(cartId: string) {
  try {
    return checkout(cartId)
  } handle Approval.approve(request) {
    const decision = Reviewers.decide(request)    // may take minutes: the fiber just waits
    resume(decision)                              // checkout continues from its call
  }
}
// Receipt!ReviewBusy | Declined | PaymentError!Inventory | Payments | Reviewers
```

While the reviewer decides, the cart's hold is still reserved and `checkout` is paused at its call, exactly as it would be during any slow service call. A clause that ends by resuming, like this one, costs no stack: the compiler ends the clause before it continues the program (Chapter 26), so a handler that resumes a million times runs in constant space.

The try block and the clauses are code of the enclosing function (D132). A `return` in either returns from the function, and `break` and `continue` leave its loops. A clause can work after `resume`: `resume` returns when the try block finishes, and when the clause then ends, the statement ends as the block did, so the block's `return` takes effect after the clause, as a pending `return` waits for a `finally` block. A statement answers every operation of each control it names (EK2108), and catch clauses and a `finally` block may follow its handle clauses, covering the block and the clauses.

The statement is short for `handle`, a function that runs a program and returns its result. Each clause receives `k`, the rest of the program from the point of the call, and `resume(k, value)` continues it:

```ts es
const receipt = handle(() => checkout("c-42"), Approval, {
  approve(request, k) {
    const decision = Reviewers.decide(request)
    return resume(k, decision)
  }
})
// receipt: Receipt
```

`resume` returns what the rest of `checkout` returned, here the `Receipt`.

#### Not continuing

A handler that returns without calling `resume` ends the handled program with the handler's result. Everything the program had open is cleaned up first.

```ts es
control Found {
  found(cell: Cell): never!!
}

function locate(grid: Grid, target: number) {
  try {
    for (const row of grid.rows) {
      using cursor = grid.open(row)
      for (const cell of cursor.cells) if (cell.value === target) Found.found(cell)
    }
    return undefined
  } handle Found.found(cell) {
    return cell      // stop: the open cursor is released before locate returns
  }
}
// Cell | undefined!!
```

`found`'s result type is `never`, so its handler cannot continue the search, and the type says so.

#### Continuing with an error

`resumeWith` continues the program by running code at the call, where the program may catch what it throws:

```ts es
function patientCheckout(cartId: string) {
  try {
    return checkout(cartId)
  } handle Approval.approve(request) {
    if (Reviewers.busy()) resumeWith(() => { throw new ReviewBusy({ request }) })
    else resume(Reviewers.decide(request))
  }
}
```

The code passed to `resumeWith` runs inside the handled program, at the call, so two rules keep its contract honest. It may throw only the operation's declared errors (`ReviewBusy` here), which every caller of `Approval.approve` already sees in E. And it may need no service: the providers around the clause are not in scope at the call, so its R must be `never`.

```text
error EK2005: resumeWith runs this code at the perform site, inside the handled program,
  where the clause's providers are not in scope. It needs Secret.
  approvals.effeck:4:16   resumeWith(() => Secret.read())
  Read it in the clause first:   const secret = Secret.read()   …   resumeWith(() => secret)
```

#### Testing with a handler

```ts es
it.effect("checkout charges when approved", () => {
  try {
    assert.isTrue(checkout("c-42").paid)
  } handle Approval.approve(_request) {
    resume(true)
  }
})
```

#### The full API

`try { … } handle Control.operation(parameters) { … }` handles the operations of the controls its clauses name, the first control's handler innermost. In a clause, `resume(value)`, `resumeWith(() => …)`, and `abandon()` act on the clause's continuation; `resume()` continues an operation whose result is `void`. The compiler writes the statement as a call of `handle` whose program and clauses return the statement's completion: a `return`, `break`, or `continue`, or normal completion (Chapter 26).

`handle(program, Operations, clauses, onReturn?)` runs `program` with a handler for `Operations`. The optional `onReturn` transforms the program's own result before `handle` returns it. Inside a clause, `k` has the type `Continuation<V, E, H>`. `V` is what the call receives, `E` is the operation's declared errors, which only `resumeWith` may raise, and `H` is the result type of the whole `handle`. `resume(k, v)` returns an `H`: the handler is deep, so the resumed program runs until it reaches the handler again, where `onReturn`, or a later clause that does not resume, produces the value. The compiler computes `H` from `onReturn`'s result (the program's own result without one) and every clause's return type. Three operations take `k`:

| Call | Meaning |
| --- | --- |
| `resume(k, value)` | Continue the program with `value` as the operation's result. Returns an `H`: what the `handle` produces when the program next reaches it |
| `resumeWith(k, () => …)` | Continue the program by running the given code at the call. It may throw only the operation's declared errors, and its R is `never` |
| `abandon(k)` | Stop the program now: its cleanup runs, its `catch` blocks do not. Fails if cleanup fails |

In a handle statement's clause, the same three calls leave out `k`: `resume(value)`, `resumeWith(() => …)`, and `abandon()`.

The standard library uses a fourth operation, `transfer`, which hands a continuation to a scope so it can be resumed after the clause returns. Producers' external iteration needs it. v0 keeps it inside the standard library (Chapter 21).

#### The rules

- **Continue at most once.** Calling `resume(k, …)` a second time is an error at the second call; nothing in the program runs twice.
- **Not continuing cleans up.** If the handler returns, fails, or is interrupted without resuming, the program's `using` and `finally` blocks run before `handle` returns.
- **`k` stays inside the clause.** You cannot store `k`, return it, or capture it in a closure that outlives the clause. A handle statement's clause never names its continuation: it calls `resume`, `resumeWith`, and `abandon` in its own code, not in a function nested in it.
- **Handlers belong to a fiber.** A program forked onto another fiber cannot reach a handler on this one; the next section shows the check.
- **Cleanup code handles its own operations.** A `finally` block or `using` release that performs a control operation must handle it inside itself, so a handler elsewhere cannot cut cleanup short.

#### Handlers and concurrency

A handler belongs to the fiber it was installed on, so a program that runs on other fibers must handle its own control operations. The compiler checks every Effect combinator that runs its argument elsewhere (Chapter 16):

```ts es
function approveAll(ids: ReadonlyArray<string>) {
  return forEach(ids, (id) => Approval.approve(id), { concurrency: 4 })
}
```

```text
error EK4201: Approval.approve is a control operation, and forEach with concurrency 4 runs
  its callback on other fibers, where the Approval handler on this fiber cannot be reached.
  Handle Approval inside the callback, or run the loop with concurrency 1.
```

The fix puts a handler inside each concurrent branch:

```ts es
function approveAll(ids: ReadonlyArray<string>) {
  return forEach(ids, (id) => {
    try {
      return Approval.approve(id)
    } handle Approval.approve(request) {
      resume(Reviewers.decide(request))
    }
  }, { concurrency: 4 })
}
```

TypeScript code is checked the same way, from its signature. A TypeScript function can fork internally without taking a program argument, where no table can see it, so a TypeScript function whose R contains a control operation cannot be called from Effeck in v0:

```text
error EK4202: retryApproval is a TypeScript function whose requirements include the control
  operation Approval. TypeScript code can perform Approval.approve on another fiber, where
  this handler cannot reach it.
  Write retryApproval in Effeck, where the compiler sees every fork, or handle Approval inside it.
```

Effeck functions in other packages are not affected: their declarations carry the compiler's metadata, and the compiler checked their bodies when it built them.

#### In Effect today

There is no equivalent. A service method can wait and return, but it cannot end its caller with a value, and it cannot hand the caller's remaining work to someone else to continue later. Code that needs those things today restructures itself around queues, `Deferred`s, and extra fibers. Part III explains how Effeck provides this on Effect's runtime, and why it is the foundation everything else in this part is built on.

### 10. Working with TypeScript

Effeck uses the TypeScript compiler and shares its types, modules, and Effect. A project can mix `.ts` and `.effeck` files freely: existing Effect code stays in TypeScript, and new code can be written in either.

#### Using TypeScript Effect code from Effeck

A TypeScript function that returns an `Effect` is called like any Effeck function. The call runs it, and its A, E, and R join the caller's contract unchanged.

```ts ts
// users-repo.ts
export const findUser = Effect.fn("findUser")(function* (id: string) {
  const db = yield* Database
  const rows = yield* db.query("select * from users where id = $1", [id])
  if (rows.length === 0) return yield* new UserNotFound({ id })
  return toUser(rows[0])
})
// (id: string) => Effect<User, UserNotFound | DbError, Database>
```

```ts es
// page.effeck
import { findUser } from "./users-repo.ts"

export function title(id: string) {
  return findUser(id).name.toUpperCase()
}
// string!UserNotFound | DbError!Database
```

Generic, overloaded, and aliased TypeScript functions keep their signatures. Services written in TypeScript as `Context.Service` classes or values are used exactly like abstract service classes: `Database.query(…)` works on either. A TypeScript function whose R contains a control operation is the one exception (EK4202, Chapter 9).

#### Generator-style code stays in TypeScript

Existing `Effect.gen` and `Effect.fn` code keeps working in `.ts` files, and Effeck calls it as above. It cannot be pasted into a `.effeck` file unchanged, because there `function*` declares a producer (Chapter 8), not a JavaScript generator:

```ts es
const findName = (id: string) => Effect.gen(function* () {
  const users = yield* Users
  const user = yield* users.find(id)
  return user.name
})
```

```text
error EK1002: Effect.gen expects a JavaScript generator, but in Effeck function* declares a producer.
  Write the body as an ordinary function: remove Effect.gen, function*, and yield*.
    function findName(id: string) { return Users.find(id).name }
```

The same holds for `Option.gen`, `Result.gen`, and any other API that takes a generator. `async` functions and `await` are not part of Effeck either: a call already waits. Converting a function is mostly deletion, as every chapter of Part I shows, and the editor refactor remains a design target (Chapter 28).

#### Using Effeck code from TypeScript

To TypeScript, an exported unannotated or Effect-contract function returns an `Effect`, and every exported producer is a function that returns a `Stream`. The compiler writes that into the declaration file:

```ts dts
// page.d.ts
export declare const title: (id: string) => Effect.Effect<string, UserNotFound | DbError, Database>
```

```ts ts
// server.ts
import { Effect } from "effect"
import { title } from "./page.js"

const program = title("42").pipe(Effect.provide(DatabaseLive))   // nothing has run yet
```

Calling an Effeck function from TypeScript builds a program and runs nothing, like calling an `Effect.fn` function. That holds even for functions that compile to plain JavaScript (Chapter 15): TypeScript sees an unannotated `add` as `(a: number, b: number) => Effect<number>`, and Effeck callers in the same compilation use the fast path. An eligible helper with a plain annotation, such as `: number`, instead keeps a plain JavaScript signature and runs when TypeScript calls it (D67). TypeScript imports `./page.js`, which the compiler resolves to `page.effeck` the way it resolves `.js` to `.ts`.

#### Readonly members

An interface or an object type in an `.effeck` file has readonly members unless a member says `mutable` (D134):

```ts es
interface Cart {
  id: string
  mutable items: Array<string>
}
// interface Cart { readonly id: string; items: Array<string> }
```

The declaration file writes `readonly`, so TypeScript code sees the same types. Readonly limits writes through the type, not the object: TypeScript assigns a mutable object to a readonly type, so TypeScript code passes its objects in unchanged. Index signatures follow the same rule. Class members keep TypeScript's rule, and arrays keep theirs: write `ReadonlyArray` or `readonly T[]` for an array the code must not change.

#### Code that is not Effect

A function that does not return an `Effect` is foreign code. Nothing in its type says whether it throws, what it reads, or whether it touches the outside world, so each kind of callee has a rule:

```ts es
import { slugify } from "./slug.ts"          // carries /** @effeck trusted */

function publish(draft: Draft) {
  const meta = JSON.parse(draft.frontMatter)  // a built-in: SyntaxError joins E
  const slug = slugify(meta.title)            // trusted: its @throws adds InvalidSlug
  const words = draft.body.split(/\s+/).length
  return Articles.save({ slug, words, ...meta })
}
// Article!SyntaxError | InvalidSlug | SaveError!Articles
```

| Callee | Rule | Diagnostic |
| --- | --- | --- |
| A function returning `Effect`, from Effeck or TypeScript | Calls run it | |
| A JavaScript built-in: `String`, `Array`, `Math`, `JSON`, `Number`, `Object`, `Map`, `Set`, `RegExp`, deterministic `Date` parsing | Ships with a trusted contract: a control class, and which exceptions are expected failures (`JSON.parse` fails with `SyntaxError`) | |
| An export of `effect` or an `@effect/*` package that does not return an `Effect`, and the functions and values it returns, such as a `Brand` constructor or the function `Match.exhaustive` returns | Trusted: `sync`, and its exceptions are defects, as inside `Effect.gen` today | |
| A TypeScript function tagged `/** @effeck trusted */` | Trusted: `sync`; each `@throws {T}` names an expected failure; any other exception is a defect | |
| A platform API: `node:*` modules and browser globals such as `fetch`, `setTimeout`, `process`, `console`, `localStorage` | Not callable from `.effeck` files; the error names the Effect module to use (Chapter 11) | EK4402 |
| A clock or randomness read: `Date.now()`, `new Date()`, `Math.random()`, `crypto.randomUUID()` | Use `Clock`, `DateTime`, and `Random`, so tests control them | EK4403 |
| Any other plain TypeScript or JavaScript function | Needs an explicit boundary or a trusted contract | EK4401 |

A trusted contract is declared once, on the TypeScript declaration:

```ts ts
// slug.ts
/**
 * @effeck trusted
 * @throws {InvalidSlug} when the title has no letters or digits
 */
export function slugify(title: string): string { … }
```

The compiler trusts the tag; it cannot check a TypeScript body. `@throws {InvalidSlug}` makes `InvalidSlug` an expected failure that joins the caller's E; any other exception from `slugify` is a defect. Each `@throws` names a class exported by a module the `.effeck` caller imports; otherwise the call is EK4401 (D101). The generated code tests `instanceof InvalidSlug`. The tag cannot be put on a function that returns a Promise or an `Effect`. To trust a function from a package you cannot edit, re-export it from a `.ts` file you own, with the tag.

EK4401 applies where a fiber runs the code. Module initialization and its plain callbacks passed to TypeScript, such as `describe` bodies and route handlers, run ordinary JavaScript calls (D100). An exception at module top level fails the import. Trusted failures stay defects inside plain callbacks, while they make free lambdas and named Effeck functions use the Effect form (D101).

Only built-ins that parse outside text declare expected failures: `JSON.parse` declares `SyntaxError`, and `decodeURI` and `decodeURIComponent` declare `URIError`. Other built-in exceptions are defects (D101).

An explicit boundary says what an exception means at the call. The boundary hatches are `sync`, `try`, and `callback` (D148):

```ts es
function parseLegacy(text: string) {
  return try(legacyParse(text), (cause) => new LegacyFormatError({ cause }))   // plain TypeScript inside
}
// LegacyDoc!LegacyFormatError!
```

`try` turns an exception or a rejection into a typed error: `UnknownError`, or what its second argument makes of it. `sync` makes them defects. `callback` adapts a callback API (Chapter 6). Each takes either a function, `try(() => { … })` or `try(async (signal) => …)`, or a single expression evaluated when the Effect runs, `try(legacyParse(text))`. A hatch's argument is plain TypeScript, not Effeck: calls there do not run, so calling an `.effeck` function gives an inert Effect value for `runPromise` or `runFork`; `throw`, `async`, and `await` keep their JavaScript meaning; platform APIs and clock reads are allowed (no EK4401, EK4402, or EK4403); and `return` returns from the hatch's function. Effect's `{ try, catch }` options object is EK4404: the hatch takes the code and the error mapping as two arguments.

`runPromise`, `runFork`, `runSync`, and `runCallback` are in the prelude as the way back in. They are called from plain TypeScript: a `.ts` file, a hatch's body, or a module's top level, never from code a fiber runs (EK1005).

An exception that no contract lists is a defect, and an Effeck `catch` does not see it, unlike a JavaScript `catch` inside `Effect.gen` (Chapter 3). That includes the exceptions of Effect's "cannot fail here" functions, such as `Option.getOrThrow` and `Schema.decodeUnknownSync`, which `effect`'s trusted contracts classify as defects.

What a boundary cannot do is set by JavaScript, not by Effeck. It cannot discover a dependency the function reaches through a global or a module variable, it cannot cancel synchronous JavaScript once it has started, and it cannot let a synchronous callback pause. Effect code today has the same limits.

This is a policy change from the original direction, which adapted every plain call automatically, with unknown errors and unknown requirements. The premise is kept: foreign code is untrusted until something vouches for it. What changes is where the vouching happens: once, explicitly, at a boundary or on a declaration, so an unprovidable `unknown` never enters a requirement and an `unknown` error never spreads to every caller. Pure helpers written in `.effeck` get inferred contracts and compile to plain JavaScript (Chapter 15), so most project code never needs a boundary at all.

#### Promises

Effeck code has no `await` outside a hatch: a call already waits for the callee. A Promise from a TypeScript library is waited on with the `try` hatch, which says what a rejection means and passes an `AbortSignal` so interruption can cancel the request:

```ts es
import { createCharge } from "./payments-sdk.ts"   // (amount: number, signal: AbortSignal) => Promise<Charge>

function charge(cartId: string, amount: number) {
  return try((signal) => createCharge(amount, signal), (cause) => new ChargeFailed({ cartId, cause }))
}
// Charge!ChargeFailed!
```

HTTP itself goes through Effect's `HttpClient`, not `fetch` (Chapter 11).

#### Callbacks into plain JavaScript

A JavaScript function such as `Array.prototype.map` calls its callback synchronously, so the callback cannot pause. Callbacks that only compute are fine, and so is an Effeck function passed by name if it compiles to plain JavaScript. A callback that calls a service is rejected, with the fix in the message:

```ts es
const trimmed = ids.map((id) => id.trim())         // fine
const users = ids.map((id) => Users.find(id))      // rejected
```

```text
error EK4301: this callback can pause (Users.find), but Array.prototype.map calls it
  synchronously. Use Effect.forEach(ids, (id) => Users.find(id)) or a for...of loop.
```

Effect's own combinators, such as `Effect.forEach`, expect callbacks that return Effects, so Effeck callbacks work there. Chapter 16 gives the rules for higher-order functions.

#### Effect 3

Effeck targets Effect 4 only. Effect 3 values do not run on the Effect 4 runtime. Code that must call an Effect 3 library wraps it as foreign code, for example with the `try` hatch around `Effect3.runPromise`: `try(Effect3.runPromise(program), (error) => …)`.

### 11. The standard library

```ts es
import { readFile } from "node:fs/promises"

function loadSettings(path: string) {
  return JSON.parse(readFile(path, "utf8"))
}
```

```text
error EK4402: node:fs/promises is a platform API, which Effeck code does not call directly.
  settings.effeck:4:21   return JSON.parse(readFile(path, "utf8"))
  Use FileSystem from "effect/FileSystem":   FileSystem.readFileString(path)
  Provide it with NodeServices.layer from @effect/platform-node, or the Bun or browser equivalent.
```

Effeck's standard library is Effect's. Node.js and browser APIs are not callable from `.effeck` files, and each error names the Effect module that replaces the API. Effeck adds no Node.js clones: the replacement is the module Effect already ships, implemented by the platform package your program runs with.

Imports, re-exports, and passing platform values to adapters are allowed (D108). For example, `NodeHttpServer.layer(createServer, { port })` passes a host function without calling it. EK4402 reports execution: calls, construction, tagged templates, and reads of platform properties, including destructuring and spreading. Namespace members used as values, such as `os.EOL`, are allowed. The current check does not follow platform functions through local aliases, parameters, or objects returned by platform calls.

There are three reasons. Platform APIs throw and reject in ways their types do not state, which is exactly what Chapter 10's boundary rules exist to contain. An Effect module states its errors in its types and is a service, so a test replaces it with a Layer. And Effect already ships these modules, with implementations for Node.js, Bun, and browsers, so cloning Node's API surface would duplicate them.

#### What replaces what

| You would use | In Effeck | Import from |
| --- | --- | --- |
| `fs`, `fs/promises` | `FileSystem` | `effect/FileSystem` |
| `path` | `Path` | `effect/Path` |
| `fetch`, `http.request`, an HTTP client library | `HttpClient`, `HttpClientRequest`, `HttpClientResponse` | `effect/http` |
| `http.createServer`, an HTTP framework | `HttpRouter`, `HttpServer`; schema-first APIs with `HttpApi` | `effect/http`, `effect/http-api` |
| `child_process` | `ChildProcess`, `ChildProcessSpawner` | `effect/process` |
| `setTimeout`, `setInterval` | `Effect.sleep`; `Effect.repeat` with a `Schedule` | `effect` |
| `process.env` | `Config` | `effect` |
| `console.log` | `Effect.log`, `Console` | `effect` |
| `Date.now()`, `new Date()` | `Clock`, `DateTime` | `effect` |
| `Math.random()` | `Random` | `effect` |
| `crypto.randomUUID()`, `crypto.subtle.digest` | `Crypto` | `effect` |
| `net`, `dgram`, `WebSocket` | `Socket`, `SocketServer`, `DatagramSocket` | `effect/socket` |
| `dns` | `Dns` | `effect/net` |
| `worker_threads`, Web Workers | `Worker`, `WorkerRunner` | `effect/workers` |
| `process.argv`, `readline`, `process.stdin` | `Command`, `Flag`, `Argument`; `Terminal`, `Stdio` | `effect/cli`, `effect` |
| `localStorage`, `sessionStorage` | `KeyValueStore` | `effect/persistence` |
| a database driver | `SqlClient`, with a driver package such as `@effect/sql-sqlite-node` | `effect/sql` |
| `EventEmitter`, DOM events | `PubSub`, `Stream.fromEventListener` | `effect` |
| `stream.Readable` | `Stream`; `NodeStream.fromReadable` at the edge | `effect`, `@effect/platform-node` |

The runtime packages supply the implementations and the runner: `@effect/platform-node` (`NodeRuntime`, `NodeServices.layer`), `@effect/platform-bun`, and `@effect/platform-browser`.

A module whose service is a `Context.Service` value, such as `FileSystem`, is imported from its module path, `effect/FileSystem`, so its methods are called directly: `FileSystem.readFileString(path)`. Importing the namespace from `effect` works too, as `FileSystem.FileSystem.readFileString(path)`.

#### Files

```ts effect
const countLines = Effect.fn("countLines")(function* (path: string) {
  const fs = yield* FileSystem.FileSystem
  const text = yield* fs.readFileString(path)
  return text.split("\n").length
})
```

```ts es
import { FileSystem } from "effect/FileSystem"

function countLines(path: string) {
  return FileSystem.readFileString(path).split("\n").length
}
// number!PlatformError!FileSystem
```

#### Schemas

```ts effect
const Todo = Schema.Struct({
  userId: Schema.Number,
  id: Schema.Number,
  title: Schema.String,
  completed: Schema.Boolean
})
interface Todo extends Schema.Schema.Type<typeof Todo> {}
```

```ts es
schema Todo {
  userId: number
  id: number
  title: string
  completed: boolean
}
```

A `schema` declaration declares both: the constant `Todo`, a `Schema.Struct`, and the interface `Todo`, its decoded type (D133). Each field is written with a type, and the compiler writes its schema: a primitive, a literal or a union, an array, a tuple, a record, an object type, or the name of another schema, which stands for that schema's decoded type. `name?: T` is `Schema.optional`, which accepts an absent key or `undefined`. A field that needs another schema, such as `Schema.DateTimeUtc` or a checked string, names a schema constant: `const Created = Schema.DateTimeUtc`, then `created: Created`. A field type with no schema form is EK2109. The interface comes from the struct, so the two cannot drift apart.

#### HTTP

```ts effect
const getTodo = Effect.fn("getTodo")(function* (id: number) {
  const response = yield* HttpClient.get(`https://jsonplaceholder.typicode.com/todos/${id}`)
  return yield* HttpClientResponse.schemaBodyJson(Todo)(response)
})
```

```ts es
import { HttpClient, HttpClientResponse } from "effect/http"

function getTodo(id: number) {
  const response = HttpClient.get(`https://jsonplaceholder.typicode.com/todos/${id}`)
  return HttpClientResponse.schemaBodyJson(Todo)(response)
}
// Todo!HttpClientError | SchemaError!HttpClient
```

`HttpClient.get` returns an `Effect`, so the call runs it. `HttpClientResponse.schemaBodyJson(Todo)` returns a plain function, which `effect` exports with a trusted contract, and calling that function with the response returns an `Effect`, which runs.

#### Timers

```ts effect
const greetLater = Effect.fn("greetLater")(function* (name: string) {
  yield* Effect.sleep("1 second")
  return `Hello, ${name}`
})

const heartbeat = Effect.log("alive").pipe(Effect.repeat(Schedule.spaced("30 seconds")))
```

```ts es
function greetLater(name: string) {
  sleep("1 second")
  return `Hello, ${name}`
}

function heartbeat() {
  repeat(log("alive"), Schedule.spaced("30 seconds"))
}
```

Time comes from the `Clock` service, so a test moves it forward with `TestClock.adjust` instead of waiting (Chapter 12).

#### Environment

```ts effect
const serverConfig = Effect.gen(function* () {
  const port = yield* Config.Port("PORT").pipe(Config.withDefault(3000))
  const databaseUrl = yield* Config.Redacted("DATABASE_URL")
  return { port, databaseUrl }
})
```

```ts es
const Port = Config.withDefault(Config.Port("PORT"), 3000)
const DatabaseUrl = Config.Redacted("DATABASE_URL")

function serverConfig() {
  return { port: Port(), databaseUrl: DatabaseUrl() }
}
// { port: number; databaseUrl: Redacted<string> }!ConfigError!
```

A `Config` is data until you call it (Chapter 13). The module's top level builds `Port` and `DatabaseUrl`; `Port()` reads the variable and fails with `ConfigError` if it is set but invalid. A test supplies values with a `ConfigProvider` instead of touching the process environment.

#### Child processes

```ts effect
const currentBranch = Effect.fn("currentBranch")(function* () {
  const spawner = yield* ChildProcessSpawner.ChildProcessSpawner
  const output = yield* spawner.string(ChildProcess.make("git", ["branch", "--show-current"]))
  return output.trim()
})
```

```ts es
import { ChildProcess } from "effect/process"
import { ChildProcessSpawner } from "effect/process/ChildProcessSpawner"

function currentBranch() {
  return ChildProcessSpawner.string(ChildProcess.make("git", ["branch", "--show-current"])).trim()
}
// string!PlatformError!ChildProcessSpawner
```

`ChildProcessSpawner.spawn` gives a handle whose output is a `Stream` and whose process belongs to a scope (its type requires `Scope`), so `using child = ChildProcessSpawner.spawn(command)` ties a long-running process to a block.

#### Providing the platform

```ts es
// main.effeck
import { NodeRuntime, NodeServices } from "@effect/platform-node"
import { FetchHttpClient } from "effect/http"

function main() {
  const branch = Effect.provide(() => currentBranch(), wire(NodeServices.layer, FetchHttpClient.layer))
  log(`on ${branch}`)
}

NodeRuntime.runMain(() => main())
```

Tests provide fakes instead: a `Layer.succeed(FileSystem, { … })` with the methods a test needs, or an in-memory implementation where Effect ships one, such as `KeyValueStore.layerMemory`.

### 12. Questions Effect users ask

Short answers, each with the code. Appendix C maps every section of Effect's documentation to Effeck, including the many features that carry over unchanged.

#### Where did `async` and `await` go?

```ts es
function loadProfile(id: string) {
  const user = Users.find(id)                       // waits for the database: no await
  const avatar = try(                               // a Promise from a TypeScript library
    (signal) => avatarSdk.fetch(user.avatarId, { signal }),
    (cause) => new AvatarError({ cause })
  )
  return { user, avatar }
}
// { user: User; avatar: Avatar }!UserNotFound | AvatarError!Users
```

A call waits for its callee, whether the callee finishes at once or after a network round trip. While it waits, the fiber is parked and other fibers run. There is no `async` function and no `await` outside a boundary hatch, so there are no "colored" functions: every function can wait. A Promise from a TypeScript library crosses with the `try` hatch, or with `sync` when a rejection is a bug (D148); inside the hatch's function, `async` and `await` keep their JavaScript meaning. Platform APIs that return Promises, such as `fetch`, are replaced by Effect modules (Chapter 11).

#### How do I run things concurrently?

```ts es
function homePage(userId: string) {
  const [feed, unread] = all(
    [Feed.load(userId), Notifications.unread(userId)],
    { concurrency: "unbounded" }
  )
  const previews = forEach(feed.items, (item) => preview(item), { concurrency: 8 })
  return { previews, unread }
}
```

Calls run in order. Concurrency is always a combinator over programs: `Effect.all`, `Effect.forEach`, `Effect.race`, `Effect.raceFirst`, or `fork` (Chapter 7). `Semaphore`, `Latch`, `Queue`, `Deferred`, and `PubSub` work unchanged.

#### Where did `Effect.gen` and `yield*` go?

```ts es
function displayName(id: string) {
  return Users.find(id).name
}
```

Every function body plays the part of the generator, and every call that returns an Effect plays the part of `yield*`. `function*` now declares a producer (Chapter 8), so APIs that take a generator, such as `Effect.gen` and `Option.gen`, stay in `.ts` files (EK1002). The editor converts an `Effect.gen` function for you (Chapter 28).

#### What does `throw` mean now?

```ts es
function parseId(raw: string) {
  if (!/^\d+$/.test(raw)) throw new InvalidId({ raw })     // a typed failure: InvalidId joins E
  if (raw.length > 64) die(new Error("caller bug"))  // a defect, on purpose
  return Number(raw)
}
// number!InvalidId!
```

`throw` is `Effect.fail`. Every thrown value joins E with its type and can be caught, including `throw new Error("unreachable")`, which inside `Effect.gen` would be a defect. To signal a bug, die explicitly with `Effect.die`. Defects also come from exceptions no contract lists, and from `Effect.orDie(() => …)` (Chapter 3).

#### What happens when code that is not Effect throws?

```ts es
function readPort(text: string) {
  try {
    return JSON.parse(text).port as number    // JSON.parse fails with SyntaxError
  } catch (e) {                               // e: SyntaxError
    return 8080
  }
}
// number!!
```

JavaScript's built-ins and Effect's own functions ship with contracts that say which exceptions are expected failures. Other TypeScript code needs an explicit boundary or a trusted contract (EK4401), and platform APIs are replaced by Effect modules (EK4402). An exception that no contract lists is a defect. Unlike a JavaScript `catch` inside `Effect.gen`, an Effeck `catch` does not see it, and a `catch` that could only have caught such exceptions is reported, EK2004 (Chapter 3).

#### How do I use `Option` and `Result`?

```ts es
function displayName(user: User) {
  return Option.fromNullishOr(user.nickname).match({
    onNone: () => user.name,
    onSome: (nickname) => nickname
  })
}

function tryCharge(cartId: string) {
  const outcome = Effect.result(Payments.charge(cartId))    // Result<Receipt, PaymentError>
  return outcome.isSuccess() ? outcome.success : undefined
}
// Receipt | undefined!!Payments
```

`Option` and `Result` (Effect 4's name for `Either`) are plain data, imported from `effect` and used as in TypeScript. `Effect.result` and `Effect.option` turn a failure into a value; `throw` turns a value back into a failure. `Option.gen` and `Result.gen` take generators, so they stay in `.ts` files.

#### How do I validate data with `Schema`?

```ts es
class Signup extends Schema.Class<Signup>("Signup")({
  email: Schema.String,
  plan: Schema.Literals(["free", "pro"])
}) {}

function register(body: unknown) {
  const signup = Schema.decodeUnknownEffect(Signup)(body)    // fails with SchemaError
  return Accounts.create(signup.email, signup.plan)
}
// Account!SchemaError | AccountError!Accounts
```

`Schema` is in the prelude and unchanged. A decoder that returns an `Effect` runs when called and fails with a typed `SchemaError`. The other forms (`decodeUnknownSync`, `is`) are trusted functions whose exceptions are defects, so `.effeck` code prefers the `Effect` forms. Errors that must cross a process boundary are `Schema.TaggedError` classes.

#### How do I work with streams?

```ts es
function digestActive(users: Stream.Stream<User, DbError>) {
  for (const user of users) {
    if (user.active) Mailer.digest(user)
  }
}
// void!DbError | MailError!Mailer

function firstErrorLines(path: string) {
  return lines(path).filter(isError).take(10).runCollect()
}
```

Effect's `Stream`, `Sink`, and `Channel` modules work unchanged: their operators take plain functions and Effect-returning callbacks, and the `Stream.run*` functions return Effects, so calling them runs the stream. `for...of` iterates a `Stream` directly, and a `function*` producer such as `lines` is a `Stream` wherever a `Stream` is expected (Chapter 8).

#### How do I test?

```ts es
import { assert, it } from "@effect/vitest"
import { TestClock } from "effect/testing"

it.effect("a reminder fires after one day", () => {
  const reminder = fork(remindLater("ada", "1 day"))
  TestClock.adjust("1 day")                          // virtual time: nothing waits
  assert.strictEqual(reminder.join(), "sent")
})
```

`@effect/vitest` works unchanged: `it.effect` runs an Effeck function as a test, `assert` comes from the same package, and `TestClock` controls time. Fake services are Layers (Chapter 4), and a control operation is scripted with a handler (Chapter 9). An `Exit` is data until called, so `assert.deepStrictEqual(Effect.exit(() => f()), Exit.succeed(1))` compares two `Exit` values. A test written as `async () => { await Effect.runPromise(p) }` stays in a `.ts` file: `.effeck` has no `await`, and a runner inside a test body is EK1005 (Chapter 13).

#### How do I log, trace, and record metrics?

```ts es
const checkouts = Metric.counter("checkouts")

function checkout(cartId: string) {
  log(`checkout ${cartId}`)
  const receipt = withSpan(Payments.charge(cartId), "charge")
  Metric.update(checkouts, 1)
  return receipt
}
```

Unchanged: `Effect.log` and its levels, `Effect.annotateLogs`, `Effect.withLogSpan`, `Effect.withSpan`, `Logger`, and `Metric`, each taking a program argument where it takes an Effect today. Functions are untraced unless you add a span or turn on `tracing` for the project (Chapter 28). Exporters come from `effect/observability`.

#### How do I read configuration?

```ts es
const Port = Config.withDefault(Config.Port("PORT"), 3000)

function main() {
  const port = Port()                         // reads PORT; ConfigError if it is invalid
  log(`listening on ${port}`)
}
```

A `Config` is data until you call it (Chapter 13), so `Config.all`, `Config.nested`, and `Config.withDefault` compose it first and a call reads it. A `ConfigProvider` replaces the environment in tests (Chapter 11).

#### How do I run a program?

```ts es
// main.effeck
import { NodeRuntime } from "@effect/platform-node"

function main() {
  with AppLive { serve(8080) }
}

NodeRuntime.runMain(() => main())
```

A program starts only at a runner: `NodeRuntime.runMain` or `BunRuntime.runMain` for a process, `it.effect` for a test, and `Effect.runPromise` or a `ManagedRuntime` where TypeScript code needs a result. A web framework that calls your handlers builds one `ManagedRuntime` from the application layer and runs each handler through it, so the layer is built once (Chapter 4). A module's top level cannot run a program (EK1001).

#### Do I still use `pipe`?

```ts es
const user = fetchUser(id) |> retry(%, { times: 3 }) |> timeout(%, "2 seconds")
```

Rarely. Statements replace sequencing, error handling, and cleanup, so most pipelines disappear. The pipe operator, `x |> f(%)`, stacks policies over a program, passing it by name where a policy expects a program, and works on data too (Chapter 5).

#### How do I pattern match?

```ts es
function area(shape: Circle | Square) {
  switch (shape._tag) {
    case "Circle": return Math.PI * shape.radius ** 2
    case "Square": return shape.side ** 2
  }
}

const describe = Match.type<Circle | Square>().pipe(
  Match.tag("Circle", (c) => `circle of radius ${c.radius}`),
  Match.tag("Square", (s) => `square of side ${s.side}`),
  Match.exhaustive
)
```

TypeScript's narrowing does most of the work: `switch` on `_tag`, `instanceof`, and `catch` narrowing on errors (Chapter 3). Effect's `Match` module works unchanged; its handlers are plain functions, so they cannot pause (EK4301). TC39's pattern matching proposal is at Stage 1 and is not used.

#### How do fibers, interruption, and cancellation work?

```ts es
function search(query: string) {
  const scan = fork(slowIndexScan(query))
  const cached = SearchCache.lookup(query)
  if (cached !== undefined) {
    scan.interrupt()               // cancels the scan; its finally blocks run first
    return cached
  }
  return scan.join()
}
```

They are Effect's. A fiber is Effect's fiber, `fork` starts one owned by the enclosing block (Chapter 7), and `Fiber.join` and `Fiber.interrupt` work unchanged. Interruption runs every `finally` block and `using` release on the way out, and `catch` never catches it (Chapter 3). `Effect.uninterruptible(() => …)` protects a region, `Effect.onInterrupt` reacts to cancellation, and `Effect.timeout` and `Effect.race` interrupt the branches they discard. A Promise is cancelled through the `AbortSignal` that the `try` and `sync` hatches pass.

#### How do I wrap a callback API?

```ts es
function ready() {
  return callback<Sdk>((resume) => {
    onceReady((sdk) => resume(sdk))
  })
}
```

The `callback` hatch adapts it (D148): its register function is plain TypeScript, `resume(value)` succeeds, `resume.fail(error)` fails, and a function the register function returns runs on interruption. A callback API that delivers a resource, such as a socket, is adapted in TypeScript with the runtime's `suspend` and `Effect.acquireRelease(…, { interruptible: true })`, so a value that arrives after cancellation is released (Chapter 6). Node.js and browser callback APIs already have Effect modules (Chapter 11).

#### Can I mix `.ts` and `.effeck` files?

```ts es
import { findUser } from "./users-repo.ts"     // Effect.fn("findUser")(function* …)

export function title(id: string) {
  return findUser(id).name.toUpperCase()
}
```

Yes, in one program: `effeck` checks both, and the editor serves both (Chapter 28). A TypeScript function that returns an `Effect` is called like an Effeck function, and TypeScript sees every Effeck function as one that returns an `Effect` (Chapter 10).

#### Is there anything from Effect I cannot use?

```ts es
const legacy = try(Effect3.runPromise(oldProgram), (cause) => new LegacyError({ cause }))
```

Five things, each with a replacement. Generator-based APIs (`Effect.gen`, `Option.gen`) cannot be written in `.effeck` files; call them from TypeScript. Effectful handlers in `Option.match` or `Match` are rejected (EK4301); use `if` or `switch`. Runners inside code a fiber runs are rejected (EK1005); call or `fork` the program. A TypeScript function whose requirements include a control operation is rejected (EK4202). Effect 3 code, including `Micro`, runs only as foreign code behind a boundary, as above. Everything else in Effect is available, and Appendix C lists it feature by feature.

#### How do I keep state that several fibers share?

```ts es
function countVisits(paths: ReadonlyArray<string>) {
  const visits = Ref.make(0)
  forEach(paths, (path) => {
    Visits.record(path)
    visits.update((n) => n + 1)
  }, { concurrency: 4 })
  return visits.get()
}
// number!VisitError!Visits
```

Local variables work inside one function, as in any JavaScript. State shared between fibers uses Effect's `Ref`, `SynchronizedRef`, and `SubscriptionRef`, unchanged: `Ref.make(0)` returns an `Effect`, so the call runs it and gives you the `Ref`.

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
| A pipe's topic, `%` in `fetchUser(id) \|> retry(%, policy)` | Where `%` stands for a program, the pipe's left side, passed by name (D155); elsewhere the left side's value, computed once before the body | `retry(Effect.gen(function* () { return yield* fetchUser(id) }), policy)` |
| A method of an Effect data type, `stream.take(3)` or `fiber.join()` | Calls the module's function with the value first (D152), then applies these rules to that call: an `Effect` runs, other data is returned | `Stream.take(stream, 3)`; `yield* Fiber.join(fiber)` |
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

#### Methods on Effect's data types

`x.name(args)` is a method call into a module (D152) when all of these hold:

1. The file is a `.effeck` file, and the access is not part of an optional chain (`x?.name(…)`).
2. `x`'s type, after TypeScript resolves type parameters to their constraints, has no member `name` of its own.
3. One of Effect's data-type modules declares `x`'s type in its declaration file, `effect/dist/<Module>.d.ts`. For a union, every member comes from the same module, as `None` and `Some` come from `Option`.
4. That module exports a function `name` with an overload whose first parameter is named `self`, Effect's name for a dual's data-first form, or ends in `OrOptions`.

The method's signatures are those overloads with the first parameter moved to `this`. TypeScript checks `x` against it and infers type parameters from it, and a type predicate on it narrows `x`:

```ts es
function describe(value: Option.Option<number>) {
  if (value.isSome()) return `some ${value.value}`   // Option.isSome(value): value is Some<number>
  return "none"
}
```

The call compiles to `Module.name(x, args)`, which the rules above then apply to: `Fiber.join(fiber)` returns an `Effect`, so `fiber.join()` runs, while `Stream.take` returns a `Stream`, which is data. The method exists only in the call: `const read = maybe.getOrElse` is EK4501 (D153), because no function value exists to bind; pass `Option.getOrElse` itself.

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
  log(`welcomed ${name}`)
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

### 14. Contracts and inference

#### What the compiler infers

An unannotated function or a function with an Effect return annotation gets a contract `Effect<A, E, R>`. An eligible function with a plain return annotation keeps its plain type (D67, Chapter 15):

- **A** is the type of what it returns, inferred and widened as TypeScript infers a function's return type: `return "ok"` gives `string`.
- **E** is every error it `throw`s, plus every error its callees can fail with, plus the expected failures named by the trusted contracts it calls, minus what its `catch` blocks handle.
- **R** is every service and control operation it uses, plus its callees' requirements, minus what it provides or handles itself, plus `Scope` if a resource it acquires without `using` outlives it (Chapter 6).

```ts es
function quoteCart(cartId: string) {
  const cart = Carts.load(cartId)              // E: CartNotFound   R: Carts
  if (cart.items.length === 0) throw new EmptyCart({ cartId })
  try {
    return Pricing.quote(cart)                 // E: PricingDown    R: Pricing
  } catch (e) {                                // e: PricingDown
    return estimate(cart)                      // a pure Effeck helper
  }
}
// Quote!CartNotFound | EmptyCart!Carts | Pricing
```

`PricingDown` is caught, so it is not in E. `Pricing` is still in R: catching an error does not remove the need for the service.

Precisely, E is the type of the expected failures in a cause made only of expected failures. A cause that also holds a defect or an interruption can carry `Fail` reasons outside E, because `catch` passes such a cause through untouched (Chapter 17); no Effeck `catch` recovers it, and only cause-level tools see those reasons.

#### Writing contracts

`.effeck` code writes `Effect<A, E, R>` as `A!E!R` (D127), anywhere a type can appear: return annotations, service members, parameters, and aliases. The parser expands it to `Effect.Effect<A, E, R>`.

| Written | Means |
| --- | --- |
| `User!NotFound!Db` | `Effect<User, NotFound, Db>` |
| `User!NotFound!` | `Effect<User, NotFound, never>` |
| `User!!Db` | `Effect<User, never, Db>` |
| `User!!` | `Effect<User, never, never>` |
| `User!NotFound` on a function with a body | `Effect<User, NotFound, infer R>`: R comes from the body |
| `User!NotFound` without a body | `Effect<User, NotFound, never>` |

A slot without a type is `never`. Dependencies left out entirely are inferred on a function with a body and `never` everywhere else. Unions bind inside a slot, so `User!NotFound | Timeout!Db` fails with `NotFound | Timeout`. A function type, a conditional type, or an Effect type inside a slot needs parentheses, and so does an object type after a `!`: a `{` there starts the function's body, so `User!NotFound! { ... }` has no dependencies. Hovers and messages about `.effeck` code show contracts in this form; declaration files keep `Effect.Effect<A, E, R>`, so TypeScript callers are unaffected. `Effect<A, E, R>` is accepted everywhere, including `infer` slots (D57).

#### Checking a written contract

When you write the contract, the compiler checks the body against it:

```ts es
export function welcome(id: string): string!UserNotFound | MailError!Users {
  const user = Users.find(id)
  Mailer.send(user.email, "Welcome")
  return user.name
}
```

```text
error EK2002: welcome needs Mailer, but its contract says R = Users.
  welcome.effeck:3:3   Mailer.send(user.email, "Welcome")
  Provide Mailer inside welcome, or add it: Effect<string, UserNotFound | MailError, Users | Mailer>.
```

| Check | Rule | Diagnostic |
| --- | --- | --- |
| Success | The returned value must be assignable to A | TypeScript's |
| Errors | Every error that can escape must be in E; `A!!R` allows `catch` inside and rejects any error that escapes | EK2001 |
| Requirements | Every requirement that remains must be in R | EK2002 |
| Width | A contract may be wider than the body needs, never narrower | EK2001, EK2002 |

A call to a function with a complete written contract contributes that contract's E and R, even when the body needs less (D73).

The contract has exactly three channels. Whether a function can pause is a separate fact the compiler tracks, its control class (Chapter 15), not part of the contract.

#### Declarations without bodies

Service members, control operations, `declare function`, and fields typed as functions have no body to infer from, so each states its contract without `infer`. A bare return type is an error, never a silent `A!!`. Service and control methods require all three Effect type arguments; `Effect<A>` is EK2101 there (D93, amending D57):

```ts es
service Rates {
  lookup(currency: string): number
}
```

```text
error EK2101: Rates.lookup has no body to infer from, so its contract must be written in full.
  rates.effeck:2:38   abstract lookup(currency: string): number
  Write Effect<number, E, R>; use Effect<number, never, never> if it cannot fail and needs nothing.
```

A service's value members are not calls, so they take any type: `abstract readonly baseUrl: string`. Callback parameter types are the one exception to full contracts, and there a missing contract means "any contract", not an empty one (Chapter 16).

#### Partial annotations

In the return annotation of a function with a body, write all three channel slots. A slot written `infer` followed by a name comes from the body; every other slot is checked (D57).

```ts es
function find(id: string): Effect<User, infer E, never> {
  return Effect.fail(new UserNotFound({ id }))
}
// Checks A = User and R = never; infers E = UserNotFound.

function load(id: string): Effect<User, never, infer R> {
  return Effect.orDie(Users.find(id))
}
// Checks A = User and E = never; infers R = Users.

function plan(id: string): Effect<infer A, UserNotFound, Users> {
  return Users.find(id).name
}
// Infers A = string; checks E = UserNotFound and R = Users.

function greeting(id: string): Effect<string, infer E, infer R> {
  return `Hello, ${Users.find(id).name}`
}
// Checks A = string; infers E = UserNotFound and R = Users.

function inferred(id: string): Effect<infer _, infer _, infer _> {
  return Users.find(id)
}
// User!UserNotFound!Users, as with no annotation.
```

The name after `infer` is documentation only and is not in scope in any channel or in the body. `infer _` is allowed. In `Effect<infer Value, infer Value, infer Value>`, the repeated name does not connect the channels.

Omission is an error in this position. Neither `Effect<User>` nor `Effect<User, UserNotFound>` says whether an absent channel should be checked as `never` or inferred. A plain return annotation such as `: User` is EK2003 only when a non-method function's body runs an Effect. An eligible plain helper keeps that annotation and compiles to one plain function (D67). Methods are exempt from EK2003 for plain annotations, including methods implementing JavaScript protocols (D74); their expected type and body determine whether they lower to Effects (D92).

```ts es
function find(id: string): Effect<User> {
  return Users.find(id)
}
```

```text
error EK2003: Effect<User> leaves contract channels unspecified. Did you mean never or inferred?
  Write Effect<User, never, never> to check E and R as never.
  Or write Effect<User, infer E, infer R> to infer E and R from the body.
```

These are two distinct quick fixes. The first can then report EK2001 or EK2002 when the body fails or needs services. The second keeps those channels inferred.

This rule applies only to the return annotation of an Effeck function with a body. Parameter types, type aliases, and `.ts` files keep TypeScript's meaning of `Effect<User>`, including its `never` defaults. Service and control methods instead require a complete contract (D93). They do not gain channel inference from this syntax. A callback parameter without an Effect contract still accepts any contract (D12, Chapter 16).

#### Recursion

A recursive Effect function's E and R come from a fixpoint over its strongly connected component of the call graph. Cached catch-variable or return types can require a written contract before those channels are known (EK2102, D71):

```ts es
function totalSize(path: string) {
  const entry = Files.stat(path)
  if (!entry.isDirectory) return entry.size
  let total = 0
  for (const child of Files.list(path)) total += totalSize(`${path}/${child}`)
  return total
}
// number!IOError!Files
```

TypeScript cannot compute this on its own: to TypeScript, the generated recursive function is an implicit `any` (TS7022). The compiler computes the fixpoint itself, inside the component only (D52). It checks the component's bodies, then rechecks those bodies alone, with the E and R found so far, until E and R stop growing. Rows only grow and are finite, so the iteration ends, and no function outside the component is checked twice. Mutual recursion is where the recheck matters:

```ts es
error Locked { path: string }

function visitDir(path: string) {
  for (const child of Files.list(path)) visitEntry(`${path}/${child}`)
}

function visitEntry(path: string) {
  if (path.endsWith(".lock")) throw new Locked({ path })
  if (Files.stat(path).isDirectory) visitDir(path)
}
// both: void!IOError | Locked!Files
```

Checked once in source order, `visitDir` meets `visitEntry` before `visitEntry` has a contract and misses `Locked`; the recheck adds it. The prototype compiler reaches the same contracts by re-checking the whole program (`prototype/esc/FINDINGS.md`, finding 7). The single-pass experiment found this case, in its `isEven` and `isOdd` probe, to be the only one that needs a second check (`prototype/single-pass/RESULTS.md`). A follows TypeScript's rule: when A depends on the function's own result, the contract must be written (EK2102).

EK2102 applies to functions that run Effects. Pure recursion keeps TypeScript's TS7023 when its return type depends on itself; a plain annotation fixes it (D72). The current checker also needs contracts in some cached-type cycles that the fixpoint model alone would solve. A cycle through a callback uses the checker's single type for that call, and generic-call inference does not fill `infer` channels in a lambda (F3's known gaps).

### 15. Which functions compile to plain JavaScript

Unannotated functions and functions with Effect annotations expose Effect contracts, but a function like `add` should cost what a JavaScript call costs. A helper with a plain return annotation keeps a plain function type (D67). The compiler emits a plain JavaScript function whenever the difference is unobservable.

```ts es
function double(n: number) {
  return n * 2
}

function area(w: number, h: number) {
  return double(w) * double(h) * Settings.scale()
}
```

```ts gen
export function double$direct(n: number): number {              // Effeck callers use this
  return n * 2
}
export const double = (n: number): Effect.Effect<number> =>     // TypeScript callers use this
  Effect.sync(() => double$direct(n))

export const area = Effect.fnUntraced(function* (w: number, h: number) {
  const left = double$direct(w) * double$direct(h)              // source order: before the service call
  const scale = yield* (yield* Settings).scale()                // a service call: may pause
  return left * scale
})
```

#### The rule

A function compiles to plain JavaScript when both hold:

1. **It performs no Effect operation at all.** Its body calls only plain-JavaScript Effeck functions, trusted built-ins, and trusted functions; it calls no service, control operation, `Effect` function, or boundary hatch such as `sync`. Its control class is `pure` or `sync`.
2. **Its E is `never`.** A typed failure leaving a plain function would cross JavaScript frames as an exception, and v0 bridges no exception across a call. Exceptions become typed failures only where they are raised: at the call of a trusted function, whose listed exceptions are converted there (Chapter 26), and inside the plain function's own `try`, whose `catch` handles them (below). Bridging across calls is experiment G15.

The test is "can it reach an Effect operation", not "are E and R empty". `Effect.sleep("1 second")` has E = `never` and R = `never`, and it pauses the fiber. So the compiler tracks a separate fact for each function, its **control class**:

| Control class | Meaning | Examples | Plain JavaScript |
| --- | --- | --- | --- |
| `pure` | No observable effect; cannot pause | `double`, string formatting, `Math.max` | Yes |
| `sync` | Synchronous work you can observe; cannot pause | Mutating an object, `Map.prototype.set`, a trusted TypeScript function | Yes; the order and count of the work are kept |
| `suspends` | May pause the fiber, or crosses an Effect boundary | `Effect.sleep`, `sync(…)`, any service method, any imported Effect | No |
| `captures` | May reach a handler (Chapter 9) | `Approval.approve`, `yield` in a producer | No |

`pure` is assigned only to Effeck functions the compiler analyzed and to built-ins on the compiler's list; a trusted TypeScript function is `sync` at most. `pure` permits one thing beyond `sync`: inlining the body at a call. It never permits removing a call whose result is unused, reordering calls, or merging two calls into one.

#### Why this is safe

- **Nothing runs early when building an Effect.** For an unannotated or Effect-contract helper, the TypeScript-facing form is `Effect.sync(() => …)`, never `Effect.succeed(double$direct(n))`. Building a program does no work, even work that looks free. A pure function can still throw a defect (`(undefined as any).x`), and that must happen when the program runs, not when it is built.
- **No point where another fiber could run disappears.** A plain call runs inside its caller's current step. Every Effect operation is such a point, including an `Effect.sync` body, so a function containing one is not eligible. Inlining a callee that performs no operation removes none.
- **Order is kept.** Generated code evaluates in source order. In `area`, both `double` calls run before `Settings.scale()`, as the source says, even though only the service call can pause.
- **Public types follow annotations.** An unannotated or Effect-contract helper exposes `(...) => Effect<A, E, R>` and gets a `$direct` shortcut. An eligible helper annotated `: A` exposes `(...) => A` and emits one plain function (D67). Plain calls run inline even at module top level (D65). The compiler does not yet emit `$direct` declaration metadata, so callers compiled against declarations use the correct, slower Effect form (Chapter 27).

#### Regions inside a plain function

A plain JavaScript function may contain `try`, `catch`, and `finally`, and they compile to JavaScript's own statements. Nothing inside can pause or be interrupted, so JavaScript's semantics match the machine's. The generated `catch` rethrows anything that is not one of the region's expected failures, so defects still pass through:

```ts es
function readPort(text: string) {
  try {
    return JSON.parse(text).port as number
  } catch (e) {                                // e: SyntaxError
    return 8080
  }
}
```

```ts gen
export function readPort$direct(text: string): number {
  try {
    return JSON.parse(text).port as number
  } catch (e) {
    if (!(e instanceof SyntaxError)) throw e     // a defect: not caught
    return 8080
  }
}
```

Any `using` declaration makes a function compile to frames, whether its initializer acquires an Effect resource or supplies an ordinary Disposable (D54, D94).

#### Methods that implement a plain-function member

```ts es
class Money implements Equal.Equal {
  constructor(readonly cents: number, readonly currency: string) {}

  [Equal.symbol](that: Equal.Equal): boolean {
    return that instanceof Money && that.cents === this.cents && that.currency === this.currency
  }

  [Hash.symbol](): number {
    return Hash.combine(Hash.number(this.cents), Hash.string(this.currency))
  }
}
```

`Equal.Equal` declares `[Equal.symbol](that): boolean` and `Hash.Hash` declares `[Hash.symbol](): number`: plain functions, which `Equal.equals` and `HashSet` call synchronously. A method that implements such a member compiles to a plain JavaScript method when it is eligible under the rule above, and is EK4301 otherwise. Effeck callers call it as a plain method, because its declared type is not an Effect. A method that implements a service member keeps compiling to an Effect function, because the service declares an `Effect` (D55).

#### Recursion

```ts es
function sumTo(n: number): number!! {
  return n === 0 ? 0 : n + sumTo(n - 1)
}

function walk(node: TreeNode): number!!Nodes {
  let total = node.size
  for (const child of Nodes.children(node)) total += walk(child)
  return total
}
```

`sumTo` is plain JavaScript, recursion included, so it runs on the JavaScript stack like any JavaScript function: fast, and a `RangeError` defect past the host's limit, about 10,000 frames on Node and 40,000 on Bun (F9). Eligibility is decided per strongly connected component of the call graph: the functions of a component compile to plain JavaScript when each one would with the calls inside the component counted as plain calls, and otherwise all of them compile to frames (D56). `walk` calls a service, so it runs on heap frames and recurses without limit: 100,001 nested calls run without overflow (T6a), at about 110 ns per frame (Chapter 25). A pure helper costs what JavaScript costs; only functions that can pause pay for frames.

#### Higher-order functions

A function that calls a callback it receives compiles to frames in v0, because its control class depends on the callback's. `mapAll` in Chapter 16 runs a loop of frames whether its callback pauses or not. Compiling a plain-JavaScript copy per call site, or per class of callback, is a later optimization that changes no rule here.

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

### 17. Failures, defects, interruption, and cleanup

```ts es
function exportReport(id: string) {
  using tmp = TempFiles.create()
  try {
    writeRows(tmp, Reports.rows(id))
    return Storage.upload(tmp)
  } catch (e) {                    // e: UploadError
    return Storage.queueRetry(id)
  } finally {
    Metrics.increment("exports")
  }
}

function exportWithDeadline(id: string) {
  return timeout(exportReport(id), "30 seconds")
}
```

If the 30 seconds run out during the upload, the upload is interrupted, the `catch` block does not run, the `finally` block runs, the temporary file is removed, and the `Effect.timeout` call fails with `Cause.TimeoutError`. Each kind of exit has one rule:

| Exit | Source | In E | Runs `catch` | Runs `finally` and `using` |
| --- | --- | --- | --- | --- |
| Expected failure | `throw` in Effeck; a callee's E; an exception a trusted contract lists | Yes | Yes, narrowed by type | Yes |
| Defect | Any other JavaScript exception, such as `(undefined as any).x`; `Effect.die` | No | No; observe it with `Effect.catchDefect` at a boundary | Yes |
| Interruption | Another fiber, a timeout, or an enclosing scope closing | No | No | Yes |
| Abandonment | A handler that does not continue its program (Chapter 9) | No | No | Yes |
| Mixed: expected failures plus a defect or an interruption | A `finally` that crashed while the block was failing | No: E describes causes made only of expected failures, so this cause can carry failures outside E (Chapter 14) | No: the whole exit propagates, every reason intact | Yes |
| Several expected failures | `Effect.race` with both branches failed | Yes | Yes; `e` is the first, and the `catch` handles the exit as a whole | Yes |

Rules:

- **E comes from `throw` and from contracts.** A `throw` written in Effeck, a callee's E, and the exceptions a trusted contract lists are expected failures. Any other JavaScript exception is a defect, as in Effect today.
- **`catch` handles a cause only when it is made entirely of expected failures.** A cause that also holds a defect or an interruption passes through `catch` untouched, including its expected failures. Effect's own `Effect.catch` recovers such a cause and drops the defect (Codex review R1, `internal/effect.ts:158-165, 2665-2685`); source `catch` therefore compiles to the runtime's `catchExpected` (K14a-K14c, Chapter 26).
- **Cleanup runs in reverse order of acquisition**, once per exit, with interruption held off until it finishes.
- **A failure during cleanup is never dropped.** It is added to the cause being reported, on every path: a block's exit, a clause that abandons, an explicit `abandon` while an interruption is pending, a scope that closes, and a fiber that exits (K12a-K12e, K13d).
- **Control cannot leave a `finally` block.** A `return`, `break`, or `continue` there would replace the block's exit, and in JavaScript it discards a failure in flight, so it is EK2006.
- **Abandonment unwinds like interruption.** Cleanup sees an interrupted exit, `catch` does not run, and the handler that abandoned the program continues only after cleanup finishes. A cleanup failure during abandonment fails the `handle` or the `abandon` call that abandoned.
- **Cleanup that uses a control operation handles it inside the cleanup** (Chapter 9). A handler outside could stop the cleanup partway.

A cause with expected failures and a defect keeps both after it passes `catch`, so a cause-level tool such as `Effect.catchCause` or a TypeScript caller's `Effect.catch` can still see the expected failure. No Effeck `catch` will.

#### Effect's operators on a mixed cause

Effect's own operators keep their Effect behavior when Effeck calls them. On a cause that holds both an expected failure and a defect, most of them act on one reason and drop the other, unlike `catch`:

| Operators | Given a `Fail` plus a `Die` |
| --- | --- |
| `Effect.catch`, `catchTag`, `result`, `option`, `match`, `ignore`, `orElseSucceed`, `firstSuccessOf`, `flip`, `partition` | Act on the expected failure and drop the defect |
| `Effect.validate`, `mapError` | Fail with the expected failure, mapped for `mapError`, and drop the defect |
| `Effect.catchDefect` | Acts on the defect and drops the expected failure |
| `Effect.retry`, `Effect.exit`, `Effect.catchCause` | Keep every reason |

These were checked against `effect@4.0.5` (review probe R1 for `Effect.catch`). Use such an operator where dropping the other reason is what the program means; an Effeck `catch` never drops one.

Interruption can be held off for a block of work with Effect's own combinators:

```ts es
function moveFunds(from: Account, to: Account, amount: number) {
  Effect.uninterruptible(() => {
    Ledger.debit(from, amount)
    Ledger.credit(to, amount)     // an interruption waits until both have run
  })
}
```

`try`, `catch`, `finally`, and `using` compile to Effect's own failure and cleanup frames, never to JavaScript `try`, in any function that compiles to frames. Effect does not run JavaScript `finally` blocks inside generators when a fiber fails or is interrupted (F1-F3), so a JavaScript `try` would skip cleanup. Chapter 26 shows the generated code. Functions that compile to plain JavaScript are the exception, because nothing in them can pause (Chapter 15).

## Part III. The runtime

This part is for reviewing how Effeck runs. It explains how the features in Parts I and II execute on Effect 4's fiber runtime, what Effect already provides, what a small runtime extension (the **kernel**) adds, and the tests that check each claim. Claims carry tags:

| Tag | Meaning |
| --- | --- |
| **[Effect]** | Verified in `effect@4.0.5` source (`experiments/node_modules/effect/src`, cited as `file:line`) or by an Effect-only test (F1-F10, conformance tests D01-D05, K15a) |
| **[Kernel]** | Verified by a kernel test: T1-T17, K10-K15, H1-H4, G6, and the conformance suite's A, F, I, O, S, T, W, and X tests. These test the language model, not ordinary Effect programs |
| **[Prototype]** | Compiled from `.effeck` by the prototype compiler and run on the kernel: an example in `prototype/esc/examples` (Chapter 27) |
| **[Rule]** | A language rule, decided in Part V; the compiler enforces it, and no test can prove a rule a checker has not been built to enforce |
| **[Experiment]** | A rule whose implementation an experiment in Chapter 30 still has to confirm |

### 18. Why a continuation runtime

```ts es
const cell = handle(() => search(grid, 42), Found, {
  found(cell, _k) { return cell }         // ends search() now; the cursors it opened close first
})

for (const line of lines(path)) {          // one value at a time; break closes the file
  if (line.startsWith("ERROR")) break
}
```

Both examples need one thing Effect does not expose today: a way for code on a fiber to take "the rest of the computation, up to here", hand it to a handler, and let the handler continue it once, later, or never. That object is a **one-shot delimited continuation**. Fubhy's advice was to build the language on it: "Start with a one-shot delimited continuation though. That is kind of the core thing that anything else should be based on." This part shows that Effect 4 already provides most of it, and what the rest takes.

#### Effect already runs on continuations

Effect 4 runs every program as a one-shot continuation. A fiber's `_stack` is the rest of the computation, reified as an array of frames. `Effect.callback` parks it and accepts one resume. Interruption unwinds it through finalizer frames. What Effect lacks is a delimiter inside a fiber: the only one is the fiber's root, so the only continuation anyone can take is the whole fiber.

Effeck adds the missing delimiter. A 871-line kernel prototype on `effect@4.0.5` pushes prompt frames onto the real fiber stack and captures the segment above a prompt as a one-shot continuation. It restores the Context and interruptibility recorded at each boundary, abandons unresumed continuations through Effect's own finalizers, keeps a continuation from outliving the fiber that captured it, and bounds cross-fiber wake-up depth. 148 tests cover nested suspension with Context and Scope, double resumption, abandonment, interruption while a continuation is held, cancellation races, a 100,001-frame capture, capture through TypeScript-authored `Effect.forEach`, retry, producers in constant stack, cause-aware `catch`, and a 54-test conformance suite written by an independent reviewer. A prototype compiler then compiled 46 example files written from this document to that kernel and ran them (Chapter 27).

Every feature in Parts I and II is a rule over that one machine:

- A function call is a frame; `throw` unwinds frames.
- A service is an operation whose handler runs in place, at the call.
- `using` and `finally` are finalizer frames.
- `fork` starts a new root that inherits the Context but no handlers.
- `handle` is a prompt, and a control operation captures the frames above it.
- A function compiles to plain JavaScript only when the compiler proves it cannot reach a pause or a capture.

#### Why not lower to `Effect.gen` first

An earlier proposal compiled Effeck through `Effect.gen` and deferred delimited control to a later experiment. That cannot be the foundation, for four reasons that are facts about the running system:

1. `Effect.gen` and `flatMap` push frames onto one fiber stack whose only delimiter is the root. They sequence work. They cannot hand "the rest of the computation up to here" to a handler, because nothing marks "here". When the handler is on another fiber, the operation is simply unhandled (T7b, T8).
2. `Effect.gen` drops its generator on failure and interruption without running JavaScript `finally` (F1-F3). A language that compiled `finally` to generator `finally` would leak on every failure.
3. Wrapping an API in `Effect.callback` parks the whole fiber or nothing. No frame inside the fiber can act as the handler for part of the stack.
4. A service method always returns to its caller exactly once. It cannot end the caller's remaining work with a value, defer it, or store it.

General delimited control therefore needs new frame kinds (`Prompt` and `Owner` above all) and capture, resume, and abandon operations. Those are a runtime extension; no choice of code generation supplies them.

#### The central test

> Does the language make Effect's continuation runtime its foundation, with everything else derived from that model?

**Claim** [falsifiable]. Every feature is a rule over the machine in [Chapter 20](#20-the-machine), which has three continuation mechanisms: frames, root capture, and prompt capture. Effect provides frames and root capture; the kernel provides prompt capture. The claim is false if any feature needs a mechanism outside the machine: a second scheduler, a second Context, a JavaScript `try` the machine cannot see, or a calling convention that skips frames where a pause or capture is reachable. Plain JavaScript calls (Chapter 15) skip frames only where the compiler proves no such point is reachable, which is the frame invariant (Chapter 26) applied, not an exception to it.

| Feature | Mechanism in the machine | Captures | Evidence |
| --- | --- | --- | --- |
| Program entry | A runner creates a fiber root for a program value | No | Chapters 13, 23 |
| Calls and returns | `Ret` frames; plain calls only where no pause or capture is reachable | No | T6a, T17; F9; `ch02-functions.effeck`, `ch13-plain.effeck` |
| `throw` and `catch` | Abortive unwinding to a `Catch` frame that recovers only causes made of expected failures | No: aborts | K14a-K14c; `resumeWith` test; `ch15-failures.effeck` |
| `finally` and `using` | `Final` frames on every exit, abandonment included | No | T1, T3, T4, T10, K11b; `regions.effeck` |
| Services | In-place evidence in the Context | No: runs in place | In-place evidence test; `ch04-services.effeck` |
| Providers and Layers | Installing evidence: `Restore` frames plus a Scope | No | Effect (Chapter 24) |
| Test overrides | Services: a `Restore` frame. Control operations: a handler for one region | Services: no. Control operations: yes | Override test; Chapter 9 |
| Async I/O, sleep, waits | Root capture with one-shot resume | Yes, at the root | T1 step ③; F5, F6 |
| Callback adapters | Root capture with a reported race and bounded wake-ups; masked or interruptible acquisition | Yes, at the root | T5b, T5c, K10a, K10b, K15a-K15c |
| Interruption | Asynchronous unwinding of the fiber's continuation, deferred by `Mask` frames | No | T4, T12 |
| Structured concurrency | New roots with a Context snapshot and no prompts; block-owned forks | No | F7, F8, T8 |
| Control handlers | Prompt capture, resume, abandon, ownership | Yes | T1-T4, T9, T13, T15; O01-O08; A01-A04; I03-I11; K12; `ref6-handlers.effeck` |
| Producers (`function*`) | One prompt capture per element; abandonment on `break`; constant stack under a tail resume | Yes | T10, T16, T17, K11a-K11d; `stack-depth.effeck` |
| Retry, race, all | Fresh runs of a program; nothing reused | No | T11 |

Features checked against the claim and kept out:

- **Multi-shot handlers** (backtracking, probabilistic choice). A multi-shot feature needs copyable frames, and Effect's frames are not copyable: they hold live generators and closures bound to one fiber.
- **Durable execution.** It needs serializable continuations, a different machine. VibeLang reversed course on it twice in ten days (`vibelang.md` §9).

**Verdict.** Yes. Two independent reviews (2026-10-16) reached the same verdict and found five defects in how the machine was implemented or lowered, not in what it is; all five are fixed with regression tests (K11-K15). A prototype compiler then lowered 46 example files to the machine mechanically and ran them, with no feature needing a mechanism outside it (Chapter 27). Two qualifications remain, both tracked in Chapter 30. Prompt capture relies on undocumented Effect fields, so the foundation is real but not yet a supported Effect API (Chapter 25, G1). And the checker rules that make it safe to program against (fiber crossing, second-class continuations, control classes) are specified here but not built (G8).

### 19. The checkout trace, step by step

This chapter follows one program through the machine before Chapter 20 defines the machine. The program is the checkout from Chapter 9, simplified, with a request-scoped audit log added so the trace shows which Context each piece of code sees. Kernel test T1 runs exactly this program, lowered by hand.

```ts es
service AuditLog { info(message: string): void!! }
service Inventory { reserve(cartId: string): Hold!!Scope }
service Reviewers { decide(request: string): boolean!! }

control Approval {
  approve(request: string): boolean!!
}

function checkout(cartId: string) {
  using hold = Inventory.reserve(cartId)        // released when checkout's body exits
  const ok = requestApproval(cartId)
  if (!ok) throw new Declined({ cartId })
  return `charged ${cartId} holding ${hold}`
}

function requestApproval(cartId: string) {
  AuditLog.info(`approval needed for ${cartId}`)
  const ok = Approval.approve(`checkout ${cartId}`)   // the capture happens here
  AuditLog.info(`approval ${ok ? "granted" : "denied"}`)
  return ok
}

const receipt = handle(
  () => Effect.provideService(checkout("c-42"), AuditLog, RequestAuditLog),
  Approval,
  {
    approve(request, k) {
      AuditLog.info(`clause handling "${request}"`)   // runs outside the provide: AppAuditLog
      const decision = Reviewers.decide(request)      // the whole fiber waits on a timer
      AuditLog.info(`clause resuming with ${decision}`)
      return resume(k, decision)
    }
  }
)
```

The generated TypeScript for the `handle` (the runtime package's API is shown as the compiler will emit it; the kernel prototype also takes a label):

```ts gen
const receipt = yield* handle(Approval,
  Effect.provideService(Effect.suspend(() => checkout("c-42")), AuditLog, RequestAuditLog),
  {
    approve: Effect.fnUntraced(function* (request: string, k: Continuation<boolean, never, string>) {
      const auditLog = yield* AuditLog
      yield* auditLog.info(`clause handling "${request}"`)
      const reviewers = yield* Reviewers
      const decision = yield* reviewers.decide(request)
      yield* auditLog.info(`clause resuming with ${decision}`)
      return tailResume(k, decision)              // a tail resume: returned, never yielded
    })
  })
```

#### The machine, step by step

Notation: K is the fiber's frame stack, listed bottom to top; Γ is its Context. `Prompt` marks where `handle` was installed, `Restore` undoes a `provideService`, `Final(S₁)` closes `checkout`'s Scope, `Owner(k)` holds the captured continuation while the clause runs, and `Clause(k)` performs a tail resume the clause returns. `checkout`'s `using` makes its body a Scope region, so `checkout` has two frames: the function frame and the region frame `checkout/scope` (Chapter 26).

```text
① just before the perform (fiber 1)
   K = … main │ Prompt(Approval, Γ₀) │ Restore(Γ₀′) │ checkout │ Final(S₁) │ checkout/scope │ requestApproval
   Γ = { AuditLog: RequestAuditLog, Approval → prompt, Scope: S₁ open }

② Approval.approve(…) captures 5 frames
   k.K₁ = [ Restore(Γ₀′) │ checkout │ Final(S₁) │ checkout/scope │ requestApproval ]
   k.Γ  = { AuditLog: RequestAuditLog, Approval → prompt, Scope: S₁ open }
   K    = … main │ Owner(k) │ Clause(k) │ clause
   Γ    = Γ₀ = { AuditLog: AppAuditLog, … }                   the clause runs at the prompt

③ Reviewers.decide → Effect.sleep → fiber 1 parks at its root
   K is intact: … main │ Owner(k) │ Clause(k) │ clause │ (sleep's frames)
   another fiber runs; S₁ is still open; the hold is still held

④ timer fires → fiber 1 wakes at the root → the clause returns tailResume(k, true)
   → Clause(k) resumes k; Owner(k) is inert and is dropped
   K = … main │ Prompt(Approval, Γ₀) │ Restore(Γ₀′) │ checkout │ Final(S₁) │ checkout/scope │ requestApproval
   Γ = k.Γ                                                RequestAuditLog and S₁ are back

⑤ requestApproval returns true → checkout returns → Final(S₁) releases the hold
   → Restore → Prompt restores Γ₀ → handle returns the receipt
```

The kernel's own trace from `bun test tests/trace.test.ts` (frame labels: `OnExit` is Effect's restore or finalizer frame, `fn x` is a compiled function frame, `Iterator` is an `Effect.gen` frame). T1 names the service `Logger` and its two implementations `AppLogger` and `RequestLogger`. T1 writes its clause by hand as a generator that ends in `return yield* resume(k, decision)`, which is not a tail resume, so its reinstated stack keeps the clause's frames (`Owner`, `Clause`, and the clause's `Iterator`) under the new prompt. Step ④ above shows the compiled shape, which drops them (K11a):

```text
capture Approve on fiber 1: 5 frames
  segment (bottom->top): OnExit | fn checkout | OnExit | fn checkout/scope | fn requestApproval
  saved context: Logger=RequestLogger, Approve->Prompt, Scope(Open)
resume Approve on fiber 1
  reinstated top: OnExit | OnExit | OnExit | Iterator | Owner | Clause | Iterator | Prompt | OnExit | fn checkout | OnExit | fn checkout/scope | fn requestApproval
  restored context: Logger=RequestLogger, Approve->Prompt, Scope(Open)

events:
  reserve c-42
  [RequestLogger] approval needed for c-42
  [AppLogger] clause handling "checkout c-42"
  other fiber ran while the main fiber was suspended
  reviewer approved "checkout c-42"
  [AppLogger] clause resuming with true
  [RequestLogger] approval granted
  release hold-1
```

#### What the trace shows

- **Two continuations exist at step ③, of different kinds.** `k` is a handler continuation: five frames owned by the clause. The fiber's own stack, parked by `Effect.sleep`, is a root continuation owned by the scheduler. Both are one-shot; the first is the kernel's, the second is Effect's.
- **The handler runs outside its own `provide`.** The clause logs through `AppAuditLog`; the resumed body logs through `RequestAuditLog` again (Chapter 22).
- **Resources stay open while a continuation waits.** The hold is released once, at step ⑤, when `checkout` returns.
- **Ownership is explicit.** `Owner(k)` stays under the clause until the clause finishes. A clause that ends with a tail resume finishes first, so its owner is dropped as `k` resumes; a clause that does work after `resume` keeps it until that work ends (Chapter 20).

### 20. The machine

```ts es
function checkout(cartId: string) {
  using hold = Inventory.reserve(cartId)
  try {
    return Payments.charge(cartId)
  } catch (e) {
    return refund(cartId, e)
  }
}
```

```text
fiber._stack while Payments.charge runs, bottom to top:

  … main │ Ret(checkout) │ Final(close scope: release hold) │ Catch(PaymentError → refund) │ Ret(charge)
           a call          `using`                            `try`/`catch`                  a call
```

Every construct in the function is a frame on the fiber's stack. The machine defines what every Effeck program means; syntax, types, and the compiler exist to produce and check its states. It is not a model of Effect's runtime that we hope matches: it is Effect's runtime plus a few frame kinds and four operations, and the kernel prototype runs it.

In the machine's terms, Effect supplies two things. It supplies the representation: frames on `_stack` and the Context. It also supplies the **root handler**: the scheduler at the bottom of every fiber, which handles the runtime operations (wait, yield, fork, interrupt) by parking and resuming whole fiber stacks. Every other handler is a prompt above the root. That is the precise sense in which the language is the effect system: every feature is a rule over frames, prompts, and one-shot continuations, and Effect's run loop is one handler in that semantics rather than a library the language calls.

#### Objects

| Term | Definition | Representation on Effect 4 |
| --- | --- | --- |
| Computation | One run of code. It returns a value, fails with an expected error, dies with a defect, is interrupted, or performs an operation. Every function body is a computation. | Primitives evaluated by a fiber |
| Program | A cold, reusable description of a computation. Constructing it runs nothing. Each run creates a new execution with fresh frames. | An `Effect` value; in source, a function with no arguments (Chapter 13) |
| Fiber | One execution: a frame stack, a Context, an interruptibility flag, a pending interruption, and child fibers. | `FiberImpl`, `internal/effect.ts:487-743` [Effect] |
| Frame | A piece of the rest of the computation: an activation waiting for a callee, or a boundary that acts when control leaves it. | An object on `fiber._stack` with success, failure, and ensure hooks, `internal/core.ts:31-61` [Effect] |
| Operation | A request a computation makes of its environment: a service method, a control operation, or a runtime request such as "wait for this callback". | A Context key |
| Tail-resumptive operation | One whose handler always resumes exactly once, as its last step. Every service method is one. | Runs in place; nothing is captured (Chapter 4) |
| Evidence | What the Context holds for an operation: an in-place implementation, or a reference to a handler's prompt. | A Context entry [Kernel] |
| Prompt | A frame that delimits capture for one handler and records the Context and interruptibility to restore when control leaves it. | Kernel `PromptFrame` [Kernel] |
| Delimited continuation | The frames between a perform site and the matching prompt, plus the Context and interruptibility at the perform site. It is live, one-shot, and owned. | Kernel `Continuation`: a spliced `_stack` segment [Kernel] |
| `Suspended` | A continuation whose ownership was transferred to a Scope region: resumable once, abandoned when its region closes or its capturing fiber exits. Internal to the standard library in v0. | Kernel `Suspended` [Kernel] |
| Execution boundary | A frame or fiber edge that limits capture, restores state, or changes ownership (Chapter 23). | |

#### State and frames

```text
Fiber = ⟨ K, Γ, ι, π, C ⟩                                    Effect 4 field
  K  frames, bottom → top                                   fiber._stack
  Γ  Context: services, references, current Scope,          fiber.context
     tracing span, operation evidence
  ι  interruptible                                          fiber.interruptible
  π  pending interruption cause                             fiber._interruptedCause
  C  child fibers                                           fiber._children

Frame                                                        Effect 4 frame
  Ret(f, pc, locals)  activation of f, waiting at pc          Iterator (generator) or OnSuccess
  Restore(Γ′)         on exit: Γ := Γ′                        OnExit pushed by updateContext
  Mask(ι′)            on exit: ι := ι′                        setInterruptible
  Final(fin)          on exit: run fin(exit) with ι = false   OnExit; Scope close
  Catch(h)            on a cause made only of Fail reasons:   catchCause frame (runtime catchExpected)
                      run h(first failure)
  Prompt(H, Γₒ, ιₒ)   delimiter for handler H;                kernel PromptFrame
                      on exit: Γ, ι := Γₒ, ιₒ
  Owner(k)            on exit: abandon k if it is still       kernel OwnerFrame
                      suspended and still owned here
  Clause(k)           on a returned tail-resume request:      kernel ClauseFrame
                      resume k
  Collector(k)        bottom of an abandoned segment:         kernel AbandonCollector
                      restores the abandoner's state
  FiberExit           bottom of a fiber that transferred:     kernel FiberExitFrame
                      abandons its still-suspended transfers
```

Every frame kind down to `Catch` already exists in Effect 4 [Effect]; `Catch` is Effect's `catchCause` frame with the handler the runtime's `catchExpected` gives it. The kernel adds the last five kinds and four operations: capture, resume, abandon, and transfer [Kernel].

#### Transitions

These rules are the specification. Within one rule, changes to K, Γ, ι, π, and continuation state happen atomically with respect to scheduling and interruption.

| Rule | When | Then |
| --- | --- | --- |
| call | `f(v)` that may suspend | Push `Ret(caller, pc)`; run the body of `f`. |
| return | `return v` meets `Ret(g, pc)` | Pop it; continue `g` at `pc` with `v`. |
| | meets `Restore(Γ′)` or `Mask(ι′)` | Pop it; restore Γ or ι; keep returning `v`. |
| | meets `Final(fin)` | Pop it; run `fin(Success v)` with ι = false; keep returning `v`, or fail with `fin`'s failure. |
| | meets `Prompt(H, Γₒ, ιₒ)` | Pop it; Γ, ι := Γₒ, ιₒ; continue with `H.onReturn(v)`. |
| | meets `Clause(k)` with `v` a tail-resume request for `k` | Pop it; perform `resume(k, x)` or `resumeWith(k, p)` as the request says. Any other `v` passes through. |
| | meets `Owner(k)` | Pop it; if `k` is suspended and owned here, abandon `k` first; keep returning `v`. |
| perform, in place | `op(v)` and Γ[op] is an in-place implementation `impl` | Call `impl(v)` here, under the current Γ. Nothing is captured. |
| perform, capture | `op(v)`, Γ[op] refers to H, and K = K₀ · Prompt(H, Γₒ, ιₒ) · K₁ | k := ⟨K₁, Γ, ι, H, suspended⟩. An abandonment abort in π that belongs to K₁ moves into `k`. K := K₀ · Owner(k) · Clause(k); Γ, ι := Γₒ, ιₒ; run the clause `H.op(v, k)`. |
| perform, unhandled | Γ[op] absent; H's prompt is on another fiber; or H's prompt has exited | Defect `UnhandledOperation`, `HandlerNotOnThisFiber`, or `HandlerPromptExited`. The checker rejects these programs first. |
| resume | `resume(k, x)`, k suspended, same fiber | k := resumed. If `Owner(k)` is on top, or `Clause(k)` over `Owner(k)`, pop them (a resume in tail position). Push `Prompt(H, Γ, ι)` then K₁; Γ, ι := k.Γ, k.ι; reinstate k's moved abort in π; deliver `x` at the perform site. `resume` returns when the handled computation reaches the reinstalled prompt. |
| resumeWith | `resumeWith(k, p)` | As `resume`, but run the program `p` at the perform site. `p` may fail only with the operation's declared E, and its R is `never` (Chapter 9). |
| abandon | `abandon(k)` or `abandonNow(k)`, k suspended, same fiber | k := abandoned; push `Collector(k)`, then K₁; Γ, ι := k.Γ, k.ι; add a fresh abort to π (never replacing a real pending interruption); unwind K₁. The abort re-raises whenever K₁ re-enters an interruptible region. `Final` frames run; `Catch` frames do not. The collector removes only its own abort from π. `abandonNow` returns the unwinding's Exit; `abandon` fails with every cleanup reason plus every real pending interruption, and never reports its own abort (K12a-K12e). |
| transfer | `transfer(k)` in the clause; standard library only (D16) | Owner of k := a fresh token registered with the Scope in Γ (the region around the handler); the capturing fiber's `FiberExit` frame also holds k. Returns `Suspended(k)`. Closing that Scope abandons k only if the same token still owns it; the fiber's exit abandons it if it is still suspended then (K13a-K13d). A Scope with parallel finalizers is rejected (K13e). |
| throw | `throw e` | Fail with `Fail(e)`. |
| fail | failure meets `Ret` | Discard the frame. |
| | meets `Catch(h)`, every reason of the cause is a `Fail`, not interrupting | Continue with `h(e)`, where `e` is the first failure. A cause with a `Die` or an `Interrupt` reason passes through intact (K14a-K14c). |
| | meets `Final(fin)` | Run `fin(Failure c)` with ι = false; keep failing with `c` plus `fin`'s failures. |
| | meets `Restore`, `Mask`, `Prompt`, `Clause` | Restore state; keep failing. |
| | meets `Owner(k)` | Abandon `k` first if it is owned here and suspended; keep failing with `c` plus any cleanup failure. |
| interrupt | request `r` from another fiber | π := π ⊕ r. If ι and parked: fail with π now. If ι and running: fail with π at the next step. If not ι: fail when a `Mask` frame restores ι = true. While π ≠ ∅ and ι, unwinding skips `Catch` frames. |
| wait (root) | `suspend(register)` | Park the fiber with K intact. `register` receives a one-shot resumer. |
| wake | `resumer.resume(v)` | Run the fiber with `v`: synchronously while the wake depth is under budget, otherwise on the scheduler (Chapter 24). |
| fork | `fork(p)` | New fiber ⟨[], Γ, true, ∅, ∅⟩ runs `p`. Its owner is the innermost block region (`fork`), the caller's Scope (`Effect.forkScoped`), the parent fiber (`Effect.forkChild`), or nothing (`Effect.forkDetach`). No prompt crosses. |

The interruption rows describe Effect 4 as it is (`internal/effect.ts:558-579`, `internal/core.ts:600-631`) [Effect]. The prompt, owner, clause, collector, fiber-exit, capture, resume, abandon, and transfer rows are the kernel (`packages/runtime/src/kernel/kernel.ts`) [Kernel].

**Tail position is structural.** A resume is in tail position when `Owner(k)` (with `Clause(k)` above it) is the top of the stack at the moment it runs. In a generator clause, `return yield* resume(k, v)` runs with the clause's own frame above them, so it is not a tail resume, and a producer looping that way keeps every element's clause frames until the loop ends (review probe R10: 3,002 frames at 1,000 elements). The compiler therefore emits `return tailResume(k, v)`: the clause's generator finishes first, and `Clause(k)` performs the resume (K11a). Inside a `try`, `catch`, `finally`, or `using` region of the clause, the compiler emits `yield* resume(…)` instead, because the region's cleanup must run after the resumed computation (K11d).

#### Choices the machine makes

| Choice | Alternatives | Decision and reason |
| --- | --- | --- |
| Deep or shallow handlers | Shallow: resume without reinstalling the prompt | **Deep** [Rule]. A loop that performs an operation many times (T9, T17) needs the handler every time. A tail resume drops its inert owner, so iterating a producer 100,000 times keeps the stack at 3 frames (K11a). Shallow handling is a different operation and can be added later. |
| Handler selection | Dynamic: nearest prompt on this fiber. Lexical: handlers passed as capabilities (Effekt) | **Dynamic** [Rule]. Effect's Context is already dynamically scoped, and DI must stay Effect-compatible. Lexical capabilities would need a second, value-passing DI system. |
| Where a clause runs | Inside or outside its prompt | **Outside**, in Γₒ [Kernel]. A clause that performs its own operation reaches the next handler out instead of recursing. |
| Context across resume | Snapshot, or delimited dynamic binding | **Snapshot** [Rule], Chapter 22. |
| Where a continuation can be consumed | Any fiber, or only the capturing fiber | **Capturing fiber only**, for resume, transfer, and abandon [Rule]. Effect frames are bound to the fiber that pushed them (T14b), and children, identity, and interruption authority stay coherent (O04, O06). The capturing fiber's exit releases what it transferred (K13a). |
| Double resume | Ignore, or defect | **Defect at the caller** for handler continuations [Rule]. Root waits keep Effect's first-wins behavior but report the loss: `resume` returns `false` (Chapter 21). |
| Which causes `catch` recovers | First failure of any cause (`Effect.catch`), or only causes made of expected failures | **Only expected failures** [Rule] (D13). A recovered mixed cause would drop a defect (review probe R1). |

### 21. One-shot ownership and races

```ts es
approve(request, k) {
  const first = resume(k, true)
  return resume(k, false)        // defect ContinuationAlreadyConsumed, raised here (T2)
}
```

```ts ts
socket.once("connect", () => {
  if (!waiter.resume(socket)) socket.destroy()   // lost the race to cancellation: still ours to release
})
```

Both snippets consume a continuation a second time, or too late, and the machine answers differently in each case. One representation has three holders, and the law for a second consumption depends on the holder:

| Holder | What it holds | Law on a second consumption | Why it differs |
| --- | --- | --- | --- |
| A handler clause | A prompt continuation `k` | Defect at the caller | A clause that resumes twice is a programming error inside Effeck |
| A foreign callback resuming a root wait (`suspend`) | The whole parked fiber | `resume` returns `false`; the caller keeps the value | Foreign producers race legitimately with cancellation and must learn they lost |
| A foreign producer under `handoff` | The whole parked fiber | `HandoffProtocolViolation` thrown into the producer | The producer promised one delivery and quiescence; breaking that promise is the producer's bug |

#### States

```text
                       resume(k, x) / resumeWith(k, program)
        suspended ─────────────────────────────────────────► resumed
          │   ▲
          │   └── transfer(k), standard library only: owner := the Scope region
          │       around the handler (a second transfer revokes the first)
          │
          │ abandon(k), or its owner exits: the clause returns, fails, or is
          │ interrupted; the owning Scope region closes; or the capturing fiber exits
          └─────────────────────────────────────────────────► abandoned

  resume on resumed or abandoned        → defect ContinuationAlreadyConsumed at the caller   [T2, O03, K13a]
  abandon on resumed or abandoned       → no effect                                          [Kernel]
  resume, abandon, or transfer from another fiber → defect ContinuationWrongFiber            [O04, T14, O06]
```

#### Owners

[Kernel] [Rule]

- **The clause.** By default, `k` is owned by the clause invocation that received it, through an `Owner(k)` frame under the clause. If the clause exits by any path while `k` is still suspended, `k` is abandoned before control leaves the clause (T3, T4). An `Owner` nested inside a captured segment abandons its own continuation when that segment is abandoned (O01, O02).
- **A Scope region, after `transfer`.** `transfer(k)` moves ownership to the Scope region around the handler, the Scope in the clause's Context, and returns a `Suspended` handle that `resume`, `resumeWith`, and `abandon` accept (T10). Closing that region abandons `k` if it is still suspended and still owned by that transfer (O03). A second transfer revokes the first (O08). Transfer into the Scope parked inside `k` itself, a direct ownership cycle, is a defect (O07). Transfer into a Scope with parallel finalizers is a defect, `TransferIntoParallelScope`, because Effect runs each parallel finalizer on a forked fiber, which could never abandon `k` (K13e).
- **The capturing fiber, always.** A transferred continuation never outlives the fiber that captured it. The first transfer on a fiber puts a `FiberExit` frame at the bottom of its stack, and when the fiber's computation ends by any path, that frame abandons every transferred continuation still suspended, newest first, before the fiber's exit is published; cleanup failures join that exit (K13a-K13d). Without it, a child that transferred into a Scope inherited from its parent and then exited left a segment no fiber could release (review probe R5).

`transfer` stays inside the standard library in v0 (D16). Producers' external iteration uses it on one fiber, where every rule above holds. One case is still a defect rather than a release: closing the owning Scope from another fiber while the capturing fiber is alive raises `ContinuationWrongFiber` at the closer, and the segment is released when the capturing fiber exits (K13c). Routing that close to the capturing fiber is G3.

Every consumption is fiber-affine, transfer and abandonment included. Effect frames are closures over the fiber that pushed them: `updateContext`'s restore frame calls `setContext` on that fiber (`internal/effect.ts:2200-2208`) [Effect]. An earlier kernel version abandoned on whichever fiber closed the Scope; T14b caught it rewriting the capturing fiber's live Context.

Garbage collection is never the cleanup mechanism. A suspended continuation with no owner cannot exist: capture assigns an owner atomically. No production runtime cleans up abandoned continuations from the collector; OCaml's attempt stalled over deadlocks (`continuation-prior-art.md` rule 4). Development builds may register suspended continuations with a `FinalizationRegistry` to report one collected while still suspended, as Swift's checked continuations log "leaked its continuation". That report is a diagnostic, never cleanup [Rule].

#### Double resumption

A second consumption is a defect raised at the second caller (T2). Nothing in the continuation runs twice. This differs from `Effect.callback`, which silently ignores every resume after the first (F5). Silent dropping suits racing external callbacks, which is why root waits report instead; it is wrong for a handler that resumes twice by mistake.

#### Cancellation versus resumption

JavaScript runs one fiber step at a time, so a race between a resume and an interruption is decided by which one reaches the fiber first.

| Case | Outcome |
| --- | --- |
| Handler continuation `k`; the fiber is interrupted while the clause holds `k` | The clause unwinds, then `Owner(k)` abandons `k`, then outer frames unwind. Order verified: clause finalizer, continuation finalizer, outer finalizer (T4). |
| Handler continuation; interruption arrives while the clause is masked | Recorded in π. Delivered at the perform site the moment `resume` restores an interruptible state; the body's finalizer runs and its next statement does not (T12). |
| A real interruption is pending, or arrives, during an abandonment | Kept in π alongside the abandonment's own abort, then delivered with every cleanup failure; the internal abort never appears in the final cause (I03, I10, K12a, K12b). |
| Root wait (`suspend`); resume wins | The value is delivered once; a second resume returns `false` (T5c). A later interruption unwinds normally. |
| Root wait; interruption wins | Registration cleanup runs; the late `resume` returns `false`, so the producer still owns the value and can release it (T5b). Plain `Effect.callback` drops the late value silently and returns nothing to the producer (T5a). |
| Root wait; resume accepted but deferred by the wake budget; interruption arrives before delivery | The kernel releases the value with the waiter's `release` function (K10b, W04). |
| Root wait; `register` accepts a value, then throws | The value is released, and the fiber dies with the registration's defect (W05). |
| Root wait inside a masked `acquireRelease` (the default) | The interruption is only recorded while the fiber waits. The value is delivered, its release is registered, the mask ends, and the release runs. The registration's cancellation hook never runs (K15a). |
| Root wait inside `acquireRelease(…, { interruptible: true })` | Cancellation wins at once; the registration's cleanup runs. A later `resume` returns `false`, and a value accepted but not yet delivered is released by `suspend`'s `release` (K15b, K15c). |
| Producer cannot check a return value (foreign event source with asynchronous shutdown) | `handoff`: cancellation seals the gate, waits for the producer to acknowledge quiescence, and releases any late delivery before cancellation completes (H2). A deferred delivery that loses is released too (H3). A delivery after quiescence is a protocol violation raised to the producer (H4). Adapted from the independent Codex design. |

**A delivered value is an ownership transfer** [Rule]. A value that needs release (a socket, a lease) is owned by exactly one party at every instant: the producer until the gate accepts it, the kernel while a delivery is in flight, and the receiving computation's Scope after delivery. The receiving side registers release before any interruption can be accepted: `Effect.acquireRelease` registers it under its mask after acquisition succeeds, in both policies (`internal/effect.ts:4143-4159`). Which policy a resource uses is the resource's choice, and `using` adds none (D25).

### 22. What resume restores

```ts es
handle(() => Effect.provideService(checkout("c-42"), AuditLog, RequestAuditLog), Approval, {
  approve(request, k) {
    AuditLog.info("in the handler")   // AppAuditLog: the clause runs outside the provide
    return resume(k, true)            // checkout continues with RequestAuditLog, its open Scope, its span
  }
})
```

A resumed computation gets back exactly the state it had when it was captured, and the handler never sees the inside of the region it handles. The full rule, state by state:

| State | At capture | While the clause runs | At resume | When the computation leaves the reinstalled prompt |
| --- | --- | --- | --- | --- |
| Frames between perform site and prompt | Moved into `k` | Parked in `k`; nothing runs; scopes stay open | Moved back above a new `Prompt` | Pop normally |
| The selected handler | Its prompt is removed | Absent. The clause runs outside its own handler, so performing the same operation reaches the next handler out [Kernel] | Reinstalled at the resume site (deep handler) | Popped |
| Context Γ: services, operation evidence, current Scope, tracing span | `k.Γ` := Γ, a pointer to an immutable snapshot | Γ := Γₒ, the Context outside the prompt (T1: the clause logs with `AppAuditLog`) | Γ := `k.Γ` (T1: `RequestAuditLog` and the open Scope are back) | Γ := the Context at the resume site (T13) |
| Interruptibility ι | `k.ι` := ι | ι := ιₒ | ι := `k.ι`; a pending interruption is delivered at once if `k.ι` (T12) | ι := the value at the resume site |
| Pending interruption π | Real requests stay with the fiber; an abandonment abort unwinding inside the segment moves into `k` | May receive new requests | Real requests are never rolled back; `k`'s moved abort is reinstated (I11) | |
| Fiber identity | Unchanged | Same fiber | Same fiber, enforced | |
| Children | Stay attached to the fiber or their Scope; keep running | | Unchanged | |
| Tracing span | Part of Γ: the clause sees the outer span | | The body sees its own span again; each span ends exactly once, when its frame exits (G6a, T01) | |
| Node `AsyncLocalStorage` | Not split. It follows the JavaScript async context, which a same-fiber capture never changes. Effect captures it only when a fiber parks and when a fork is scheduled (`internal/effect.ts:586-598, 5545`) | A store set inside the handled region is visible in the clause (G6b) | Unchanged | |

#### Snapshot Context, not delimited dynamic binding

[Rule] Two rules exist in prior art, and they disagree:

| Rule | What a resumed computation sees for a binding made outside the delimiter | Who uses it |
| --- | --- | --- |
| Delimited dynamic binding (DDB) | The resumer's binding | Kiselyov, Shan, and Sabry (ICFP 2006); Racket continuation marks; Koka evidence vectors; handler lookup in OCaml 5 and WasmFX |
| Snapshot at suspension | The binding it had when it suspended | TC39 AsyncContext; Effect 4 (PR #8629: "a fiber resumes in the async context it had when it suspended"); Kotlin `CoroutineContext`; Swift task-locals; tokio task-locals |

TC39's AsyncContext (Stage 2) is the closest analogue in JavaScript itself. Its specification text snapshots the context at each `await`, so code after the `await` resumes in the context it had at the `await`, and it runs a generator body in the context captured when the generator object was created, never in the context of whoever calls `.next()` (`docs/research/tc39-syntax.md` §8).

Effeck takes the snapshot rule for three reasons:

1. **Effect's frames can only carry a snapshot.** `updateContext` pushes a frame that restores an absolute previous Context, captured in a closure (`internal/effect.ts:2186-2211`). Under DDB, a segment spliced under a different resumer would still restore the capture-time outer Context when its inner `provide` exits. That is wrong under DDB, and closures cannot be rebased. DDB would need Effect to store Context deltas instead.
2. **Dependency injection stays stable.** A resumed computation keeps the providers it selected. Under DDB, where and when a continuation is resumed would change which `Database` a half-finished service method talks to.
3. **Leaking a resumer's Context is a security bug class.** Request-scoped services (current user, tenant, credentials) must not flow into another request's computation. Effect 3 shipped exactly that leak through `AsyncLocalStorage` (GHSA-38f7-945m-qr2g, CVE-2026-32887).

The cost is real, and it applies to handler lookup. A continuation transferred out of its outer handlers' region, then resumed where a different handler for the same operation is installed, does not rebind to that handler. It fails loudly with `HandlerPromptExited` instead (T15) [Kernel]. The independent Codex design reached the same choice for the same DI reason. A program that needs the rest of a computation under different providers runs a fresh program under them; it does not rebind a live continuation.

#### `AsyncLocalStorage` is fiber-wide, not delimited

[Kernel] [Rule] Node offers no public way to switch the async context in the middle of a run loop; `AsyncResource.runInAsyncScope` only scopes a callback. G6b shows the consequence: a store entered inside a handled region leaks into the clause. Request-scoped values in Effeck therefore live in Context, which the kernel does delimit. `AsyncLocalStorage` remains a bridge for foreign libraries at fiber granularity, and Effeck code does not write to it.

#### Why the reinstalled prompt records the resume site's Context

After `resume(k, x)` returns, the clause continues; it must see its own Context, not the body's. T13 resumes from inside a clause-local `provide` and checks three things. The body after resumption sees its own provider. The next clause invocation sees the resume site's provider. The clause sees the resume site's provider after `resume` returns.

The reference machine (strategy B, Chapter 25) does not implement this rule yet. Review probe R9 resumes under a clause-local provider and performs again: the kernel's next clause sees the resume site's provider, as specified, while strategy B reuses its old prompt and sees the `handle` site's. The rule stands; aligning strategy B with it is the first law of G16.

### 23. Boundaries and finalization

```ts es
const approved = handle(() => {
  const viaMap = ids.map((id) => Approval.approve(id))                    // EK4301: a JavaScript frame
  const viaFork = forEach(ids, (id) => Approval.approve(id), {
    concurrency: 4                                                        // EK4201: other fibers
  })
  return forEach(ids, (id) => Approval.approve(id))                // fine: same fiber, Effect frames
}, Approval, { approve: (_request, k) => resume(k, true) })
```

Three calls look alike and meet three different boundaries. A capture moves frames between a perform site and its prompt, so whatever sits between them decides whether the capture can happen. The compiler rejects the first two; the runtime would otherwise fail them with `UnhandledOperation` and `HandlerNotOnThisFiber`.

#### Execution boundaries

| Boundary | Created by | Capture through it | State it restores | Owner of what it delimits |
| --- | --- | --- | --- | --- |
| Runner (program entry) | `Effect.runPromise`, `NodeRuntime.runMain`, a test runner such as `it.effect` | Creates a fiber root for a program value. Effeck code never creates roots by calling; only runners and `fork` do (Chapter 13) | | The runner |
| Fiber root | Runners, `fork` | Stops every capture. Runtime operations (wait, sleep, yield) park the whole stack here. | None | The scheduler |
| Prompt | `handle`, the loop of a `for...of` over a producer (Chapter 26) | Stops capture for its operations. Other operations pass through; this prompt then moves with the captured segment. | Γₒ, ιₒ | The clause invocation, or a Scope after `transfer` |
| Scope region | A block that declares `using` or calls `fork` | Transparent: its `Final` frame moves with a captured segment and closes the Scope when the segment unwinds | | The block: its `using` acquisitions and forked fibers, nothing else |
| Catch, Final, Restore, Mask | `try/catch`, `finally`, `provide`, masks | Transparent: the frames move with a captured segment and act when it unwinds | Their own | |
| Owner, Clause | Capture | Transparent to outer captures | None | Its continuation |
| Foreign JavaScript frame | A call into untransformed JavaScript: an `Array.prototype.map` callback, `Effect.runSync` | Opaque. An operation performed below it cannot reach a handler above it. `runSync` starts a new root with an empty Context, so the operation is unhandled there (foreign-frame test). | | |
| TypeScript Effect code | A TypeScript function returning `Effect` | Transparent on the same fiber (T7a), but its body can fork where the checker cannot see, so a TypeScript function whose R holds a control operation is EK4202 | | |
| Plain JavaScript region | Compiler optimization (Chapter 15) | Must contain no capture or pause point | | |
| Program run | Calling a program | Each run allocates fresh frames and continuations. Nothing crosses runs; a continuation from one attempt cannot be resumed in another (T11). | | |
| Workflow execution | Effect's `effect/workflow`: `Workflow.execute`, `toLayer` | The body runs on a forked child (`workflow/Workflow.ts:695-780`), so a handler outside it cannot be reached. Durable suspension interrupts that fiber (`Workflow.ts:1031-1036`): held continuations are abandoned through their owners. Every execution, first or replay, is a fresh program run, and only activities and durable state are journaled (D42) | | The workflow engine |
| Fork | `fork`, concurrent combinators | A new root. Context crosses as a snapshot (F7); prompts do not (T8; probes in Chapter 16). | | The block region, a Scope, the parent fiber, or nothing |

#### Regions and ownership

A block opens a Scope region when it directly declares `using` or calls `fork`; the region closes when the block exits, by any path. Only the `using` acquisitions and the forked fibers of that block belong to it. Any other acquisition in the block keeps `Scope` in the function's requirements, so the caller owns it (D14, D19). Whole-block ownership was the alternative, and review probe R8 shows why it was rejected: it closes a connection the function returns to its caller.

#### Finalization

| Exit | What runs | Evidence |
| --- | --- | --- |
| Return | `Final` frames in LIFO order with `Success` | Effect `OnExit`, `internal/effect.ts:4173-4214` [Effect] |
| Expected failure | Unwind to the nearest `Catch` whose cause is made only of expected failures; `Final` frames on the way run first with `Failure(Fail e)` | [Effect]; K14a |
| Expected failure plus a defect or interruption | Unwind past every `Catch`; `Final` frames run; the cause keeps every reason | K14b, K14c [Kernel] |
| Defect | Unwind past every `Catch`; `Final` frames run | [Effect] |
| Interruption | Like a defect. While the fiber has a pending interruption π and is interruptible, even cause-level `catchCause` frames are skipped (`internal/core.ts:611-627`). A self-raised interrupt cause with no π, such as `Effect.interrupt`, can still be caught by `catchCause` (conformance test D04), but not by source `catch`. `Final` frames run masked | [Effect] |
| Abandonment | The segment unwinds as if interrupted. Finalizers see an interruption Exit, `catchCause` inside the segment does not run, and the abandoner continues only after the unwinding finishes. | T3 [Kernel] |
| Abandonment inside a mask | A recovery handler inside the mask may observe the abort, but the abort re-raises when the mask ends. A segment entirely inside a mask that swallows the abort and returns normally is a defect, `AbandonedContinuationContinued`, at every owner: clause invocation, `abandon`, and Scope (kernel-edges; A04). | [Kernel] |
| Interruption while a continuation is held | Clause finalizers, then the held continuation's finalizers, then outer finalizers | T4 [Kernel] |
| Finalizer failure during abandonment | A release that dies fails the boundary that abandoned: the `handle` (kernel-edges), the caller of `abandon` (A02), the closing Scope region (A01), or the exiting fiber (K13d). The abort itself is not reported; a real pending interruption is reported beside the failure (K12a-K12e) | [Kernel] |
| The capturing fiber exits while it still holds transferred continuations | Each is abandoned on that fiber, newest first, before the fiber's exit is published | K13a-K13d [Kernel] |
| A finalizer that performs a control operation handled outside it | Works when the outer handler resumes, even mid-abandonment: the abort moves with the capture and returns at resume (I11). If the outer handler abandons instead, the finalizer is cut short. Effeck therefore requires a finalizer's control operations to be handled inside the finalizer [Rule; checker] | I11, I06 [Kernel] |

JavaScript `finally` inside an `Effect.gen` body is not a finalizer: Effect never calls the generator's `return()` or `throw()` (F1, F2, F3). Effeck `finally` and `using` therefore compile to `Final` frames, never to JavaScript `finally`, in every function that compiles to frames (Chapter 26).

### 24. What Effect provides and what the kernel adds

The kernel is a TypeScript library over `effect@4.0.5`. This is kernel test T3, written directly against it: a handler that returns without resuming, and the cleanup that follows.

```ts ts
const Ask = operation<void, number>("Ask")

const body = Effect.scoped(Effect.gen(function* () {
  yield* Effect.acquireRelease(note("acquire"), () => note("release"))
  yield* Effect.addFinalizer((exit) => note(`finalizer sees interrupt=${Exit.hasInterrupts(exit)}`))
  const n = yield* Ask(undefined)                  // captures up to the handle below
  yield* note(`never reached ${n}`)
  return n
}))

const result = yield* handle("h", [Ask], body, {
  Ask: () => Effect.as(note("clause returns without resuming"), -100)
})
```

```text
acquire
clause returns without resuming
finalizer sees interrupt=true
release
handle returned -100
```

Everything in that test except `operation`, `handle`, and the capture is ordinary Effect: `Effect.scoped`, `acquireRelease`, `addFinalizer`, the fiber, and the finalizers that run during abandonment. This chapter lists what Effect provides, what the kernel adds, and the Effect behaviors the design works around.

#### Provided by Effect 4, unchanged

| Machine requirement | Effect 4.0.5 mechanism | Evidence |
| --- | --- | --- |
| A reified, heap-allocated continuation | `fiber._stack: Array<Primitive>`; `runLoop` is a trampoline | `internal/effect.ts:516, 625-679` [Effect]; 100,001 frames captured and resumed (T6a) |
| One-shot capture at the root | `Effect.callback`: the first resume wins, later ones are ignored | `internal/effect.ts:1141-1190`; F5 [Effect] |
| Resumption | `fiber.evaluate(effect)` runs the fiber synchronously on the resumer's JS stack | `internal/effect.ts:1155-1164, 583-599`; F6 [Effect] |
| Finalization | `OnExit` frames run masked; Scopes close finalizers in reverse order, sequentially by default | `internal/effect.ts:4173-4214, 3982-4003` [Effect] |
| Cause-aware recovery | `Effect.catchCause` sees every reason of a cause | `internal/effect.ts:2646-2685` [Effect]; the runtime's `catchExpected` is built on it (K14) |
| Dynamic binding | `fiber.context` plus restore frames; services keyed by string | `internal/effect.ts:2186-2211`; `Context.ts:609` [Effect] |
| Interruption | Masks, delivery points, recovery handlers skipped while interrupting | `internal/effect.ts:558-579, 4557-4653`; `internal/core.ts:600-631` [Effect] |
| Interruptible acquisition | `acquireRelease(acquire, release, { interruptible: true })` runs the acquire under `restore` and still registers the release under the mask | `Effect.ts:13366-13370`; `internal/effect.ts:4143-4159`; K15a-K15c [Effect] |
| Structured concurrency | `forkChild` children are interrupted and awaited before the parent's exit is published; `forkIn` and `forkScoped` tie a fiber to a Scope | `internal/effect.ts:600-606, 5491-5673`; F8 [Effect] |
| Fork snapshots Context | A child starts with the parent's Context object; nothing merges back | `internal/effect.ts:5536`; F7 [Effect] |
| Fairness | `shouldYield` after `MaxOpsBeforeYield` = 2048 ops; per-fiber dispatchers batch into one `setImmediate` | `Scheduler.ts:198-200, 294-297, 231-257` [Effect] |
| Layer sharing and ownership | `MemoMap` keyed by Layer object identity; each `Effect.provide` owns one Scope | `Layer.ts:418-466`; `internal/layer.ts:8-22` [Effect] |
| `AsyncLocalStorage` across root waits | Fiber re-enters through a captured `AsyncResource` (effect#8629) | `internal/effect.ts:586-598` [Effect] |

#### Missing, supplied by the kernel

| Requirement | Why Effect cannot express it today | Kernel mechanism | Evidence |
| --- | --- | --- | --- |
| A delimiter inside a fiber | The only delimiter is the root; nothing captures part of `_stack` (`internal/effect.ts:516, 680-701`) | `PromptFrame` pushed from `Effect.withFiber` | T1, T9 |
| Partial capture and resume | No API splices `_stack` | `capture` splices the segment above the prompt; `resume` reinstalls a prompt plus the segment and restores Γ, ι | T1, T13, T6a |
| Tail resume from a compiled clause | A generator clause's frame sits above the owner until the resumed computation returns | `ClauseFrame` performs a `tailResume` request the clause returns, after the clause's frames are gone | K11a-K11d |
| Abandonment with finalization | A dropped segment skips its `OnExit` frames | `abandon` re-pushes the segment over a collector and unwinds it with a sticky interruption that joins, never replaces, real pending interruptions, travels with any capture made during the unwinding, and reports every cleanup reason | T3, T4, kernel-edges, A01-A04, I03, I10, I11, K12a-K12e |
| Ownership | Nothing owns a detached segment | `OwnerFrame` under each clause; `transfer` to the Scope region around the handler, with an owner token; `FiberExitFrame` releases transfers when the capturing fiber exits | T3, T10, T14, O01-O08, K13a-K13e |
| Recovery from expected failures only | `Effect.catch` recovers the first `Fail` of a cause that also holds a defect | `catchExpected` re-fails any cause with a `Die` or `Interrupt` reason | K14a-K14c |
| Double-resume detection | `Effect.callback` silently ignores later resumes | `ContinuationAlreadyConsumed` defect at the caller | T2 |
| Reporting a lost race to the producer | The late resume returns nothing | `suspend`'s resumer returns `false`; `handoff` releases late values after producer quiescence | T5a, T5b, H1-H4 |
| Bounded wake-up depth | No depth guard on synchronous wake-ups | `suspend` delivers inline under a budget of 64 nested wake-ups, then schedules | F10a, K10a, K10b |

#### Effect 4.0.5 hazards the language designs around

| Hazard | Evidence | Consequence for Effeck |
| --- | --- | --- |
| `Effect.gen`, `Effect.fn`, `fnUntraced`, and `fnUntracedEager` never call the generator's `throw()` or `return()`. JavaScript `catch`, `finally`, and `using` inside the body do not run on failure or interruption. | F1-F4 [Effect]; research §8. Effect's language service exempts `try/finally` in `Effect.gen` as "valid cleanup" (language-service#442), which the runtime contradicts. | Source `catch`, `finally`, and `using` compile to Effect frames, never to JavaScript `try`, in any function that can pause (Chapter 26). |
| `Effect.catch` recovers a cause that holds both a `Fail` and a `Die`, and drops the `Die`. | Review probe R1; `internal/effect.ts:158-165, 2665-2685` [Effect] | Source `catch` compiles to `catchExpected` (D13, K14). |
| `Effect.acquireRelease` masks its acquire by default, so a wait inside it cannot be cancelled. | Review probe R3, K15a [Effect] | Resources that wait on the outside world pass `{ interruptible: true }`; `using` keeps the resource's choice (D25). |
| Synchronous wake-ups have no depth guard. A chain of `Deferred`, `PubSub`, fiber-exit, or `Effect.callback` wake-ups overflows: one fiber dies with `RangeError`, and every fiber after it waits forever with no error surfaced. | F10a [Effect]. `scripts/wakeup-chain-depth.ts` breaks on Node 26.10 after about 940 to 1,260 hops, and on Bun 1.4.1 after about 4,100; the research run saw 815 on Node and a Bun livelock at 98% CPU. A regression: effect 3.22.3 completes the same chain at 100,000 hops on both hosts (F10b). | The kernel bounds its own wake-ups (K10a). Effect's own `Deferred` and `PubSub` need an upstream fix (G5). |
| A generator that yields completed Exits bypasses the op budget. | Research §2: 200,000 yields of `Effect.void` before another fiber ran. | The compiler never emits a yield of a known-completed effect inside a loop. |
| Hidden forks: `timeout`, `race`, `raceFirst`, concurrent `all`/`forEach`, `zip` with `concurrent: true`, parallel Scope finalizers, and `Layer.merge` run their arguments on other fibers. | T7b and the fiber-behavior probes [Kernel]; research §4. | Control operations cannot cross these combinators (EK4201), and TypeScript functions with a control operation in R are rejected (EK4202). |
| A service's runtime identity is its key string; two services with the same string alias silently. | `Context.ts:609`; research §5. | The compiler generates key strings from package and module path and rejects duplicates program-wide (Chapter 4). |
| `Effect.ensuring` requires a cleanup whose E is `never`; `Effect.onExit` lets the cleanup fail. | `Effect.ts` (`ensuring`, `onExit` signatures); prototype finding 11 | `finally` lowers to `Effect.onExit` (D46), so a failing cleanup is reported, never rejected by the types. |
| Effect's `Context.Service` class form makes the class's instance type the service identifier. | Prototype finding 19; `prototype/esc/scratch/probe2.ts` | Services compile to an interface plus a `Context.Key` class, so `implements` and parameters typed with the service work (D45). |
| Structurally identical abstract classes are one TypeScript type. | `experiments/scratch-abstract-services/probe.ts`: providing one of two same-shaped classes leaves R empty, and the program dies with `Service not found` | Service classes are nominal (Chapter 4). |
| Frames are closures over the fiber that pushed them. | `internal/effect.ts:2200-2208`; T14b [Kernel]. | Every continuation consumption is fiber-affine (Chapter 21). |
| `fnUntracedEager` runs the body when the function is called. | Research §8. | Never emitted: it breaks the rule that constructing a program runs nothing. |
| `Effect.fn` costs 1.1 µs (Bun) to 2.0 µs (Node) per call; `fnUntraced` costs about 0.1 µs. | Research §12. | Compiled functions use untraced frames; spans are opt-in (`tracing`, Chapter 28). |
| Sibling `Effect.provide` calls do not share Layer builds unless something gives them a common `MemoMap`: an enclosing provide, a `ManagedRuntime`, or an explicitly supplied `Layer.CurrentMemoMap`. The rc.118 migration note says sharing is the default. | Research §6; conformance test D05. | Each `Effect.provide` builds its layers once; provide the application layer once at `main`, or run entry points through a `ManagedRuntime` (D21, Chapter 4). |
| Two providers for one service in a `Layer.merge`: the later one wins, with no warning. | A3 probe [Effect]. | Ambiguity is a compile-time error in `wire` (EK3202, Chapter 4). |
| A Layer cycle through `Layer.suspend` hangs on its own in-progress build. | Research §6. | `wire` detects provider cycles statically (EK3203, Chapter 4). |
| Effect 3 values die in the Effect 4 runtime and vice versa. | Research §10. | Target Effect 4 only. Effect 3 code is foreign code behind a boundary (Chapter 10). |

The [upstream draft index](../upstream/README.md) collects reproductions and proposed fixes for wake-up depth, generator cleanup and tooling, mixed-cause recovery, fairness, Layer memoization documentation, and interruption diagnostics, plus supported-prompt and segmented-stack proposals. Every entry is drafted, not filed (D59). Kernel review defects are tracked separately in `experiments/KERNEL-CHANGES.md`.

#### Scheduling, stack safety, and cancellation

**Scheduling** [Effect] [Rule]. A fiber runs until it parks, completes, or reaches 2048 run-loop steps, then yields through its dispatcher. Woken fibers run on the waker's JS stack (`Deferred`, `PubSub`, fiber exit, `Effect.callback`) or through a dispatcher task (`Queue.offer`, `Latch.open`, `Semaphore.release`). Effeck makes no promise about whether a woken fiber runs before or after the waker's next step; programs that need an order must synchronize explicitly. Fairness is promised only at run-loop steps. A plain JavaScript region is atomic, exactly as an `Effect.sync` body is.

Kotlin and Swift never run a woken coroutine on the waker's stack: Kotlin to avoid stack overflow, Swift to avoid taking over the waker's thread for an unbounded time (`continuation-prior-art.md` rule 12). The kernel keeps Effect's synchronous fast path and bounds its depth instead (K10a). Both choices satisfy the rule above, and Effeck keeps the bounded fast path: always scheduling would add one dispatcher hop to every wake-up, and Effect's own wake-ups already run inline. A measurement that shows waker-blocking time mattering in real programs would change this default, not the rule.

**Stack safety** [Effect] [Kernel] [Rule]. Heap frames make recursion depth a heap question: T6a nests 100,001 compiled calls and captures all of them in one operation. Direct JavaScript recursion is bounded by the host: about 10,150 frames on Node 26.10 and about 39,900 on Bun 1.4.1 for a one-line recursive function (F9). Plain JavaScript functions recurse on that stack, as any JavaScript does (Chapter 15). Capture and resume copy the frames between the perform site and the prompt; the cost is linear in that distance (Chapter 25). Cross-fiber wake-ups are stack-safe only when bounded (K10a); Effect's own are not (F10a).

**Cancellation** [Effect] [Kernel]. Interruption is Effect's: a request is recorded, delivered at once to a parked fiber and at the next step to a running one, deferred by masks, and unwinds with recovery handlers skipped. The kernel adds three things. A held continuation is abandoned through its owner as part of the same unwinding (T4). Abandonment re-raises if a masked handler swallows it (kernel-edges). And a pending interruption is reported beside every cleanup failure, never instead of one (K12a).

#### Division of labor

| Concern | Effect 4, unchanged | Kernel (runtime extension) | Compiler |
| --- | --- | --- | --- |
| Frames, trampolining, fairness | Run loop, `_stack`, op budget | | Emits frames that satisfy the frame invariant (Chapter 26); never yields known Exits in loops |
| Root suspension | `Effect.callback` | `suspend`: reports lost races, bounds wake-up depth, releases deferred values | Lowers adapters (Chapter 10) |
| Partial capture | | `Prompt`, `Owner`, `Clause`, capture, `resume`, `tailResume`, `resumeWith`, `abandon`, `transfer` | Lowers `handle`, operation calls, and tail resumes; checks second-class continuations (Chapter 9) |
| Finalization | `OnExit`, Scopes | Abandonment by sticky interruption; fiber-exit release of transfers | Lowers `finally` and `using` to frames (Chapter 26) |
| Context and DI | `updateContext`, Context keys, Layers, `MemoMap` | Prompt evidence entries | Service keys and accessors, contract checks (EK3101), `wire` graphs (EK3201-EK3204) |
| Interruption | Masks, delivery, skip loop | Owner abandonment during unwinding | Nothing beyond masks in source |
| Fork | `forkChild`, `forkIn`, Context snapshot | Prompts do not cross (`HandlerNotOnThisFiber`) | Block regions for `fork`; fiber-behavior table; EK4201, EK4202 |
| Typed failures | `Fail` causes, `catchCause` | `catchExpected` | `throw` → fail; `catch` → `catchExpected`; contracts (EK2001, EK2002) |
| Plain JavaScript calls | | | Control classes; `$direct` variants (Chapter 15) |
| Producers | | `Yield` as a control operation; `transfer`; the `Stream` adapter | `function*` and `for...of` lowering (Chapter 26) |

### 25. Implementation strategies and costs

```text
$ bun bench/perform.ts          (Bun 1.4.1, Apple silicon; indicative, not a benchmark suite)
plain Effect service call (yield* Counter; next)               105 ns/op
kernel perform, in-place evidence                               87 ns/op
kernel perform, capture + resume, 0 frames in segment          247 ns/op
control: in-place evidence under 10 compiled frames           1141 ns/op
kernel perform, capture + resume, 10 frames in segment        1526 ns/op
control: in-place evidence under 100 compiled frames         10853 ns/op
kernel perform, capture + resume, 100 frames in segment      13101 ns/op
control: in-place evidence under 1000 compiled frames       118108 ns/op
kernel perform, capture + resume, 1000 frames in segment    134409 ns/op
```

A service call through the kernel costs what a service call costs in Effect today. A capture plus resume costs about 165 ns plus 16 to 22 ns per captured frame. Entering one compiled generator frame costs about 110 ns, and that dominates. Two rules follow. Services must run in place, never through capture. And compiled frames, not capture, are the main cost to optimize (Chapter 15).

The run above predates the tail-resume fix, which adds one frame to every clause invocation. A rerun on 2026-10-16, on a machine busy with other work, measured capture plus resume at 223 ns with 0 frames, 1,650 ns with 10, and 14,661 ns with 100: the same shape, with the per-frame cost unchanged.

#### Five candidates

The machine in Chapter 20 is fixed. These are ways to run it. Two were built and tested.

| Strategy | Same-fiber semantics | Capture through TypeScript-authored Effect frames | Depends on Effect internals | Status |
| --- | --- | --- | --- | --- |
| **A. Effect-native prompts.** Prompt and owner frames live on Effect's real `_stack`; capture splices it. | Yes | Yes: through `Effect.forEach`, `Effect.scoped`, `provideService` (T1, T7a) | Yes: `_stack`, `interruptible`, `_interruptedCause`, frame keys, and the interruption skip loop's reliance on `ensureCont` | Built: `packages/runtime/src/kernel/`; 148 tests pass in under a second, including a 54-test conformance suite written by an independent reviewer |
| **B. Explicit frame machine.** A defunctionalized interpreter inside one Effect fiber, using only public APIs. | Yes | No: an embedded Effect is an opaque step; its frames are not the machine's | No | Built independently by Codex (GPT-6.1 Sol): 20 tests pass in about 18 s. Disagrees with A on the Context a reinstalled prompt records (review probe R9), and lacks `transfer` and `onReturn` |
| **C. One fiber per prompt.** The body runs on a child fiber; operations are messages to the parent. | No: the clause and the body run on different fibers, and interruption is relayed | Partially | No | Not built |
| **D. Generator kernel.** Each activation is a generator driven by an Effeck loop inside one fiber. | Yes | No, for the same reason as B | No | Not built; VibeLang's checkpoint built one beside its own host loop |
| **E. CPS or state machines.** A representation of compiled function frames, not a capture strategy. | Under A, yes | Under A, yes | No | Evaluated below |

#### Deep capture

Capture is linear in segment length, so a program that captures at every level of a deep recursion copies quadratically. `bun bench/deep-capture.ts` measures it on strategy A:

```text
depth  1000:     6.6 ms,     501,501 frames copied
depth  2000:     9.6 ms,   2,003,001 frames copied
depth  4000:    23.4 ms,   8,006,001 frames copied
depth  8000:    78.9 ms,  32,012,001 frames copied
```

The growth is quadratic, but the constant is about 2.5 ns per frame copy, so it matters only for very deep producers. The Codex machine's deep test hits the same shape and takes most of its 18 s. Segmented stacks would remove the quadratic term for both strategies, but only Effect can make `_stack` segmented (G17).

#### Decision

**Strategy A is the runtime** [Rule]. It is the only candidate where TypeScript-authored Effect code on the same fiber is transparent to capture (T7a). Seamless Effect interop requires that. It also reuses Effect's finalizers, masks, scheduler, and tracing instead of re-implementing them.

**Strategy B is the reference semantics** [Rule]. Two independent implementations of the same laws make the laws, not an implementation, the specification. Today they pass separate suites, and they do not agree everywhere: B records the `handle` site's Context in a reinstalled prompt where Chapter 22 requires the resume site's (R9). Passing tests in both is therefore not evidence of agreement. G16 writes the Chapter 20 laws once and runs them against both; R9 is its first law. A law that one passes and the other fails is a bug in an implementation or an ambiguity in Chapter 20, never something to paper over.

**What strategy A costs** [Rule]. The kernel reaches undocumented fields structurally, because `package.json` maps `effect/internal/*` to `null`. The pin points:

```text
fiber._stack              read, splice, push, unshift          internal/effect.ts:516
fiber.interruptible       read, write                          internal/effect.ts:514
fiber._interruptedCause   read, write (sticky abort)           internal/effect.ts:520
fiber._deferredInterrupt  cleared when a mask begins           internal/effect.ts:523, 636
frame keys                ~effect/Effect/successCont …         internal/core.ts:46-58
skip loop                 frames stop it only via              internal/core.ts:613-627
                          ensureCont lowering ι
restore closures          bound to the pushing fiber           internal/effect.ts:2200-2208
stack bottom              FiberExitFrame at _stack[0]          internal/effect.ts:516
```

Mitigation: pin `effect` exactly, check the version at load (`kernel/internals.ts`), and run the law suite on every Effect release. The durable fix is upstream: a supported prompt primitive, or at least a public stack-splice and interruptibility API on `Fiber`. The kernel is the concrete reference for the supported-prompt proposal (G1). The [supported-prompt draft](../upstream/effect-prompt-primitive.md) states the capability and its laws; D59 sets the status to drafted, not filed.

#### Representations of a compiled frame

Under strategy A, capture moves whatever objects sit on `_stack`. How a compiled function's activation is represented is a separate choice, judged by the frame invariant (Chapter 26) and by what Effect does to it on failure (F1-F4).

| Representation | Satisfies the frame invariant | Behavior on failure and abandonment | Cost and codegen | Verdict |
| --- | --- | --- | --- | --- |
| Generator frame (`Effect.fnUntraced`) | Yes: each activation is one `Iterator` frame; calls are `yield*` of the callee's Effect, so JavaScript depth stays constant (T6a) | Effect drops the generator without `return()` (F1-F3), so source `catch`, `finally`, and `using` must compile to Effect frames. Nothing else is lost: the frame holds no cleanup | ~110 ns per activation; simplest codegen; readable output; debuggable as ordinary generators | **v0** |
| Explicit state machine (regenerator style: a frame object with `pc` and a locals record) | Yes, directly: the object is `Ret(f, pc, locals)` | Same as generators: the frame holds no cleanup, so the same region lowering applies | Avoids allocating a generator and an iterator per activation; codegen is larger and harder to read | **Later**, if profiles show generator frames dominate |
| CPS with continuation closures | Only if each continuation closure is pushed as a frame. Calling continuations directly builds JavaScript stack and breaks the invariant | A closure called on the JavaScript stack cannot be captured or abandoned | Closure per call site; needs a trampoline to stay stack-safe, which turns it back into frames | **Rejected**; it is the state machine above with extra allocation |
| Plain JavaScript | Only for regions with no capture or pause point (Chapter 15) | A JavaScript exception is a defect (F4) | Fastest | **Optimization** for proven `pure`/`sync` code |

### 26. What compiled code must guarantee

```ts es
function displayName(id: string) {
  return Users.find(id).name
}
```

```ts gen
export const displayName = Effect.fnUntraced(function* (id: string) {
  const users = yield* Users          // in-place evidence from the current Context
  const user = yield* users.find(id)  // runs on this fiber, under this Context
  return user.name
})
```

```ts dts
export declare const displayName:
  (id: string) => string!UserNotFound!Users
```

#### One obligation

> **Frame invariant** [Rule]. At every point where a computation can capture or park, everything needed to continue it lives in frames on the fiber's `_stack`. No JavaScript activation of compiled or foreign code sits between the run loop and that point.

Capture moves `_stack` frames. It cannot move a JavaScript activation. An activation between the run loop and a perform site would be lost on capture, or would force re-execution. Everything else in this chapter follows from that one rule.

#### What a function compiles to

A function that may capture or park compiles to one heap frame per activation. Today that frame is Effect's generator frame, made by `Effect.fnUntraced`, as above. A call to another computation is `yield*` of that computation's Effect, which the run loop evaluates as a separate frame. That keeps JavaScript stack depth constant (T6a).

The generator here is only a frame representation. Control comes from the kernel's prompts, not from `Effect.gen`. The compiler never emits `Effect.fnUntracedEager`, which runs at call time, and traces a function with `Effect.fn` only when the project's `tracing` option asks for it (Chapter 28), because traced frames cost 10 to 20 times more (Chapter 24). A later backend can replace generators with explicit state machines without changing any rule here (Chapter 25).

#### Regions compile to frames, never to JavaScript `try`

```ts es
function profile(raw: string) {
  const id = parseId(raw)                 // may throw InvalidId
  try {
    return Users.find(String(id))
  } catch (e) {                           // e: UserNotFound
    return guest
  } finally {
    Metrics.increment("profile")
  }
}
// User!InvalidId!Users | Metrics
```

```ts gen
export const profile = Effect.fnUntraced(function* (raw: string) {
  const id = yield* parseId(raw)
  return yield* Effect.onExit(                         // Final: finally
    catchExpected(                                     // Catch: causes made only of expected failures
      Effect.fnUntraced(function* () {                 // try region
        return yield* (yield* Users).find(String(id))
      })(),
      (_e) => Effect.succeed(guest)                    // catch region
    ),
    () => Effect.fnUntraced(function* () {             // finally region: it may fail (D15)
      yield* (yield* Metrics).increment("profile")
    })()
  )
})
```

| Source | Frame | Generated with |
| --- | --- | --- |
| `try { } catch (e) { }` | `Catch(h)`: causes made only of expected failures | The runtime's `catchExpected`, built on `Effect.catchCause` (K14a-K14c) |
| `finally { }` | `Final(fin)` | `Effect.onExit`, never `Effect.ensuring`, whose cleanup must have E = `never` (D46) |
| `throw e` | Fail | `return yield* Effect.fail(e)`: the `return` keeps TypeScript's narrowing after the `throw` |
| A block that declares `using` or calls `fork` | A Scope region: `Final` closes it | `Effect.scopedWith` |
| `using x = acquire()` | Registers in the block's Scope | `Scope.provide(acquire(), scope)` |
| `fork(() => p())` | A child owned by the block's Scope | `Effect.forkIn(p(), scope)` |
| A provider override | `Restore(Γ′)` | `Effect.provideService` / `Effect.provide` |
| Masks | `Mask(ι′)` | `Effect.uninterruptible`, `Effect.interruptible` |
| `handle(...)` | `Prompt` | The runtime's `handle` |
| A clause's last `return resume(k, v)` | `Clause(k)` performs it | `return tailResume(k, v)` (K11a) |

Region bodies are closures over the function's variables, so assignments inside a region are visible after it. `return`, `break`, and `continue` that leave a region produce a completion record (`{ kind: "return", value }`, `{ kind: "break", label }`, `{ kind: "normal" }`) that the enclosing frame dispatches on. This is the same transform TypeScript's own downlevel generator emit performs, applied to Effect frames. Because closures hide assignments from TypeScript's definite-assignment analysis, the compiler checks definite assignment on the source, before lowering (D52).

The prototype compiler (`prototype/esc`) emits these shapes and runs them on the kernel. Its `regions.effeck` example reproduces plain JavaScript's results and `finally` order through labeled `break`, `continue`, and `return` across nested `try`, `catch`, and `finally`, and a `using` in a loop body releases once per iteration on every exit (`prototype/esc/FINDINGS.md`, findings 15 and 17).

#### Scope regions

A region's Scope is given only to the block's `using` acquisitions and forked fibers (D14, D19). Everything else in the block runs under the Scope its caller supplies, so an acquisition without `using` stays in the function's R:

```ts gen
// openAndLog from Chapter 6
export const openAndLog = Effect.fnUntraced(function* (url: string) {
  return yield* Effect.scopedWith((scope) => Effect.fnUntraced(function* () {
    const connection = yield* (yield* Database).connect(url)                     // the caller's Scope: R keeps Scope
    const log = yield* Scope.provide((yield* Logs).open("connect.log"), scope)   // `using`: this block's Scope
    yield* log.write(`connected to ${url}`)
    return connection
  })())
})
```

`Effect.scopedWith` creates the region's Scope without providing it, so `Database.connect` still sees the caller's `Scope`, and only the `using` acquisition receives the region's. Acquisition is masked or interruptible as the resource's `acquireRelease` says; `using` adds no policy (D25). `using` on a TypeScript `Disposable` registers a finalizer that calls `[Symbol.dispose]()` [Rule].

Block-owned fibers use the same Scope:

```ts gen
// serve from Chapter 7
export const serve = Effect.fnUntraced(function* (port: number) {
  return yield* Effect.scopedWith((scope) => Effect.fnUntraced(function* () {
    const server = yield* Scope.provide((yield* Http).listen(port), scope)   // `using server`
    yield* Effect.forkIn(heartbeat(), scope)                                 // registered after server: closed before it
    return yield* server.awaitShutdown()
  })())
})
```

A Scope finalizer registered later runs earlier, so the child is interrupted and awaited before resources acquired earlier in the block are released. A block that calls `fork` without declaring `using` is a region too (D47); without that rule, the prototype compiler's fork-only block found no `Scope` and died with `Service not found` (finding 18).

#### Clauses

A clause compiles like any function, with one rule for its last step (D18). When a clause's last action is `return resume(k, v)` and no `try`, `catch`, `finally`, or `using` region of the clause encloses it, the compiler emits `return tailResume(k, v)`: the request is returned, never yielded, so the clause's generator finishes and the `Clause(k)` frame performs the resume with the clause gone (K11a, K11c). Inside such a region the compiler emits `yield* resume(k, v)`, because the region's cleanup must run after the resumed computation, and the stack grows by design (K11d). In the prototype compiler, producer loops, handler clauses, and loop bodies with `try` and `finally` stay under 50 frames over 100,000 elements (`stack-depth.effeck`, finding 23).

#### Producers

A `function*` body may pause, so it cannot keep JavaScript's generator meaning, whose body runs synchronously inside `next()`. Effeck defines `function*` by the machine instead [Rule]:

- Calling a `function*` runs none of its body. The result is a **producer**: a cold program that performs the built-in control operation `Yield<T>` once per element.
- `yield v` performs `Yield<T>`.
- `for (const x of p)` over a producer is **internal iteration**. It installs a `Yield` handler and runs `p` under it. The clause runs the loop body, then tail-resumes `p`. `break`, `return`, or `throw` out of the body returns from the clause without resuming, so the owner abandons `p` and its finalizers run before the loop exits.
- Pulling elements one at a time from outside, as zipping two producers does, is **external iteration**. The standard library's clause transfers `k` and returns the element with the `Suspended` handle; the next pull resumes it, on the same fiber.

```ts gen
// The loop in firstErrors from Chapter 8
const producer = yield* lines(path)                  // constructs the producer; runs none of its body
yield* handle(Yield,
  producer,
  {
    Yield: Effect.fnUntraced(function* (line: string, k: Continuation<void, never, void>) {
      if (line.startsWith("ERROR")) found.push(line)
      if (found.length === 3) return                 // break: Owner(k) abandons the producer
      return tailResume(k, undefined)                // RETURN the request; never yield* it
    })
  })
```

A loop body with `using`, `try`, or `finally` runs those regions inside the clause; they return completion records, and the clause tail-resumes only after every region has closed (K11b).

Evidence [Kernel]:

- T16 runs internal iteration over an infinite producer with a finalizer. `break` after three elements runs the producer's finalizer before the loop exits.
- K11a runs this exact generator-clause lowering over 100,000 elements; the deepest stack is 3 frames. K11b adds a per-element `using` and `finally`: 5 frames, with every region closed before each resume. Before the fix, the same clause written as `return yield* resume(k, undefined)` reached 200,003 frames (review probe R10; Fable review finding 1).
- T17 covers the `flatMap`-shaped clause, which the kernel also drops in constant stack.
- T10 runs external iteration with `transfer`. Each element is one capture and one resume. A producer is one-shot per run: iterating it again runs it again from the start.

If a producer's own loop iterated another producer, abandoning the outer one would reach the inner loop's owner frame and abandon the inner producer too, finalizers included (O01).

**The `Stream` adapter** [Experiment]. Where a `Stream` is expected, or across the TypeScript boundary, a producer becomes a `Stream` that runs the producer on a fiber of its own, under internal iteration, and hands over one element per pull through a zero-capacity handoff: the clause completes the pending pull and waits for the next one before it resumes the producer. A buffer of one element, as the prototype compiler used (`Stream.callback` with `bufferSize: 1`), lets the producer run one element ahead, which D26 forbids (prototype finding 26). Closing the stream interrupts the producer's fiber, which abandons the producer and runs its finalizers. `for...of` over a `Stream` needs no adapter: it opens a Scope region, takes the stream's pull with `Stream.toPull`, and loops over chunks until the pull signals the end. G12 is the experiment that confirms the adapter.

#### Trusted calls

A call to a trusted function that lists expected failures compiles to a plain call wrapped in a JavaScript `try` with no pause point inside, which the frame invariant allows:

```ts gen
// const slug = slugify(meta.title), where slugify carries @throws {InvalidSlug}
let slug: string
try {
  slug = slugify(meta.title)
} catch (e) {
  if (e instanceof InvalidSlug) return yield* Effect.fail(e)    // an expected failure
  throw e                                                       // a defect, as in any generator body (F4)
}
```

## Part IV. The compiler

### 27. Compiler architecture

```ts dts
import type { Effect } from "effeck-runtime"
export declare function double(n: number): number!!
```

The compiler writes ordinary TypeScript declarations with Effect contracts.
TypeScript consumers call an Effect-returning function; Effeck consumers
run that Effect through an ordinary call. The current implementation is in
`typescript/tsc/internal/effeck/`. `typescript/EFFECK.md` lists the
hooks, acceptance cases, and remaining limitations.

#### The Go fork

The repository contains `microsoft/TypeScript` at `257e874`, the source of
`typescript@next` 7.1.0-dev.20261015.1, imported without upstream test baselines
(D60). The Go module requires Go 1.27. Build from the repository root:

```sh
cd typescript/tsc
CGO_ENABLED=0 GOFLAGS=-trimpath go build -o ../built/local/tsc ./cmd/tsc
```

Use `sh scripts/build-effeck.sh` instead to place the native executable in
`packages/compiler/native/effeck` for the package launcher. `-trimpath` lets jj
workspaces share Go build-cache entries. The compiler embeds its library
`.d.ts` files, so the executable needs no adjacent library directory.

Effeck lives in-tree and registers hooks through a blank import in
`cmd/tsc/main.go`. Upstream seams handle `.effeck` recognition, module resolution,
checker queries and extension state, diagnostics, compiler options, and
transform registration. The implementation uses no unsafe struct mirrors or
`go:linkname` bindings. The seams are small commits in the repository's jj
history, rather than a separately applied patch series.

#### Pipeline

```text
.effeck and .ts sources
  -> TypeScript parser and binder
  -> TypeScript checker with Effeck hooks
       prelude and service keys
       lambda forms inside overload resolution (D56)
       contracts, failures, requirements, provider graphs
       trust levels and diagnostics
  -> Effeck AST transform under the checker lock
       Effect.fnUntraced generators and Effect regions
       runtime helpers and plain $direct variants
  -> TypeScript emit
       JavaScript, source maps, and ordinary .d.ts files
```

The parser keeps TypeScript's grammar. The checker resolves prelude names as
implicit imports from `effeck-runtime`, and the other exports of `effect`'s
root as implicit imports from `effect`, unless a local declaration or import
shadows them (D4, D48, D146). Class hooks give fully abstract service classes
the static side of an Effect Context key and the instance shape of their
abstract members (D1, D45).

Lowering runs before TypeScript's type erasure. It queries the checker while
holding the checker lock and rewrites the TypeScript AST directly. The
computation IR described by the machine chapters is a semantic model; the
current compiler does not materialize a separate IR.

A call that returns Effect itself runs; structured Effect subtypes such as
Config remain data until called (D24). An unannotated or Effect-contract pure function gets a `$direct` variant
for inline calls and an Effect form for the public ABI (D9, D10). A plain
return annotation instead preserves one plain function (D67). Calls and
operands retain source order. `throw` adds an expected failure; `try`, `catch`,
and `finally` become regions with completion records for jumps (D13, D15, D46).

#### One checker pass

Lambda hooks participate in TypeScript's own overload resolution. Each
candidate asks for a plain function, an Effect function, or a program; the
first accepting candidate selects the form (D44, D56). A return in a program
is checked against the Effect's success type. `pipe` looks ahead to its first
stage (D8), and function values and collections convert where the parameter
expects programs (D7, D49). D10 eligibility uses a greatest fixpoint over the
callee group. D12 callback parameters carry inferred E and R type parameters.

D52 requires recursive E and R inference inside a call-graph component. The
compiler's acceptance cases and `typescript/EFFECK.md` record its current
coverage; no whole-program iterative lowering pass runs.

The earlier `prototype/esc` uses TypeScript 5.9.3 and repeated checks as an
executable reference. Its 46 examples are now native-compiler acceptance cases
under `typescript/effeck/cases/`. `prototype/single-pass/RESULTS.md`
records the experiments that established D56. The native implementation does
not duplicate source text per lambda form.

#### Runtime boundary

Generated JavaScript imports only `effect` and `effeck-runtime` (D30).
The runtime contains the continuation kernel and the helpers the compiler
emits. It runs on Effect's fiber, Context, and Scope; it adds no scheduler.
The package checks `effect@4.0.5` at load time and rejects other versions.

| Runtime helper | Purpose |
| --- | --- |
| `$programs` | Convert collections of program functions |
| `$catch`, `$finally` | Expected-failure handling and cleanup |
| `$expected`, `$caught` | Expected failures in plain JavaScript regions |
| `$region`, `$using`, `$fork` | Block-owned scopes, resources, and children |
| `$service`, `$reference`, `$control` | Service keys, defaults, and control operations |
| `$trusted`, `$trustedPlain` | Classify listed foreign exceptions |
| `handle`, `resume`, `$tailResume` | Continuation handling |
| `$produce`, `$yield`, `$forOf`, `$forStream`, `$toStream` | Producers and Stream adaptation |

The runtime's README lists the full ABI. Compiler and helper changes belong
in the same commit. The smoke harness builds the runtime and links every case
to that workspace's package, so incompatible versions cannot pass unnoticed.

#### Declarations and metadata

Services emit as runtime key classes. Declaration emit writes the instance
shape as an interface and a class whose base has the `ServiceKey<Users>` type
(D45), so ordinary TypeScript can implement and provide the service:

```ts dts
export interface Users {
  find(id: string): User!UserNotFound!
  readonly "~effeck/service"?: "@acme/app/src/users#Users"
}
declare const Users_base: import("effeck-runtime").ServiceKey<Users>
export declare abstract class Users extends Users_base {}
```

A default service uses `ReferenceKey`. Declaration transforms add type imports
for prelude names, preserve D12's callback parameters, and rewrite source
specifiers to `.js` (D31). TypeScript consumers need only the `.js`, `.d.ts`,
`effect`, and runtime packages.

Chapter 15's control-class and `$direct` JSDoc metadata is a design target.
The current compiler does not emit or read that metadata. A consumer compiled
against a declaration uses the Effect form, which preserves correctness at
the cost of the direct-call optimization. Trusted TypeScript declarations do
support `/** @effeck trusted */` and `@throws {T}` (D23): each listed
exception class joins E and generated code tests it with `instanceof` (D51).

#### Acceptance checks

```sh
cd typescript
sh effeck/smoke.sh --status
```

The harness compiles cases in parallel, runs successful programs on Node, and
checks expected failure codes and optional diagnostic positions. Cases marked
`pending` report their owning compiler lane and do not fail the suite.
Source-map and tracing cases check original `.effeck` frames and generated spans
(D34, D36). The docs checker independently checks user-guide snippets with
`bun docs/tools/check-snippets.ts` from the repository root.

### 28. Tooling

```sh
bun create effeck my-app        # or: npm create effeck@latest my-app
cd my-app
bun run build
bun run start
bun run test
bun run dev
bun x --no-install effeck --noEmit
```

Effeck 0.0.1 installs from npm as unscoped packages: `effeck`,
`effeck-runtime`, `effeck-vite`, `create-effeck`, and one `effeck-<os>-<cpu>`
package per platform. `create-effeck`
copies the starter, pins the Effeck packages to its own version, and installs
dependencies; `--no-install` skips installation. Projects need Node 26 or later
and Bun 1.4 or later. The compiler is a native binary: `effeck` lists one
package per platform as an optional dependency, and the package manager
installs the one that matches the machine, the per-platform distribution D30
planned:

| Platform | Package | Tested for 0.0.1 |
| --- | --- | --- |
| macOS on Apple silicon | `effeck-darwin-arm64` | Yes |
| macOS on Intel | `effeck-darwin-x64` | No |
| Linux on x64 | `effeck-linux-x64` | Yes |
| Linux on arm64 | `effeck-linux-arm64` | No |
| Windows on x64 | `effeck-win32-x64` | No |
| Windows on arm64 | `effeck-win32-arm64` | No |

To work on Effeck itself, install from a checkout instead. `sh scripts/setup.sh`
requires Go, Bun, and Node on PATH; the tested versions are Go 1.26 (with Go
1.27 downloaded automatically), Bun 1.4.1, and Node 26.10. Setup builds
`packages/compiler/native/effeck`, installs and builds the runtime, and copies
`templates/app` into `~/.bun-create/effeck`, a local template that
`bun create effeck` prefers over the npm package. That template uses absolute
`file:` dependencies on the checkout's compiler and Vite packages and on the
runtime tarball that setup packs (D120). Keep the checkout available; after
updating it, rerun setup and run
`bun update effeck-runtime effeck effeck-vite`
in projects created from it.

Setup links `~/.local/bin/effeck` when that directory is already on PATH and no
unrelated executable occupies the path. Project scripts find their own compiler
without that global link. `EFFECK_BINARY` overrides the launcher's native binary.
Setup preserves unrelated templates and never edits shell configuration.

#### Packages and versions

| Package | Current implementation |
| --- | --- |
| `effeck` | The compiler and language server: a Node launcher that forwards arguments, streams, signals, and exit status to the native binary of the matching platform package (`EFFECK_BINARY` overrides it), plus the VS Code extension at `editor/effeck.vsix` |
| `effeck-<os>-<cpu>` | The native `effeck` binary for one platform |
| `effeck-runtime` | Continuation kernel, prelude, and generated-code helpers; see its README for the ABI |
| `effeck-vite` | Whole-project compiler subprocess integration for Vite 7 and 8 and Vitest 5 |
| `create-effeck` | Scaffolds the starter for `bun create effeck` and `npm create effeck` |
| VS Code extension | Ships inside `effeck` as `editor/effeck.vsix`; not on the VS Code Marketplace or Open VSX |

The starter pins `effect`, `@effect/platform-node`, and `@effect/vitest` to
4.0.5, and Vitest to 5.0.5. The runtime checks Effect's version on load. The
compiler is the TypeScript 7.1 checker, with ordinary TypeScript semantics for
`.ts` files (D30). Its emitted declarations are ordinary `.d.ts` files.

`effeck` accepts TypeScript compiler flags: bare `effeck` checks and emits,
`--noEmit` checks, `--watch` watches, and `-b` builds references. The same native
binary exposes `--lsp --stdio` and the upstream `--api` transport. The current
Vite plugin uses subprocess compilation instead of the API transport (D105,
amending D31).

#### Building and testing

The starter's configuration emits Node ESM into `dist/`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "sourceMap": true,
    "rootDir": "src",
    "outDir": "dist",
    "noEmitOnError": true,
    "rewriteRelativeImportExtensions": true,
    "skipLibCheck": true,
    "types": []
  },
  "include": ["src/**/*.effeck"]
}
```

`build` runs `effeck`. `start` builds and runs Node with `--enable-source-maps`.
`dev` builds once, then runs the compiler watcher and Node output watcher;
Ctrl+C stops both. Libraries can add `declaration` and `declarationMap` to
emit declarations. The starter does not enable them.

Import local sources as `./welcome.effeck` or `./format.ts`. The compiler emits
`.js` specifiers and declarations that name `.js` outputs (D31). In `.ts`,
`./welcome.js` resolves to `welcome.effeck`. Only `.effeck` sources are selected by the
starter's include glob; imported `.ts` integrations also join the program.

The starter tests source files through the Vite plugin:

```ts ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import effeck from "effeck-vite"

export default defineConfig({
  plugins: [effeck()],
  test: { include: ["src/**/*.test.effeck"] }
})
```

Register `it.effect` from `@effect/vitest` at the top level or in `describe`
(D38). Keep foreign assertions inside a boundary such as
`sync(assert.strictEqual(actual, expected))`. `bun run test`
needs no prior build and maps assertion failures to `.effeck` lines.

For a Vite application, add the same plugin to `vite.config.ts`. The plugin
runs one compiler subprocess for the project on the first request and after
source changes. Requests share compilation. It emits JavaScript and source
maps into its own directory under `node_modules/.effeck/`, reports errors
in the terminal and overlay, and invalidates all loaded `.effeck` modules and their
importers after an edit. It does not compare exported contracts or use the
API transport yet. Sources must stay under Vite's root so emit paths and source maps agree (D107). Its README documents `project` and `compiler` overrides.

#### The editor

The server is the fork's own, `effeck --lsp --stdio`, from the project's compiler
(D32). The same binary identifies as `Effeck 0.0.1 (TypeScript 7.1.0-dev)`
with `--version`; there is no separate compiler entry point (D75). `effeck`
ships the extension as `editor/effeck.vsix`. Install it from a project:

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
# Or use cursor --install-extension with the same path.
```

Open the project directory so the extension finds its local compiler. Install
the extension again after updating `effeck`. In a checkout, setup also builds
the extension and prints its path.

The extension finds `effeck` from `effeck.server.path`, then the nearest
`node_modules/.bin/effeck`, then PATH. It requires a trusted workspace (D79).
Its grammar reuses VS Code's TypeScript grammar under `source.effeck`,
with inner `.ts` scopes so existing themes apply.

For `.ts` ownership, disable VS Code's built-in TypeScript extension, or use
Cursor's `useTsgo` setting (D76). The extension serves `.effeck` while the built-in
TypeScript extension remains active, but leaves `.ts` to that extension.
Do not set `typescript.validate.enable` to false: this server reads that
setting too and would hide its Effeck diagnostics.

Other editors can configure their LSP client for `.effeck` and `.ts`, with
`tsconfig.json` as the root marker, and disable their other TypeScript server
for this project. Add `*.effeck linguist-language=TypeScript` to `.gitattributes`
for GitHub highlighting.

Hover shows a running call's success value as "Runs here and returns `T`",
and a plain helper's direct signature alongside its TypeScript-facing type
(D77). Prelude names and service members complete in scope (D78). A method of
an Effect data type hovers as the function it calls, "Method syntax: calls
`Option.getOrElse(maybe, …)`", and completes after a dot (D152). A call passed
by name hovers as "Passed by name to `fork` as a program" (D147), and so does a
pipe's topic that passes its left side by name; any other topic hovers as its
value, "(topic) %: T" (D155). TypeScript
files retain TypeScript hover behavior. D33's control-class hover text,
catch-specific completions, mechanical EK code actions, and migration
refactors remain design targets. Diagnostic messages include repair suggestions
as text; the current compiler does not implement those editor refactors.

#### Diagnostics

Every EK diagnostic is an error, except two. Each error marks code that has no meaning, or code whose generated output would break a rule of Parts II and III, so none can be lowered and `// @ts-expect-error` does not apply to them; it keeps its TypeScript meaning for TypeScript's own codes. The two exceptions are policy, not soundness: EK2004, a `catch` that can never run, is a warning by default, and EK4403, a clock or randomness read, is an error by default. A project may set either to `"error"`, `"warning"`, or `"off"` in `compilerOptions.effeck.diagnostics`; an EK4403 read then compiles as a trusted `sync` call that tests cannot control.

| Option in `compilerOptions.effeck` | Values | Default |
| --- | --- | --- |
| `diagnostics` | `"EK2004"` and `"EK4403"`, each `"error"`, `"warning"`, or `"off"` | EK2004 `"warning"`, EK4403 `"error"` |
| `tracing` | `"none"`: no spans. `"exported"`: exported functions compile with `Effect.fn` spans. `"all"`: every named lowered function and method gets a span; anonymous lambdas do not (D104) | `"none"` |

`effeck` exits non-zero when any error is reported. The CLI prints `error EK1003:`; LSP sends source `EK` and numeric code `1003`, which VS Code displays as `EK(1003)` (D61). Diagnostics print at `.effeck` spans, in the same form as the examples in this document, in the terminal, the editor, and Vite's overlay.

The diagnostic registry reserves these codes. This table describes each rule
and the intended repair (D33, D53). A registry entry does not prove that its
check or editor code action is implemented; the acceptance suite and compiler
README track coverage. Repairs below are suggestions, not shipped code actions:

| Code | What it reports | Suggested repair | Chapter |
| --- | --- | --- | --- |
| EK1001 | A module's top level runs a program | Export a function instead | 13 |
| EK1002 | A `function*` passed to an API whose parameter returns a library Generator or Iterator, such as `Effect.gen` (D64) | Convert the function | 10 |
| EK1003 | `async`, `await`, `for await`, or `await using`: a call already waits | Remove them | 10 |
| EK1004 | `using` at a module's top level or in a class static block | Move it into a function | 6 |
| EK1005 | A runner inside code that a fiber runs | Call the program, or `fork` it | 13 |
| EK1006 | A pipe whose body does not use its topic, as in `name \|> "anonymous"` (D156) | Write `%` where the value goes: `name \|> greet(%)` | 5 |
| EK1007 | `%` outside every pipe's body (D156) | Use `%` only after `\|>`; `%` between two operands is still the remainder | 5 |
| EK1008 | A pipe that passes its left side by name to one program and also uses it as a value (D155) | Bind the value first with `const`, or pass it by name everywhere | 5, 13 |
| EK2001 | An expected failure escapes the written contract | Catch it, widen the contract, or `Effect.orDie` | 2, 14 |
| EK2002 | A requirement escapes the written contract | Widen the contract, or provide the service | 14 |
| EK2003 | An Effect return annotation with missing channel slots, or a plain annotation on a non-method function whose body runs an Effect (D67, D74) | For `Effect<T>`, suggest `T!!` or removing the annotation; for a bare `T`, suggest `T!!` or `T!Errors` | 14 |
| EK2004 | A `catch` that can never run (a warning by default) | Use the form that can fail, or `try` | 3 |
| EK2005 | A `resumeWith` program that needs a service or fails outside the operation's contract | Read the service in the clause first | 9 |
| EK2006 | A `return`, `break`, or `continue` that leaves a `finally` block, which in JavaScript discards the failure in flight | Move it after the `try` statement | 17 |
| EK2101 | A declaration without a body that has no full contract | Write `T!Errors!Dependencies`, or `T!!` | 14 |
| EK2102 | A recursive Effect function whose success, catch-variable, or return type needs a contract before its cycle's channels are known (D71, D72) | Write the contract | 14 |
| EK2103 | A member of a `service` or `control` declaration with a body or an initializer (D128) | Implement it in a layer, or make it static | 4 |
| EK2104 | A setup statement of a `layer` declaration that follows one of its members (D129) | Move the statement above the members | 4 |
| EK2105 | A typed catch clause whose type has a member without a literal `_tag` (D131) | Catch a tagged error, or test the value in an untyped clause | 3 |
| EK2106 | A catch clause after an untyped catch clause (D131) | Move the untyped clause last | 3 |
| EK2107 | A handle clause whose control is not a control declaration, such as a service (D132) | Provide a service with a layer; handle only a control's operations | 9 |
| EK2108 | A handle statement that leaves an operation of a control it handles without a clause, or gives one two clauses (D132) | Add the missing clause, or remove the second | 9 |
| EK2109 | A field of a `schema` declaration whose type has no schema form, or names a value that is not a schema (D133) | Declare the schema with `Schema.Struct`, or name a schema constant | 11 |
| EK3001 | Two services share a key | Rename one | 4 |
| EK3002 | A class extends a service | Replace `extends` with `implements` | 4 |
| EK3003 | `new` on a service | Use `Users()` for the implementation | 4 |
| EK3004 | `instanceof` on a service | None: a service is a key, not a base class | 4 |
| EK3005 | A generic service class | Use an interface for the generic type | 4 |
| EK3006 | A static member with the same name as a service member | Rename one | 4 |
| EK3101 | A provider looks a service up per call where the contract allows no requirements | Capture the service when the layer is built, or declare it | 4 |
| EK3201 | No provider for a required service, with the path that needs it | Add the provider the program defines to `wire` | 4 |
| EK3202 | Two providers for one service | Remove one | 4 |
| EK3203 | Providers form a cycle | None | 4 |
| EK3204 | A `wire` argument whose type does not name its outputs | Compose it with `Layer.provide` | 4 |
| EK4201 | A control operation inside a combinator that runs it on another fiber | Handle the operation inside the branch | 9, 16 |
| EK4202 | A TypeScript function whose R holds a control operation | None in v0 | 9 |
| EK4301 | A callback that can pause or fail where a plain function is expected | Rewrite as `Effect.forEach` or `for...of` | 10, 16 |
| EK4401 | A plain TypeScript call in code a fiber runs, with no boundary or trusted contract (D100) | Wrap it in `try`, or tag the declaration `@effeck trusted` | 10 |
| EK4402 | Running a Node.js or browser API; importing and passing values is allowed (D108) | Use the Effect module the error names | 11 |
| EK4403 | A clock or randomness read (an error by default) | Use `Clock`, `DateTime`, or `Random` | 10 |
| EK4404 | `try` given Effect's `{ try, catch }` options object, which it would treat as a value | Pass the code and the error mapping as two arguments: `try(parse(text), (error) => …)` | 10 |
| EK4501 | A method of an Effect data type named without a call, as in `const f = stream.filter` (D153) | Call it, or pass `Stream.filter` itself | 13 |

#### Linting

D35 assigns type and language diagnostics to `effeck` and reserves type-aware
linting for `.ts` files. A stock TypeScript checker models different call
semantics and cannot type-check Effeck correctly. Run `effeck --noEmit`
for `.effeck` checking. `effeck-oxlint-config` is not shipped, and Vite Plus
integration has no acceptance coverage yet. Do not install an unpublished
lint-config package from the registry.

#### Debugging

`effeck` writes source maps when `sourceMap` is on, and the Vite plugin always serves them. Each map records original `.effeck` positions on the lowered AST through TypeScript emit, so any debugger that reads standard source maps maps breakpoints and paused frames to `.effeck` lines: VS Code's JavaScript debugger for Node.js, Bun, and Chrome targets; Chrome and Edge DevTools; and JetBrains IDEs.

```text
Paused on breakpoint, requestApproval  checkout.effeck:22
Call stack (JavaScript)
  requestApproval            checkout.effeck:22         the running Effeck function
  runLoop                    effect/dist/internal/effect.js
  evaluate                   effect/dist/internal/effect.js
```

What a debugger shows follows from the frame invariant (Chapter 26):

- **One Effeck frame at a time.** Compiled functions are generator functions named after their source, so the running function appears by name at its `.effeck` line. Its callers are frames on the fiber's `_stack`, not JavaScript activations, so the debugger's call stack shows Effect's run loop below it, not the Effeck call chain.
- **Regions** are closures named after their function and block: `profile$try`, `profile$catch`, `profile$finally`, `profile$using`.
- **Clauses and producers.** A clause is a generator function named after its operation member (`approve`), and a producer's body is one named after the producer (`lines`). A breakpoint in a clause shows the clause; the handled program's frames are inside `k`, a heap value, and appear again when the program resumes. A breakpoint in a producer shows the producer, and the loop body runs in the clause of the `for...of` that iterates it.
- **Stepping over a call that pauses** continues on a later run-loop step, so the debugger steps into Effect's run loop; set a breakpoint on the next line instead, as with `Effect.gen` code today.
- **Logical call stacks** across calls and suspensions come from spans: set `tracing` to `"exported"` or `"all"`, and use Effect's `DevTools` layer (`effect/devtools`) with its VS Code extension, which shows fibers and the span stack.

## Part V. Decisions

### 29. Decisions

```text
settled before this document      1-7     the owner's original decisions
decisions since                  D1-D124 the owner's calls and reported build defaults; gaps reserved
rules of the runtime and compiler P1-P22  adopted (D29), revised where the 2026-10-16 reviews broke them
end-to-end build calls          D61-D124 recorded below where reported; unreported numbers stay unassigned
```

Every rule in this document traces to one of these. The log with full reasons is `docs/design/DECISIONS.md`; a default is a decision made to complete the design, and the owner can overturn any default.

#### Settled before this document

These come from the earlier conversation and are not reopened.

1. Every function participates in one computation model. No `effect function` keyword and no `effect(...)` wrapper. Ordinary calls compose computations and expose successful results without a marker at each call. A way to construct, store, pass, retry, and concurrently compose programs as values is required (D7 spells it).
2. Explicit contracts are written `Effect<A, E, R>` as a return annotation. The annotation describes the whole computation, and the return expression is checked against A. Most code infers. In the owner's words: "Success, errors, and requirements must each support inference or explicit declaration. We have not finalized the shorthand for partially annotating those channels. Preserve that requirement without inventing a premature spelling or silently confusing inference with `never`. Use the same contract model for function types, callbacks, service methods, and exported declarations."
3. `throw` contributes to the owning computation's inferred E. An explicit E constrains what escapes. `Effect<A, never, R>` may handle expected failures internally but rejects escaping ones. Expected failures, defects, and interruption stay distinct.
4. Optimization preserves computation semantics. E = never and R = never do not prove purity or synchronous completion. Constructing a cold program never executes its work.
5. Dependency injection is core and is designed with the continuation semantics: identity, contracts, inferred and declared requirements, acquisition, composition, missing, ambiguous, and cyclic providers, acquisition failures, residual requirements, scoped sharing and release, test overrides, Context across suspension and children, and interoperability with Effect Context keys and Layers. No second DI container.
6. New syntax must earn its place. Ordinary function syntax and the `Effect<A, E, R>` annotation reuse TypeScript syntax.
7. Broad npm interoperability is not an MVP constraint. Interop with TypeScript code that already uses Effect is.

D57 supplies the partial-annotation spelling required by decision 2: explicit `infer Name` slots.

#### Decisions since

"Will" marks the owner's decisions; every other row is a default. **Controversial** marks a decision the owner is likely to revisit after using the language.

| # | Decision | Reason and evidence | Chapters |
| --- | --- | --- | --- |
| D1 | Services are abstract classes whose instance members are all abstract; nominal; keys derived and unique; no `extends`, `new`, `instanceof`, or generics; emitted as D45 says | Uses no valid TypeScript meaning (TS2339, TS2348); structurally equal classes would alias (probe); rare in real code | 4 |
| D2 | Control operations are abstract classes that extend the global `Control` | One declaration form for services and operations; no keyword | 9 |
| D3 | Errors use the global `TaggedError` (Will) | Effect's `Data.TaggedError`; `Schema.TaggedError` for serialized errors | 3 |
| D4 | The prelude: `Effect`, `Schema`, `Layer`, `TaggedError`, `Control`, and the built-ins; imports shadow it | What 77-81%, 39-44%, and 34-44% of Effect files import | 1 |
| D5 | Calls to non-Effect code need a declared trust level (Will) | Foreign code may throw; platform APIs are replaced; clock reads go through services | 10 |
| D6 | The standard library is Effect's (Will) | Effect ships the modules and the platform implementations | 11 |
| D7 | Program values are functions with no arguments | No new syntax; `() =>` already means "not yet" | 5, 13 |
| D8 | No `\|>` in v0; the `pipe(program, …stages)` built-in stays. **Superseded by D154** | Calls run before a pipe sees them; Stage 2 since 2021; 1.1% and 2.6% of pipes keep two stages | 5 |
| D9 | A function value where a plain function is expected passes its plain-JavaScript form, or EK4301 | Fable review, finding 3 | 13, 16 |
| D10 | Plain JavaScript when a function performs no Effect operation and E is never; source order kept; pure recursion on the JavaScript stack | Codex review R4; settled decision 4; the owner's `add` feedback | 15 |
| D11 | Higher-order functions compile to frames in v0 | Fable review, finding 5 | 15, 16 |
| D12 | A callback type without a contract accepts any contract; unchanged by D57 | Settled decision 2 forbids reading it as `never` | 16 |
| D13 | `catch` handles a cause only when it holds no defect and no interruption | Codex review R1; K14a-K14c | 3, 17, 26 |
| D14 | Only `using` acquisitions belong to the block | Codex review R8 | 6, 23, 26 |
| D15 | A failure during cleanup is never dropped | Codex review R2b; K12a-K12e | 17, 21 |
| D16 | v0 exposes `handle`, `resume`, `resumeWith`, `abandon`; `transfer` stays in the standard library | Codex review R5; K13 now bounds a transfer by its fiber, and G3 remains | 9, 21 |
| D17 | `resumeWith` runs code whose R is `never` | Codex review R7 | 9, 20 |
| D18 | A clause whose last step is `resume` compiles to the tail-resume shape | Both reviews (R10, Fable 1); K11a-K11d | 20, 26 |
| D19 | `fork` is block-owned | Fable review, finding 9 | 7, 26 |
| D20 | A TypeScript function whose R holds a control operation is EK4202 in v0 | Codex review R6 | 9, 16 |
| D21 | Each `Effect.provide` builds its layers once; provide at `main`; `ManagedRuntime` for several entry points | Fable review, finding 6; conformance test D05 | 4 |
| D22 | Effect's own packages are trusted, with the functions and values they return | D6; `Option` and `Result` are data in Effect 4 | 10 |
| D23 | A trusted contract is the `@effeck trusted` JSDoc tag, with `@throws` | TypeScript syntax only; the compiler's metadata channel | 10, 27 |
| D24 | A call runs only `Effect` itself; subtypes are data until called; `new` never runs | `Config`, `Exit`, keys, and errors extend `Effect` | 13 |
| D25 | `using` adds no masking policy | K15a-K15c | 6, 21 |
| D26 | Producers: `for...of` by type; a producer is a `Stream` to TypeScript; the adapter runs it on its own fiber behind a zero-capacity handoff | Stream operators pull on other fibers; continuations are fiber-affine; a one-element buffer runs ahead (prototype finding 26) | 8, 26 |
| D27 | `wire` is an intrinsic with a bounded contract (EK3201-EK3204) | Layer types carry outputs and inputs | 4 |
| D28 | A `catch` over several expected failures receives the first | K14a | 3, 17 |
| D29 | P1-P22 are the spec's rules, as revised below | The reviews' verified findings | 29 |
| D30 | Installation: `effeck` (`effeck`) and `effeck-runtime`; `effect` 4.0.5 exactly | Mirrors `typescript@7` packaging | 28 |
| D31 | Builds: `effeck --watch`; `effeck-vite` for Vite | One checker for CLI, editor, and bundler | 28 |
| D32 | The editor: `effeck --lsp --stdio`; the `effeck.effeck` extension | The fork's binary already serves LSP | 28 |
| D33 | v0 editor features: hover, completions, quick fixes, four refactors | The diagnostics with mechanical fixes | 28 |
| D34 | Diagnostics: errors, except the policies EK2004 (a warning) and EK4403; `compilerOptions.effeck` | Soundness errors cannot be lowered | 28 |
| D35 | One owner per diagnostic: `effeck` for types and language, Oxlint for lint | tsgolint embeds TypeScript's checker | 28 |
| D36 | Debugging through composed source maps; spans for logical stacks | The frame invariant keeps callers off the JavaScript stack | 28 |
| D37 | `catch` never sees an unlisted exception; a dead `catch` is the warning EK2004 | Coverage audit X2; prototype finding 16 | 3, 10 |
| D38 | Runners only where no fiber runs the code; EK1005 elsewhere | Coverage audit X4 | 13 |
| D39 | A function-or-Effect parameter: a function if plain, a program if it can pause | Coverage audit X9 | 13 |
| D40 | Service statics: `layer`, `make`; `static readonly default` makes a `Context.Reference` | Coverage audit X12 | 4 |
| D41 | `S.member(…)` always adds `S` to R; service members win over the key's members; EK3006 for a static that shares a member's name | Coverage audit X13; prototype findings 3, 4 | 4, 13 |
| D42 | Workflow bodies are ordinary programs: interrupted on durable suspension, rerun on replay | Coverage audit X17; `workflow/Workflow.ts:695-780, 1031-1036` | 23 |
| D43 | `Effect.callback` is an explicit boundary; its `resume` takes an Effect, and `.effeck` passes an `Exit` | Coverage audit X11; `Effect.ts` | 6, 10 |
| D44 | "The expected type" is the declared, uninstantiated parameter type of the resolved overload | Prototype finding 1: the contextual type is inferred from the lambda itself | 13 |
| D45 | A service compiles to an interface plus a `Context.Key` class, with an optional brand | Prototype finding 19: Effect's class form breaks `implements` (TS2720) | 4, 27 |
| D46 | `finally` lowers to `Effect.onExit`; `throw` to `return yield* Effect.fail(e)`; E is the type of causes made only of expected failures | Prototype findings 11, 12, 14 | 14, 17, 26 |
| D47 | A block that declares `using` or calls `fork` is a Scope region | Prototype finding 18 | 23, 26 |
| D48 | `Scope` joins the prelude, as module and type | Contracts name it constantly; Effect 4's type is `Scope.Scope` | 1 |
| D49 | Arrays and records of programs convert where Effects are expected | Prototype finding 5 | 5, 13 |
| D50 | `Continuation<V, E, H>`: `resume` returns the `handle`'s result type | Prototype finding 24 | 9 |
| D51 | Plain calls at a module's top level are ordinary calls; `@throws {T}` names a class | Prototype finding 21 | 10 |
| D52 | E and R of recursion by a fixpoint inside the component, never a whole-program pass (EK2102 for A); A widens; definite assignment on the source | Prototype findings 7, 10, 13; the single-pass probe `isEven`/`isOdd` | 14, 26, 27 |
| D53 | Diagnostic codes follow the prototype | Prototype diagnostics | 28 |
| D54 | Inside a plain-JavaScript function, `try`, `catch`, and `finally` are JavaScript's own; `catch` rethrows anything unexpected | D10 with D5: nothing in the function can pause | 15 |
| D55 | A method implementing a plain-function member (`[Equal.symbol]`, `[Hash.symbol]`) compiles to a plain JavaScript method, or EK4301 | The member's declared type is not an Effect | 15 |
| D56 | A lambda's form (plain, Effect function, program) is decided against each overload candidate's declared parameter type, and the first candidate that accepts it wins; never by the lambda's own inferred type. D10 eligibility is computed on demand | Single-pass experiment: 0 disagreements in 46 files; 6 probe lambdas with two self-consistent lowerings | 13, 15, 27 |
| D57 | **Controversial.** Partial annotations use `infer Name` per channel in a function return annotation with a body; omitted slots are EK2003. D12 stands | TypeScript 7 parses `infer`; explicit slots distinguish inference from `never` | 2, 14, 16, 28 |
| D58 | **Controversial.** Keep EffectScript, `esc`, `.es`, and `@effectscript`; superseded by D125 | Another compile-to-JavaScript language uses the name | 28 |
| D59 | Draft upstream issues and proposals in `docs/upstream/`; status: drafted, not filed | Filing speaks for the owner in a third-party project and needs his go-ahead | 24, 25, 30 |
| D60 | **Will.** One private repository, `github.com/roninjin10/effectscript`; the compiler lives in `typescript/` | Spec and implementations change together; the imported TypeScript tree omits 349 MB of baselines | 27, 28 |

#### Decisions from the end-to-end build (2026-10-16)

The following rows record build defaults, not new language decisions. Lane U recorded D97-D99 from F7's compiler notes because its inbox report was absent. The decision log groups known gaps by area and distinguishes landing snapshots from later fixes. D114-D119 have no reported decisions.

| # | Decision | Reason and evidence | Chapters |
| --- | --- | --- | --- |
| D61 | **Controversial.** An EK diagnostic is a TypeScript diagnostic with source `EK`, on upstream's content-mapper path | One diagnostic path for CLI and LSP | 28 |
| D62 | The prelude is the runtime's exports | Lazy imports preserve files that use no prelude names | 1, 27 |
| D63 | A `.effeck` file imports `./x.effeck` or `./x.ts` by source name with no flag (no TS5097); specifiers become `.js` in JavaScript and `.d.ts` output | Source names for checking, output names for JavaScript | 10, 27, 28 |
| D64 | A failing compiler case passes when its exact set of error codes matches (TypeScript codes count, warnings do not); a runnable case compares only its `@@result` line, so programs may log | Exact codes detect regressions while result markers allow logging | 10, 27, 28 |
| D65 | A call from a `.effeck` file to a plain function runs inline as `f$direct(a)` everywhere, including a module's top level, because it computes a value and runs no program | Plain computation runs no program | 13, 15 |
| D66 | An overload candidate that asks for a plain function rejects an `.effeck` function value that can pause, as it rejects such a lambda (D56 extended to function values); a TypeScript function that returns an Effect passes as data; when every candidate rejects, EK4301 reports against the first candidate's form | Named values and lambdas need the same callback safety | 13, 16 |
| D67 | **Controversial.** A function with a plain return annotation keeps that type and compiles to one plain function; only unannotated or Effect-contract functions get the `$direct` plus `Effect.sync` pair, whose TypeScript type is `(…) => Effect<A, never, never>` (chapter 15's one calling convention) | Plain annotations support JavaScript protocols | 10, 14, 15 |
| D68 | **Controversial.** D12 (a callback parameter without a contract accepts any contract) applies to declarations, methods, and function expressions that are not call arguments; the added type parameters are named E and R (then E2, R2) and appear in `.d.ts` output | Missing callback channels must not mean never | 16, 27 |
| D69 | A program lambda lowers to `Effect.gen(function* …)`, a converted function value to `Effect.suspend(v)`, and an array or record of programs built at run time to `$programs` | Conversions must preserve cold execution | 5, 13, 27 |
| D70 | **Controversial.** A `return`, `break`, or `continue` that leaves a `finally` block is a new error, EK2006; in JavaScript such a jump discards the failure in flight, and compiling `finally` to Effect cleanup cannot honor it | Effect cleanup cannot honor jumps that discard failures | 17, 28 |
| D71 | EK2102 also covers a `catch` variable or return type typed before its cycle's E and R are known; the fix is a written contract | The checker cannot revise cached recursive types | 14, 28 |
| D72 | EK2102 applies only to functions that run Effects; a pure recursive function keeps TypeScript's TS7023, which a plain annotation fixes (consistent with D67) | Pure recursion needs no Effect channel analysis | 14, 15 |
| D73 | A call to a function with a complete written contract adds the contract's E and R, not what its body does | Callers rely on the public contract | 14 |
| D74 | Methods are exempt from EK2003 for a plain annotation, because a method implementing a plain-function member is declared that way (D55) | Plain protocol members need plain annotations | 14, 15 |
| D75 | The `tsc/cmd/tsc` binary identifies as Effeck; there is no separate cmd/effeck | One upstream entry-point seam | 27, 28 |
| D76 | **Controversial.** The editor serves `.ts` only when the built-in TypeScript extension is disabled, or with Cursor's useTsgo | Disabling validation also hides Effeck diagnostics; amends D32 | 28 |
| D77 | **Controversial.** Hover shows a running call's success value and a plain helper's direct and TypeScript-facing signatures; `.ts` hover stays unchanged | Explain both call semantics and helper ABI | 15, 28 |
| D78 | Prelude names and service members complete in scope; local declarations shadow prelude names | Discoverability without explicit imports | 1, 4, 28 |
| D79 | Reuse TypeScript TextMate grammar; resolve effeck from setting, project, then PATH; require workspace trust | Keep theme support and project compiler ownership | 28 |
| D80 | The runtime checks Effect's version by importing effect's package metadata with a static JSON import attribute; works in Node, Bun, and bundlers | JSON attributes work in Node, Bun, and bundlers | 27, 28 |
| D81 | The runtime exports `suspend`, `handoff`, `Resumer`, and `HandoffProtocolViolation` for TypeScript callback adapters (D43) | Resource adapters need cancellation and lost-race reporting | 6, 21 |
| D82 | The extraction kept the prototype helpers' behavior and continuation parameter order; discrepancies with the spec are documented in packages/runtime/README.md, not fixed silently | Extraction must preserve compiler compatibility | 26, 27 |
| D83 | Setup writes absolute `file:` dependencies on the repository's packages into new projects instead of `bun link`; nothing registers globally and nothing is published, so a project keeps working only while the checkout stays where it is | Local setup needs no registration or publication | 28 |
| D84-D89 | No decisions reported in the build inbox | Leave these numbers unassigned | |
| D90 | The member-call rule (D41) is structural: it applies to any Context key whose service type declares the member, which covers `.effeck` services, TypeScript `Context.Service` classes, and Effect's `FileSystem` and `Path` | Context keys and Effeck services share call behavior | 4, 13 |
| D91 | **Controversial.** A control-operation call lowers like a service call, `yield* (yield* Approval).approve(x)`, as the prototype did (amending the original Chapter 13 lowering); `handle` provides an implementation that performs the kernel operations | Preserves the prototype implementation pending F7 | 9, 13 |
| D92 | A method in a `.effeck` file becomes an Effect when its body runs or looks up an Effect, or when its expected type returns one; it stays a method whose body is `return Effect.gen({ self: this }, function* () { … })` | Generators must retain the method receiver | 15, 27 |
| D93 | **Controversial.** Service keys use package, module, and name; defaults require `static readonly`; EK2101 requires all three Effect channels; wire emits consumers first | Stable service identity and bounded provider analysis | 4, 14 |
| D94 | Effect acquisitions receive the block Scope; other initializers need Disposable, null, or undefined; Disposable results also run Symbol.dispose | Effect acquisitions and ordinary Disposables have different ownership | 6 |
| D95 | In an Effect function, `for (using x of values)` makes each iteration's body a Scope region; the value is taken first, and `using` or `fork` inside the body join that region | Cleanup belongs to the iteration that owns it | 6, 7 |
| D96 | **Controversial.** A `fork` belongs to the innermost block around it; an arrow function with an expression body has no block, so `() => fork(p)` is `Effect.forkScoped` and keeps `Scope` in R | An expression body has no owning block | 7 |
| D97 | **Controversial.** Continuation H is the program's A, or never with onReturn; handle unions program and clause results. Recorded from the compiler notes | TypeScript fixes H before checking onReturn; mixed early returns can narrow resume too far | 9 |
| D98 | Producers return lazy Streams; direct loops use the consumer fiber, Stream operators use a separate fiber and a handoff with no buffer. Recorded from the compiler notes | Lazy interoperability, cleanup, and bounded stack without running ahead | 8, 26 |
| D99 | EK4201 checks known fiber boundaries and control-requiring producer arguments; EK4202 checks foreign callees' declared control requirements. Recorded from the compiler notes | Diagnose unreachable handlers; runtime enforces fiber ownership | 9, 16 |
| D100 | Code no fiber runs is plain JavaScript for EK4401 too: a module's top level and the plain callbacks it passes to TypeScript functions (`describe` bodies, route handlers), the same places D38 allows runners | Framework callbacks run outside a fiber | 10, 13 |
| D101 | **Controversial.** Only built-ins that parse outside text declare expected failures (`JSON.parse` with `SyntaxError`, `decodeURI` and `decodeURIComponent` with `URIError`); every other built-in's exceptions are defects | Unlisted built-in exceptions remain defects | 10, 26 |
| D102 | **Controversial.** Original platform-import restriction; import half superseded by D108; globals and allowed web standards retained | Platform work needs typed, replaceable Effect services | 10, 11 |
| D103 | EK1001 covers any top-level call that would run a program, including service member calls and Effect-value calls, reported once at the outermost call | One outer call identifies attempted program execution | 13, 28 |
| D104 | Compiled generators are named after their function, variable, or property (an object method `{ *name() {} }.name` when the body refers to that name); `tracing: "exported"` spans exported functions, `"all"` every named lowered function plus methods through `Effect.withSpan("Class.method")`; anonymous lambdas get no span | Names and spans identify source failures | 19, 26, 28 |
| D105 | Vite compiles a project through one shared effeck subprocess per source revision, with cached output; amends D31 | Whole-project checking keeps caller contracts consistent | 28 |
| D106 | After an edit the plugin invalidates every loaded `.effeck` module and its transitive importers, because callers compile differently when a contract changes | Contract changes can change caller lowering | 28 |
| D107 | **Controversial.** Sources must sit under Vite's root; the plugin overrides emit paths and source roots there so output lookup and source maps are predictable | One source root makes output lookup predictable | 28 |
| D108 | EK4402 reports platform execution, including calls and property reads; imports and passing values are allowed. Supersedes the import half of D102 | Effect adapters must accept host function values | 10, 11, 28 |
| D109 | An expected callback or program asks for the Effect form only when its type is effect's own `Effect` interface, or a type parameter constrained to it; subtypes such as `Config` are data (D24), so `Config.orElse`'s fallback stays plain | D24 keeps structured subtypes as data | 13, 16 |
| D110 | The `pipe` look-ahead (D8) reads the first stage's callee signatures, or a function value's own signatures; it runs only when the first argument can be a program, and never checks the stage early | Reading signatures avoids circular inference | 5, 13 |
| D111 | The D49 conversion covers any iterable (`Set`, a `Map`'s values) through its iterated type; an iterable never converts as a record, matching `Effect.all` at run time | Matches Effect.all at run time | 5, 13 |
| D112 | A callback whose expected return mixes plain and Effect (`boolean \| Effect<…>`) is plain when D10 allows and an Effect function otherwise; the same holds wherever both forms fit | D10 selects the eligible form without Effect work | 13, 16 |
| D113 | A lambda with a contract that takes the plain form has type `(…) => A`; where only a plain function fits, inference from its annotation starts from A | Plain inference needs A rather than its Effect wrapper | 14, 16 |
| D120 | Local runtime dependencies use setup's tarball; refresh all three packages with bun update; examples omit bun.lock | Avoid a second Effect copy that prevents child interruption; amends D83 | 27, 28 |
| D121 | **Controversial.** Effect trust follows re-exports and returned values, including assertions from @effect/vitest | Tests need usable assertions; direct Vitest imports and local TypeScript retain boundary checks | 10, 16 |
| D122 | Every runner checks all services it does not supply; it.effect supplies Scope and its layer services | Diagnose missing services before execution | 4, 28 |
| D123 | Declaration prelude imports use free aliases when names are already bound | Avoid duplicate identifiers in declaration consumers | 27 |
| D124 | Setup relinks effeck launchers pointing into another checkout or at missing files | Keep the launcher usable after workspace changes | 28 |
| D125 | **Controversial.** The language is Effeck: binary `effeck`, files `.effeck`, codes `EK`, unscoped npm packages `effeck`, `effeck-runtime`, `effeck-vite` | The `@effectscript` npm scope belongs to another language | 28 |
| D126 | `error NotFound { id: string }` declares a tagged error class with readonly fields | Fewer tokens for the most common declaration | 3 |
| D127 | **Controversial.** Effect types are written `Result!Error!Dependencies`; an empty slot is never, and left-out dependencies are inferred on a function with a body | Fewer tokens than `Effect<A, E, R>` in every contract | 14 |
| D128 | `service` and `control` declare abstract classes whose bodiless members are abstract; an implemented member is EK2103 | Fewer tokens than `abstract class` and `abstract` on every member | 4, 9 |
| D129 | `layer UsersLive: Users { setup; find(id) { ... } }` is `Layer.effect` (or `Layer.succeed` without setup); setup after members is EK2104 | Fewer tokens than `Layer.effect(S, () => { return { ... } })` | 4 |
| D130 | `with AppLive { ... }` provides a layer to a block whose statements stay code of the enclosing function | Fewer tokens than `Effect.provide(() => { ... }, AppLive)`, and `return` keeps its meaning | 4 |
| D131 | `catch (e: NotFound)` handles only failures with that type's `_tag`; several clauses may follow a `try`, and failures no clause names propagate | Fewer tokens than narrowing and rethrowing | 3 |
| D132 | `try { ... } handle Approval.approve(request) { resume(true) }` handles a control's operations; the block and the clauses stay code of the enclosing function | Fewer tokens than `handle(() => ..., Approval, { approve(request, k) { return resume(k, true) } })`, and `return` keeps its meaning | 9 |
| D133 | `schema User { id: string; role: "admin" \| "member" }` declares a `Schema.Struct` and the interface of its decoded type | Fewer tokens than a struct and a type alias, and the two cannot drift apart | 11 |
| D134 | **Controversial.** Members of interfaces and object types are readonly unless written `mutable` | Immutable data is the common case and no longer repeats `readonly` | 10 |
| D135 | A module that exports a parameterless `main` runs it with `NodeRuntime.runMain`, or the runtime's copy without `@effect/platform-node`, when the host starts the module | No `run.effeck` file or runner line per program; importing the module still runs nothing | 1 |
| D146 | Every export of `effect`'s root is in scope; JavaScript's own globals win, host globals do not | Imports from `effect` were 5.9% of the docs' tokens | 1, 27 |
| D147 | **Controversial.** A call is passed by name where a parameter expects a program: `fork(fetchReport())` | Thunks around single calls were 2.6% of the docs' tokens | 5, 13 |
| D148 | **Controversial.** `sync`, `try`, and `callback` are boundary hatches whose arguments are plain TypeScript | One visible escape hatch replaces five Effect constructors and their lambda rules | 10, 28 |
| D149 | `try(…)` parses as a call in `.effeck`; Effect's `{ try, catch }` object is EK4404 | A try statement needs a block, so `try (` is unambiguous | 10, 28 |
| D150 | `runPromise`, `runFork`, `runSync`, `runCallback` are in the prelude, for TypeScript and hatch bodies | The way back in; EK1005 still applies where a fiber runs | 10, 13 |
| D151 | `sleep`, `log`, `retry`, `timeout`, `all`, `forEach`, `race`, and seven more Effect functions need no prefix | They were 329 of the 538 `Effect.` member uses in the docs | 1, 27 |
| D152 | **Controversial.** A value of an Effect data type takes its module's data-first functions as methods: `stream.filter(p).take(3)` is `Stream.take(Stream.filter(stream, p), 3)`; own members win | Removes the module prefix and nesting from data-type code; the emitted calls tree-shake as Effect's do | 5, 13 |
| D153 | A data type's method exists only in a call; any other access is EK4501 | No function value exists to bind | 13, 28 |
| D154 | **Controversial.** The Hack pipe operator in `.effeck` files: `x \|> f(%, a)`, topic `%`, left-associative, looser than every binary operator; supersedes D8 | Fewer tokens and no nesting for operator chains; any argument position | 5, 27 |
| D155 | **Controversial.** A topic follows the by-name rule: where `%` stands for a program, the left side is passed by name; elsewhere it runs once, before the body; both in one pipe is EK1008 | `fetchUser(id) \|> retry(%, policy)` must retry the call, as `retry(fetchUser(id), policy)` does | 5, 13 |
| D156 | A pipe's body must use `%` (EK1006); `%` outside a pipe's body is EK1007; after an operand `%` is still the remainder | The proposal's early errors; `%` keeps its JavaScript meaning where it already has one | 5, 28 |

#### Rules of the runtime and compiler

Proposed in the first draft as P1-P22 and adopted by D29. A row marked **revised** changed because a review verified a counterexample.

| # | Rule | Evidence | Chapters |
| --- | --- | --- | --- |
| P1 | The machine in Chapter 20 is the specification. Every feature is a rule over it | Chapter 18 | All |
| P2 | Runtime: Effect-native prompts (strategy A). Reference semantics: an explicit machine (strategy B), once G16 runs one law suite on both. **Revised**: the two disagree today on the Context a reinstalled prompt records (R9), so passing tests in both are not agreement | 148 kernel tests; 20 reference tests; T7a; review probe R9 | 25 |
| P3 | Deep handlers, dynamic selection, clauses run outside their prompt | T9, clause-outside test | 20 |
| P4 | Context is a snapshot across capture; the reinstalled prompt records the resume site's Context | T1, T13, T15 | 22 |
| P5 | Every consumption of a continuation (resume, transfer, abandon) happens on the capturing fiber | T14, T14b, O04, O06 | 21 |
| P6 | A second resume is a defect at the caller; a root wait that loses a race reports it | T2, T5a-T5c | 21 |
| P7 | An owner frame abandons an unresumed continuation. `transfer` (standard library only) moves ownership to the Scope region around the handler, revokes an earlier transfer, and rejects ownership cycles and parallel Scopes. **Revised**: the capturing fiber's exit is a second owner, so a transfer never outlives its fiber (R5) | T3, T4, T10, O01-O08, K13a-K13e | 21 |
| P8 | Abandonment unwinds like interruption and never disturbs real pending interruptions; a masked handler that swallows it is re-raised. **Revised**: every cleanup failure is reported beside pending interruptions, on every abandoning path (R2b) | kernel-edges, A01-A04, I03, I10, I11, K12a-K12e | 20, 23 |
| P9 | Services always use in-place evidence. Control operations use prompt evidence, or in-place evidence when TypeScript supplies an implementation that always resumes; the checker treats them as fiber-bound either way | In-place and override tests; T8 | 4, 9, 16 |
| P10 | Control operations cannot cross fibers. **Revised**: the runtime is the guarantee (`HandlerNotOnThisFiber`); the fiber-behavior table is the diagnostic (EK4201), and TypeScript functions with a control operation in R are rejected (EK4202) | T7b, T8, probes; review probe R6 | 9, 16 |
| P11 | The frame invariant; source regions compile to Effect frames, never JavaScript `try`, in functions that compile to frames | F1-F3 | 26 |
| P12 | **Revised** as D10: plain JavaScript only with no Effect operation and E = never; pure recursion on the JavaScript stack; `pure` permits inlining only | F9, T6a; review probe R4 | 15 |
| P13 | The kernel bounds synchronous wake-up depth | F10a, K10a, K10b | 24 |
| P14 | A callback type without a contract is effect-polymorphic (D12); D57 leaves this rule intact | Settled decision 2 | 16 |
| P15 | Effect 4 only | Research §10 | 10 |
| P16 | Fork `microsoft/TypeScript` `tsc/` at a published nightly; in-tree package; no shims | Compiler research | 27 |
| P17 | A declaration without a body states its full contract; a bare return type there is an error, never `never` | Settled decision 2 | 14 |
| P18 | **Revised** as D17: `resumeWith` may raise only the operation's declared E, and its R is `never` | Settled decision 3; review probe R7 | 9 |
| P19 | A finalizer's control operations must be handled inside the finalizer | I06, I11 | 17, 23 |
| P20 | Programs start only at runners; calls never create fibers; a module's top level cannot run a program (EK1001). **Revised**: runners are called only where no fiber runs the code (EK1005, D38) | Fable review | 13, 23 |
| P21 | `for...of` over a producer is internal iteration; pulling from outside uses `transfer` | T10, T16 | 8, 26 |
| P22 | **Revised** as D18: a clause whose last action is `resume`, outside any region of the clause, compiles to `return tailResume(k, v)` and iterates in constant stack. The first draft's evidence, T17, covered only a `flatMap`-shaped clause | K11a-K11d; review probe R10 | 20, 26 |

#### Former open questions

| # | Question | Now |
| --- | --- | --- |
| O1 | How are program values spelled? | D7 |
| O2 | Declaration syntax for services and control operations? | D1, D2 |
| O3 | Does v0 expose the handler API? | D16 |
| O4 | Does `function*` mean producer? | D26 |
| O5 | Which owner does `fork` use? | D19 |
| O6 | Is `wire` a compiler intrinsic? | D27 |
| O10 | Does deep pure recursion overflow like JavaScript? | D10 |
| O11 | What does a call to plain TypeScript mean? | D5, D22, D23 |

#### Closed calls for the end-to-end build

| Former item | Decision | Result |
| --- | --- | --- |
| O7 | D57 (**controversial**) | `infer Name` marks an inferred channel; all three return-annotation slots are required; D12 stands |
| O8 | D59 | Upstream issues and proposals are drafted in `docs/upstream/`, not filed |
| O9 | D58, superseded by D125 (**controversial**) | Effeck: `effeck`, `.effeck`, `EK` codes, and unscoped npm packages |
| Remote and repository layout | D60 | The one repository is private at `github.com/roninjin10/effectscript`, with the compiler in `typescript/` |

No language or repository decision waits on the owner. D59's filing boundary remains: these drafts are ready to file, and none is filed without his go-ahead. The build lanes land locally; the orchestrator handles any push.

### 30. Risks and experiments

```text
first, in the Go fork          G19  one checker pass: candidate forms (D56), component fixpoint (D52)
the largest external risk      G1   prompt capture uses undocumented Effect fields
the semantic gate              G16  one law suite for both runtime strategies (R9 first)
the checker's foundation       G8   second-class continuations and the finalizer rule
```

No design question in this list is open: every row states the rule that stands, what could go wrong in building it, and the experiment that checks it. D57-D60 close the former open calls (Chapter 29); D57 and D58 are marked controversial. Upstream filing follows D59.

#### Risks, in the order to work on them

| # | Rule that stands | Risk | Experiment | What would change the rule |
| --- | --- | --- | --- | --- |
| G19 | The call rule is decided by types (Chapter 13) in one checker pass: a lambda's form per overload candidate (D44, D56), E and R of a recursive component by a fixpoint inside it (D52) | The hypothesis survives on the corpus and the probes (`prototype/single-pass/RESULTS.md`): one patched check reaches the prototype's lowering in 46 of 46 files with 0 disagreements, and each probe disagreement is a lambda with two self-consistent lowerings, which D56 decides, or mutual recursion. What remains is the iteration inside a recursive component, which the harness lacks, and holding the three forms in the Go checker, where the harness duplicated each lambda's text | Build both in the Go fork (`typescript/` in this repository, D60), compile the corpus and `prototype/single-pass/probes/`, and compare every lowering decision with the single-pass harness's and every contract with the prototype's | A construct that needs a check outside its own component; its rule changes so that one pass decides it |
| G20 | The prototype covers the language's core (Chapter 27) | It does not yet emit `.d.ts` files, `$direct` metadata, source maps, or hovers; implement EK4202 or external iteration of producers; or test `this` inside Effect methods | Extend the prototype in that order, then port its decisions to the Go fork | A part whose rule cannot be implemented as written |
| G1 | Strategy A is the runtime (Chapter 25) | An Effect release can break capture or abandonment without notice | Run the law suite against every Effect 4 patch release in CI. Prepare a supported primitive (an opaque same-fiber stack segment plus interruption state, or `Effect.prompt`) using the kernel as the reference; the [proposal](../upstream/effect-prompt-primitive.md) is drafted, not filed (D59) | A release breaks a law with no workaround, or the maintainers reject any splice API; capture-heavy code then falls back to strategy B and loses transparency through TypeScript combinators |
| G16 | Strategy B is the reference semantics | The two strategies disagree today (R9) and pass separate suites | Write the Chapter 20 laws once against a small interface with an adapter per strategy; run both. First law: the reinstalled prompt records the resume site's Context | A law the two cannot both express, which would mean Chapter 20 is ambiguous |
| G8 | Continuations are second-class; a finalizer handles its own control operations (Chapter 9) | The static checks are specified, not built | Implement both checks in a checker prototype; run them over the reference programs; count false positives | Correct handlers, such as schedulers, that need escapes `transfer` cannot express |
| G9 | Callback types without contracts are effect-polymorphic (D12); inferred contracts are shown on hover | Unreadable inferred types defeat inference | Infer the reference programs' contracts with the rules of Chapters 14 and 16; check multiple callbacks, returned callbacks, aliases, and declaration round trips | A callback that needs more than one row variable to read correctly |
| G10 | Plain JavaScript only where nothing observable changes (D10) | An optimization that changes behavior breaks settled decision 4 | Compile a corpus twice, all frames and optimized; compare results, event order, and deterministic interruption traces | Any observable difference |
| G2 | The fiber-behavior table is the diagnostic; the runtime is the guarantee (P10) | An unlisted combinator that forks costs a conservative error | Extend `tests/fiber-behavior.test.ts` to every Effect export that takes a program, and generate the checker's table from its results | An export whose fiber behavior depends on a run-time value the type cannot see |
| G3 | A continuation is consumed only on its capturing fiber (P5) | Closing an owning Scope from another fiber while the capturer is alive raises `ContinuationWrongFiber` at the closer; the segment is released only when the capturer exits (K13c) | A per-fiber pending-abandonment queue, drained when the capturing fiber next runs or exits; test producers shared across fibers | A common, correct pattern that needs cross-fiber consumption |
| G12 | A producer is a `Stream` to TypeScript; the adapter runs it on its own fiber (D26) | Stream cancellation might not reach the producer's owner | Adapt a producer to a pull `Stream`; pull from another fiber with `Stream.merge`; cancel early; check that the producer's finalizers ran once and that it never ran ahead | Stream cancellation that cannot reach the producer's fiber |
| G13 | `wire` reports missing, duplicate, and cyclic providers from Layer types (D27) | Layer types might not carry enough to order a graph | Implement `wire` over Layer types; test missing, ambiguous, cyclic, failing-acquisition, residual-requirement, and run-time-selected graphs | Layer types that do not carry enough to order the graph |
| G11 | The ABI and diagnostics map through `.effeck` spans (Chapters 27, 28) | The Go fork emits declarations, but its generated nodes carry no source positions and its language service is untested | In the fork, lower `displayName`; check the declaration, a type error at the `.effeck` span, and hover | Spans that lowering cannot map back to the `.effeck` source |
| G18 | Workflow bodies are ordinary programs under replay (D42) | A producer or handler inside a replayed body might keep state across executions | Run a workflow whose body iterates a producer and installs a handler, suspend it durably mid-iteration, replay it, and check that finalizers ran once per execution and nothing carried over | State that survives into a replay |
| G5 | Effect's own wake-ups are unbounded (F10), and its generator frames skip `finally` (F1-F3) | Effeck code waiting on `Deferred` chains inherits the overflow, and TypeScript users leak resources | The [upstream index](../upstream/README.md) links minimal reproductions, expected behavior, and suggested fixes; every entry is drafted, not filed (D59) | Upstream treats unbounded synchronous wake-ups as intended; Effeck then wraps `Deferred` and `PubSub` waits in `suspend` |
| G14 | Abandonment unwinds as an interruption (P8) | Finalizers see a sentinel interrupt id; diagnostics cannot yet say "abandoned" | Annotate the abort cause, or add a reason, and check what `Cause.pretty` and finalizers show | |
| G15 | Typed failures do not cross JavaScript frames in v0 (D10, D9) | A pure helper that throws a typed error compiles to frames, not plain JavaScript | Bridge typed failures as branded exceptions caught at the nearest frame; measure cost and correctness | |
| G17 | Capture copies the segment (Chapter 25) | Capture at every level of a deep recursion copies quadratically (79 ms at depth 8,000) | Follow the [segmented-stack draft](../upstream/effect-segmented-stack.md): prototype opaque chunked segments in a patched Effect build and rerun `bench/deep-capture.ts` | Real producers nest deep enough for the quadratic term to dominate their own work |

#### Closed

| # | Question | Result |
| --- | --- | --- |
| G4 | A handoff for producers that cannot act on a lost race | Built and tested (H1-H4, Chapter 21) |
| G6 | Spans and `AsyncLocalStorage` across prompt capture | Spans correct (G6a, T01). `AsyncLocalStorage` is not delimited (G6b), hence the rule in Chapter 22 |
| G7 | The cost of quadratic capture | Measured (Chapter 25); remaining work is G17 |
| W | Hand-lowering the reference programs to Chapter 26's shapes and running them on the kernel | Done by the prototype compiler, which lowers mechanically instead: its 46 example files, including the six reference programs (`ref1` to `ref6`), compile and run on the kernel (`prototype/esc`) |
| Prototype | Thirty findings from compiling the corpus | Settled: D44-D53 |
| Single pass | Whether the call rule needs more than one checker pass | Not for lowering: one patched check per file reaches the prototype's output in 46 of 46 files (817 call sites, 257 lambdas), in 46 checks instead of 124. The probes' 16 lowering disagreements settled D56; mutual recursion amended D52; the Go work stays G19 (`prototype/single-pass/RESULTS.md`) |
| Review, first draft | Five kernel bugs from the conformance review (abandonment Exit, pending interruptions, abort leaking through a finalizer's capture, transfer revocation, registration leak) | Fixed; the review's 54 tests run with the suite |
| Reviews, 2026-10-16 | Five defects in implementation and lowering: tail resume from a compiled clause (R10, Fable 1), a cleanup defect dropped by explicit `abandon` (R2b), a transfer outliving its fiber (R5), `Effect.catch` recovering a defect (R1), and masked acquisition blocking cancellation (R3, Fable 2) | Fixed. Regression tests K11-K14 cover the kernel changes, and 12 of them fail on a copy of the kernel with the fixes reverted; K15 pins Effect's own acquisition behavior (`experiments/KERNEL-CHANGES.md`) |
| Reviews, 2026-10-16 | Rule gaps: catch with mixed causes, `using` ownership, `resumeWith` requirements, fork outside a region, provider sharing, control operations through TypeScript, function values in plain positions, trust for foreign code | Decided: D13, D14, D17, D19, D21, D20, D9, D5 with D22 and D23 |
| Coverage audit | Seventeen gaps found by mapping every Effect docs page (`docs/research/effect-feature-coverage.md`) | Decided: D22-D28 and D37-D43; tooling D30-D36; partial annotations are settled by D57 |

#### Reference programs

Programs the compiler must handle end to end. The prototype runs a version of each (`prototype/esc/examples/ref1` to `ref6`):

1. **Existing Effect service.** Import a generic, overloaded, aliased TypeScript Effect function; compose it without changing E or R; provide a Layer from TypeScript.
2. **Provider graph.** A shared database, two dependent services, a service with a default, test overrides, a failing acquisition, a missing provider, an ambiguous provider, and a cycle. Acquisition dependencies must not leak to callers.
3. **Resourceful retry.** Acquire, fail, release, retry with fresh locals, then cancel during a later attempt, including an interruptible acquisition. Each acquisition is released exactly once.
4. **Callbacks and foreign code.** A pure `Array.map`; a rejected callback that pauses; an effect-aware traversal; a trusted function with `@throws`; a callback API adapted with `suspend` under cancellation.
5. **Programs as data.** Store, return, retry, and concurrently compose a program without running it early; read a composed `Config`; compare two `Exit` values.
6. **Handlers.** Nested prompts, one resumption, a rejected double resumption, abandonment with finalization, a producer with early `break` over 100,000 elements in constant stack, a producer consumed as a `Stream` from another fiber, and a control operation rejected at a fork.

## Appendices

### A. Evidence index

Run everything with:

```sh
cd experiments
bun install        # effect@4.0.5 exactly
bun test           # 148 tests in 19 files, about a second
bun test ./scratch-review-codex/                     # the Codex review's probes R1-R10
bun test ./scratch-review-fable/suspend-in-acquire.test.ts ./scratch-review-fable/tail-resume-generator.test.ts
bun bench/perform.ts
bun bench/deep-capture.ts
node scripts/wakeup-chain-depth.ts       # F10 depths on this host (also runs under bun)
node scripts/v3-wakeup-chain.mjs 100000  # F10b: the same chain on effect 3.22.3
cd reference-machine && bun install && bun test   # strategy B, 20 tests, about 18 s
```

The design document's build (`docs/tools/render.ts`) runs `bun test` and refuses to build if any test fails.

#### How this revision was checked

The first draft had two independent reviews. Codex (GPT-6.1 Sol) wrote 54 conformance tests against the kernel and found five bugs, all fixed. Fable reviewed the draft against the original request; its findings shaped Chapters 9, 13, and 26.

The second draft had two more, on 2026-10-16: Codex (GPT-6 Astra, `docs/reviews/2026-10-16-codex-astra.md`) and Fable (`docs/reviews/2026-10-16-fable.md`). Both passed the central test and verified counterexamples with probes. Five were kernel or lowering defects, now fixed with regression tests K11-K15; 12 of those tests fail on a copy of the kernel with the fixes reverted, and K15 pins Effect's own acquisition behavior (`experiments/KERNEL-CHANGES.md`). The rest were rule gaps, now decided (D9-D21). A feature-coverage audit then mapped every section of Effect's documentation to the language (`docs/research/effect-feature-coverage.md`, 309 features); its gaps are decided in D22-D28 and D37-D43, and its map is Appendix C. Then a prototype compiler compiled 46 example files written from this document and ran them on the kernel; its findings are decided in D44-D53. Last, a single-pass experiment reached the same lowering with one patched TypeScript check per file, byte for byte; its probes are decided in D56 and D52.

#### Effect facts

| ID | File | What it shows |
| --- | --- | --- |
| F1-F3 | `tests/effect-facts.test.ts` | JavaScript `finally` and `catch` in `Effect.gen` do not run when a yielded effect fails or the fiber is interrupted |
| F4 | same | A JavaScript exception in a generator body is a defect |
| F5, F6 | same | `Effect.callback` ignores every resume after the first; an asynchronous resume runs the fiber on the resumer's stack |
| F7, F8 | same | A child snapshots Context at fork, with nothing flowing back; `forkChild` children are interrupted when the parent completes |
| F9 | same | Direct recursion limit: about 39,900 frames on Bun 1.4.1 and about 10,150 on Node 26.10 |
| F10a, F10b | `tests/wakeup-chain.test.ts`, `scripts/` | A chain of `Deferred` wake-ups overflows in one fiber and strands the rest; effect 3.22.3 completes the same chain at 100,000 hops |
| T5a, T6b | `tests/kernel.test.ts` | `Effect.callback` drops a late resume silently; plain JavaScript recursion overflows |
| D01-D05 | `tests/conformance/source-claims.test.ts` | Corrections the conformance review made to the first draft's Effect claims: synchronous callback resume, fork-time async context, `zip`'s `concurrent` flag, `catchCause` on a self-raised interrupt, sibling provides sharing a supplied `MemoMap` |
| A3 | `tests/fiber-behavior.test.ts` | Two providers for one key in `Layer.merge`: the later one wins silently |
| K15a | `tests/review-fixes.test.ts` | Default `acquireRelease` masks a wait: interruption waits for the value, which is then released |

#### Kernel laws

| ID | File | What it shows |
| --- | --- | --- |
| T1 | `tests/trace.test.ts` | Nested suspension and resumption keep Context and Scope; the clause sees the outer Context; the fiber waits at its root while the continuation is held |
| T2-T4 | `tests/kernel.test.ts` | Double resume is a defect; a clause that returns without resuming abandons first; interruption order is clause, continuation, outer |
| T5b, T5c, K10a, K10b | `tests/kernel.test.ts`, `tests/wakeup-chain.test.ts` | `suspend` reports lost races, delivers once, survives a 100,000-fiber chain, and releases a deferred value that loses to cancellation |
| T6a | `tests/kernel.test.ts` | 100,001 nested compiled frames captured in one operation and resumed |
| T7a, T7b, T8 | same | Capture through TypeScript-authored `Effect.forEach`; forking combinators and child fibers cannot reach the parent's prompt |
| T9-T11 | same | Deep handlers; external iteration with `transfer`; retry runs fresh continuations |
| T12-T15 | `tests/kernel-edges.test.ts` | Pending interruption at resume; resume-site Context; fiber-affine abandonment (T14b caught a real bug); `HandlerPromptExited` instead of rebinding |
| T16, T17 | `tests/producers.test.ts` | `for...of` lowering: `break` runs the producer's finalizer first; 100,000 elements in constant stack with a `flatMap`-shaped clause |
| K11a-K11d | `tests/review-fixes.test.ts` | A generator clause that returns `tailResume` iterates 100,000 elements at 3 frames, 5 with a per-element `using` and `finally`; yielding a tail-resume request is a defect; a resume inside a `finally` region is not a tail resume |
| K12a-K12e | same | Abandonment reports every cleanup failure beside real pending interruptions: explicit `abandon`, `abandonNow`, owner exit, and Scope exit |
| K13a-K13e | same | A transferred continuation never outlives its capturing fiber; transfer into a parallel Scope is rejected |
| K14a-K14c | same | `catchExpected` recovers only causes made of expected failures, the first one first; mixed causes pass through intact |
| K15b, K15c | same | Interruptible acquisition: cancellation wins, and late or deferred values are released |
| H1-H4 | `tests/handoff.test.ts` | Handoff: one delivery; late delivery during asynchronous shutdown released before cancellation completes; deferred delivery released; delivery after quiescence rejected |
| G6a, G6b | `tests/context-bridges.test.ts` | Spans are correct across capture; `AsyncLocalStorage` is not delimited |
| Probes | `tests/fiber-behavior.test.ts` | 17 combinators keep control on the performing fiber; 7 move it to other fibers |
| A01-A04 | `tests/conformance/abandonment.test.ts` | Finalizer failures and swallowed aborts surface at every abandoning boundary |
| F01-F09 | `tests/conformance/frames.test.ts` | Effect's own frames (restore closures, masks, nested prompts, iterator frames, finalizers) behave correctly when spliced |
| I01-I11 | `tests/conformance/interruption.test.ts` | Interruption at capture, at resume, and during abandonment; real interrupts are never lost or mixed with the internal abort |
| O01-O08 | `tests/conformance/ownership.test.ts` | Nested owners, transfer, revocation, fiber affinity, and the ownership-cycle check |
| S01, S02 | `tests/conformance/scopes.test.ts` | `Effect.scoped`, `acquireRelease`, and Layer resources across resume and abandonment |
| T01, T02 | `tests/conformance/tracing-cache.test.ts` | `Effect.fn` and `withSpan` frames across capture; `Context.Reference` and fiber caches at every boundary |
| W01-W05 | `tests/conformance/wake.test.ts` | Wake budget ordering and release on every losing path |
| X01, X02 | `tests/conformance/async-frames.test.ts` | Effect's async finalizer frame inside a captured segment |
| B | `reference-machine/` | The independent strategy-B machine (Codex, GPT-6.1 Sol): 20 tests on the same laws |

The conformance suite (A, D, F, I, O, S, T, W, X) was written by Codex (GPT-6.1 Sol) as an independent review of the first kernel. Its failing tests found the first five bugs. Tests that encoded the old behavior as counterexamples were rewritten to assert the corrected behavior.

#### Review probes

Counterexamples from the 2026-10-16 reviews, kept as observational tests. Where a defect was fixed, the probe now asserts the fixed behavior.

| ID | File | What it showed |
| --- | --- | --- |
| R1 | `scratch-review-codex/probes.test.ts` | `Effect.catch` recovers a cause holding both a failure and a defect, and drops the defect (D13, K14) |
| R2, R2b | same | Owner-driven abandonment keeps a cleanup defect; explicit `abandon` dropped it while an interruption was pending (D15, K12) |
| R3 | same | Default `acquireRelease` shields `suspend` from cancellation (D25, K15) |
| R4 | same | Inlining an `Effect.sync` operation removes an interruption point (D10) |
| R5 | same | A child that transfers into its inherited Scope and exits stranded the segment (D16, K13) |
| R6 | same | A TypeScript function forks its operation without taking a program argument (D20) |
| R7 | same | A provider around `resumeWith` cannot satisfy the injected program's R (D17) |
| R8 | same | A block-wide Scope closes acquisitions not marked `using` (D14) |
| R9 | same | Strategies A and B disagree on the next clause's Context after a resume (G16) |
| R10 | same | A generator clause written `return yield* resume(k, v)` defeats constant-stack iteration (D18, K11) |
| Fable probes | `scratch-review-fable/` | The masked adapter's race branches never run; generator clauses grow 200,003 frames over 100,000 elements before K11 |
| Services | `scratch-abstract-services/probe.ts` | Structurally equal abstract classes alias in R, and the program dies with `Service not found` (D1) |

#### Prototype compiler

The prototypes predate the rename to Effeck, so their sources keep the old `.es` extension.

```sh
# From the repository root:
bun prototype/esc/examples/run-all.ts       # 46 of 46 example files pass, about 16 s
bun prototype/esc/src/cli.ts build prototype/esc/examples/ch01-first-look.es
bun prototype/esc/src/cli.ts run prototype/esc/examples/ch09-handlers.es
```

| What | Where | What it shows |
| --- | --- | --- |
| The compiler | `prototype/esc/src/` | Type-directed lowering of `.es` to TypeScript on `typescript@5.9.3`, with the experiments kernel as its runtime; about 3,560 lines |
| 26 runnable examples | `prototype/esc/examples/*.es` | Each compiles with no errors and produces its stated result, including the six reference programs (`ref1` to `ref6`) |
| 20 expected failures | `prototype/esc/examples/*.fail.es` | Each reports its diagnostic code at the right span |
| `regions.es` | same | Completion records through nested `try`, `catch`, and `finally` match plain JavaScript's results and order |
| `stack-depth.es` | same | Producer loops and handler clauses stay under 50 frames over 100,000 elements |
| `scratch/probe2.ts` | `prototype/esc/scratch/` | The service emission of D45: nominal R, working `implements`, and a parameter typed with the service meaning its methods |
| Findings | `prototype/esc/FINDINGS.md` | Thirty findings, settled by D44-D53 |

#### Single-pass experiment

```sh
cd prototype/single-pass      # from the repository root
bun patch/apply.ts            # copy TypeScript 5.9.3 to typescript/ and patch it
bun harness/compare.ts        # the corpus: 46 of 46 files, about 10 s
bun harness/compare.ts --dir "$PWD/probes" --verbose
bun harness/passes.ts         # the prototype's decision changes per pass
```

| What | Where | What it shows |
| --- | --- | --- |
| The patched checker | `prototype/single-pass/patch/es-checker.js` | Each lambda classified per overload candidate inside `chooseOverload`, D10 on demand, and narrowing through the call rule: one 668-line block and 15 hooks in TypeScript 5.9.3 |
| The comparison | `prototype/single-pass/harness/compare.ts` | One check per file reaches the prototype's lowering in 46 of 46 files: 817 call sites, 257 lambdas and methods, 0 disagreements, byte-identical output, 46 checks instead of 124 |
| The probes | `prototype/single-pass/probes/` | 17 disagreements: 16 from 6 lambdas with two self-consistent lowerings (D56), 1 from mutual recursion (D52) |
| Results | `prototype/single-pass/RESULTS.md` | The method, the per-file table, and the rules for the Go checker (Chapter 27) |

#### The Go fork

```sh
cd typescript                    # from the repository root
effeck/smoke.sh            # builds tsc (about 75 s cold), then compiles and runs every case
effeck/smoke.sh effeck-infer   # one case
```

| Case | What it shows |
| --- | --- |
| `effeck-wrap` | Functions lower to `Effect.fnUntraced` generators, and JavaScript runs them |
| `effeck-lower` | Calls run by type: pure helpers stay plain, Effect-returning calls run, operands run left to right, and a `Config` stays data |
| `effeck-infer` | Unannotated functions get `Effect<A, E, R>`: a typed failure flows from `lookup` into `main`, and the `.d.ts` declares it |

`typescript/EFFECK.md` records the base, the 11 commits, the upstream lines it changes, and the differential check against the prototype.

#### Research and drafts

- `docs/design/DECISIONS.md`: the decision log, D1-D60.
- `docs/research/effect4-runtime.md`: Effect 4.0.5 runtime facts, with 23 scripts in `experiments/scratch-agent-a/scripts/`. Scripts written for this design live in `experiments/scripts/`.
- `docs/research/continuation-prior-art.md`: OCaml 5, Kotlin, Swift, Loom, WasmFX, Koka, Effekt, Effection, js_of_ocaml, and the papers.
- `docs/research/vibelang.md`: VibeLang `main` (`c60eca97`) and the unmerged checkpoint (`ebe89a2b`).
- `docs/research/typescript-native-compiler.md`: the native compiler upstream and effect-tsgo.
- `docs/research/independent-design-codex.md`: the independent semantics Codex wrote from the same brief.
- `docs/research/pipe-census.md`: how real Effect code uses `pipe` (D8).
- `docs/research/tc39-syntax.md`: the status of every TC39 proposal that could give syntax to an Effect feature, fetched 2026-10-16.
- `docs/research/effect-feature-coverage.md`: every Effect docs section mapped to the language (Appendix C).
- `experiments/KERNEL-CHANGES.md`: the 2026-10-16 kernel fixes.
- `prototype/esc/FINDINGS.md`: what the prototype compiler found.
- `prototype/single-pass/RESULTS.md`: the single-pass experiment (D56, D52).
- `typescript/EFFECK.md`: the Go fork's base, commits, seams, and gaps.
- [Upstream draft index](../upstream/README.md): nine Effect runtime, tooling, documentation, and API drafts with reproductions; all drafted, not filed (D59).

### B. Prior art

Full notes, with quoted sources, are in `docs/research/continuation-prior-art.md`. Rule numbers below refer to its final section.

| Question | Prior art | Effeck |
| --- | --- | --- |
| Second resume | A runtime error everywhere it is checked: OCaml `Continuation_already_resumed`, Kotlin "Already resumed", Swift's checked continuation traps (6.4 rejects it at compile time), Loom "Continuation terminated", WasmFX traps. `Effect.callback` and Effection drop it silently | Defect at the caller (T2). Root waits keep first-wins and report the loss (rule 1) |
| Abandoned continuations | No system cleans them up from the GC. Deterministic cleanup exists only where a handler or scope owns the continuation: Koka's final yield, Effection's `return()`, OCaml's `discontinue`, WasmFX's `resume_throw` | Owner frames, Scope ownership, and the capturing fiber's exit; development-only leak reports (rules 3, 4) |
| Cancellation versus resumption | One take-once cell. Only Kotlin hands the losing value to a release hook (`onCancellation`) | Kernel `suspend` reports the loss and releases deferred values (T5b, K10b); `handoff` releases late values after producer quiescence (H2-H4) (rule 2) |
| `finally` in generated code | Effection loses a halt when a delegated `finally` performs an effect; Effect 4 never calls `return()` | Finalizers compile to Effect frames, never JavaScript `finally` (rule 5) |
| Dynamic state across resume | DDB in the delimited-control literature, Racket, and Koka; snapshot in TC39 AsyncContext (at each `await`, and a generator's creation-time context), Effect 4, Kotlin, Swift, and tokio | Snapshot (Chapter 22). It differs from the research recommendation (rule 6); the reasons and the cost are in Chapter 22 |
| Tail-resumptive operations | Run in place: 10× on Koka's counter benchmark, under the handler's evidence | Run in place under the caller's Context, as Effect services do; call-time lookups must be in the contract (Chapter 4). Differs from rule 7 on whose Context |
| Handler identity | Generative tags (WasmFX), labels (Koka), capabilities (Effekt) | Handler instances, found through Context evidence (rule 8) |
| Multi-shot | Excluded by every fiber or thread runtime; Kyo stays single-shot so it never re-enters a released bracket | Excluded (rule 9) |
| Foreign frames | Every system refuses to capture through them (OCaml `Unhandled`, js_of_ocaml, JSPI `SuspendError`, Racket barriers) | Rejected at compile time (EK4301, EK4202); unhandled at run time (rule 10) |
| Which code gets transformed | Selective: Koka cut CPS-translated functions by over 80%; Stopify measured generators at about 2× slower than exception-based capture | Frames only where capture or parking is reachable (Chapter 15; rule 11) |
| Cross-fiber resume | Kotlin and Swift always schedule | Bounded synchronous wake-ups (Chapter 24; rule 12) |
| Resumable handlers on Effect-TS or ZIO | None found by searches of npm, GitHub code (`fiber._stack`, `"~effect/Effect/successCont"`), and Effect issues and discussions on 2026-10-15. Effect's maintainer: generators "are only valid to express `one-shot` effect types" | No other implementation is known; this kernel is one-shot by design. The claim fails if a search turns one up |

#### TC39 proposals and Effeck

Stages as fetched on 2026-10-16 (`docs/research/tc39-syntax.md`).

| Proposal | Stage | In Effeck |
| --- | --- | --- |
| Explicit Resource Management (`using`) | 4 since 2026-05-19, for ES2027 | Adopted: `using` with Effect's guarantees (Chapter 6) |
| Pipeline operator (Hack `\|>`) | 2 since 2021-08; topic token undecided; "somewhat stuck" (January 2026) | In `.effeck` files with the topic `%`; a topic that stands for a program passes the left side by name (Chapter 5, D154, D155; supersedes D8) |
| AsyncContext | 2 | Prior art for the snapshot rule (Chapter 22). Effeck code keeps request-scoped values in Effect's Context |
| Partial application (`f~(x, ?)`) | 1 | Not adopted. A function with no arguments is the program spelling (D7) |
| Pattern matching (`match`) | 1 | Not adopted. `switch` and `catch` narrowing, and Effect's `Match` module |
| Throw expressions | 2, unchanged since 2017 | Not adopted. `throw` is a statement and a typed failure |
| Extractors | 2 | Not in v0. A possible future syntax for Schema decoding |
| Import defer | 3 | Not used. Deferring a module's evaluation does not change EK1001: its top level still cannot run a program |
| Await dictionary (`Promise.allKeyed`) | 3 | Not needed: `Effect.all` takes a record |
| Abort Controller, Concurrency control | 1 | Not adopted. Effect's interruption and `concurrency` options |
| F#-style pipes; `Function.pipe` and `flow` | Failed Stage 2; withdrawn | Not applicable |

### C. Every Effect feature in Effeck

```ts es
const Port = Config.withDefault(Config.Port("PORT"), 3000)      // unchanged by design: Effect's API, as is

function loadUser(id: string) {
  return retry(Users.find(id), { times: 3 })        // changed: the program is a function
}
```

Every section of Effect 4's documentation, the Effect 3 pages that version 4 dropped, and the `effect@4.0.5` modules the site omits (HTTP, SQL, CLI, AI, cluster, workflow, RPC), mapped to Effeck. **Changed** means Effeck has its own form. **Unchanged by design** means you use Effect's API as you do today, on purpose: the call rule (Chapter 13) makes Effect's API work directly, and a second way to do the same thing would be ceremony. The map comes from the feature-coverage audit (`docs/research/effect-feature-coverage.md`, 309 features), updated to the decisions in Chapter 29. D57 completes partial annotations with explicit `infer` slots.

#### C1. Getting started: the Effect type, creating, running, generators, pipelines

| Feature | In Effeck | Form |
| --- | --- | --- |
| The `Effect<A, E, R>` type | Changed | Inferred for every function; written as a return annotation to have it checked (Chapter 2) |
| Type helpers (`Effect.Success`, `Error`, `Services`) | Unchanged by design | Apply them to the TypeScript type of an Effeck function |
| Annotate one channel, infer the rest | Changed (D57) | In a return annotation with a body, write `Effect<A, infer E, infer R>`; all three slots are required |
| Installation | Changed | `effect@4.0.5`, `effeck-runtime`, and `effeck` (Chapter 28) |
| Importing Effect | Changed | Every module of `effect`'s root needs no import (D146); subpath packages such as `effect/http` are imported |
| `Effect.succeed` | Changed | `return 42` |
| `Effect.fail`, `failSync` | Changed | `throw new HttpError()` |
| `Effect.sync` around a side effect | Changed | Call the code; platform side effects go through Effect modules (Chapter 11), or the `sync` hatch (D148) |
| `Effect.try`, `tryPromise`, `promise` | Changed | The hatches `try(…)` and `sync(async () => …)`, whose arguments are plain TypeScript (D148) |
| `Effect.suspend` | Changed | A call passed by name where a program is expected, or `() => …` (D147) |
| `Effect.callback` | Changed | The hatch `callback((resume, signal) => …)`: `resume(value)`, `resume.fail(error)` (D43, D148) |
| `Effect.void`, `never`, `yieldNow`, `interrupt` | Unchanged by design | Call the value: `Effect.yieldNow()` |
| Runners (`runPromise`, `runSync`, `runFork`, `runCallback`) | Changed | In the prelude; `runPromise(main())` from TypeScript, a top-level call, or a hatch's body, where no fiber runs the code (EK1005, D38, D148) |
| `Effect.gen` and `yield*` | Changed | An ordinary function body, and calls (Chapter 2); `Effect.gen` in `.effeck` is EK1002 |
| `Effect.gen({ self: this }, …)` | Changed | An ordinary method; `this` keeps its JavaScript meaning |
| `Effect.fn`, `fnUntraced` | Changed | `function f() { … }`, untraced unless `tracing` asks for spans (Chapter 28) |
| `map`, `flatMap`, `andThen`, `tap`, `as` | Changed | Statements |
| `pipe` and `.pipe` over programs | Changed | `pipe(() => f(), Effect.retry(…), Effect.timeout(…))` (D8) |
| `pipe` over data | Unchanged by design | `pipe(items, Array.filter(isActive))` |
| `Effect.all`, `zip`, `zipWith` | Changed | Programs as functions: `Effect.all([() => a(), () => b()])` |
| `Effect.fromOption`, `fromResult`, `fromNullishOr` | Unchanged by design | `const n = Effect.fromOption(parse(x))` |
| Control flow inside `Effect.gen` | Changed | `if`, `for`, `while`, and early `return` in an ordinary function |
| Devtools | Changed | The fork's language server for `.effeck`; `@effect/tsgo` stays on `.ts`; Effect's `DevTools` for fibers and spans (Chapter 28) |

#### C2. Error management

On a cause that mixes an expected failure and a defect, Effect's operators below keep their Effect behavior, which differs from Effeck's `catch`; Chapter 17 lists how.

| Feature | In Effeck | Form |
| --- | --- | --- |
| Expected errors and defects | Changed | `throw` is expected; an exception no contract lists, `Effect.die`, and `Effect.orDie` are defects (Chapter 3) |
| A JavaScript `throw` | Changed | Always a typed failure; `Effect.die(new Error(…))` for a bug |
| A JavaScript `catch` around a throwing call | Changed | `catch` sees expected failures only; a dead `catch` is EK2004 (D37) |
| `Data.TaggedError`, `Data.Error`, `Schema.TaggedError` | Changed | `error E { ... }` declares a `TaggedError` class; `Schema.TaggedError` is unchanged |
| Error tracking in E | Changed | Inferred: what a function throws, plus its callees' E, minus what it catches |
| `Effect.catch`, `catchTag`, `catchTags`, `catchIf` | Changed | `try`/`catch` with narrowing; the combinators still work with a program |
| `catchReason`, `catchReasons`, `unwrapReason` | Changed | Narrow `e._tag` and `e.reason._tag` inside `catch` |
| `Effect.die`, `orDie`, `catchDefect`, `catchCause`, `tapDefect` | Changed | The same functions with a program: `Effect.orDie(() => f())` |
| `Effect.result`, `option`, `exit` | Changed | `Effect.result(() => task())` |
| `orElseSucceed`, `firstSuccessOf` | Changed | `try`/`catch`, or `Effect.firstSuccessOf([() => a(), () => b()])` |
| `Effect.match`, `matchEffect`, `matchCause` | Changed | `try` with statements; `Effect.exit(() => f())`, then `Exit.match` |
| `Effect.ignore`, `ignoreCause` | Changed | `Effect.ignore(() => task())` |
| `Effect.retry`, `retryOrElse` | Changed | `Effect.retry(() => request(), { times: 5 })`, with fresh locals per attempt |
| `Effect.timeout`, `timeoutOption`, `timeoutOrElse` | Changed | `Effect.timeout(() => task(), "1 second")`; E gains `Cause.TimeoutError` |
| `Effect.sandbox` | Changed | `Effect.sandbox(() => task())` inside `try`; `catch (cause)` gets the cause |
| `Effect.validate`, `partition` | Unchanged by design | The same calls, with Effeck callbacks |
| `Effect.all` with `mode: "result"` | Changed | Programs as functions, the same options |
| `mapError`, `mapBoth`, `tapError`, `flip`, `filterOrFail` | Changed | `catch (e) { throw new Other({ e }) }`; `if (…) throw new E()` |
| Parallel and sequential errors (`Cause.reasons`) | Unchanged by design | `catch` gets the first expected failure (D28); `Effect.catchCause` sees all |
| Yieldable errors | Changed | `throw new E()`; `new` never runs anything (D24) |
| A contract with E = `never` | Changed | Checked by the compiler (EK2001) |

#### C3. Requirements management: services, layers, memoization

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Context.Service` class | Changed | `service Users { find(id: string): User!UserNotFound }` (D1, D128) |
| Service keys | Changed | Derived from package, path, and class name; duplicates are EK3001 |
| `yield* Users`, then a method | Changed | `Users.find(id)`; `Users()` gives the implementation |
| `Users.use`, `useSync` | Changed | Call the member: `Users.find(id)` |
| A member that returns a plain value (`Path.join`) | Changed | `Path.join(a, b)`; the service joins R (D41) |
| A member that is an `Effect` value | Unchanged by design | Call it: `Terminal.readLine()` |
| The `make` option and `static layer` | Changed | `static make()` and `static readonly layer` on the service class (D40) |
| `Context.Reference` with a default | Unchanged by design | `Context.Reference(key, { defaultValue })` as is, or `static readonly default` on a service class (D40) |
| `Effect.serviceOption` | Unchanged by design | `Effect.serviceOption(Mailer)` |
| Default services (`Clock`, `Console`, `Random`, `Tracer`, `ConfigProvider`) | Unchanged by design | `Clock.currentTimeMillis()`; R stays empty |
| `Effect.provideService` | Changed | `Effect.provideService(() => program(), Mailer, quiet)` |
| `Context.make`, `Context.add` | Unchanged by design | Values, provided with `Effect.provide(() => program(), context)` |
| `Layer.succeed`, `effect`, `sync` | Changed | `Layer.effect(Users, () => { const db = Database(); return { … } })` |
| Layers that hold resources | Changed | Acquire without `using`; the layer's Scope owns the resource |
| Requirements that leak into methods | Changed | EK3101 |
| `Layer.merge`, `mergeAll`, `provide`, `provideMerge` | Unchanged by design | The same functions; `wire(…)` orders them and reports graph errors (D27) |
| Missing, duplicate, and cyclic providers | Changed | EK3201, EK3202, EK3203 |
| `Effect.provide` | Changed | `Effect.provide(() => program(), AppLive)`; each `provide` builds its layers once (D21) |
| Layer memoization, `Layer.fresh`, MemoMaps | Unchanged by design | As in Effect; provide at `main`, or run entry points through one `ManagedRuntime` (D21) |
| `Layer.launch`, `build`, `unwrap`, `mock`, `tap`, `catch` | Unchanged by design | Values and calls as in Effect |
| Fake services | Changed | `Layer.succeed(Users, { find: (id) => ada, rename: () => {} })` |

#### C4. Resource management

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Effect.scoped` | Changed | A block that declares `using` |
| `acquireRelease`, `acquireUseRelease` | Changed | `using lock = Effect.acquireRelease(() => acquire(), (l) => release(l))` |
| Interruptible acquisition | Unchanged by design | `acquireRelease(…, { interruptible: true })`; `using` keeps the choice (D25) |
| A resource the caller owns | Changed | Acquire without `using`; `Scope` stays in R (D14) |
| `ensuring`, `onExit`, `onError`, `addFinalizer` | Changed | `finally`; `Effect.onExit(() => f(), (exit) => …)` when the exit matters |
| `Scope.make`, `close`, `provide` | Unchanged by design | The same calls |
| `Disposable` resources | Changed | `using x = disposable` registers `[Symbol.dispose]()` |
| `RcRef`, `RcMap`, `Pool`, `ScopedRef`, `ScopedCache`, `LayerMap` | Changed | `using pool = Pool.make({ acquire: () => connect(), size: 10 })` |
| Cleanup when a handler abandons a program | Changed | No Effect equivalent; `using` and `finally` run before `handle` returns |

#### C5. Observability

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Effect.log` and its levels | Unchanged by design | `Effect.logWarning("disk low")` |
| A minimum log level for a region | Changed | `Effect.provideService(() => program(), References.MinimumLogLevel, "Debug")` |
| `annotateLogs`, `withLogSpan` | Changed | With a program: `Effect.withLogSpan(() => handle(req), "handler")` |
| `annotateLogsScoped` | Unchanged by design | The same call; a `using` block or the caller supplies the Scope |
| `Logger.make`, `Logger.layer`, formats, `Logger.toFile` | Unchanged by design | Values, provided as layers |
| `Console` | Unchanged by design | `Console.log("hi")`; `console.log` is EK4402 |
| Metrics (`counter`, `gauge`, `histogram`, `summary`, `frequency`, `timer`) | Unchanged by design | Defined at the top level; `Metric.update(requests, 1)` |
| `trackSuccesses`, `trackDuration` | Changed | `Effect.trackDuration(() => task(), latency)` |
| `Metric.CurrentMetricAttributes` | Changed | Provided around a program with `Effect.provideService` |
| Spans per function (`Effect.fn("name")`) | Changed | `"tracing"` in `tsconfig.json`, or `Effect.withSpan` where you want one (Chapter 28) |
| `Effect.withSpan` | Changed | `Effect.withSpan(() => loadUser(id), "load-user")` |
| `annotateCurrentSpan`, `currentSpan`, `linkSpans` | Unchanged by design | The same calls |
| Logs as span events | Unchanged by design | Spans stay correct across handler capture (G6a) |
| Exporting traces, logs, and metrics (`OtlpTracer`, `OtlpLogger`, `OtlpMetrics` from `effect/observability`) | Unchanged by design | Layers, provided as in Effect: `OtlpTracer.layer({ url, resource: { serviceName } })` |
| `FiberSet`, `FiberMap`, `FiberHandle` | Changed | `using set = FiberSet.make()`, then `FiberSet.run(set, () => task())` |
| Supervisors (Effect 3) | Unchanged by design | Effect 4 has none; `FiberSet` and `fork` |

#### C6. Configuration

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Config.String`, `Port`, `Int`, `Boolean`, `Duration`, `URL`, `Redacted` | Unchanged by design | Data until called: `const Port = Config.Port("PORT")`, then `Port()` (D24) |
| `Config.all`, `withDefault`, `nested`, `option`, `orElse`, `schema` | Unchanged by design | Composed as data, read once by calling |
| `ConfigProvider` (`fromEnv`, `fromUnknown`, `layer`) | Unchanged by design | Provided as a layer |
| `ConfigProvider.fromDotEnv()`, `fromDir()` | Unchanged by design | Calls that run, inside a function |
| `config.parse(provider)` in tests | Unchanged by design | `AppConfig.parse(provider)` returns an Effect, so the call runs it |
| A Layer chosen by config (`Layer.unwrap`) | Changed | `Layer.unwrap(() => …)`, with a program |
| `Redacted` | Unchanged by design | `Redacted.value(DatabaseUrl())` |

#### C7. Runtime

| Feature | In Effeck | Form |
| --- | --- | --- |
| `NodeRuntime.runMain`, `BunRuntime.runMain` | Changed | `NodeRuntime.runMain(() => main())` at a module's top level |
| `runMain` options | Changed | The same options, with a program |
| One-shot runners and `run*With(context)` | Changed | Programs as functions, only where no fiber runs the code (D38) |
| `ManagedRuntime` | Unchanged by design | `ManagedRuntime.make(AppLive)`; `runtime.runPromise(() => handle(req))` in a framework callback, or from TypeScript |
| Top-level `await` | Changed | No `await`; the top level builds values and calls runners (EK1001) |
| Scheduling and fairness | Unchanged by design | 2048 operations per slice; a plain JavaScript region is atomic |
| `ErrorReporter`, `Effect.withErrorReporting` | Changed | `Effect.withErrorReporting(() => program())`: the thunk is a program (D39) |

#### C8. Scheduling

| Feature | In Effeck | Form |
| --- | --- | --- |
| Schedule constructors (`spaced`, `fixed`, `exponential`, `recurs`, `cron`) | Unchanged by design | `Schedule.exponential("250 millis")` |
| Schedule combinators | Unchanged by design | `.pipe` over a schedule value |
| Effectful schedule callbacks (`tap`, `modifyDelay`) | Unchanged by design | Effeck callbacks |
| `Effect.repeat` | Changed | `Effect.repeat(() => poll(), { schedule, until: (s) => s === "done" })` |
| `Effect.retry` with a schedule | Changed | `Effect.retry(() => f(), Schedule.exponential("100 millis"))` |
| `Effect.schedule`, `repeatOrElse` | Changed | Programs as functions |
| `Cron.parse`, `Schedule.cron` | Unchanged by design | `Effect.fromResult(Cron.parse("0 3 * * *", "UTC"))` |
| A cron job's lifetime | Changed | `fork(() => Effect.schedule(() => report(), Schedule.cron(cron)))` inside `main` |

#### C9. State management

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Ref` | Unchanged by design | `const ref = Ref.make(0); Ref.update(ref, (n) => n + 1)` |
| Programs stored as object fields | Changed | `inc = () => Ref.update(this.value, (n) => n + 1)` |
| `SynchronizedRef.updateEffect` | Unchanged by design | `SynchronizedRef.updateEffect(ref, (xs) => [...xs, load()])` |
| `SubscriptionRef.changes` | Unchanged by design | A `Stream`: iterate it with `for...of` |
| Transactions (`TxRef`, `Effect.tx`) | Changed | `Effect.tx(() => TxRef.update(ref, (n) => n + 1))` |

#### C10. Caching

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Effect.cached` | Changed | `const get = Effect.cached(() => load())`, then `get()` |
| `cachedWithTTL`, `cachedInvalidateWithTTL` | Changed | Programs as functions; `get()` and `invalidate()` |
| `Cache` | Unchanged by design | `Cache.make({ capacity: 100, lookup: (id) => fetchUser(id) })` |

#### C11. Batching

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Request.Class`, `TaggedClass` | Unchanged by design | The same classes |
| `RequestResolver.make`, `fromEffect`, `makeGrouped`, `batchN` | Unchanged by design | Callbacks are Effeck functions |
| `Effect.request` | Unchanged by design | `Effect.request(new GetUser({ id }), resolver)` |
| Batches form under concurrency | Unchanged by design | Sequential calls never batch, as in Effect |
| `RequestResolver.withCache` | Unchanged by design | Inside a function or a Layer |
| A resolver that needs a service | Changed | `Layer.effect(Todos, () => { const http = Http(); … })` |

#### C12. Concurrency

| Feature | In Effeck | Form |
| --- | --- | --- |
| The `concurrency` option | Changed | Programs as functions, the same options |
| Struct and record forms of `Effect.all` | Changed | Record fields convert to programs |
| A computed array of programs | Changed | `Effect.all(stores.map(makeSync))` (D9) |
| `Effect.forEach` with concurrency | Unchanged by design | The same call |
| `Effect.zip` with `concurrent: true` | Changed | Programs as functions |
| `race`, `raceFirst`, `raceAll` | Changed | Programs as functions; each has its own exit rule (Chapter 7) |
| `forkChild`, `forkDetach`, `forkScoped`, `forkIn` | Changed | Programs as functions |
| Background work owned by a block | Changed | `fork(() => heartbeat())` (D19) |
| `Fiber.join`, `await`, `interrupt`, `joinAll`, `interruptAll` | Unchanged by design | The same calls |
| Interruption is not an error | Changed | `catch` never sees it; `finally` and `using` always run |
| `uninterruptible`, `uninterruptibleMask`, `interruptibleMask` | Changed | Programs as functions |
| `onInterrupt` | Changed | `Effect.onInterrupt(() => task(), () => Console.log("stopped"))` |
| `Effect.forever` | Changed | `while (true) tick()` |
| `Deferred` | Unchanged by design | `Deferred.make<number>()`, `Deferred.await(d)` |
| `Deferred.complete` with a program | Changed | `Deferred.complete(d, () => task())` |
| `Queue` | Unchanged by design | `Queue.offer(queue, 1)` |
| `PubSub.subscribe` | Changed | `using subscription = PubSub.subscribe(pubsub)` |
| `PubSub.publish` | Unchanged by design | The same call |
| `Semaphore` | Unchanged by design | `Semaphore.make(1)`; `mutex.withPermits(1)(() => task())` |
| `Latch` | Unchanged by design | `latch.open()`: an `Effect` member runs when called |
| What a forked fiber sees | Unchanged by design | A snapshot of the parent's Context |
| Control operations across fibers | Changed | EK4201 (Chapter 9) |

#### C13. Async interop

| Feature | In Effeck | Form |
| --- | --- | --- |
| `async` and `await` | Changed | Only inside a hatch's function; elsewhere a call waits (D148) |
| `Effect.promise`, `Effect.tryPromise` | Changed | `sync(async (signal) => …)` and `try(async (signal) => …)`; the `AbortSignal` cancels on interruption (D148) |
| `Effect.callback` | Changed | The `callback` hatch; `resume(value)`; `suspend` for resources (D43, D148) |
| `Effect.runPromise` from Promise code | Changed | `runPromise`, from TypeScript, a top-level callback, or a hatch's body (D38, D148) |
| Async iterables and event emitters | Unchanged by design | `Stream.fromAsyncIterable`, `Stream.fromEventListener` |
| Synchronous code that throws | Changed | `try(…)`; a dead `catch` is EK2004 (D37) |
| Effect 3 libraries | Changed | Foreign code: `try(Effect3.runPromise(p), (error) => …)` |

#### C14. Streams

| Feature | In Effeck | Form |
| --- | --- | --- |
| A sequence produced by code that waits between values | Changed | A `function*` producer and `for...of` (Chapter 8) |
| Streams from data (`make`, `range`, `fromIterable`, `tick`) | Unchanged by design | `Stream.range(1, 10)` |
| `fromEffect`, `fromEffectRepeat` | Changed | Programs as functions |
| `unfold`, `paginate` | Unchanged by design | Effeck callbacks |
| `Stream.callback`, `fromQueue`, `fromPubSub`, `fromEventListener` | Unchanged by design | The same calls; inner JavaScript callbacks cannot pause (EK4301) |
| Operators (`map`, `filter`, `mapEffect`, `merge`, `buffer`, `debounce`, `groupedWithin`) | Unchanged by design | The same calls; a producer converts to a `Stream` (D26) |
| `runCollect`, `runForEach`, `runFold`, `runDrain`, `run` | Unchanged by design | Calls that run the stream |
| `Stream.toPull` | Changed | `for...of` over a `Stream` (D26) |
| `Stream.catch`, `retry`, `timeout` | Unchanged by design | The same calls; `try`/`catch` applies to the call that runs a stream |
| `Stream.scoped` | Unchanged by design | `Stream.scoped(stream)`: it takes a Stream |
| `Stream.ensuring` | Changed | `Stream.ensuring(stream, () => cleanup())`: the finalizer is a program |
| `Stream.partition`, `broadcastN` | Changed | `using`, because they need a Scope |
| Producers and streams together | Changed | A producer is a `Stream` wherever one is expected (D26) |
| `Stream.toAsyncIterable` | Unchanged by design | From TypeScript, with `for await` |

#### C15. Sinks

| Feature | In Effeck | Form |
| --- | --- | --- |
| Sinks (`sum`, `count`, `collect`, `head`, `drain`) | Unchanged by design | `Stream.run(stream, Sink.sum)` |
| `Sink.fold`, `foldUntil`, `forEach` | Unchanged by design | Effeck callbacks; pure ones pass as plain functions (D9) |
| A producer into a sink | Changed | The producer converts to a `Stream` (D26) |

#### C16. Testing

| Feature | In Effeck | Form |
| --- | --- | --- |
| `it.effect` | Changed | `it.effect("name", () => { … })` |
| `it.live`, `.each`, `.prop`, `it.flakyTest` | Unchanged by design | The same calls |
| `layer(AppLive)("suite", (it) => …)` | Unchanged by design | The same call |
| Fake services | Changed | Object literals in `Layer.succeed` |
| An override for one region | Changed | `Effect.provideService(() => program(), Mailer, quiet)` |
| Scripted answers to requests | Changed | `handle` (Chapter 9) |
| `TestClock` | Unchanged by design | `TestClock.adjust("1 minute")` |
| `TestConsole`, `TestCrypto` | Unchanged by design | `TestConsole.logLines()` |
| Asserting on an `Exit` | Unchanged by design | `Exit.succeed(1)` is data (D24) |
| Promise-style tests | Changed | Stay in `.ts`, or move to `it.effect` (D38) |
| Property tests | Unchanged by design | `it.effect.prop`, `Arbitrary.checkEffect` |

#### C17. Code style

| Feature | In Effeck | Form |
| --- | --- | --- |
| Run once, at the edge | Changed | `NodeRuntime.runMain(() => main())` |
| Avoid tacit calls | Unchanged by design | The same advice |
| Dual APIs | Changed | Data-first with a program; data-last inside `pipe` |
| Branded types | Unchanged by design | A failed check throws a defect; `Int.result(n)` for a typed error |
| Pattern matching with pure handlers (`Match`) | Unchanged by design | The same module |
| Pattern matching with effectful branches | Changed | `switch` or `if`; effectful `Match` handlers are EK4301 (D9) |
| Do notation (`Effect.Do`, `bind`, `let`) | Changed | Statements |
| Control flow operators (`Effect.when`, `whileLoop`) | Changed | `if` and `while` |

#### C18. Data types

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Option` | Unchanged by design | Plain data; `Option.gen` stays in `.ts` |
| An `Option` to a typed failure | Changed | `Effect.fromOption(found)`, or `throw` |
| `Result` (Effect 3's `Either`) | Unchanged by design | Plain data |
| `Effect.result`, `Effect.exit` | Changed | Programs as functions |
| `Effect.fromResult` | Unchanged by design | The same call |
| `Exit` constructors and readers | Unchanged by design | Data (D24); `Exit.match` |
| `Cause` | Unchanged by design | `Effect.catchCause(() => f(), (cause) => …)` |
| `Data.Class`, `TaggedClass`, `taggedEnum` | Unchanged by design | The same functions |
| Error classes | Changed | `error E { ... }` declarations; `throw` |
| `new` of a class whose instances are Effects | Changed | `new` constructs and never runs (D24) |
| `Chunk`, `HashSet`, `HashMap`, `Duration`, `BigDecimal`, `Redacted` | Unchanged by design | The same modules |
| `DateTime` | Unchanged by design | `DateTime.now()`; `new Date()` is EK4403 |
| A time zone for a region | Changed | Programs as functions |

#### C19. Traits and behaviours

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Equal`, `Hash` | Unchanged by design | The same functions; `===` keeps its JavaScript meaning. A class implements `[Equal.symbol]` and `[Hash.symbol]` with plain methods (D55, Chapter 15) |
| `Equivalence`, `Order` | Unchanged by design | The same modules |

#### C20. Schema

| Feature | In Effeck | Form |
| --- | --- | --- |
| Defining schemas | Unchanged by design | Values, built at the top level |
| `decodeUnknownEffect`, `decodeEffect`, `encodeEffect` | Unchanged by design | Calls that run; `SchemaError` joins E |
| `decodeUnknownSync`, `is`, `asserts` | Unchanged by design | Trusted; their exceptions are defects (D37) |
| `decodeUnknownExit` | Unchanged by design | An `Exit` value (D24) |
| Filters and transformations | Unchanged by design | The same functions |
| `transformEffect`, `checkEffect` | Unchanged by design | Effeck callbacks |
| Defaults given as Effects (`withConstructorDefault`) | Changed | A program: `Schema.withConstructorDefault(() => Clock.currentTimeMillis())` |
| `Schema.Class`, `TaggedClass` | Unchanged by design | The same classes |
| `Schema.TaggedError` | Changed | `throw new HttpError({ status: 500 })` |
| `make`, `makeEffect` | Unchanged by design | `make` throws defects; `makeEffect` runs |
| Annotations, formatters, JSON Schema, Arbitrary, compilers | Unchanged by design | The same functions |
| `Config.schema` | Unchanged by design | Data (D24) |

#### C21. Platform

| Feature | In Effeck | Form |
| --- | --- | --- |
| Node.js and browser APIs | Changed | Not callable from `.effeck` (EK4402); Chapter 11 names each replacement |
| Platform layers (`NodeServices.layer`) | Changed | `Effect.provide(() => main(), NodeServices.layer)` |
| `FileSystem` | Unchanged by design | `FileSystem.readFileString(path)`, from `effect/FileSystem` |
| `FileSystem.open` (a scoped `File`) | Changed | `using file = FileSystem.open(path)` |
| `FileSystem.layerNoop` | Changed | Its methods return values, not Effects |
| `Path` | Unchanged by design | `Path.join(a, b)`; `Path` joins R (D41) |
| `Terminal`, `Stdio` | Unchanged by design | `Terminal.readLine()` |
| Child processes | Unchanged by design | `ChildProcessSpawner.string(ChildProcess.make("git", ["status"]))`; the command is data (D24) |
| `KeyValueStore` | Unchanged by design | From `effect/persistence` |
| Sockets | Changed | `using` for a scoped reader |
| Workers | Unchanged by design | Effeck handlers |

#### C22. HTTP

| Feature | In Effeck | Form |
| --- | --- | --- |
| `HttpClient` | Unchanged by design | `HttpClient.get(url)` runs |
| Response bodies | Unchanged by design | `response.json()` |
| Client middleware | Unchanged by design | `.pipe` over a client value |
| `HttpRouter.add` handlers | Unchanged by design | A handler function, or a program (D39) |
| Serving | Changed | `NodeRuntime.runMain(() => Layer.launch(HttpRouter.serve(routes)))` |
| `HttpApi` definitions, middleware, security | Unchanged by design | Values |
| `HttpApi` handlers | Unchanged by design | Effeck functions |
| `HttpApiClient` | Unchanged by design | Calls that run |

#### C23. SQL

| Feature | In Effeck | Form |
| --- | --- | --- |
| `SqlClient` | Unchanged by design | `const sql = SqlClient.SqlClient()` |
| `` sql`…` `` statements | Unchanged by design | Data until called (D24): `` sql`SELECT * FROM users`() `` runs it, and `.stream` stays reachable |
| Fragments (`sql.in`, `sql.insert`, `sql.and`) | Unchanged by design | The same calls |
| `sql.withTransaction` | Changed | `sql.withTransaction(() => insertAll())` |
| `SqlSchema`, `SqlResolver`, `SqlModel`, migrations | Unchanged by design | The same modules |
| Driver layers read from config | Unchanged by design | `Config` arguments are data (D24) |

#### C24. CLI

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Command.make`, `withSubcommands`, `withHandler` | Unchanged by design | Data (D24), declared at the top level |
| Flags and arguments | Unchanged by design | Values |
| Prompts | Unchanged by design | Data until called |
| Running a command | Changed | Inside `main`, handed to a runner |

#### C25. AI

| Feature | In Effeck | Form |
| --- | --- | --- |
| `LanguageModel.generateText`, `generateObject` | Unchanged by design | Calls that run |
| `LanguageModel.streamText` | Unchanged by design | A `Stream` |
| `Tool.make`, `Toolkit.make` | Unchanged by design | Data (D24) |
| Tool handlers | Changed | `toolkit.toLayer(() => ({ search: ({ q }) => runSearch(q) }))` |
| `Chat` | Unchanged by design | `Chat.empty()` |
| `Effect.withExecutionPlan` | Changed | `Effect.withExecutionPlan(() => program(), plan)` |
| `AiError` reasons | Changed | Narrow `e.reason._tag` in `catch` |
| Provider layers read from config | Unchanged by design | `Config` arguments are data (D24) |

#### C26. Cluster, workflow, and RPC

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Rpc.make`, `RpcGroup.make` | Unchanged by design | Values |
| `group.toLayer(handlers)` | Unchanged by design | An object of Effeck handlers (D39) |
| RPC clients and servers | Changed | `using client = RpcClient.make(UserRpcs)` |
| Entities | Unchanged by design | `Counter.client()` |
| Workflows | Unchanged by design | Bodies are Effeck functions (D42) |
| Activities | Unchanged by design | Data until called (D24) |
| `DurableClock.sleep`, `DurableDeferred.await` | Unchanged by design | Calls that run |
| Replay | Unchanged by design | Each execution is a fresh program run; held continuations end with it (D42) |

#### C27. Effect 3 features that Effect 4 removed

| Feature | In Effeck | Form |
| --- | --- | --- |
| `Micro` | Unchanged by design | Effect 4 has none; Effect 3 code is foreign code (Chapter 10) |
| `Supervisor` | Unchanged by design | `FiberSet` and `fork` |
| STM | Unchanged by design | `Effect.tx` with `TxRef` |
| `FiberRef`, `Effect.locally` | Changed | `Effect.provideService(() => program(), References.CurrentLogLevel, "Debug")` |
| `Either` | Unchanged by design | `Result` |
| `Mailbox` | Unchanged by design | `Queue` |
| `Runtime<R>` | Changed | `ManagedRuntime`, or `run*With` under D38 |
| `Effect.Service`, `Effect.Tag` accessors | Changed | Abstract service classes, called as `Users.find(id)` |
| `withConcurrency`, `withClock`, `withRandom` | Unchanged by design | Per-call options and `References` |
