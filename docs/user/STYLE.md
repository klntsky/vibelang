# Writing the Effeck user docs

These docs mirror the Effect 4 documentation one page for one page: same sections, same order, same topics (see `inventory.md`). Every page is original writing about Effeck. Read the upstream page to learn what it covers; never copy its prose or code (see `ATTRIBUTION.md`).

## Sources of truth

1. `docs/design/DECISIONS.md`: the decision log. It wins every conflict.
2. `docs/design/effeck-design.md`: the spec, especially Part I (the language by example) and Part II (the rules). Chapter numbers are changing; refer to topics, not chapter numbers.
3. Effect 4.0.5's API, in `experiments/node_modules/effect/src`. Check every Effect name you use there.

## Voice

- Write for a developer learning Effeck. Many readers know Effect in TypeScript; serve them with a short "Coming from Effect" section at the end of a page, not by comparing on every line.
- Plain, concrete, active sentences. Code first, then the explanation. Short paragraphs.
- No em dashes or en dashes. No hedging ("might", "could potentially"). No marketing words.

## Page template

```md
# Page title

One paragraph: what this is and when you use it.

## A section per topic in the stub's topic list

Example first, then two to five sentences.

## Coming from Effect        (optional)

- `Effect.gen(function* () { … })` becomes an ordinary function.
```

Code blocks use the `ts` fence and are Effeck unless the first line says otherwise (`// TypeScript (.ts)`). Show a file name in a first-line comment when it matters (`// main.effeck`). After a function, show the inferred contract as a comment when it teaches something: `// User!UserNotFound!Users`.

## The language in one page

- **Every function is an Effect, and calls run.** `const user = Users.find(id)` gives a `User`. There is no `Effect.gen`, `yield*`, `async`, or `await` in `.effeck` files.
- **An Effect value runs when called.** A function that returns an Effect runs when called (`Effect.log("hi")`, `Effect.sleep("1 second")`). An Effect value you hold runs when you call it: `const now = Clock.currentTimeMillis()`.
- **Programs are functions with no arguments.** Pass `() => f(x)` wherever Effect expects an Effect: `Effect.retry(() => fetchUser(id), Schedule.exponential("100 millis"))`, `Effect.timeout(() => slowQuery(), "1 second")`, `Effect.all([() => a(), () => b()], { concurrency: 2 })`, `Effect.forEach(ids, (id) => load(id), { concurrency: 4 })`, `Effect.race(() => fromCache(id), () => fromNetwork(id))`.
- **Pipelines** take the program first: `pipe(() => fetchUser(id), Effect.retry({ times: 3 }), Effect.timeout("2 seconds"))`. Statements replace `map`, `flatMap`, `andThen`, `tap`, `catch`, and `ensuring`.
- **Contracts are inferred.** Write `function f(): A!E!R` to check a promise; an empty slot is `never`, and dependencies left out of a return annotation with a body, as in `A!E`, are inferred (D127). `Effect<A, infer E, infer R>` still works. D12's callback rule stands.
- **Errors.** `error UserNotFound { id: string }` declares a tagged error class with readonly fields (short for `class UserNotFound extends TaggedError("UserNotFound")<{ readonly id: string }> {}`); `throw new UserNotFound({ id })` adds it to E; `try`/`catch` handles expected failures only; `catch (e: NotFound)` handles one type, `catch (e: A | B)` several, and a final untyped clause the rest; `finally` always runs. Serializable errors use `Schema.TaggedError`. `Effect.orDie(() => f())` turns failures into defects; `Effect.catchDefect` observes defects at a boundary.
- **Readonly by default.** Members of interfaces and object types are readonly; write `mutable count: number` for a member the code assigns. Do not write `readonly` on members; arrays still need `ReadonlyArray`.
- **Data.** `schema User { id: string; role: "admin" | "member"; tags: ReadonlyArray<string>; home: Address }` declares the struct schema `User` and its decoded type in one declaration (short for `const User = Schema.Struct({ ... })` with `interface User extends Schema.Schema.Type<typeof User> {}`). A field may name another schema or a schema constant, such as `const Created = Schema.DateTimeUtc`. Use `Schema.Struct` for checks, transformations, and exact optional keys.
- **Services are `service` declarations**, abstract classes whose members state their contracts. Provide a layer to a block with `provide AppLive { ... }`, the keyword that matches `Effect.provide`.

  ```ts
  service Users {
    find(id: string): User!UserNotFound
    static readonly layer = Layer.effect(Users, () => {
      const db = Database()                       // captured once, when the layer is built
      return { find: (id) => db.findUser(id) }
    })
  }
  ```

  Call `Users.find(id)`; `Users()` returns the implementation. Provide with `Effect.provide(() => main(), layer)`; compose with `wire(UsersLive, DatabaseLive)` or Effect's `Layer` functions. Test with `Layer.succeed(Users, { find: (id) => fakeUser })`.
- **Resources.** `using file = Files.open(path)` releases at block exit. `Effect.acquireRelease(() => acquire(), (x) => release(x))` defines one. Only `using` acquisitions belong to the block; anything acquired without `using` belongs to the caller (`Scope` stays in R).
- **Concurrency.** Effect's combinators with programs, as above. `fork(() => work())` starts a fiber owned by the enclosing block, interrupted when the block exits; `Fiber.join(fiber)` waits. `Effect.forkScoped` and `Effect.forkDetach` give longer lifetimes.
- **Producers.** `function*` with `yield` produces values one at a time and may call services between them; `for (const x of producer())` consumes it, and `break` runs its cleanup. Consume an Effect `Stream` with Stream's functions: `Stream.runForEach(stream, (x) => handle(x))`.
- **Control operations** (advanced): `control Approval { approve(request: string): boolean!ReviewBusy }`, handled with `try { return checkout(id) } handle Approval.approve(request) { resume(true) }`. A `return` in the try block or a clause returns from the function. `handle(() => checkout(id), Approval, { approve(request, k) { return resume(k, true) } })` is the expression form.
- **Globals** (no import): `Effect` (also the `Effect<A, E, R>` type), `Schema`, `Layer`, `TaggedError`, `Control`, `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, `abandon`. Import everything else from `effect`: `import { Option, Stream, Schedule, Duration, Fiber, Queue, Ref } from "effect"`.
- **Code that is not Effect.** JavaScript built-ins (`String`, `Array`, `Math`, `JSON`, `Map`, `Set`, `Number`, `Object`, `RegExp`) are called directly. Running Node.js and browser APIs is EK4402 (D108); imports and passing bindings as values are allowed. Use Effect's modules: `FileSystem`, `Path`, `HttpClient`, `effect/process`, `Config`, `Console`, `Clock`, `DateTime`, `Random`, `KeyValueStore`. Any other plain function is called through `Effect.try` or `Effect.tryPromise`, which say what an exception or rejection means.
- **Running.** Export `function main()` from the entry module: `node dist/main.js` runs it with `NodeRuntime.runMain`, and importing the module runs nothing. Call `NodeRuntime.runMain(() => Effect.provide(() => main(), AppLive))` at the top level yourself to provide layers or options. Tests: `it.effect("name", () => { … })` from `@effect/vitest`.

## Pages about APIs that do not change

Many modules work exactly as in Effect: `Option`, `Chunk`, `HashMap`, `Duration`, `DateTime`, `BigDecimal`, `Equal`, `Hash`, `Order`, `Schedule`, most of `Schema`, and more. Say so in the first paragraph ("`Duration` works in Effeck exactly as in Effect.") and show Effeck usage, where building a value is a plain call and only Effect-returning functions run.

## When the spec has no answer

Write the answer most consistent with the rules above, and mark it with a block quote: `> **Proposed.** …`. If no consistent answer exists, write `> **Not designed yet.** <the precise question>`. List every such case, with the page path, in your batch's notes file under `docs/user/notes/`.

## Updates after the spec rewrite (decisions D22-D54)

The spec is now `docs/design/effeck-design.md` with 30 chapters; chapter 11 is the standard library, chapter 12 the FAQ, chapter 28 tooling, and appendix C maps every Effect feature. Where a page disagrees with these, the page changes.

- **Prelude** adds `Scope` (D48): the module, and the type `Scope` means `Scope.Scope`. Write `File!IOError!Scope`.
- **Data until called** (D24): a call runs its result only when its type is `Effect` itself. `Config`, `Exit`, Context keys, and error instances are data: `const Port = Config.withDefault(Config.Port("PORT"), 3000)` composes, and `const port = Port()` reads it.
- **Effect's own packages are trusted** (D22): `Option.map`, `Schema.is`, `Duration.millis` are plain calls; their exceptions are defects. A plain TypeScript function is trusted with `/** @effeck trusted */` and `@throws {T}` (D23).
- **`catch`** sees expected failures only (D13, D37). A defect from `Option.getOrThrow` or `Schema.decodeUnknownSync` is not caught; use the typed forms (`Option.getOrElse`, `Effect.fromOption`, `Schema.decodeUnknownEffect`). A `catch` that cannot run is warning EK2004. With several expected failures, `catch` receives the first (D28).
- **Runners** (D38) run only where no fiber runs the code: a module's top level, or a plain callback such as a `describe` body. Tests in `.effeck` use `it.effect`; Promise-style tests stay in `.ts`.
- **Services** (D40, D41): `static readonly layer` holds the default Layer; `static make()` builds the implementation; `static readonly default` makes the service a `Context.Reference` that needs no provider. `S.member(args)` adds `S` to R whatever the member returns; value members are read as `Settings().baseUrl`.
- **Collections of programs convert** (D49): `Effect.all(stores.map(makeSync), { concurrency: 4 })` works.
- **Producers and streams** (D26): a producer is a `Stream` wherever a `Stream` is expected, and `for...of` over a `Stream` pulls from it and closes it when the loop exits.
- **Tooling** (D30-D36): install `effeck`, which ships the `effeck` binary (it accepts every `tsc` flag); Vite apps use `effeck-vite`; the language server is `effeck --lsp --stdio`; diagnostics are configured in `compilerOptions.effeck` in `tsconfig.json`; Oxlint lints, the compiler type-checks; source maps go from `.effeck` to JavaScript. Read chapter 28 for exact names before writing about tooling.
- **Diagnostic codes** follow D53 and chapter 13 onward of the spec; cite only codes the spec defines.
