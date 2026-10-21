---
name: vibelang
description: Write, edit or review VibeLang (.vibe files), Effect programs in TypeScript syntax. Use for any .vibe file, diff or question about VibeLang.
---

# VibeLang

A `.vibe` file is TypeScript in which every function is an Effect. Read and write it as Effect code; that reading is never wrong (D282). `vibelang` (a TypeScript 7 compiler) checks `.vibe` and `.ts` files together and emits JavaScript and declarations. `D123` cites a decision in `docs/design/DECISIONS.md`; `EK1234` is a VibeLang diagnostic. Pages named below are routes on https://vibelang.sh.

## The model

- A call runs where it stands and gives its success value: `const user = Users.find(id)` is a `User`. There is no `Effect.gen`, `yield*`, `Effect.map` chain or `async`/`await`.
- Where a parameter expects an Effect, a call is passed by name (D147): `retry(fetchUser(id), policy)` retries the call; `Effect.all([a(), b()])` gets two programs. A program of several statements is `() => { ... }` or a block param.
- Contracts are inferred. `A!E!R` writes `Effect<A, E, R>` (D127): `User!NotFound!Users`. An empty slot is `never`: `User!!` cannot fail and needs nothing. `A!E` on a function with a body infers R. `Effect<A, infer E, R>` also works.
- `throw` fails with an expected error, everywhere outside a hatch or a `module` body (D282). `catch` receives expected failures only, in plain functions too; defects, JavaScript exceptions included, and interruption pass through. `finally` always runs.
- A function with no Effect operation, no expected failure and no wider written contract compiles to plain JavaScript; callers can't tell.
- Every export of `effect`'s root (`Console`, `Option`, `Schema`, `Layer`, `Stream`, `Schedule`, `Clock`, ...) is in scope without an import (D146), except where a JavaScript or shared web global of the same name wins (`Array`, `String`, `Request`, D264): import that module under another name. Bare Effect functions (D151, D179): `sleep log logDebug logInfo logWarning logError retry repeat timeout race all forEach die withSpan exit interrupt yieldNow`. Import subpaths (`effect/http`, `effect/testing`), platform packages and sibling modules, with the extension: `import { User } from "./user.vibe"`.
- Values of Effect's data types take their module's functions as methods (D152): `stream.filter(p).take(3)`, `maybe.map(f).getOrElse(() => 0)`, `fiber.join()`, `ready.await()`, and data-first functions whose receiver isn't named `self`, such as `cause.findErrorOption()` (D295). A method exists only in a call (EK4501).
- Data stays data until called (D24): a call that returns a `Config`, `Exit`, service key or error instance returns it; `Port()` reads the config `Port`.

## Forms

| Effect TypeScript | VibeLang |
| --- | --- |
| `class E extends Data.TaggedError("E")<{ readonly id: string }> {}` | `error E { id: string }`, `error Timeout` (D126) |
| an error class with a constructor, methods or a subclass | `error E { id: string; constructor(raw: string) { super({ id: raw.trim() }) } }`, `error Sub extends Base { path: string }` (D278) |
| `Schema.TaggedError` class | `schema error E { id: string }` (D279) |
| `Context.Service<Users, {...}>()("app/Users")` | `service Users { find(id: string): User!NotFound; region: string }` (D128, D223) |
| a service of another shape | `service Prefix: string`, `service Search: SearchApi { static members }` (D281); the type `Search` is the key, so type an implementation `SearchApi`; a shape cannot refer to its service key (EK2103) |
| `Schema.Struct({...})` and its type | `schema User { id: string; nick?: string; role: "admin" \| "member"; home: Address }` (D133) |
| `Schema.Literals([...])` and its type | `schema Code = "a" \| "b"` (D280) |
| `readonly` members | the default for interface and object-type members; `mutable count: number` for one you assign (D134) |
| `Layer.effect(Users, Effect.gen(...))` | `Layer.effect(Users) { setup; return { find(id) { ... } } }`; no setup: `Layer.succeed(Users, { ... })` (D221) |
| `yield* Users` then a member | `Users.find(id)`; `Users()` is the implementation |
| `Effect.provide(p, AppLive)` | `provide AppLive { ... }`; one service `provide Random = seeded { ... }`; both `provide A, Key = v { }` (D172, D173) |
| `Effect.fail(e)` | `throw new E({ id })`; in an expression `x ?? throw new E({ id })` (D186) |
| `Effect.catchTag(s)` | `try { ... } catch (e) { return match (e) { when E: ...; default: throw e; } }` (D216) |
| `Effect.catch(p, f)`, `Effect.orElseSucceed(p, () => v)` | `p catch (e) => f(e)`, `p catch v` (D236) |
| `Effect.catchCause`, `Effect.failCause(c)` | `match (exit(p)) { when Exit.Success(let v): v; when Exit.Failure(let cause): ...; }` (D217), `throw cause` (D177) |
| `Effect.ensuring`, `Effect.onExit` | `try { } finally { }`, `try { } finally (exit) { }` (D178) |
| `Effect.scoped` + `acquireRelease` | `using r = Effect.acquireRelease(() => open(), (r) => close(r))`, released at block exit |
| `Effect.fn("name")(function* ...)` | `function f() { return withSpan("name") { ... } }` |
| `Effect.withSpan(p, "n")`, `it.effect("n", () => ...)` | block params: `withSpan("n") { ... }`, `it.effect("n") { ... }` (D188) |
| `p.pipe(Effect.retry(s), Effect.timeout(t))` | `p \|> retry(%, s) \|> timeout(%, t)` (D154) |
| `Match.value(x)...Match.exhaustive` | `match (x) { when "a": 1; when E and { let id }: id; default: 0; }` (D206) |
| `Option.match`, `Exit.isFailure` | `when Option.Some(let v):`, `when Option.None:`, `if (o is Exit.Failure(let cause))` (D208, D211) |
| `Effect.forkChild` + `Fiber.join` | `const f = fork work()`, `join f`, `join [a, b]` (D166, D225) |
| `Effect.scoped` + `forkScoped(...)`, `forkDetach`, `forkIn` | `fork { ... }`, owned by the enclosing block; `detach(p)`; `forkIn(scope, p)` (D170). `Effect.forkScoped(p)` alone still ties the fiber to the caller's scope (`Scope` in R) |
| `Effect.all(xs, { concurrency: "unbounded" })` | `await.all [a(), b()]`, `await.all { x: a(), y: b() }` (D196) |
| `Effect.all(xs, { ..., mode: "result" })` | `await.allSettled [a(), b()]`: typed failures become `Result`s (D258) |
| `Effect.raceFirst`, `Effect.raceAll` | `await.race [a(), b()]` (first to finish), `await.any [...]` (first success) |
| `Effect.forEach(xs, f, { concurrency: 8, discard: true })` | `for concurrent(8) (const x of xs) f(x)` (D169) |
| `Effect.uninterruptibleMask`, `interruptible` | `uninterruptible { ... restore { ... } }`, `interruptible { ... }` (D180) |
| a Stream from a generator | a producer: `function*` with `yield`; `for (const x of p())` pulls it |
| `Effect.sync`, `Effect.promise` | `sync(() => f())`, `sync(crypto.randomUUID())`, `sync(async () => load())` |
| `Effect.try`, `Effect.tryPromise` | `try((t) => JSON.parse(t), (e) => new Bad({ e }), text)`, `try(async (url, signal) => fetch(url, { signal }), url)` |
| `Effect.callback` | `callback((resume) => { const t = setTimeout(() => resume(7), 10); return () => clearTimeout(t) })` |
| `NodeRuntime.runMain(main)` | `export function main() { ... }`: runs when Node starts the module, not on import (D135) |
| none (VibeLang's own) | `control Approval { approve(r: string): boolean!! }`, then `try { ... } handle Approval.approve(r) { resume(true) }` (D132) |
| none (a TypeScript escape) | `module text { export function clean(s: string): string { ... } }`, `import { clean } from text` (D283) |


`module name { ... }` is an inline TypeScript module: outside hatches, its body is the only place in a `.vibe` file where JavaScript’s `throw` and `async` work; import its exports with `import { x } from name` (D283, D303).

## Rules

1. `await x` is an error (EK4601 to EK4603; a fiber is `join x`, EK4609; a Deferred `d.await()`, EK4610). `async` is EK1003. Inside a hatch or a `module` body both mean JavaScript, as in an `async function*` annotated `AsyncGenerator` (D271).
2. Platform APIs (`console.log`, `fetch`, timers, `Date.now()`, `process`) don't run in VibeLang code (EK4402, clock EK4403). Use `Console`, `Clock`, `sleep`, `FileSystem`, `Path`, `HttpClient`, `Config`, `Random`. Importing or passing one as a value is fine.
3. In code a fiber runs, a plain function from a `.ts` file or a package runs only through a hatch (EK4401) unless trusted (module initialization and its plain callbacks are exempt): Effect's own packages, `"vibelang": { "trustedPackages": ["@scope/*"] }` at the top level of `tsconfig.json` (D261, D275), or `trusted(impl, [Errors])` (import it with `import { trusted } from "vibelang-runtime"`), in a `.ts` or `.vibe` file, for its expected failures (D249).
4. A hatch's first argument (`sync`, `try`, `callback`) is self-contained TypeScript. It may use its own parameters, globals, prelude names, imports, types and the file's `error` declarations; any other value of the `.vibe` file arrives as an input after it, filling its first parameters (EK4405, D229, D230). `try`'s mapper stays second (D231). `runPromise` and `runFork` lead back in from a hatch or a `.ts` file. No catch expression in a hatch (EK2114).
5. Outside a hatch or a `module` body, `throw` is always a typed failure (D282). A function with a plain return annotation never fails and needs no service: a throw that can escape it, in a statement, a throw expression, a `match` arm or a `do` block, is EK2003, as is running an Effect; the quick fix writes the contract (`number!BadInput!`), or move the throw into a hatch. It may throw what its own `catch` handles. A plain callback that can fail is EK4301. Its calls of TypeScript code are JavaScript calls: their exceptions pass through its `catch` (EK2004 warns) and become defects at an effectful caller; a trusted call's listed failure is typed only inside the function's own `try`.
6. A module's top level declares: running a program there is EK1001, except through a runner such as `NodeRuntime.runMain(...)`. A `const` declared with an Effect type holds a cold program (D262, D269).
7. `catch (e: E)` and `catch (cause: Cause)` are retired (EK2112, EK2113): catch, then `match` or `is`. A `match` without `default` over a finite union must cover it (EK4701), and then runs its last arm without testing it (D294), so write no `default` there. `is` binds only in an `if` or `while` condition (EK4703). No regex or relational patterns (EK4704): guard with `when let n and if (n < 10):`.
8. An `error` is a class (D278): `abstract`, `extends` another error, `implements`, a constructor that calls `super`, methods, accessors and static or private members mean what they mean in TypeScript. A field is a property with a type and no initializer; the constructor takes it. An error's tag is its bare class name, a subclass's its own: `catchTag("NotFound")` matches it, and `when NotFound` and `is NotFound` match by class. A family's base writes `_tag: string` so its subclasses' tags fit (EK2117; an `abstract error` needs none), and catching the base's own tag leaves the base in E (D304). A subclass with fields and no constructor takes its base's fields and its own as one object; when the base's constructor takes other arguments, write one (EK2118).
9. Don't write `readonly` on interface or object-type members (D134); arrays still need `ReadonlyArray`. Class members keep TypeScript's rule, and an `error`'s fields are always readonly.
10. `throw` in an expression takes a unary operand: write `throw (c ? a : b)` (EK1010). `x catch y` binds looser than `??` and `||`, tighter than `? :`, and chains left to right (D237). Both sides take the expression's contextual type, so `{ _tag: "Found", ... } catch (e) => ({ _tag: "Error", e })` checks against a declared union (D293).
11. Retired spellings warn: `with X { }` (EK1009), `service class` (EK2120), `layer X: S { }` (EK2121). `select` is the error EK4607: use `await.race`.
12. Effect code in TypeScript's style warns where VibeLang has a form (D284 to D290): an Effect error class (EK4830; a `Schema.TaggedError` class EK4837, without a fix), a `Context.Service` class (EK4831), a `Schema.Struct` constant (EK4832), `Effect.Effect<A, E, R>` (EK4833), a running `Effect.succeed` or `Effect.fail` (EK4834), `Effect.catchTag`, `catchTags` or `catch` (EK4835), and `Effect.withSpan`, `provide` or `provideService` (EK4836). Write the VibeLang form; `vibelang -p tsconfig.json --fix` applies every safe fix (D291).
13. Tests in `.vibe` use `it.effect` (test clock: `sleep` stalls until `TestClock.adjust`) or `it.live` (real clock) from `@effect/vitest`. Promise-style tests stay in `.ts`. A runner in code a fiber runs is EK1005. Vitest hides `console.log` under an agent: assert instead.
14. An unannotated `function*` is a producer (a Stream). Annotate `Generator<...>` for a JavaScript generator (D271); it never fails either (D296): a throw that can escape it is EK2003, and its `catch` lets an exception passed in with `generator.throw()` through. A coroutine that speaks a protocol by throwing belongs in a `module` block or a `.ts` file.
15. `do { ...; value }` is a block with a value, for a `match` arm or an `await.all` element; `return` inside it is TS1108 (D187). A block param instead needs `return` (D188).
16. A synchronous callback, such as `Array.map`'s, can't run an Effect or fail (EK4301): use `forEach` or a loop.
17. A resource acquired without `using` belongs to the caller (`Scope` in R); don't `using` a resource a function or service hands out, or it closes first.
18. A handler covers only its own fiber: a forked branch installs its own (EK4201).
19. A factory declared with an Effect return type, `(input: In): A!E!R => { ...; return () => ... }`, runs plain code at the call, then returns a program on every path (EK2116); its outer body runs no Effect and never throws (EK2115, D269, D282): fail in the program, `return () => { throw e }`.

## Idioms

- Errors: an `error` per failure, `throw` to fail, `x catch fallback` for a default, `match` to choose: `error-management/014-expected-errors`, `016-fallback`, `017-matching`.
- Services: `service` declarations; `Layer.effect(S) { }`; `provide` once around `main`: `requirements-management/025-services`, `027-layers`.
- Retries and timeouts: bare `retry` and `timeout` or a pipe; per-attempt timeout inside the retry: `error-management/018-retrying`, `019-timing-out`.
- Concurrency: the `await` operations, `fork`/`join`, `for concurrent`: `concurrency/007-basic-concurrency`, `047-fibers`.
- Resources: `using` in the block that owns them: `resource-management/029-introduction`.
- Data: `schema` declarations; `Schema.Struct` for checks, transformations and exact optional keys: `schema/087-basic-usage`.
- Matching: `vibelang-features/116-match-and-is`. Handlers: `111-control-operations-and-handlers`. Producers: `112-producers`.
- TypeScript at the edges: hatches, trust, factories (D269): `vibelang-features/114-vibelang-for-effect-users`.
- Every form beside its Effect code: `vibelang-features/115-cheat-sheet`. The rules: `spec/`.

```vibe
error NotFound { id: string }

service Users {
  name(id: string): string!NotFound
}

const UsersLive = Layer.succeed(Users, {
  name(id) {
    if (id !== "42") throw new NotFound({ id })
    return "Ada"
  }
})

function nameOf(id: string): string!!Users {
  return Users.name(id)
    |> timeout(%, "1 second")
    |> retry(%, { times: 3, schedule: Schedule.exponential("100 millis") })
    catch (e) => match (e) { when NotFound: `no user ${e.id}`; default: "timed out"; }
}

export function main() {
  provide UsersLive {
    const [ada, missing] = await.all [nameOf("42"), nameOf("7")]
    Console.log(ada, missing)
  }
}
```
