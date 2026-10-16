# EffectScript decision log

Working log, 2026-10-16. The spec (`src/`), the prototypes, and the end-user docs follow this file. Each entry says who decided: **Will** for the owner's decisions, **default** for calls made to produce a confident spec, which Will can overturn. Evidence paths are relative to the repository root.

## Declarations: TypeScript syntax only

**D1. Services are abstract classes** (default, from Will's idea). The `service` keyword is removed.

```ts
abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
  abstract rename(id: string, name: string): Effect<void, UserNotFound, never>

  static readonly layer = Layer.effect(Users, () => {
    const db = Database()
    return { find(id) { … }, rename(id, name) { … } }
  })
}
```

- A class declared in an `.es` file is a service when it is `abstract`, has no `extends` clause, has no type parameters, and every instance member is abstract (methods with full contracts, `abstract readonly` values). Static members are allowed and stay on the class, matching Effect 4's `static readonly layer` idiom.
- The compiler makes service classes nominal: plain abstract classes with the same members collapse structurally, so providing one service type-checks as providing another, and the program dies at run time with `Service not found` (`experiments/scratch-abstract-services/probe.ts`).
- The key is derived from package, module path, and class name, and duplicates are rejected program-wide (unchanged from the earlier `service` rule).
- Emit: an Effect 4 `Context.Service` class, so TypeScript code sees an ordinary service. (Superseded by D45: the class form breaks `implements`; the emission is an interface plus a key class.)
- Implement a service with an object literal or a class that `implements Users`. `extends Users`, `new Users()`, `instanceof Users`, and a generic service class are errors with fixes.
- Evidence that this reuses no valid TypeScript meaning: `Users.find(id)` on an abstract member is TS2339 and `Users()` is TS2348 today. Evidence that it rarely collides with ordinary inheritance: Smithers has no abstract classes in 4,461 files; opencode has 5 in 3,235, and its only fully abstract one extends another class.

**D2. Control operations are abstract classes that extend the global `Control`** (default). The `control` keyword is removed.

```ts
abstract class Approval extends Control {
  abstract approve(request: string): Effect<boolean, ReviewBusy, never>
}
```

**D3. Errors use the global `TaggedError`** (Will). `class UserNotFound extends TaggedError("UserNotFound")<{ readonly id: string }> {}` is exactly Effect's `Data.TaggedError`. Errors that must be serialized use `Schema.TaggedError`, which real code already prefers (Smithers: 312 `Schema.TaggedError` to 133 `Data.TaggedError`). Deriving the tag from the class name is a later proposal, not v0.

**D4. The prelude** (default, from Will's "some things should be global"). In scope in every `.es` file without an import: `Effect` (namespace and the `Effect<A, E, R>` type), `Schema`, `Layer`, `TaggedError`, `Control`, and the built-ins `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, `abandon`. Everything else is imported from `effect` as usual. Measured: `Effect` is imported by 77-81% of Effect files, `Schema` by 39-44%, `Layer` by 34-44%, and nothing else by more than 22% (Smithers 2,149 files, opencode 1,031). `Context` is not needed once services are classes; `Option` and `Array` would shadow JavaScript names.

## Code that is not Effect

**D5. Calls to non-Effect code need a declared trust level** (Will: "we need to be careful. We don't know if it can throw"; replaces the O11 recommendation).

| Callee | Rule |
| --- | --- |
| A function returning `Effect` (EffectScript or TypeScript) | Calls run it (unchanged) |
| JavaScript language built-ins: `String`, `Array`, `Math`, `JSON`, `Number`, `Object`, `Map`, `Set`, `RegExp`, deterministic `Date` parsing | Trusted contracts shipped with the compiler give each one a control class (`pure` or `sync`) and say which exceptions are expected failures (for example `JSON.parse` fails with `SyntaxError`) and which are defects |
| Platform APIs: `node:*` modules and browser globals such as `fetch`, `setTimeout`, `process`, `localStorage` | Not callable from `.es` files (ES4402); the error names the Effect module to use (D6) |
| Clock and randomness reads: `Date.now()`, `new Date()`, `Math.random()`, `crypto.randomUUID()` | Use `Clock`, `DateTime`, and `Random` so tests control them (ES4403) |
| Any other plain TypeScript or JavaScript function | Needs an explicit boundary: `Effect.try` or `Effect.tryPromise`, or a trusted contract declared once for that function. A bare call is an error (ES4401) whose fix offers both |

Pure helpers written in `.es` get inferred contracts and compile to plain JavaScript (D10), so most project code never needs a boundary.

**D6. The standard library is Effect's** (Will: "utilize that not necessarily reinvent the wheel with one to one node.js apis"). EffectScript adds no Node.js clones. Mapping: files → `FileSystem`; paths → `Path`; HTTP → `HttpClient` and `HttpApi`; child processes → `effect/process`; timers → `Effect.sleep` and `Schedule`; environment → `Config`; console → `Console` and `Effect.log`; time → `Clock` and `DateTime`; randomness → `Random`; crypto → `Crypto`; sockets → `effect/socket`; workers → `effect/workers`; terminal → `effect/cli` and `Terminal`; storage → `KeyValueStore`; SQL → `effect/sql`. Runtimes come from `@effect/platform-node`, `-bun`, and `-browser`.

## Calls, programs, and pipelines

**D7. Program values are functions with no arguments** (default; O1 option A).

**D8. No `|>` in v0; keep the `pipe(program, …stages)` built-in** (amended by D110) (default). TC39's pipeline operator collides with "calls run": in `f(x) |> Effect.retry(%, p)` the call runs before the pipe sees it, and every stage that calls a combinator would run too. The pipe census shows the payoff is small: outside tests, 1.1% (Smithers) and 2.6% (opencode) of Effect pipes keep two or more stages once statements replace `map`, `flatMap`, `catch`, and `ensuring` (`docs/research/pipe-census.md`). Revisit if the TC39 proposal reaches Stage 3.

**D9. A function value passed where a plain JavaScript function is expected** (amended by D66, D112, D113) passes the plain-JavaScript variant if the function is eligible (D10); otherwise it is ES4301 (Fable review, finding 3).

**D10. Plain JavaScript compilation** (amended by D65, D67) (default, from Will's first feedback). A function compiles to plain JavaScript when it performs no Effect operation at all (control class `pure` or `sync`, which now excludes `Effect.sync` bodies and every other Effect boundary) and its E is `never`. Generated code keeps source evaluation order. `pure` permits inlining, never removal or reordering. Pure recursion runs on the JavaScript stack, like any JavaScript function; past the host limit it is a `RangeError` defect (resolves O10). Functions that can pause use heap frames and recurse without limit.

**D55. A method that implements a plain-function member** (amended by D67, D74) (default, 2026-10-16; with D9 and D10). When a class in a `.es` file implements an interface member whose declared type is a plain function, such as `Equal.Equal`'s `[Equal.symbol](that): boolean` or `Hash.Hash`'s `[Hash.symbol](): number`, the method compiles to a plain JavaScript method if it is eligible under D10; otherwise it is ES4301. EffectScript callers call it as a plain method, because its declared type is not an Effect. A method that implements a service member, whose declared type returns an Effect, compiles to an Effect function as before (prototype finding 30).

**D11. Higher-order functions compile to frames in v0**; per-call-site specialization comes later (Fable, finding 5).

**D12. A function type without a contract accepts any contract** (amended by D68) (default; P14 kept). It means "inferred from each call's argument", never `never`. Codex's review argues this is a partial-annotation spelling; it is listed with O7 for Will.

## Errors and cleanup

**D13. `catch` handles a failure only when the cause contains no defect and no interruption**; otherwise the whole cause propagates (Codex R1: `Effect.catch` swallows a defect from a mixed cause).

**D14. Only `using` acquisitions belong to the block.** A resource acquired without `using` belongs to the caller, even when the same block also has a `using` (Codex R8).

**D15. A failure during cleanup is never dropped**, including during explicit `abandon` under interruption (Codex R2b; kernel fix).

## Handlers and concurrency

**D16. v0 exposes `handle`, `resume`, `resumeWith`, and `abandon`. `transfer` stays inside the standard library** (producers' external iteration, the Stream adapter) until a continuation can no longer outlive its capturing fiber (Codex R5; G3). Resolves O3.

**D17. `resumeWith` runs code whose R is `never`** (Codex R7: an injected requirement resolves in the capture-time Context).

**D18. A clause whose last step is `resume` compiles to the kernel's tail-resume shape**, so producers run in constant stack (both reviews: the generator-shaped clause leaks two frames per element).

**D19. `fork` is block-owned** (amended by D96): the child is interrupted when its block exits. `Effect.forkScoped` (adds `Scope` to R) and `Effect.forkDetach` give longer lifetimes (resolves O5).

**D20. A TypeScript function whose R contains a control operation cannot be called from EffectScript in v0** (ES4202), because it may fork internally where the handler cannot reach (Codex R6).

**D21. Each `Effect.provide` builds its layers once.** Provide the application layer once, at `main`; programs with several entry points use `ManagedRuntime` (Fable, finding 6).

## Calls the confident spec needed (2026-10-16)

These fill gaps the decisions above left open. All are defaults; the spec's chapters cite them.

**D22. Effect's own packages are trusted** (default). An export of `effect` or an `@effect/*` package that does not return an `Effect` is a `sync` call whose exceptions are defects, exactly as inside `Effect.gen` today; a callback it calls synchronously follows D9. Values and functions that those exports return are trusted on the same terms: a `Brand` constructor and its methods, the function `Match.exhaustive` returns, and the stage a dual API returns when called without its data argument (amended 2026-10-16 after the docs consistency pass). Without this, D5 would demand a boundary around every `Option.map`. Evidence: D6 makes Effect the standard library; `Option` and `Result` are plain data in Effect 4 (`experiments/node_modules/effect/src/Option.ts:57`, `Result.ts:68`).

**D23. A trusted contract is a JSDoc tag on the TypeScript declaration** (amended by D100, D101, D108) (default; the spelling D5 left open). `/** @effectscript trusted */` declares a synchronous function `sync`; each `@throws {T}` names an exception class that is an expected failure and joins E; any other exception is a defect. The tag cannot be put on a function that returns a Promise or an `Effect`. A function from a package you cannot edit is trusted by re-exporting it from a `.ts` file you own, with the tag. Inside the callback of an explicit boundary (`Effect.try`, `Effect.tryPromise`, `Effect.sync`, `Effect.promise`), calls to plain TypeScript are allowed; platform APIs (ES4402) and clock or randomness reads (ES4403) are not. Reasons: TypeScript syntax only, as in D1; it is the same metadata channel the compiler already writes into declarations; `@throws` is standard JSDoc.

**D24. A call runs its result only when the call's type is `Effect<A, E, R>` itself** (amended by D90, D109) (default). A type that extends `Effect` and carries structure of its own (`Config`, `Exit`, a service class, a `TaggedError` instance) is data when a call returns it; calling such a value runs it; `new` never runs anything. Without this, `Config.String("X")` would read the variable before `Config.withDefault` could see it, and a function returning an error instance would fail. Evidence: `Config<T> extends Effect<T, ConfigError>` (`Config.ts:113`), `Exit` (`Exit.ts:89`), `Context.Key` (`Context.ts:68`), `YieldableError` (`Cause.ts:1312`).

**D25. `using` adds no masking policy of its own** (default; KERNEL-CHANGES.md left it open). An acquisition is masked or interruptible as its `Effect.acquireRelease` says: masked by default, so cancellation waits for the acquisition and then releases it; `{ interruptible: true }` for a wait on the outside world that cancellation must reach. Evidence: K15a-K15c.

**D26. Producers** (amended by D82) (default; resolves O4 and the design half of G12).
- `function*` in an `.es` file declares a producer; `yield` performs the built-in `Yield` operation. Generator-style Effect code (`Effect.gen`, `Option.gen`) stays in `.ts` files (ES1002).
- `for...of` is chosen by the iterated type: over a producer it installs a `Yield` handler (internal iteration); over a `Stream` it pulls with `Stream.toPull` and closes the stream when the loop exits; over any other iterable it is JavaScript's loop.
- To TypeScript, and wherever a `Stream` is expected, a producer is a `Stream`. The adapter runs the producer on its own fiber and hands over each element only when the stream pulls it, through a zero-capacity handoff (a one-element buffer would let the producer run one element ahead, prototype finding 26), so a pull from any fiber works and the producer never runs ahead; closing the stream interrupts that fiber, which runs the producer's cleanup. Reason: Stream operators such as `merge` pull on other fibers, and a continuation is fiber-affine (P5). G12 is the confirming experiment.

**D27. `wire` is a compiler intrinsic with a bounded contract** (default; resolves O6). It reads each argument's `Layer<Out, E, In>` type. An argument whose type does not name its outputs is ES3204; compose it with `Layer.provide`. Inputs no argument provides stay in the result's `In`, as with `Layer.provide`, and are reported with their dependency path (ES3201) where the program's R must be empty. Duplicate outputs are ES3202, cycles ES3203.

**D28. A `catch` over several expected failures receives the first** (default; K14a). The `catch` handles the cause as a whole, as `Effect.catch` does; `Effect.catchCause` sees every failure.

**D29. Proposals P1-P22 are the spec's rules**, as revised in Chapter 29 of the spec where the 2026-10-16 reviews broke them (default).

## Tooling (2026-10-16)

All defaults; Chapter 28 of the spec states them with examples. Package names use the `@effectscript` scope and change with the name (O9).

**D30. Installation.** (amended by D80, D83) Two npm packages. `@effectscript/compiler` ships the fork's single native binary, `esc`, through per-platform optional dependencies, as `typescript@7` does. `esc` accepts every `tsc` flag and adds no new commands: `esc` type-checks and emits, `esc --noEmit` checks, `esc --watch` and `esc -b` rebuild, `esc --lsp --stdio` serves editors, and `esc --api` serves build tools. `@effectscript/runtime` holds the kernel and the runtime helpers; generated code imports only it and `effect`. v0 supports `effect` 4.0.5 exactly (the runtime checks the version at load); the compiler is the TypeScript 7.1 checker (P16), so `.ts` files in the same program check as TypeScript 7.1 checks them; the runtime runs where `effect@4.0.5` and its platform packages run, and the evidence was gathered on Node 26.10 and Bun 1.4.1.

**D31. Build integration.** (amended by D105, D106, D107) Libraries and servers build with `esc` or `esc --watch`. Applications built with Vite use `@effectscript/vite`, which compiles each requested `.es` module through `esc --api`, returns JavaScript with composed source maps, forwards diagnostics to Vite's overlay, and invalidates a module's importers for hot reload when the module's exported contracts or control classes change, because callers compile differently. Imports name source files: `.es` code imports `./welcome.es`, the emitted JavaScript imports `./welcome.js`, and TypeScript code imports `./welcome.js`, which the compiler resolves to `welcome.es` as it resolves `.js` to `.ts`.

**D32. The editor.** (amended by D76, D79) The language server is the fork's own: `esc --lsp --stdio`, from the project's `@effectscript/compiler`. It serves `.es` and `.ts` files of the same program, so one server owns both. The VS Code extension `effectscript.effectscript` registers the `effectscript` language for `*.es`, starts that server, and asks VS Code's built-in TypeScript features to stand down in workspaces that contain `.es` files. Other editors configure their LSP client to run `esc --lsp --stdio` for `*.es` and `*.ts`. GitHub highlights `.es` as JavaScript unless `.gitattributes` says `*.es linguist-language=TypeScript`.

**D33. v0 editor features.** (amended by D77, D78) Hover shows a function's inferred contract and control class and, at a call, the success value. Completions add service members, prelude names, and `catch` narrowing by error class. Quick fixes exist for every ES diagnostic that has a mechanical fix (Chapter 28 lists them). Refactors: convert an `Effect.gen` or `Effect.fn` function into an EffectScript function, convert a `Context.Service` class into an abstract service class, implement a service (scaffold an object literal or an `implements` class), and write the inferred contract as an annotation.

**D34. Diagnostics configuration.** (amended by D61, D104) A `compilerOptions.effectscript` block in `tsconfig.json`. Every ES diagnostic is an error and cannot be lowered, because each marks code with no meaning or code that would compile unsoundly, with two exceptions that are policy, not soundness (amended after the prototype): ES2004 (a `catch` that can never run) is a warning by default, and ES4403 (a clock or randomness read) is an error by default; a project may set either to `"error"`, `"warning"`, or `"off"`, and an ES4403 read then compiles as a trusted `sync` call. The block also holds `"tracing": "none" | "exported" | "all"` (default `"none"`), which compiles the chosen functions with `Effect.fn` spans. `esc` exits non-zero on any error; diagnostics print at `.es` spans.

**D35. One owner per diagnostic.** The compiler owns every type and language diagnostic for `.es` and `.ts` files in the program, in the terminal, the editor, and the Vite overlay. Oxlint owns lint: it parses `.es` as TypeScript (EffectScript adds no syntax) and runs only syntactic rules there; its type-aware mode (tsgolint) skips `.es` files, because tsgolint embeds TypeScript's checker and would analyze the wrong semantics. `@effectscript/oxlint-config` sets that override and turns off syntactic rules whose premise EffectScript changes, such as `require-yield`. Vite Plus runs the two through those same two pieces, the Vite plugin and the Oxlint config, so no rule has two owners and nothing needs deduplication.

**D36. Debugging.** (amended by D104) `esc` and the Vite plugin write source maps composed from `.es` through generated TypeScript to JavaScript, so any debugger that reads standard source maps (VS Code's JavaScript debugger for Node.js, Bun, and Chrome targets; Chrome and Edge DevTools; JetBrains IDEs) maps breakpoints and paused frames to `.es` lines. Compiled functions, clauses, and producers are generator functions named after their source; region closures are named `<function>$try`, `$catch`, `$finally`, and `$using`. A JavaScript debugger shows only the active JavaScript frames: a continuation held by a clause is a heap value, so a breakpoint in a clause shows the clause, and the body's frames reappear when it resumes. Logical stacks across suspensions come from spans (`tracing`) and Effect's span-based devtools.

## Gaps from the feature-coverage audit (2026-10-16)

`docs/research/effect-feature-coverage.md` left these open after D22-D36. All defaults.

**D37. `catch` never sees an exception no contract lists, and a `catch` that cannot run is reported** (resolves X2). Inside `Effect.gen`, a JavaScript `catch` sees any synchronous exception. In `.es`, `catch` handles expected failures only (D13), so an exception no contract lists, such as one from `Option.getOrThrow`, `Schema.decodeUnknownSync`, or a trusted function's unlisted exception, is a defect it does not see. This difference is intended: those are Effect's "cannot fail here" forms, and each has a typed form for code that can fail (`Option.getOrElse`, `Effect.fromOption`, `Schema.decodeUnknownEffect`). A `try` block that can raise no expected failure makes its `catch` dead code, which is ES2004, a warning by default (D34; prototype finding 16), with the typed forms and `Effect.try` as fixes. Evidence: Effect.gen's behavior verified on 4.0.5 in the audit; 487 Smithers and 70 opencode files mix JavaScript `try` with `Effect.gen`.

**D38. Runners** (resolves X4). The compiler knows runners by resolved symbol: `Effect.runPromise`, `runPromiseExit`, `runSync`, `runSyncExit`, `runFork`, `runCallback` and their `*With` forms; `ManagedRuntime`'s `run*` methods; `NodeRuntime.runMain` and `BunRuntime.runMain`; and `@effect/vitest`'s `it.effect`, `it.live`, their `.each` and `.prop` forms, and `layer(…)`. A runner may be called where no fiber runs the code: at a module's top level, and in a plain callback passed from such a place to a TypeScript function (a `describe` body, a framework's route handler). Anywhere else it is ES1005, because it would start a root with an empty Context and no handlers, outside the caller's structured concurrency; call the program or `fork` it. Tests in `.es` use `it.effect`; Promise-style tests stay in `.ts`.

**D39. A parameter that accepts a function or an `Effect`** (resolves X9). A function argument with parameters is passed as a function. A function argument without parameters is passed as a function when it compiles to plain JavaScript, and is converted to a program when it can pause; the overload or union member that accepts that form is selected. Effect APIs that decide at run time with `Effect.isEffect` (`HttpRouter.add`, `RpcGroup.toLayer`, `Effect.withErrorReporting`) then see the form the compiler chose.

**D40. Effect 4 service features on abstract-class services** (resolves X12). Static members stay on the emitted class: `static readonly layer` holds the default Layer, and a static `make` method plays the part of Effect 4's `make` option (`Layer.effect(Users, () => Users.make())`). A service class that declares `static readonly default`, a value of the service's own type, compiles to a `Context.Reference` whose `defaultValue` returns it, so using it adds nothing to R. Effect's own `Context.Reference(key, { defaultValue })` works unchanged, because a call that returns a key is data (D24).

**D41. Members of a service** (amended by D90) (resolves X13). `S.member(args)` looks the service up, so `S` joins R whatever the member returns, and then applies the call rule to the member: an `Effect` runs; a plain value is returned through a trusted call when the service comes from Effect's packages (D22) or the member carries `@effectscript trusted`, and is ES4401 otherwise. A member that is an `Effect` value runs when called: `Terminal.readLine()` lowers to `yield* (yield* Terminal).readLine`, a row of its own in the call table (prototype finding 3). Value members are read from the implementation: `Settings().baseUrl`. A member the service declares wins over the key's own members (`pipe`, `key`, `of`, `use`, and, because the key is a class, `name`, `length`, `call`, `apply`, `bind`), so `Rpc.call(…)` reaches the service; a static member with the same name as a service member is ES3006 (amended after prototype finding 4).

**D42. Workflow bodies are ordinary programs** (resolves X17). Effect's workflow engine runs a body on a forked child (`workflow/Workflow.ts:695-780`) and suspends it durably by interrupting that fiber (`Workflow.ts:1031-1036`); every execution, first or replay, reruns the body from the start. A body written in EffectScript therefore behaves as any interrupted and rerun program: held continuations are abandoned through their owners and finalizers run (T4), each execution captures fresh continuations (T11), and no producer position or handler state survives into the next execution. Side effects belong in activities, as in Effect. A handler installed outside `Workflow.execute` cannot reach a control operation inside the body, which runs on another fiber (ES4201 applies). Confirming experiment: G18.

**D43. `Effect.callback` is an explicit boundary** (narrows X11). Its register function may call plain TypeScript, as the D23 boundaries may, and its `resume` takes an `Effect<A, E, R>` (`Effect.ts`). `.es` code passes an `Exit`, `resume(Exit.succeed(value))`: an `Exit` is an Effect, and a call returns it as data (D24), whereas `Effect.succeed(value)` would run where it is written. Platform APIs stay behind Effect modules (ES4402), and an adapter for a resource uses the runtime's `suspend` in TypeScript, which reports a lost race (Chapter 6).

## Defaults from the prototype compiler (2026-10-16)

`prototype/esc/FINDINGS.md`: a prototype compiler on `typescript@5.9.3` and `effect@4.0.5` compiles 46 example files written from the spec (26 run, 20 fail with their expected codes) and matches 25 of the spec's 28 claimed contracts. These defaults settle what it found underspecified.

**D44. "The expected type" is the declared, uninstantiated parameter type of the resolved overload** (finding 1). TypeScript's instantiated contextual type is circular: in `ids.map((id) => Users.find(id))`, `U` is inferred from the lambda itself. If overload resolution fails, the candidates whose arity fits are used. The instantiated type is used only when the declared type is a bare type parameter fixed by another argument, such as `Layer.succeed`'s `NoInfer<S>`.

**D45. A service compiles to an interface plus a key class** (finding 19; supersedes D1's emit clause). Effect's `Context.Service` class form makes the instance type the identifier, so `class UsersFake implements Users` is TS2720 and a parameter typed `db: Users` means the identifier. The emission that works is an interface for the shape, with an optional brand `readonly "~esc/service"?: "<key>"`, merged with a class whose static side is a `Context.Key` (`prototype/esc/scratch/probe2.ts`): R stays nominal, `implements` works, and a parameter typed with the service means its methods. Control operations and services with a default emit the same way from the runtime's `controlKey` and `referenceKey`.

**D46. Failure lowering** (findings 11, 12, 14). `finally` lowers to `Effect.onExit`, because `Effect.ensuring` requires a cleanup whose E is `never`, and D15 lets cleanup fail. `throw e` lowers to `return yield* Effect.fail(e)`, so narrowing after the `throw` is kept. E is defined as the type of the expected failures in causes made only of expected failures: a cause that also holds a defect or an interruption may carry `Fail` reasons outside E, which no EffectScript `catch` recovers (K14).

**D47. A block is a Scope region when it declares `using` or calls `fork`** (finding 18), even with no `using`; without this rule a fork-only block found no `Scope`.

**D48. `Scope` joins the prelude** (default). In Effect 4, `Scope` is a module, and the type is `Scope.Scope`; contracts name it constantly. The prelude's `Scope` is the module, and the type `Scope` means `Scope.Scope`, as `Effect` does for `Effect.Effect`.

**D49. Collections of programs convert** (amended by D69, D111) (finding 5). An iterable or record of functions without parameters converts to Effects where an iterable or record of Effects is expected, so `Effect.all(stores.map(makeSync), { concurrency: 4 })` type-checks and runs.

**D50. `Continuation<V, E, H>`** (finding 24). V is what the perform site receives, E the operation's declared errors (what `resumeWith` may raise), and H the result type of the enclosing `handle`. Under a deep handler, `resume(k, v)` returns H: the resumed program runs until it reaches the reinstalled prompt, where `onReturn`, or a later clause that does not resume, produces the value. The checker computes H as the least type containing `onReturn`'s result (the program's A without one) and every clause's return type, taking each `resume`'s type as H.

**D51. Foreign calls at a module's top level, and `@throws`** (amended by D100, D101) (finding 21). ES4401 applies inside functions only: a module's top level runs outside every program, where `Effect.try` would be ES1001, so a plain call there is an ordinary JavaScript call and an exception fails the module's import, as in TypeScript. Each `@throws {T}` names an exported class, and the generated code tests `instanceof T`.

**D52. Inference details** (amended by D71, D72, D73) (findings 7, 10, 13). E and R of a recursive function come from a fixpoint over its strongly connected component of the call graph; when A depends on the recursive result, the contract must be written (ES2102). The iteration stays inside the component (amended 2026-10-16 after the single-pass experiment): the checker checks the component's bodies, then rechecks only those bodies, with the E and R found so far as the members' contracts, until E and R stop growing. The rows are finite and each step only adds to them, a `catch` that removes a fixed set of failures included, so the iteration ends. No function outside the component is rechecked, and no lowering decision waits for it. The probe `isEven`/`isOdd`, whose cycle carries `Odd`, needs it: one pass without it gives `isEven` an E of `never`. A is widened as TypeScript widens a function's inferred return type. Definite assignment is checked on the source, before lowering, because region closures defeat TypeScript's own analysis.

**D53. Diagnostic codes follow the prototype.** (amended by D67, D70, D71, D74) ES1003 is `async` or `await`, ES1004 is `using` at a module's top level, ES1005 is a runner inside code a fiber runs (D38), ES2003 is a return annotation that is not a full `Effect<A, E, R>` (until O7), ES2004 is a dead `catch`, ES2005 is a `resumeWith` program outside the operation's contract (D17), ES2102 is a recursive A that needs a contract, and ES3001-ES3006 are a duplicate key, `extends`, `new`, `instanceof`, a generic service, and a static that shares a service member's name. Chapter 28 lists every code.

**D54. Regions inside a plain-JavaScript function** (default; completes D10 with D5). A function that compiles to plain JavaScript may contain `try`, `catch`, and `finally`, and they compile to JavaScript's own statements, because nothing inside can pause or be interrupted. The generated `catch` rethrows any exception that is not one of the region's expected failures, so D13 holds and defects pass through. A function that declares `using` compiles to frames.

## Defaults from the single-pass experiment (2026-10-16)

`prototype/single-pass/RESULTS.md`: TypeScript 5.9.3, patched to classify each lambda per overload candidate inside `chooseOverload`, reaches the prototype's final lowering for all 46 example files with one check per file: 817 call sites, 257 lambdas and methods, 0 disagreements, and byte-identical output, in 46 checks instead of 124. Two synthetic probe files produce 17 disagreements: 16 that follow from 6 lambdas with two self-consistent lowerings, where the prototype keeps the one its first guess reaches, and 1 from mutual recursion, settled by D52's amendment.

**D56. A lambda's form is decided against each overload candidate** (default; refines D44). A lambda passed as an argument compiles in one of three forms: a plain function, an Effect function, or a program (D7). While TypeScript checks the arguments against a candidate, the lambda takes the form that candidate's declared parameter type asks for (D44), and the candidate that TypeScript's first-applicable rule selects decides the form: with overloads, the first candidate that accepts the lambda in the form that candidate expects wins. The lambda's own inferred type never decides its form; the prototype's two answers came from letting it. A lambda with no expected type, such as a returned lambda or an argument for a bare type parameter that no other argument fixes, is plain when D10 allows and an Effect function otherwise; D8's look-ahead for `pipe` stays. Each lambda is checked at most once per form, and its first classification against a candidate stands, so a deferred re-check never reclassifies it. The call rule applies to the call node itself, so a type guard still narrows (`Exit.isFailure(x) ? x.cause : …`), and a call to a D10 function from plain code has the plain variant's type. D10 eligibility is computed on demand, when a caller's typing needs it, as a greatest fixpoint over the callee's strongly connected component: the component compiles to plain JavaScript when each member would with the calls inside the component counted as plain.

## Calls made for the end-to-end build (2026-10-16)

Will, 2026-10-16: "I'm not going to make calls. It's your job to just use your best judgement. If you want to mark a decision as controversial go ahead and mark it." These close every item that was waiting on him. **Controversial** marks a call he is likely to revisit after using the language.

**D57. Partial annotations use `infer`** (amended by D67, D74, D93) (default, **controversial**; resolves O7). In the return annotation of a function with a body, a channel written `infer` followed by any name is inferred; every other channel is checked.

```ts
function find(id: string): Effect<User, infer E, never> { … }       // checks A and R, infers E
function load(id: string): Effect<User, never, infer R> { … }       // promises no failures, infers R
function plan(id: string): Effect<infer A, PlanError, Database> { … }
```

- Leaving a channel out is ES2003 in that position, never a silent `never` and never a silent inference: `Effect<User>` there asks "did you mean `never` or inferred?", and both fixes are offered (`Effect<User, never, never>` and `Effect<User, infer E, infer R>`). Everywhere else (parameter types, aliases, abstract members, `.ts` files), `Effect<User>` keeps its TypeScript meaning, defaults included.
- The name after `infer` is documentation only and is not in scope anywhere. `infer _` is allowed.
- Why `infer`: it is TypeScript syntax that the TypeScript 7 parser already accepts in that position (verified on the fork, 2026-10-16), and its existing meaning, "let the compiler find this type", is the meaning here. The alternatives fail: `?` does not parse in TypeScript 7 (TS1110), `_` is an ordinary type name a program may declare, and omission is the silent confusion settled decision 2 forbids.
- D12 stands: a function type without a contract accepts any contract.

**D58. The name stays EffectScript** (default, **controversial**; resolves O9). The binary is `esc`, the extension `.es`, the npm scope `@effectscript`, and the repository is `github.com/roninjin10/effectscript`, private. An unrelated compile-to-JavaScript language uses the name at `github.com/EffectScript/effectscript`; a rename before any public release is a search-and-replace over the scope, the binary, and the docs, and nothing is published to npm yet.

**D59. Upstream issues are drafted, not filed** (default; resolves O8). Filing on Effect-TS's repositories speaks for Will in a third-party project, so the drafts live in `docs/upstream/` ready to file, and none is filed without his go-ahead.

**D60. One private repository, `github.com/roninjin10/effectscript`** (Will, 2026-10-16: "Generally yes i do want you to push to github under roninjin10 no org" and "spec and implementations should be the same repo no reason to have 2 repos"). The compiler is microsoft/TypeScript's tree in `typescript/`, imported in one commit without its 349 MB of test baselines, with the EffectScript compiler commits on top; the spec, docs, runtime, and tools live beside it, so one commit can change the compiler, the runtime, and the docs together.

## Decisions from the end-to-end build (2026-10-16)

These entries record the build lanes' defaults. D97-D99 (lane F7) have not reported yet; their numbers remain reserved. The inbox also contains no decisions for D84-D89. The known gaps below are snapshots at each lane's finish, not a new acceptance run.

**D61. ES diagnostics use the TypeScript transport** (default, lane F1). An ES diagnostic is a TypeScript diagnostic with source `ES`, on upstream's content-mapper path. The CLI prints `error ES1003:`; the language server sends `{source: "ES", code: 1003}`, which VS Code shows as `ES(1003)`. `@ts-ignore` and `@ts-expect-error` do not apply to ES diagnostics. An ES diagnostic can hide the TypeScript errors it explains. Bad values in `compilerOptions.effectscript` use TypeScript's own option codes (TS5023, TS6046, TS5024). **controversial**: editors show `ES(1003)`, not `ES1003`. The CLI and editor share one diagnostic path.

**D62. The runtime exports define the prelude** (default, lane F1). The prelude is the runtime's exports. A `.es` file loads the runtime only when it uses a prelude name it does not declare at top level, so files without prelude names compile as plain TypeScript. Prelude names resolve after every scope and before globals. Emitted JavaScript imports `Effect`, `Layer`, `Schema`, and `Scope` as `import * as X from "effect/X"` and the rest as named imports from `@effectscript/runtime`; `.d.ts` files use `import type`. Lazy imports preserve plain TypeScript behavior when no prelude name is used.

**D63. Local imports name source files** (default, lane F1). A `.es` file imports `./x.es` or `./x.ts` by source name with no flag (no TS5097); specifiers become `.js` in JavaScript and `.d.ts` output. `./x.js` probes `.es` after `.ts` and `.tsx`, so `.ts` wins when both exist; `./x.es` falls back to `x.d.ts`. Source names work during checking and output names work in JavaScript.

**D64. Acceptance cases compare diagnostic sets and result lines** (default, lane F1). A failing compiler case passes when its exact set of error codes matches (TypeScript codes count, warnings do not); a runnable case compares only its `@@result` line, so programs may log. ES1002 is defined by type: a `function*` passed where the parameter's type returns a lib `Generator` or `Iterator`. ES1004 also covers class static blocks. Exact codes catch regressions while result markers allow application logging.

**D65. Plain EffectScript calls run inline everywhere** (default, lane F2). A call from a `.es` file to a plain function runs inline as `f$direct(a)` everywhere, including a module's top level, because it computes a value and runs no program. 

**D66. Plain overloads reject suspending function values** (default, lane F2). An overload candidate that asks for a plain function rejects an `.es` function value that can pause, as it rejects such a lambda (D56 extended to function values); a TypeScript function that returns an Effect passes as data; when every candidate rejects, ES4301 reports against the first candidate's form. Named functions and lambdas follow the same callback safety rule.

**D67. Plain return annotations preserve plain function types** (default, lane F2). A function with a plain return annotation keeps that type and compiles to one plain function; only unannotated or Effect-contract functions get the `$direct` plus `Effect.sync` pair, whose TypeScript type is `(…) => Effect<A, never, never>` (chapter 15's one calling convention). **controversial**: it interacts with ES2003, D55 methods need the annotation, and editors show plain helpers with an Effect type unless hover says otherwise. An explicit plain type supports JavaScript protocols and avoids an unwanted Effect wrapper.

**D68. Contract-less callback parameters carry E and R** (default, lane F2). D12 (a callback parameter without a contract accepts any contract) applies to declarations, methods, and function expressions that are not call arguments; the added type parameters are named E and R (then E2, R2) and appear in `.d.ts` output. **controversial**: a contract-less callback cannot be called inside a synchronous JavaScript callback such as a `sort` comparator (ES4301). D12 needs effect-polymorphic declarations without treating missing channels as never.

**D69. Programs lower according to their source form** (default, lane F2). A program lambda lowers to `Effect.gen(function* …)`, a converted function value to `Effect.suspend(v)`, and an array or record of programs built at run time to `$programs`. Each conversion preserves cold execution and the collection shape.

**D70. Jumps cannot leave a finally block** (default, lane F3). A `return`, `break`, or `continue` that leaves a `finally` block is a new error, ES2006; in JavaScript such a jump discards the failure in flight, and compiling `finally` to Effect cleanup cannot honor it. Added to chapters 17 and 28. **controversial**. Effect cleanup cannot preserve JavaScript jumps that discard an in-flight failure.

**D71. Recursive catch and return types can require a contract** (default, lane F3). ES2102 also covers a `catch` variable or return type typed before its cycle's E and R are known; the fix is a written contract. The checker cannot revise a type already cached before the recursive channels are known.

**D72. Pure recursion keeps TypeScript diagnostics** (default, lane F3). ES2102 applies only to functions that run Effects; a pure recursive function keeps TypeScript's TS7023, which a plain annotation fixes (consistent with D67). Only functions that run Effects need EffectScript recursive-channel inference.

**D73. Calls use the written contract** (default, lane F3). A call to a function with a complete written contract adds the contract's E and R, not what its body does. A caller must rely on the public contract rather than a narrower implementation.

**D74. Methods allow plain annotations** (default, lane F3). Methods are exempt from ES2003 for a plain annotation, because a method implementing a plain-function member is declared that way (D55). D55 requires methods that implement plain JavaScript protocols.

**D75. The compiler binary identifies as EffectScript** (default, lane F4). The binary built from `tsc/cmd/tsc` always identifies as EffectScript (`esc --version` prints `EffectScript 0.0.1 (TypeScript 7.1.0-dev)`); there is no `cmd/esc`, which keeps the upstream diff to one seam. A single entry point keeps the upstream compiler seam small.

**D76. TypeScript editor ownership requires disabling the built-in extension** (default, lane F4). The VS Code extension serves `.ts` files only when the built-in TypeScript extension is off (disabled, or `useTsgo` in Cursor); this replaces the spec's "set `typescript.validate.enable` to false", because the server reads that setting and would drop all its diagnostics, `.es` included. **controversial**: D32 expects an automatic stand-down. The language server also reads the validation setting, so disabling validation would hide EffectScript diagnostics.

**D77. Hovers distinguish calls and plain helper signatures** (default, lane F4). In `.es` files a hover adds "Runs here and returns `T`" at a call that runs, and shows a plain function's direct signature with what TypeScript callers see; `.ts` files keep TypeScript's hover. **controversial** (wording). A call exposes its success value while a helper must explain its TypeScript-facing Effect form.

**D78. Prelude and service members complete in scope** (default, lane F4). Prelude names complete as in-scope names that local declarations shadow, and `Users.` lists the service's members. Programmers need the same discoverability as explicit imports and Context members.

**D79. The editor reuses TypeScript grammar and project-local esc** (default, lane F4). The TextMate grammar is VS Code's TypeScript grammar with root scope `source.effectscript` (inner scopes keep `.ts` so themes apply); `esc` is found from the `effectscript.server.path` setting, then the nearest `node_modules/.bin/esc`, then `PATH`; the extension does not run in untrusted workspaces. Existing themes and project compiler versions must keep working, and untrusted workspaces cannot launch the server.

**D80. The runtime checks Effect metadata at load** (default, lane R). The runtime checks Effect's version by importing effect's package metadata with a static JSON import attribute; works in Node, Bun, and bundlers. 

**D81. Callback adapters use the public kernel API** (default, lane R). The runtime exports `suspend`, `handoff`, `Resumer`, and `HandoffProtocolViolation` for TypeScript callback adapters (D43). TypeScript adapters need cancellation-aware handoff and lost-race reporting.

**D82. Runtime extraction preserves the prototype ABI** (default, lane R). The extraction kept the prototype helpers' behavior and continuation parameter order; discrepancies with the spec are documented in packages/runtime/README.md, not fixed silently. Inherited limits: Stream adaptation keeps a one-element buffer (D26's zero-capacity handoff is unfinished, for F7); `$programs` handles iterables, not records (for F2); `$trusted` matches exceptions by name or tag (for F8). Package extraction must not silently change compiler and runtime behavior.

**D83. Scaffolds use absolute file dependencies** (default, lane C). Setup writes absolute `file:` dependencies on the repository's packages into new projects instead of `bun link`; nothing registers globally and nothing is published, so a project keeps working only while the checkout stays where it is. Local projects work without global package registration or publication.

**D90. Service member calls are structural** (default, lane F5). The member-call rule (D41) is structural: it applies to any Context key whose service type declares the member, which covers `.es` services, TypeScript `Context.Service` classes, and Effect's `FileSystem` and `Path`. Calling an Effect value with no arguments runs it (chapter 13's `p()` rule, applied generally). Both rules apply only inside `.es` function bodies. Effect Context keys and EffectScript services need the same member-call behavior.

**D91. Control calls look up their implementation** (default, lane F5). A control-operation call lowers like a service call, `yield* (yield* Approval).approve(x)`, as the prototype did (chapter 13 prints `yield* Approval.approve(x)`); `handle` provides an implementation that performs the kernel operations. **controversial**: F7 may change it. This preserves the prototype lowering while handle installs kernel-backed operations.

**D92. Methods lower when their body or expected type needs Effect** (default, lane F5). A method in a `.es` file becomes an Effect when its body runs or looks up an Effect, or when its expected type returns one; it stays a method whose body is `return Effect.gen({ self: this }, function* () { … })`. The method must retain its receiver while running a generator body.

**D93. Service keys and declarations have a stable shape** (default, lane F5). A service key is the package name (the directory name when there is none), the module path, and `#Name`. Declaration files use the runtime types `ServiceKey` and `ReferenceKey`. A default service needs `static readonly default` (without `readonly` it is an ordinary static). ES2101 requires all three type arguments on `Effect<…>`. `wire` emits its arguments consumers first; ES3204 applies when `Out` is `any` or `unknown`. **controversial**: the `readonly` requirement and ES2101's strictness. Package and module identity keep services distinct and explicit contracts bound provider analysis.

**D94. Using classifies its initializer** (default, lane F6). `using` decides by its initializer. A call that runs an Effect is an acquisition: it gets the block's Scope, `Scope` leaves R, and no `Symbol.dispose` is required. Any other initializer must be a Disposable, `null`, or `undefined`. A Disposable value of either kind also gets `[Symbol.dispose]()` at block exit, so an Effect whose own release already disposes it disposes twice. Effect acquisitions need the block Scope; ordinary values follow JavaScript disposal.

**D95. For-using bodies own a region per iteration** (default, lane F6). In an Effect function, `for (using x of values)` makes each iteration's body a Scope region; the value is taken first, and `using` or `fork` inside the body join that region. Resources and children must end with the iteration that owns them.

**D96. Expression-bodied forks retain Scope** (default, lane F6). A `fork` belongs to the innermost block around it; an arrow function with an expression body has no block, so `() => fork(p)` is `Effect.forkScoped` and keeps `Scope` in R. **controversial**: it differs from `() => { return fork(p) }`; alternatives are treating expression bodies as blocks or reporting a returned block-owned fiber. An expression body has no enclosing block to own the child.

**D100. Foreign calls are plain wherever no fiber runs** (default, lane F8). Code no fiber runs is plain JavaScript for ES4401 too: a module's top level and the plain callbacks it passes to TypeScript functions (`describe` bodies, route handlers), the same places D38 allows runners. Top-level framework callbacks need the same boundary treatment as module initialization.

**D101. Expected foreign failures come from explicit parsing contracts** (default, lane F8). Only built-ins that parse outside text declare expected failures (`JSON.parse` with `SyntaxError`, `decodeURI` and `decodeURIComponent` with `URIError`); every other built-in's exceptions are defects. Trusted failures make named functions and free lambdas Effects but stay defects in plain callbacks. Each `@throws` class must be exported by a module the `.es` file imports, else the call is ES4401. **controversial**: the plain-callback leniency. Parsing outside text has expected failures; unlisted exceptions remain defects.

**D102. Platform APIs and globals are restricted** (default, lane F8). ES4402 at value imports of Node built-ins and `bun:` modules (type-only imports allowed) and at each reference to a listed platform global, including inside boundaries and at top level; web standards that touch nothing outside the program are allowed (`URL`, `TextEncoder`, `AbortController`, `structuredClone`, `Headers`, `Response`). **controversial**: `console.log` and `createServer` need a `.ts` module (examples/http-api moved its server layer to `src/server.ts`). Revised by the orchestrator, see D108. Effect services provide typed errors and replaceable platform access. D108 supersedes the import restriction.

**D103. Top-level program calls have one diagnostic** (default, lane F8). ES1001 covers any top-level call that would run a program, including service member calls and Effect-value calls, reported once at the outermost call. The outermost call identifies the attempted execution without duplicate errors.

**D104. Generated names and spans follow source names** (default, lane F8). Compiled generators are named after their function, variable, or property (an object method `{ *name() {} }.name` when the body refers to that name); `tracing: "exported"` spans exported functions, `"all"` every named lowered function plus methods through `Effect.withSpan("Class.method")`; anonymous lambdas get no span. Named generators and spans make source-level failures identifiable.

**D105. Vite uses one project compiler subprocess** (default, lane V). Amends D31's mechanism, preserving the behavior. `@effectscript/vite` compiles the whole project with one shared `esc` subprocess per source revision and serves the output cached under `node_modules/.effectscript/`, instead of `esc --api` (amends D31's mechanism, same behavior). Whole-project compilation keeps contracts consistent without depending on the API transport.

**D106. Edits invalidate all loaded EffectScript modules** (default, lane V). After an edit the plugin invalidates every loaded `.es` module and its transitive importers, because callers compile differently when a contract changes. A changed contract can change the lowering of callers.

**D107. Vite sources stay under its root** (default, lane V). Sources must sit under Vite's root; the plugin overrides emit paths and source roots there so output lookup and source maps are predictable. **controversial** for monorepos. A shared source root makes output lookup and source maps predictable.

**D108. ES4402 reports platform execution, not imports** (default, orchestrator; implemented by lane F8). Supersedes the import half of D102; its globals list and allowed web standards stand. ES4402 fires where `.es` code runs a platform API (a call, `new`, tagged template, or property read of one, including by destructuring or spreading), not where it names one. Imports, re-exports, value uses, and namespace member reads as values (`os.EOL`) are allowed, so `NodeHttpServer.layer(createServer, { port })` compiles in `.es`. A call ES4402 reports is not also ES4401. Known gap: a platform function reached through a local alias, a parameter, or an object a platform call returned is not followed. Platform integrations must be able to pass host functions to Effect adapters without calling them.

**D109. Only Effect itself selects an Effect callback form** (default, lane F2). An expected callback or program asks for the Effect form only when its type is effect's own `Effect` interface, or a type parameter constrained to it; subtypes such as `Config` are data (D24), so `Config.orElse`'s fallback stays plain. Structured subtypes such as Config must remain data under D24.

**D110. Pipe looks ahead without checking its first stage early** (default, lane F2). The `pipe` look-ahead (D8) reads the first stage's callee signatures, or a function value's own signatures; it runs only when the first argument can be a program, and never checks the stage early. Reading signatures avoids circular inference from a prematurely checked stage.

**D111. Program collection conversion covers iterables** (default, lane F2). The D49 conversion covers any iterable (`Set`, a `Map`'s values) through its iterated type; an iterable never converts as a record, matching `Effect.all` at run time. Compile-time conversion must match Effect.all at run time.

**D112. Mixed plain and Effect callback returns prefer plain when eligible** (default, lane F2). A callback whose expected return mixes plain and Effect (`boolean | Effect<…>`) is plain when D10 allows and an Effect function otherwise; the same holds wherever both forms fit. Where both forms fit, D10 gives the form without Effect operations.

**D113. Plain contract lambdas expose their success type** (default, lane F2). A lambda with a contract that takes the plain form has type `(…) => A`; where only a plain function fits, inference from its annotation starts from A. Plain callback inference needs A rather than an Effect wrapper.

### Known gaps

- **Runtime (R):** At extraction, Stream adaptation retained a one-element buffer instead of D26's zero-capacity handoff; `$programs` handled iterables but not records; `$trusted` matched exception names or tags. F2, F7, and F8 own the follow-up behavior; later decisions and the runtime README take precedence over this extraction snapshot.
- **Setup and examples (C, X):** C's initial end-to-end build stopped on compiler features; F1 later reported `scripts/e2e.sh` passing. X validated seven projects with the prototype. F6 later reported the native examples passing except file-tree and pipeline, which need F7's producers.
- **Build integration (V):** Plugin tests passed 4/4 at landing; template and example runs initially stopped on compiler errors. F8 later reported the template compiling, running, and passing two Vitest tests. Sources outside Vite's root remain unsupported (D107).
- **Compiler foundation (F1):** Only ES module output is supported. Incremental build metadata ignores `compilerOptions.effectscript`. Dynamic `import()` specifiers are not rewritten. F4 subsequently supplied prelude completions (D78).
- **Programs and callbacks (F2):** Declarations carry no `$direct` metadata, so declaration consumers run the correct, slower Effect form. Context-sensitive lambda parameters use the first candidate that types them. The `Money` protocol example needs an explicit boolean return annotation.
- **Contracts and recursion (F3):** Cached types in recursive components can require an extra written contract (ES2102). Cycles through lambdas use the checker's single call type rather than the fixpoint. Region closures capture their own `arguments`. Generic-call type argument inference does not fill a lambda's `infer` channels.
- **Services and wiring (F5):** ES3201 is checked at runners, not where a contract omits a service. ES3202 omits provider file positions. F4 subsequently supplied service-member completion (D78). The top-level service-call gap reported here was subsequently covered by F8's D103. Expected Effect method types are read once rather than per overload candidate; lowered methods do not preserve `super` or `arguments`. The file-tree example has duplicate `effect/Effect` imports.
- **Resources and concurrency (F6):** C-style `for` heads retain TypeScript `using` behavior, without Effect failure or interruption cleanup; Effect acquisitions there report TS2850. Region closures capture their own `arguments`. Returning a block-owned fork returns an interrupted fiber without a warning. Plain-call Disposables register release one step late. Declaration emit can duplicate Effect imports. User Disposables need `ESNext.Disposable` in `lib`; runtime and program must load one copy of Effect.
- **Boundaries and tracing (F8):** Platform APIs reached through local aliases, parameters, or objects returned by platform calls are not followed by ES4402. Anonymous lambdas have no automatic tracing span.
- **Editor (F4):** Hover omits control classes. ES quick fixes, refactors, and catch-specific narrowing completions remain unimplemented (D33). Zed settings are untested. Headless editor checks passed on Linux with VS Code 1.98.2 and 1.105.1, not on this Mac. Serving `.ts` files requires disabling the built-in TypeScript extension or Cursor's `useTsgo` setting (D76).
- **Handlers, producers, and streams (F7):** Decisions D97-D99 and the final gap report are pending. Do not treat these reserved numbers as settled rules.
