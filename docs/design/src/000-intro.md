# VibeLang

A language for writing Effect programs. Specification, 2026-10-16. Runtime: `effect@4.0.5`.

VibeLang is TypeScript syntax in which every function is an Effect. You call functions normally, `throw` and `catch` typed errors, and ask for services by name. The compiler infers each function's `Effect<A, E, R>` and emits the Effect code you would write by hand, so TypeScript uses it like any other Effect library.

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
// Effect<string, UserNotFound | MailError, Users | Mailer>
```

You don't write the comment under the VibeLang version: it is the inferred contract, which your editor shows when you hover over `welcome`.

The two functions do the same work, with two differences. `Effect.fn("welcome")` opens a tracing span on every call; VibeLang adds one only where you ask (Chapter 2). And the generated code looks each service up at the call that uses it, so a missing `Mailer` fails after `Users.find` runs, not before.

## How to read this document

| Part | What it covers | Read it if |
| --- | --- | --- |
| [I. The language by example](#part-i-the-language-by-example) | Functions, errors, dependency injection, pipelines, resources, concurrency, producers, handlers, TypeScript interop, the standard library, and the questions Effect users ask, each beside today's Effect code | You use Effect. Start here. |
| [II. The rules](#part-ii-the-rules) | The precise rules behind Part I: what a call means, how contracts are inferred and checked, which functions compile to plain JavaScript | You review the language design |
| [III. The runtime](#part-iii-the-runtime) | How VibeLang runs on Effect's fiber runtime: one-shot delimited continuations, ownership, cancellation, and the tests that check them | You care how it runs, or review the runtime |
| [IV. The compiler](#part-iv-the-compiler) | The TypeScript compiler fork, the pipeline, declarations, and editor support | You build it |
| [V. Decisions](#part-v-decisions) | Every decision with its reason and evidence, the closed end-to-end build calls, and the experiments that confirm the implementation | You check or change a decision |
| [Appendices](#appendices) | The evidence index, prior art, and every Effect feature mapped to VibeLang | You check a claim |

To learn VibeLang by task, not by rule, start with the [user guide](../../user/src/001-onboarding.md). The [cheat sheet](../../user/src/vibelang-features/115-cheat-sheet.md) puts every change beside the Effect code it replaces.

**Status.** Every rule in this specification is decided. `docs/design/DECISIONS.md` logs the owner's decisions and the defaults that complete the design (D1-D60); [Part V](#29-decisions) collects them and records the closed end-to-end build calls. D57 and D58 are controversial defaults. The Part III runtime is a working prototype: {{testCount}} tests pass in `experiments/`. The compiler has two prototypes: `prototype/esc`, on TypeScript's API, compiles 46 example files written from this document, and the Go fork of TypeScript in `typescript/` lowers calls by type and infers contracts (Chapter 27). The compiler, spec, runtime, and tooling share one private repository (D60).

## Effect to VibeLang at a glance

| In Effect today | In VibeLang | Chapter |
| --- | --- | --- |
| `Effect.gen(function* () { … })`, `Effect.fn(…)` | an ordinary `function` | [2](#2-functions-and-calls) |
| `yield* effect` | call it: `f(x)` | [2](#2-functions-and-calls) |
| `Effect.map`, `Effect.flatMap`, `Effect.andThen`, `Effect.tap` | statements: `const x = f(); g(x)` | [2](#2-functions-and-calls) |
| `Effect.fail(e)` or `return yield* new E()` | `throw new E()` | [3](#3-errors-the-e-channel) |
| `class E extends Data.TaggedError("E")<{ ... }> {}` | `error E { ... }` | [3](#3-errors-the-e-channel) |
| `Effect.catchTag("E", h)` | `try { … } catch (e) { … }` | [3](#3-errors-the-e-channel) |
| `Effect.ensuring(cleanup)` | `finally { … }` | [3](#3-errors-the-e-channel) |
| `class S extends Context.Service<…>()("key") {}` | `service S { method(): Effect<A, E> }` | [4](#4-dependency-injection-the-r-channel) |
| `const s = yield* S` then `yield* s.method()` | `S.method()` | [4](#4-dependency-injection-the-r-channel) |
| `Layer.effect(S, Effect.gen(…))` | `Layer.effect(S) { … }` | [4](#4-dependency-injection-the-r-channel) |
| `program.pipe(Effect.provide(layer))` | `provide layer { program() }` | [4](#4-dependency-injection-the-r-channel) |
| `effect.pipe(Effect.retry(…), Effect.timeout(…))` | `f() \|> retry(%, …) \|> timeout(%, …)` | [5](#5-programs-as-values-retry-timeout-and-pipe) |
| `Effect.scoped` with `Effect.acquireRelease` | `using x = acquire()` | [6](#6-resources) |
| `Effect.all([a, b], { concurrency: 2 })` | `Effect.all([() => a(), () => b()], { concurrency: 2 })` | [7](#7-concurrency) |
| `Effect.forkScoped(effect)` | `fork(() => f())`, owned by the enclosing block | [7](#7-concurrency) |
| a `Stream` you build by hand | a `function*` producer and `for...of` | [8](#8-producers-streams-of-values) |
| none | `control Op { ... }` and `handle` | [9](#9-handlers-defining-your-own-effects) |
| `node:fs`, `fetch`, `process.env`, `child_process` | `FileSystem`, `HttpClient`, `Config`, `ChildProcessSpawner` | [11](#11-the-standard-library) |

[Chapter 12](#12-questions-effect-users-ask) answers where `async`, `Option`, `Schema`, `Stream`, testing, and the rest of Effect fit; [Appendix C](#c-every-effect-feature-in-vibelang) answers feature by feature.
