# EffectScript fork of the TypeScript compiler

This directory is microsoft/TypeScript with a short stack of commits that teach the Go compiler (`tsc/`) EffectScript: TypeScript syntax in which every function is an Effect (effect@4.0.5). A call to a function that returns an Effect runs it, and the compiler infers each function's `Effect<A, E, R>`.

It is a first prototype. It lowers `.es` files with checker types, runs the output on Effect 4, and emits `.d.ts` files with the inferred Effect types. Services, control operations, failures and regions, Scope regions (`using` and `fork`), the prelude, trust levels at TypeScript boundaries, platform APIs, runners, and debugging and tracing work (see their sections); the acceptance table below says which lane builds each remaining feature. The design is `../docs/design/effectscript-design.md`; the syntax decisions are `../docs/design/DECISIONS.md`.

## Base

| Item | Value |
| --- | --- |
| Upstream commit | `257e87406c6a8be277cb8cd5ef8d7b458a650390`, "[api] Fix named pipe / UDS creation (#64818)", 2026-10-14 |
| Why that commit | It is the `gitHead` of npm `typescript@next` 7.1.0-dev.20261015.1 (`npm view typescript@next gitHead version`) |
| Go | `tsc/go.mod` says `go 1.27`; with Go 1.26 installed, `GOTOOLCHAIN=auto` downloads go1.27.0 |
| History | `typescript/` is upstream's tree at that commit without `tsc/testdata` (61,721 test baselines, 349 MB), imported by one commit of the EffectScript repository. The compiler commits below follow it. To move to a newer upstream commit, replace `typescript/` with that tree, again without `tsc/testdata`, and rebase the compiler commits onto it. |

## Build and test

Run these from `typescript/`:

```sh
cd tsc && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o ../built/local/tsc ./cmd/tsc
effectscript/smoke.sh                 # builds tsc and the runtime, then runs every case
effectscript/smoke.sh --status        # one line per case: name, owner, and status
effectscript/smoke.sh ch04 ref2       # the cases whose names contain ch04 or ref2
effectscript/smoke.sh -v es-infer     # one case, with what tsc printed
effectscript/lsp-smoke.sh             # builds esc and the runtime, then runs the language server test
./built/local/tsc -p effectscript/cases/ts-hello
```

The binary is EffectScript's compiler, `esc`, under any file name; `effectscript/lsp-smoke.sh` builds it as `built/local/esc`, and `../scripts/build-esc.sh` as `../packages/compiler/native/esc`.

The build embeds the `lib.*.d.ts` files; Herebyfile's `-tags=noembed` build copies them next to the binary instead. A cold build takes about 75 s and fills about 1.7 GB of Go build cache. Always pass `-trimpath`: without it, Go keys each package's cache entry by its absolute directory, so every jj workspace cold-builds and adds its own copy. `-gcflags=all=-dwarf=false` shrinks the entries; panics still print stack traces.

### Cases

`effectscript/smoke.sh` builds `tsc`, builds `../packages/runtime` with that `tsc`, and runs `effectscript/harness.mjs`, which compiles the cases in parallel and runs them. Each directory in `effectscript/cases` that has a `tsconfig.json` is a case, and its sources decide its kind:

| Kind | Marked by | Passes when |
| --- | --- | --- |
| Failing | A source whose first line is `// expect: ES2001 ES2002` | The error codes `tsc -p` reports are exactly those codes. Warnings and message text do not count |
| Runnable | A source with a line `// expect-result: <json>` that exports `main()` | `tsc -p` reports no errors, and `out/run.mjs`, which the harness writes, prints `@@result <json>`: the JSON of the success value of the Effect that `main()` returns. What the program logs does not count |
| Plain | Neither | `tsc -p` reports no errors, every file in `expected/` other than `stdout.txt` and `diagnostics.txt` equals the file at the same path under `out/`, and `run.mjs`, or else `out/main.js`, prints `expected/stdout.txt` |

A case of any kind may also have `expected/diagnostics.txt`, which lists the diagnostics `tsc` must report, exactly, one per line in `tsc`'s order and without message text: `main.es(5,23): error ES1001`. Use it where a diagnostic's position or count matters, such as a rule that must report one call and not its neighbor, or a warning that a runnable case must still print.

A case with a file named `pending` is pending. The file's first word names the lane that owns the missing feature and the rest says what is missing, for example `F5 wire (D27)`. A pending case runs but never fails the suite. When it passes, the harness reports `XPASS`, and its owner deletes `pending` in the commit that makes it pass. The suite fails when a case that is not pending fails.

Each case gets two symlinks in its `node_modules`: `effect`, from `$EFFECT_NODE_MODULES` (default `../experiments/node_modules`, else the main checkout's `~/effectscript/experiments/node_modules`), and `@effectscript/runtime`, from `$EFFECTSCRIPT_RUNTIME` (default `../packages/runtime`). `smoke.sh` builds the runtime's `dist/` from source with the fork's `tsc`, so a commit that changes the compiler and the runtime is tested as one. It links `effect` into the runtime's own `node_modules` when that directory has none, so a case loads one copy of `effect`.

| Case | Proves |
| --- | --- |
| `ts-hello` | The stock compiler still compiles and runs plain TypeScript |
| `es-parse` | An `.es` file parses and checks as TypeScript and emits `main.js` and `main.d.ts`; JavaScript prints its value, because `console` is a platform API |
| `es-wrap` | Exported functions and arrow functions lower to `Effect.fnUntraced`, and JavaScript runs them with `Effect.runSync` |
| `es-lower` | Type-directed `yield*`: pure helpers stay plain, Effect-returning TypeScript calls and `.es` calls run, operands run left to right, a `Config` stays data |
| `es-infer` | Unannotated functions get `Effect<A, E, R>`: a typed failure flows from `lookup` into `main`, the `.d.ts` declares it, and the module's top level passes a program to a runner |
| `es-service-keys` | Service classes compile to Context key classes that JavaScript uses as Effect keys; an `implements` class is an implementation; two services with the same members are different types |
| `es-services` | `Users.find(id)` and `Users()` look the service up, value members read from `Settings()`, a provider captures another service, and object literal and class methods run Effects |
| `es-services-ts` | A TypeScript `Context.Service` class and Effect's `Path` are used as services: an Effect-valued member runs when called, and the service's own `pipe` wins over the key's |
| `es-services-default` | `static readonly default` makes a service a `Context.Reference` that never joins R; `static make` and `static readonly layer` stay on the class |
| `es-control` | `extends Control` declares a control operation; `Approval.approve(…)` joins R until the runtime's `handle` answers it |
| `es-service-misuse.fail` | ES3002-ES3006: a service that is extended, constructed, tested with `instanceof`, declared generic, or given a static member with a service member's name |
| `es-service-contract.fail` | ES2101: a service method with a bare return type, a partial `Effect<A>`, or no annotation; a value member takes any type |
| `es-service-keys-shared.fail` | ES3001: two services in one module share a key |
| `es-layer-programs` | A `Layer.effect` program whose object has untyped methods still infers `Layer<Users, never, Database>` |
| `es-wire` | `wire` takes its arguments in any order and emits them consumers first, so one `Database` serves `Users` and `Orders` |
| `es-wire.fail` | ES3202, ES3203, ES3204: a duplicate provider, a cycle, and an argument typed `Layer.Any` |
| `es-wire-missing.fail` | ES3201 at a runner: `MailerLive needs Smtp` |
| `es-provider-escapes.fail` | ES2001 names the provider member that a failure escapes, in an object literal and in an `implements` class |
| `es-provider-audited.fail` | ES3101 at a per-call lookup, in an object literal and in an `implements` class |
| `es-effect-value-calls` | Calling an `Exit` replays it and calling a tagged error instance fails with it (D24): `Effect<A, E, never>` and `Effect<never, EmptyDocument, never>` |
| `es-concise-comments` | A lowered concise arrow keeps the comment after it outside its block, and a lowered file keeps its last comment |
| `es-service-dts` | A service's `.d.ts` is an interface plus a key class: TypeScript code compiled against it with `skipLibCheck` off implements `Users`, provides it with `Layer.succeed`, and yields `Users` and a default service in `Effect.gen` |
| `failures-throw` | `throw` is `Effect.fail`: a throw alone makes a function an Effect, the thrown type joins E (`throw "bad"` adds `string`), a `catch` narrows the caught union by `instanceof` and `_tag` and rethrows, and the `.d.ts` declares each E |
| `failures-causes` | `catch` skips a defect, a mixed cause, and an interruption (D13); `finally` runs on each, and a failing `finally` keeps both failures (D15) |
| `failures-plain` | A function that runs no Effect and catches its own throws stays plain JavaScript, with JavaScript's `try`; its `catch` receives expected failures and rethrows a `TypeError` defect (D54) |
| `es2004-dead-catch.fail` | A `catch` over a `try` that raises no expected failure is ES2004, which `compilerOptions.effectscript` makes an error here; a live `catch` is not reported |
| `contracts` | Written contracts: returns checked against A, a contract wider than the body, a contract on a body that runs nothing, `infer` in each channel (D57), and the `.d.ts` with the inferred channels filled in |
| `contracts-success.fail` | A returned value that A does not accept is TypeScript's TS2322 |
| `es2001-escaping-failure.fail` | A failure from a call and one from a `throw` escape their contracts; a `catch` keeps a third function within its contract |
| `es2002-escaping-requirement.fail` | A service the body needs escapes the contract's R |
| `es2003-bare-return.fail` | A plain return annotation is ES2003 on a function that runs an Effect or throws, and allowed on one that compiles to plain JavaScript (D67) |
| `es2003-missing-channels.fail` | `Effect<string>` and `Effect<string, UserNotFound>` leave channels out |
| `recursion` | `isOdd` and `isEven` learn each other's failures through their cycle (D52), a caller outside reads the solved contract, and a self-recursive `totalSize` needs no contract |
| `es2102-recursive-success.fail` | Effect functions whose A depends on their own result need a contract; a pure recursive function keeps TypeScript's rules |
| `es2102-cycle-catch.fail` | A `catch` whose `try` block calls into a cycle with inferred contracts needs a written contract on the cycle |
| `es2006-finally-jump.fail` | A `return` and a `continue` that leave `finally` blocks are ES2006; a `break` that stays inside one is not |
| `regions-jumps` | `try`, `catch`, and `finally` run as regions: labeled and unlabeled `break` and `continue`, `return` from nested regions, a labeled `try`, a `try` as an `if` body, `switch` inside a region, and `var` in a region give JavaScript's results and `finally` order |
| `lambda-forms` | A lambda takes the form the overload candidate under check declares, and the first candidate that accepts it in that form wins: the overload probes behind `../prototype/single-pass/RESULTS.md`, including a returned lambda, which is plain |
| `programs` | A lambda with no parameters is a program where Effect expects an Effect: `retry`, `race`, `timeout`, `all` over an array and a record, `provide`, a stored lambda that reruns, and an object property typed through `Layer.succeed`'s `NoInfer<S>` |
| `program-values` | A function value converts where it is passed: `Effect.retry(flaky, …)`, array and record literals of programs, and arrays and records built at run time through `$programs` |
| `programs-iterables` | `Effect.all` over a tuple, a record, a `Set`, and a `Map`'s values of programs (D49) |
| `effect-values` | Calling an Effect value runs it: a clock read, a cached program, a `Config`, and a Context key |
| `config-fallbacks` | `Config.orElse`'s fallback lambda returns a `Config` and stays plain: a `Config` extends Effect but is data (D24) |
| `pipe-programs` | `pipe` takes a program first when its first stage takes an Effect (D8), and stays effect's `pipe` on plain data |
| `pipe-generic-stages` | Generic stages infer from the value before them: `Match.when` after `Match.type` and `Match.value`, `Layer.tapError` after a layer, and `Effect.mapError` after a program |
| `plain-functions` | A pure function compiles to a `$direct` variant and an Effect form; calls run inline in source order; a pure recursive pair is plain (a greatest fixpoint); `xs.map(double)` passes `double$direct` |
| `plain-imports` | Another `.es` module's plain functions run inline through named, renamed, and namespace imports |
| `plain-methods` | `[Equal.symbol]` and `[Hash.symbol]` compile to plain methods that `Equal.equals` and `HashSet` call (D55) |
| `callbacks-pausing.fail` | ES4301 for a lambda and for a function value that can pause where `Array.prototype.map` and `forEach` call them |
| `plain-methods-pausing.fail` | ES4301 for an `[Equal.symbol]` that pauses (D55) |
| `callback-contracts` | A callback parameter without a contract accepts any contract (D12): `mapAll` and `withRetry` infer E and R from each call, and the `.d.ts` writes `<A, B, E, R>` |
| `generic-callees` | A call to a generic `.es` function adds the callee's E and R as the call instantiates them |
| `d39-unions` | Where a parameter takes a function or an Effect (D39), a lambda with a parameter is a function, and one without is a program when it can pause and a function when it is plain |
| `callbacks-result-unions` | A callback declared to return a plain value or an Effect, as `Arbitrary.checkEffect`'s property is, is plain when D10 allows it and an Effect function when it can pause |
| `contract-lambdas-plain` | A lambda with a contract where only a plain function fits is plain and returns A: `Schema.suspend` and `SchemaTransformation.transform`. Pending on F3's D57 return check |
| `trust-levels` | `JSON.parse`'s `SyntaxError` and a trusted function's `@throws` class join E and reach a `catch`, in Effect functions and in plain ones (D54); an unlisted exception is a defect; operands that run Effects run before the trusted call; Effect's exports, a boundary, a trusted re-export, a trusted method, and a parameter's callee need nothing |
| `es4401-plain-call.fail` | ES4401 for a plain TypeScript call, one that returns a Promise, one in a plain callback, and a trusted contract whose `@throws` class the file cannot import; none for a trusted call, a boundary, or the top level |
| `es4401-effect-reexports.fail` | D22 through re-exports: `assert` and `expect` imported from `@effect/vitest`, which re-exports vitest's, are trusted, and so is `.toBe` on what `expect` returned; the same `expect` imported from `vitest`, and the program's own method on a value `Option.getOrThrow` returned, keep ES4401 |
| `es4402-platform.fail` | ES4402 where code runs a platform API (D108): imported functions called inside boundaries, a namespace member called, a property read on a default import and on a namespace member, `fetch`, `setTimeout`, `console.log`, destructuring `process`, and `new WebSocket`; importing, passing `createServer` and `os.EOL` as values, a type-only import, `URL`, and `TextEncoder` are allowed |
| `platform-values` | Importing `createServer` from `node:http` and `os` from `node:os`, and passing them as values, compiles and runs |
| `es4403-clock.fail` | ES4403 for `Date.now()`, `new Date()`, `Math.random()`, `crypto.randomUUID()`, `performance.now()`, and `Date()`, at the top level too; `new Date(text)`, `Date.parse`, and `Date.UTC` are allowed |
| `es4403-warning` | With `"ES4403": "warning"`, the reads are warnings and run as plain calls |
| `es1001-top-level.fail` | ES1001 for an `.es` program, an Effect, a program passed straight to a runner, a service member call, a service, and an Effect value at the top level; none for values, lambdas, or a runner given a program |
| `es1005-runners.fail` | ES1005 for runners in code a fiber runs, `ManagedRuntime` and `*With` forms included, and for `it.effect` in a test body; none at the top level or in a plain callback the top level passes to TypeScript (`describe`, a route handler, `layer(…)`); `stubs/` holds the real declarations of `NodeRuntime.runMain` and `@effect/vitest` |
| `debug-stack` | With `sourceMap`, Node's stack traces point at `.es` lines and name the function, a region closure (`audited$try`), and a recursive function whose generator is a method (D36) |
| `tracing-exported` | `"tracing": "exported"` gives exported functions `Effect.fn` spans named after them |
| `tracing-all` | `"tracing": "all"` gives every named Effect function a span, a method an `Effect.withSpan` span named `Class.method`, and a callback none |
| `plain-const` | A function that declares `const` and `let` and runs no Effect compiles to plain JavaScript, so `Array.prototype.map` calls it; only `using` needs frames (D54) |
| `scopes-using` | `using` releases in reverse order on success, failure, and interruption; once per loop iteration through `continue`, a labeled `break`, and `return`; before a `catch` runs; in a method and an arrow function; a TypeScript Disposable and `null`; an acquisition without `using` stays the caller's (D14); a masked acquisition finishes before a timeout takes effect and an interruptible one does not start (D25); `for (using x of values)` |
| `scopes-fork` | A block interrupts the fiber it forked and waits for its cleanup before releasing what it acquired earlier: in a function body, an inner block, a loop body, and a block that fails; `Fiber.join` returns the child's result or its failure; a `fork` that no block encloses uses the caller's Scope; `Effect.forkScoped` and `Effect.forkDetach` keep Effect's meaning |
| `scopes-types` | `Scope` leaves R only for `using` acquisitions and forks a block owns; `contracts.ts` checks each contract exactly |
| `scopes-callback.fail` | ES4301 for a callback that declares `using`, of a Disposable or an acquisition, passed to `Array.prototype.map` or `Effect.sync` (D54) |
| `lambda-this` | A lowered arrow function keeps its enclosing function's `this`: a program passed to `Effect.retry`, an Effect function passed to `Effect.forEach`, and a forked program in a method |

### Acceptance cases

The prototype's 46 examples (`../prototype/esc/examples/`) are cases named after their files, such as `ch04-wire` and `ref2-cycle.fail`. Each keeps its `.es` source and `.ts` helpers as the prototype has them, except that kernel imports name `@effectscript/runtime` and `@effectscript/runtime/kernel`. They compile with `module: nodenext` and `rewriteRelativeImportExtensions`, as the app template does. Every compiler lane turns its cases green and never turns a green case red. Each case is pending on the lane that owns its feature; a case that mixes features belongs to the lane that owns its first missing feature, and its `pending` file names the other lanes it needs:

| Owner | Feature | Cases |
| --- | --- | --- |
| F1 | Diagnostics, prelude, imports between files | `ch10-effect-gen.fail` |
| F2 | Programs as values, plain functions, callbacks | `ch05-programs`, `ch10-map-pausing-callback.fail`, `ch13-plain`, `ch14-callbacks`, `ref4-callbacks`, `ref4-pausing-function-value.fail`, `ref5-programs-as-data` |
| F3 | `throw`, regions, contracts | `ch02-contract-narrower.fail`, `ch02-functions`, `ch03-errors`, `ch03-safelookup-no-try.fail`, `ch12-contracts`, `ch12-welcome-missing-mailer.fail`, `ch15-failures`, `nominal-services.fail`, `regions` |
| F5 | Services, layers, `wire` | `ch01-first-look`, `ch04-provider-escapes.fail`, `ch04-services`, `ch04-users-audited.fail`, `ch04-wire`, `ch12-rates-bare-return.fail`, `ref1-existing-service`, `ref2-ambiguous-provider.fail`, `ref2-cycle.fail`, `ref2-missing-provider.fail`, `ref2-opaque-layer.fail`, `ref2-provider-graph` |
| F6 | `using`, `fork`, Scope regions | `ch06-disposable`, `ch06-resources`, `ch07-concurrency`, `ref3-resourceful-retry` |
| F7 | Handlers, producers, streams | `ch08-producers`, `ch08-streams`, `ch09-approve-all-forked.fail`, `ch09-handlers`, `ref6-fork.fail`, `ref6-handlers`, `stack-depth` |
| F8 | Boundaries, runners | None pending: `ch02-plain-ts-call.fail`, `ch10-fetch.fail`, `ch10-typescript`, `ch11-top-level-run.fail`, `ch24-profile`, and `d5-clock-read.fail` pass |

The `pending` files are the source of truth: `effectscript/smoke.sh --status` prints each case's current owner.

## The stack

Each commit builds and passes `smoke.sh`. The change IDs are jj's and survive rewrites.

| # | Change | Subject | Why |
| --- | --- | --- | --- |
| 1 | `pzxkrxsm` | effectscript: add a build-and-run smoke test | A one-command check that every later commit must pass |
| 2 | `ltlztyux` | tspath: accept .es files as TypeScript sources | `.es` root files, globs, `ScriptKindTS`, and `foo.js`/`foo.d.ts` output |
| 3 | `xvqlzzvl` | compiler: add a script transformer registration hook | The only emit seam: hook transformers run before type erasure; `main.go` blank-imports the package |
| 4 | `wzsmttko` | effectscript: lower .es functions to Effect.fnUntraced generators | The rewriting mechanics, syntax only: no call runs yet |
| 5 | `uvwpnsyr` | checker: let emit-time extensions query the checker | `EmitResolver.WithChecker` holds the checker lock that parallel emit needs |
| 6 | `wzmxpyyu` | effectscript: run Effect-typed calls with yield* | The type-directed decision: which calls run and which functions are Effects |
| 7 | `wnyqnmso` | checker: add a call expression type hook | Seam for typing a call by what it does |
| 8 | `vlkoxlmy` | effectscript: type a call that runs as the Effect's success value | `const name = findName(id)` is a `string` inside an `.es` function |
| 9 | `xnptkzuz` | checker: add an inferred return type hook | Seam for an inferred return type, plus `NewTypeReference` |
| 10 | `ukynlwvn` | effectscript: infer Effect<A, E, R> for .es functions | The ABI: callers outside `.es` bodies and `.d.ts` files see an Effect |
| 11 | `wzntprnu` | effectscript: describe the fork in EFFECTSCRIPT.md | This file |
| 12 | `zwwnumoq` | effectscript: build with -trimpath in smoke.sh | Workspaces share Go build-cache entries |
| 13 | `wnytpnpp` | runtime: export the kernel as @effectscript/runtime/kernel | The `stack-depth` case reads the kernel's hooks |
| 14 | `mzuwnztz` | effectscript: run cases with a parallel harness | Failing, runnable, and pending cases, and `--status` |
| 15 | `ylzmpsoz` | effectscript: port the prototype's 46 examples as cases | The acceptance suite, each case pending on its owner |
| 16 | `wskpkmqv` | checker: add hooks for file checks and diagnostic filters | Report ES diagnostics while checking, and hide the TypeScript errors they explain |
| 17 | `wvstzskn` | compiler: exempt registered diagnostic sources from comment directives | `@ts-ignore` cannot hide an ES diagnostic |
| 18 | `yuprxynn` | tsoptions: add the effectscript compiler option | `compilerOptions.effectscript` (D34) |
| 19 | `wuupxsnv` | effectscript: add the ES diagnostics registry and reporting path | Every Chapter 28 code, with `report`, `note`, and `cover` |
| 20 | `qqszuvwv` | effectscript: report ES1003 and ES1004 | `async` and `await`; `using` at a module's top level |
| 21 | `msunyvzt` | effectscript: report ES1002 for generator-based APIs | `Effect.gen` and friends in a `.es` file |
| 22 | `rkproqwz` | checker: add a class base hook | Seam for a class whose bases an extension chooses: a service's static side is a Context key |
| 23 | `mxvtplyt` | effectscript: compile abstract classes to service keys | D1, D45: the key class, the branded shape, and the runtime import |
| 24 | `zympzwnv` | checker: add member receiver and call signatures hooks | Seams for `Users.find(id)` and for calling an Effect value |
| 25 | `ssslmlqz` | effectscript: call service members through the current Context | D41: `yield* (yield* Users).find(id)`, `Users()`, and R |
| 26 | `ptlsrptn` | effectscript: lower methods to Effect functions | Providers and `implements` classes have methods that run Effects |
| 27 | `sxoqtkxo` | effectscript: give a service with a static default a reference key | D40: `static readonly default` needs no provider |
| 28 | `qlnyylqp` | effectscript: declare control operations | D2: `extends Control` registers kernel operations; the entry point for handlers |
| 29 | `ozrknmym` | effectscript: document services in EFFECTSCRIPT.md | This section |
| 30 | `ykwlmytt` | checker: add a catch clause variable type hook | Seam for typing a `catch` variable as the `try` block's expected failures |
| 31 | `yullvuom` | checker: expose an expression's type before literal widening | `throw "bad"` adds `string`, as `return "bad"` does |
| 32 | `mssmulnm` | effectscript: lower try, catch, and finally to Effect regions | Regions, completion records, and E through a `try` |
| 33 | `qsxrpnuo` | effectscript: lower throw to Effect.fail and add its type to E | `throw` is an expected failure (D46) |
| 34 | `vlzoymlw` | effectscript: keep JavaScript's try in plain functions (D54) | A plain function's `catch` sees expected failures only; `$expected` and `$caught` in the runtime |
| 35 | `ywvpypzw` | checker: add a name resolution hook before the global scope | Prelude names behave like imports |
| 36 | `zlxmxpnz` | compiler: add an implicit import hook | The program loads `@effectscript/runtime` for `.es` files |
| 37 | `owtkpmql` | runtime: type the prelude's Effect and Scope | `Effect<A, E, R>` and `Scope` are types too |
| 38 | `swxmkoyv` | effectscript: point the runtime at the cases' effect in smoke.sh | One copy of `effect` per program |
| 39 | `urznvokp` | effectscript: resolve and import the prelude in .es files | Every `.es` file sees the prelude (D4, D48) |
| 40 | `uwozskvx` | effectscript: compare a runnable case's result, not its logs | Cases that log, such as `ch01-first-look`, compare their result |
| 41 | `oywqrtxx` | module: resolve .es source files | `./welcome.es` and, from `.ts`, `./welcome.js` find `welcome.es` |
| 42 | `urkxzymx` | core: let .es files import TypeScript sources by extension | No TS5097 for `./welcome.es` or `./format.ts` in `.es` |
| 43 | `ntslxkmz` | effectscript: emit .js specifiers for imports of sources | Imports between files (D31) |
| 44 | `rplwrxxw` | effectscript: record a prelude use only at a source reference | No duplicate `Effect` import when the file never names it |
| 45 | `nvukvnsq` | effectscript: test the runtime import's free name | Helpers import as `runtime_1` when the file declares `runtime` |
| 46 | `srwuqnot` | effectscript: report misused service classes | ES2101 and ES3001-ES3006 |
| 47 | `yvlmtqkl` | effectscript: lower a method's body as an Effect body only | A method lowered inside a plain function takes Effect lowering only |
| 48 | `truwtzrw` | checker: add a skipped function type hook | Seam for the placeholder a context-sensitive function has in overload resolution's first pass |
| 49 | `qzmttpum` | effectscript: infer a program whose result is context sensitive | `Layer.effect(Users, () => ({ find(id) { … } }))` infers E and R |
| 50 | `zvvqmzyy` | effectscript: order wire's arguments and check its provider graph | D27: consumers first; ES3202, ES3203, ES3204 |
| 51 | `vwtxzvnt` | effectscript: report a service that wire leaves unprovided at a runner | ES3201 with the provider that needs the service |
| 52 | `ktslyskx` | effectscript: check providers against their service's contracts | ES2001 and ES3101 name the member at fault |
| 53 | `xorurrnt` | effectscript: document service checks and wire in EFFECTSCRIPT.md | This section |
| 54 | `rwmokwmv` | checker: add an entity name visibility hook | A declaration file may write a prelude name |
| 55 | `knrysqlq` | compiler: add a declaration transformer hook | Extensions can add imports to a declaration file |
| 56 | `psvzumxw` | module: resolve an .es specifier to its declaration file | `./users.es` finds `users.d.ts` in a published package |
| 57 | `nxmqvpsu` | effectscript: write portable declaration files for .es files | `declaration: true` works with the prelude and `.es` imports |
| 58 | `llryttxs` | effectscript: declare a service as an interface and a key class | D45 in `.d.ts`: TypeScript code uses an `.es` service as a Context key |
| 59 | `zummuosy` | effectscript: document service declaration files in EFFECTSCRIPT.md | This section |
| 60 | `ksxlppnq` | runtime: test trusted exceptions with instanceof | D51: `$trusted` tests the classes the compiler passes; `$trustedPlain` for plain functions (D54) |
| 61 | `ywyktuko` | effectscript: compare a case's diagnostics with expected/diagnostics.txt | A case can pin each diagnostic's position and count |
| 62 | `rvpuznxt` | effectscript: report ES4401 for calls without a trust level | D5, D22, D23: trust levels by declaration; boundaries; code outside fibers is JavaScript (D51) |
| 63 | `uspovlno` | effectscript: lower the expected failures of trusted calls | `@throws` classes and built-in contracts join E and compile to `$trusted` |
| 64 | `nuwqloyu` | effectscript: report platform APIs (ES4402) and clock reads (ES4403) | D6: each error names the Effect module that replaces the API |
| 65 | `olsmqqqy` | effectscript: report programs started outside a runner (ES1001, ES1005) | D38: the runner registry, which ES3201 shares |
| 66 | `zunmxlrk` | effectscript: name generators after their source (D36) | Stack traces and debuggers name the `.es` function |
| 67 | `xnmknson` | effectscript: trace functions with Effect.fn spans (D34) | `tracing`: `"exported"` and `"all"` |
| 68 | `stmorqmo` | effectscript: document boundaries, runners, debugging, and tracing | The sections from "Code that is not Effect" to "Tracing" |
| 69 | `ooknlorp` | effectscript: report ES4402 where code runs a platform API (D108) | Importing a platform API and passing it as a value, as `NodeHttpServer.layer(createServer, …)` does, is allowed |
| 70 | `okrmuzvz` | effectscript: add a pending case for calling Exit and error values | The D24 gap the user docs found, as a failing case |
| 71 | `rvysxzyx` | effectscript: run a call to an Exit or an error value | D24: `outcome()` and `error()` run; the docs lose two compiler-gap markers |
| 72 | `ysknkpmu` | effectscript: keep the comment after a concise arrow out of its block | A block made from a concise body has no source range for the printer's closing-brace comments |
| 73 | `vsvzswzz` | effectscript: keep the last comment of a lowered file | The statement list keeps its source range when the lowering adds imports |
| 74 | `twymwmrr` | effectscript: document calls to Exit and error values in EFFECTSCRIPT.md | This section |
| 75 | `oqrpqyrk` | effectscript: report ES2004 for a catch that can never run | D37: a `catch` over a `try` that cannot fail is dead code |
| 76 | `rtqqsqrv` | declarations: add a hook to declare a type from the checker instead of its annotation | A contract's inferred channels reach the `.d.ts` |
| 77 | `kxzmxurw` | checker: add hooks for checked return types and infer outside conditional types | Seams for checking returns against A and for `infer Name` channels |
| 78 | `stuypkuq` | effectscript: check written contracts (ES2001, ES2002, ES2003, D57) | A written contract is checked, and `infer` channels come from the body |
| 79 | `ovsoxmyu` | effectscript: solve E and R of call cycles as a fixpoint (D52, ES2102) | `isEven` and `isOdd` both fail with what either throws |
| 80 | `uvuursyx` | effectscript: report ES2006 for a jump out of a finally block | A `finally` that returns would discard the failure in flight |

Lane F2's commits, which landed among these, are listed in [Programs, plain functions, and callbacks](#programs-plain-functions-and-callbacks), lane F6's in [Scope regions: using and fork](#scope-regions-using-and-fork), and lane F4's in [The editor and the esc command](#the-editor-and-the-esc-command).

## Seams in upstream files

EffectScript code lives in `tsc/internal/effectscript/`, which may import any internal package because it sits under `tsc/internal/`. It registers its hooks from `init`. Upstream code changes in three ways: one-line call sites in existing files, new files that hold the hooks, and the `effectscript` compiler option, which is added by hand to the generated option tables because the generator's type model has no nested objects.

| Upstream file | Change | Lines (+/-) |
| --- | --- | --- |
| `tsc/internal/tspath/extension.go` | `ExtensionEs` in the TypeScript extension lists; `.es` declares to `.d.ts` | +9 / -8 |
| `tsc/internal/core/core.go` | `.es` is `ScriptKindTS` | +1 / -1 |
| `tsc/internal/compiler/emitter.go` | Run hook transformers first in `getScriptTransformers` | +2 / -0 |
| `tsc/cmd/tsc/main.go` | Blank import of `internal/effectscript` | +2 / -0 |
| `tsc/internal/checker/checker.go` | Apply call-type hooks where `checkCallExpression` returns and on the `getQuickTypeOfExpression` call shortcut; wrap `getReturnTypeFromBody`; hold extension state; run file check hooks at the end of `checkSourceFile`; filter what `getDiagnostics` returns; ask class base hooks in `getBaseConstructorTypeOfClass` and `resolveBaseTypesOfClass`; ask member receiver hooks before `checkPropertyAccessExpressionOrQualifiedName` looks up a member; ask call signatures hooks in `resolveCallExpression` when the callee has none; set `ResolveBeforeGlobals`; ask skipped function type hooks where `checkFunctionExpressionOrObjectLiteralMethod` skips a context-sensitive function | +23 / -5 |
| `tsc/internal/checker/checker.go` | Ask the catch clause variable type hook in `getTypeForVariableLikeDeclaration`; a catch variable is always initialized | +4 / -1 |
| `tsc/internal/checker/checker.go` | Ask using initializer hooks before `checkVariableLikeDeclaration` requires a `using` initializer to be a Disposable | +1 / -1 |
| `tsc/internal/checker/checker.go` | Check what a function returns against `unwrapReturnTypeOf` at the four places that check it against `unwrapReturnType`; complete an annotated return type through annotated return type hooks in `getReturnTypeOfSignature`; ask infer type hooks in `checkInferType` | +9 / -7 |
| `tsc/internal/transformers/declarations/transform.go` | `ensureType` serializes a type from the checker when a type annotation hook asks | +1 / -1 |
| `tsc/internal/checker/checker.go` | Tell call candidate hooks which candidate `inferTypeArguments` and `isSignatureApplicable` check, and let candidate filter hooks reject it; pass the type of a function expression, a contextual return type (`getContextualReturnType` wraps the old body as `getContextualReturnTypeWorker`), and every checked expression through hooks; append hook type parameters in `getSignatureFromDeclaration` and `getOuterTypeParameters`; let parameter type hooks replace an annotation's type in `getTypeForVariableLikeDeclaration`; let annotated return inference hooks replace the return annotation's type in `inferFromAnnotatedParametersAndReturn` | +15 / -5 |
| `tsc/internal/compiler/program.go` | Comment directives skip diagnostics from exempt sources | +1 / -1 |
| `tsc/internal/compiler/fileloader.go` | Resolve each module an implicit import hook names for a file | +4 |
| `tsc/internal/binder/nameresolver.go` | `NameResolver.ResolveBeforeGlobals`, consulted before the global scope | +6 |
| `tsc/internal/module/resolver.go` | Try `.es`, then `.d.ts`, for `./welcome.es`, and `.es` after `.tsx` for `./welcome.js` | +19 |
| `tsc/internal/module/util.go` | An `.es` resolution is always allowed | +2 / -1 |
| `tsc/internal/module/util.go` | An `.es` file's output extension is `.js`, so auto-import can name it | +1 / -1 |
| `tsc/internal/tsoptions/parsedcommandline.go` | `.es` in a project's wildcard include globs, so a new `.es` file joins the project | +4 / -4 |
| `tsc/internal/lsp/server.go` | `.es` in the `willRenameFiles` filter; exit with status 0 after `shutdown` and `exit` | +3 / -3 |
| `tsc/internal/ls/hover.go` | Apply quick info hooks to each hover | +1 |
| `tsc/internal/ls/completions.go` | Add completion scope hooks' symbols to global completions; let member completion hooks replace the type left of the dot | +2 |
| `tsc/internal/execute/tsc/help.go` | The `--version` line, the `--help` title, its examples' command, and its icon come from the identity | +6 / -6 |
| `tsc/internal/core/compileroptions.go` | A `.es` file may import a TypeScript source by its extension | +5 / -1 |
| `tsc/internal/checker/emitsupport.go` | Ask entity name visibility hooks in `isEntityNameVisible` | +4 |
| `tsc/internal/compiler/emitter.go` | Run declaration transformer hooks after TypeScript's | +2 / -2 |
| `tsc/internal/core/options_generated.go` (generated) | `CompilerOptions.EffectScript`, in `Clone` and `Equals` | +6 |
| `tsc/internal/tsoptions/declarations_generated.go` (generated) | The `effectscript` declaration | +8 |
| `tsc/internal/tsoptions/options_generated.go` (generated) | Parse, merge for `extends`, compare for semantic diagnostics and emit, and serialize for `--showConfig` | +15 / -1 |
| `tsc/internal/tsoptions/commandlineoption.go`, `tsconfigparsing.go` | The option's extra validation | +4 |
| `tsc/internal/compiler/extensions.go` (new) | `RegisterScriptTransformerHook`, `RegisterDirectiveExemptSource`, `RegisterImplicitImportHook`, `RegisterDeclarationTransformerHook` | +106 |
| `tsc/internal/checker/extensions.go` (new) | `WithChecker`, `RegisterCallExpressionTypeHook`, `RegisterInferredReturnTypeHook`, `NewTypeReference`, `ExtensionState`, `CompilerOptions`, `AddDiagnostic`, `RegisterSourceFileCheckHook`, `RegisterDiagnosticsFilterHook`, `RegisterCatchVariableTypeHook`, `GetTypeOfExpressionBeforeWidening`, `RegisterNameResolutionHook`, `RegisterEntityNameVisibilityHook`, `RegisterUnwrappedReturnTypeHook`, `RegisterAnnotatedReturnTypeHook`, `RegisterInferTypeHook`, `GetReturnTypeFromBody` | +288 |
| `tsc/internal/transformers/declarations/extensions.go` (new) | `RegisterTypeAnnotationHook` | +27 |
| `tsc/internal/checker/extensions_classes.go` (new) | `RegisterClassBaseHook`, `Program`, `GetStringLiteralType`, `NewOptionalReadonlyPropertyType` | +48 |
| `tsc/internal/checker/extensions_calls.go` (new) | `RegisterMemberReceiverHook`, `RegisterCallSignaturesHook`, `NewCallSignature` | +52 |
| `tsc/internal/checker/extensions_functions.go` (new) | `RegisterSkippedFunctionTypeHook`, `GetNonInferrableAnyType` | +35 |
| `tsc/internal/checker/extensions_forms.go` (new) | `RegisterCallCandidateHook`, `RegisterCandidateFilterHook`, `RegisterFunctionExpressionTypeHook`, `RegisterContextualReturnTypeHook`, `RegisterAnnotatedReturnInferenceHook`, `NewFunctionTypeWithReturnType` | +142 |
| `tsc/internal/checker/extensions_values.go` (new) | `RegisterExpressionTypeHook`, `CreateArrayType`, `MapObjectType`, `IteratedType` | +55 |
| `tsc/internal/checker/extensions_callbacks.go` (new) | `RegisterTypeParametersHook`, `RegisterParameterTypeHook`, `NewTypeParameter` | +61 |
| `tsc/internal/checker/extensions_using.go` (new) | `RegisterUsingInitializerHook` | +29 |
| `tsc/internal/ls/extensions.go` (new) | `RegisterQuickInfoHook`, `RegisterCompletionScopeHook`, `RegisterMemberCompletionHook` | +92 |
| `tsc/internal/execute/tsc/extensions.go` (new) | `SetIdentity` | +56 |
| `tsc/internal/checker/extensions_signatures.go` (new) | `SignatureWithReturnType` | +8 |
| `tsc/internal/core/effectscript.go` (new) | `EffectScriptOptions` | +22 |
| `tsc/internal/tsoptions/effectscript.go` (new) | Parse, validate, and serialize `compilerOptions.effectscript` | +149 |

A rebase onto a newer nightly can conflict only in the existing files above. If upstream regenerates the option tables, add the `effectscript` lines again; `tsoptions/effectscript.go` lists where they go.

## What the lowering does

The transform runs on each `.es` file before the TypeScript transforms. It holds the checker lock, plans the file, then rewrites it:

```ts
// main.es                                   // main.js
function lookup(id: number) {                const lookup = Effect_1.fnUntraced(function* (id) {
  if (id >= names.length)                        if (id >= names.length)
    Effect.fail(new NotFound({ id }))                yield* Effect.fail(new NotFound({ id }));
  return names[id]                               return names[id];
}                                            });
```

- **Which calls run.** A call runs, as `yield* f(x)`, when its resolved signature returns effect's own `Effect` interface, or when it calls an `.es` function that is an Effect. Types that extend `Effect` (`Config`, `Exit`, Context keys, `TaggedError` instances) are data (design D24). Only calls in a lowered function's own body run; calls at the module top level, in parameter initializers, and in nested plain functions stay plain. A call to a function that compiles to plain JavaScript runs inline, as `double$direct(x)`, anywhere in the file.
- **Which functions become Effects.** A function declaration becomes `Effect.fnUntraced(function* …)` when it performs an Effect operation or can fail; otherwise it compiles to plain JavaScript (D10), as a `$direct` variant and an Effect form. A lambda takes the form its position asks for: a plain function, an Effect function, or a program. [Programs, plain functions, and callbacks](#programs-plain-functions-and-callbacks) gives the rules.
- **Output shape.** A declaration becomes a `const` (with `export` or `export default`), and the file imports `* as Effect from "effect/Effect"` under a free name. Operands keep JavaScript's order, for example `(yield* greet("a")) + (yield* greet("bc"))`.
- **Types.** Inside a lowered body, a call that runs has the Effect's success type, so `findName(2).toUpperCase()` checks. An unannotated `.es` Effect function has return type `Effect<A, E, R>`. A comes from its returns; E and R are the unions of the error and requirement channels of every Effect it runs. The `.d.ts` declares `main(): Effect.Effect<void, NotFound, never>`, and a callback such as `(e) => Console.log(…)` passed to `Effect.catch` checks and runs as an Effect function.

Checking takes one pass. The analyzer decides from function bodies and callee types, and the hooks type each call and lambda inside TypeScript's own overload resolution.

## Programs, plain functions, and callbacks

A function value compiles in the form its position asks for (Chapter 13, "Function values"):

```ts
// main.es                                       // main.js
Effect.retry(() => sync(store), policy)          Effect.retry(Effect.gen(function* () { return yield* sync(store); }), policy)
Effect.forEach(ids, (id) => find(id))            Effect.forEach(ids, Effect.fnUntraced(function* (id) { return yield* find(id); }))
ids.map((id) => id.trim())                       ids.map((id) => id.trim())
xs.map(double)                                   xs.map(double$direct)
Effect.all(stores.map(makeSync))                 Effect.all(runtime.$programs(stores.map(makeSync$direct)))
```

### Lambda forms (D56)

A lambda, an arrow function or function expression, takes one of three forms. The body is typed the same way in each, because a call that runs has its Effect's success type wherever the lambda sits; only the lambda's own type depends on the form, so each form is computed once from the same body.

| Form | The checker sees | Lowers to |
| --- | --- | --- |
| Plain | `(x) => A`, what TypeScript infers | the lambda as written |
| Effect function | `(x) => Effect<A, E, R>` | `Effect.fnUntraced(function* (x) { … })` |
| Program | `Effect<A, E, R>` | `Effect.gen(function* () { … })`, which runs the body each time it runs |

An arrow function that refers to `this` keeps its enclosing function's `this`, as JavaScript's arrow functions do: its Effect function is bound with `.bind(this)`, and its program is `Effect.gen({ self: this }, function* () { … })`, so `fork(() => this.poll())` in a method reaches the instance (case `lambda-this`).

The position decides, inside TypeScript's own overload resolution (`forms.go`, `expected.go`):

1. `inferTypeArguments` and `isSignatureApplicable` tell the call candidate hook (`callCandidate`) which candidate they check a call's arguments against.
2. When the checker types a lambda among the arguments, also inside an array literal or as an object literal property, the function expression type hook (`functionExpressionType`) reads that candidate's declared, uninstantiated parameter type (D44) and follows the path to the lambda through element and property types. A bare type parameter says nothing, so the lambda falls back to its contextual type, instantiated with the inferences made so far, as `Layer.succeed`'s `NoInfer<S>` needs.
3. The expected type decides the form: a function type that returns an Effect asks for an Effect function, another function type for a plain function, and an Effect for a program. Only effect's own Effect interface counts, so a callback expected to return a `Config`, which extends Effect but is data (D24), stays plain. Where a parameter accepts a function or an Effect (D39), a lambda with parameters is a function and one without is a program when it can pause. Without an expected type, as for a returned lambda, a lambda is plain when D10 allows it and an Effect function otherwise, and so is a lambda whose expected type returns a plain value or an Effect, such as `boolean | Effect<boolean, E, R>`. A lambda with a contract, `Effect<A, E, R>`, that takes the plain form returns A, and inference from its annotation infers from A (the annotated return inference hook, `annotatedReturnInference`).
4. A plain function cannot run an Effect, so when a candidate asks a lambda that pauses for the plain form, the candidate filter hook (`filterCandidate`) rejects the candidate and resolution tries the next one. The first candidate that accepts the lambda in the form it asks for wins, as TypeScript's first-applicable rule says.
5. Each candidate's first answer stands. The lambda compiles to the form the candidate resolution selected asked for, read back from the call's resolved signature.

A return statement in an Effect function or program is contextually typed by the success type of the Effect expected of it (the contextual return type hook, `contextualReturnType`), so the object a `Layer.effect` program returns gets the service's shape. `pipe` looks ahead (D8): when its first argument is a function with no parameters and its first stage takes an Effect, that argument is a program (`pipe.go`). The look-ahead reads the signatures of the stage's callee instead of checking the stage, so a generic stage such as `Match.when(…)` is inferred once, from what `pipe` passes it.

### Programs as values (D7, D49)

A function value converts where it is passed, as a lambda takes its form there. The expression type hook (`expressionType`, `conversions.go`) gives an argument, array element, or property value the type of its conversion against the candidate under check:

| Expected | Value | Lowers to |
| --- | --- | --- |
| An Effect | A function with no required parameters that returns an Effect | `Effect.suspend(value)` |
| An iterable or record of Effects | An iterable, such as an array, a `Set`, or a `Map`'s values, or a record of such functions, built at run time | `runtime.$programs(value)`, which makes an iterable an array and keeps a record's keys |
| A plain function | An `.es` function that compiles to plain JavaScript | `f$direct` (D9) |
| A plain function | An `.es` function that can pause | rejected, then ES4301 |

A syntactic array or object literal of programs converts element by element, so `Effect.all([job, () => fetch(id)])` needs no helper. Calling an Effect value runs it (`Clock.currentTimeMillis()`, a cached program, a `Config`): the call signatures hook (`effectValueCall`, `services.go`) gives such a callee the signature `() => Effect<A, E, R>`.

### Plain JavaScript (D9, D10, D55)

```ts
// math.es                                       // math.js
export function double(n: number) {             export function double$direct(n) { return n * 2; }
  return n * 2                                   export function double(...args) { return Effect.sync(() => double$direct(...args)); }
}
// main.es
import { double } from "./math.es"               import { double } from "./math.js";
                                                 import { double$direct } from "./math.js";
const quad = (n: number) => double(double(n))    const quad = (n) => double$direct(double$direct(n));
```

- **Eligibility.** A named function declaration compiles to plain JavaScript when it performs no Effect operation, no throw can end it, and it declares no `using` (`plain.go`). The decision is a greatest fixpoint over the declarations its calls reach (D56): each is plain when it would be with the calls inside the group counted as plain, so `isEven` and `isOdd` calling each other are plain, and so is a function that calls them.
- **Two forms, one calling convention.** A plain function without a return annotation, or with an Effect contract, compiles to `double$direct` and `double`, whose type is `(n: number) => Effect<number, never, never>` for TypeScript and in the `.d.ts` (Chapter 15). A function annotated with a plain return type, such as `function double(n: number): number`, keeps that type and compiles to one plain function.
- **Calls.** A call from an `.es` file runs inline through `$direct`, at a module's top level too: it computes a value and runs no program. Another module reaches the variant through a named or namespace import, and the lowering adds `import { double$direct } from "./math.js"` next to the original import. A reference that cannot reach it, such as a default import, uses the Effect form, which is always correct.
- **Function values.** Passed where a plain function is expected, `double` is `double$direct`; where Effect expects an Effect function, it is `double`.
- **Methods.** A method that implements a plain function member, such as `Equal.Equal`'s `[Equal.symbol]`, compiles to a plain method; if its body pauses it is ES4301 (D55).

ES4301 (`callbacks.go`) reports a lambda in plain form that pauses, an `.es` function value that cannot be plain, and such a method. The candidate that rejected them is TypeScript's argument error with no TypeScript message, so ES4301 is the only diagnostic.

### Callbacks without contracts (D12)

```ts
// mapAll.es                                      // mapAll.d.ts
function mapAll<A, B>(                           export declare function mapAll<A, B, E, R>(
  items: ReadonlyArray<A>,                         items: ReadonlyArray<A>,
  f: (a: A) => B) { … }                            f: (a: A) => Effect<B, E, R>): Effect<B[], E, R>;
```

A parameter whose annotation is a function type returning something other than an Effect accepts any contract (`callbackcontracts.go`). The type parameters hook adds `E` and `R` to the function's signature and to the outer type parameters of what it declares, and the parameter type hook gives the parameter the type `(a: A) => Effect<B, E, R>`, so a call to the callback runs it, and each call of the function infers `E` and `R` from its argument. A lambda passed as an argument keeps the types its call gives it. A declaration transformer hook writes the type parameters and the contracts into the `.d.ts`, before the prelude transform imports `Effect`. A call to a generic `.es` function, such as `mapAll`, adds the callee's E and R as the call instantiates them.

### Commits

| Change | Subject |
| --- | --- |
| `oozozxtw` | checker: add overload candidate hooks |
| `vvpuorns` | checker: add a function expression type hook |
| `kptnxtrk` | checker: add a contextual return type hook |
| `smmzorwv` | effectscript: decide a lambda's form per overload candidate |
| `rokrkspy` | effectscript: compile a lambda where an Effect is expected as a program |
| `rnpxuunq` | checker: add an expression type hook |
| `vkznolxv` | effectscript: convert function values where Effects are expected |
| `xkssspqy` | effectscript: add a case for calling Effect values |
| `xuttxlwl` | effectscript: report ES4301 for callbacks that can pause |
| `kmonzlpz` | effectscript: decide plain functions as a greatest fixpoint |
| `nmlrmkpo` | effectscript: give plain functions a $direct variant and an Effect form |
| `tlvxnnyq` | effectscript: let pipe take the program first |
| `okvzuusr` | effectscript: hand ch13-plain and ref4-callbacks to the lanes they wait on |
| `pylwouuv` | effectscript: report ES4301 for a method that implements a plain member |
| `nwnutlmz` | effectscript: read a generic .es callee's E and R from the call |
| `uuwrwppk` | checker: add type parameter and parameter type hooks |
| `tmmqumtk` | effectscript: let a callback parameter without a contract accept any contract |
| `ztwuloyu` | effectscript: keep a lambda that must return a Config plain |
| `wpukknuy` | effectscript: look ahead in pipe without checking the first stage |
| `lmsyqkzu` | checker: export the iterated type of an iterable |
| `lzqulkkl` | effectscript: convert any iterable of programs for Effect.all |
| `tsvpvvxs` | effectscript: let a callback that may return an Effect take either form |
| `pqtovvuk` | effectscript: name the collection and callee in ES4301 notes correctly |
| `nzkrouwn` | checker: add an annotated return inference hook |
| `nzplpwps` | effectscript: type a plain lambda with a contract as returning A |
| `uqxznpul` | effectscript: document the F2 follow-up fixes in EFFECTSCRIPT.md |

## Diagnostics

ES diagnostics are TypeScript diagnostics whose source is `ES`. `tsc` prints them as `main.es(7,13): error ES1003: …`, with each fix line indented under the headline, and the language server publishes them with `source: "ES"` and the numeric code, which VS Code shows as `ES(1003)`. The checker reports them while it checks a file, so `tsc`, `tsc --noEmit`, the language server, and `tsc --api` see the same diagnostics. `@ts-ignore` and `@ts-expect-error` do not apply to them, so an `@ts-expect-error` above one is itself reported as unused (TS2578).

`tsc/internal/effectscript/diagnostics.go` is the registry: one `Message` for each code in Chapter 28, `ES1001` through `ES4403`, with its default category and the headline the spec prints. A Go test keeps the registry equal to Chapter 28's table. A check is a function in its own file:

```go
func init() { fileChecks = append(fileChecks, checkContracts) }

func checkContracts(c *checker.Checker, file *ast.SourceFile) {
	d := report(c, ret, ES2001, "UserNotFound", "greeting", "its contract says E = never")
	note(d, "Catch it inside greeting, or change the contract to {0}.", contract)
	cover(c, ret) // ES2001 explains TypeScript's TS2322 at this return
}
```

| Name | What it does |
| --- | --- |
| `fileChecks` | Functions that run once for each `.es` file a checker checks, after TypeScript's own checks of that file |
| `report(c, node, m, args...)` | Adds a diagnostic with message `m` at `node`'s error span, in the category the project gives `m`'s code, and returns it; returns nil when the project turns the code off |
| `reportAt(c, file, loc, m, args...)` | `report` at a span that is not a node's error span, such as one keyword |
| `note(d, text, args...)` | Adds an indented line, such as a fix, under `d` |
| `cover(c, node)` | Hides TypeScript's own diagnostics inside `node`, because an ES diagnostic there explains them |
| `m.With(text)` | A second headline for `m`'s code, for a situation the registry's text does not fit |

Report only from a file check, and only at nodes of the file being checked: the program reads each file's diagnostics from the checker that checks that file. A check that finds a problem while the checker types another file records it and reports it from the file check of the file that holds the node.

The checks so far:

| Code | Reported at | File |
| --- | --- | --- |
| ES1001 | The outermost call at a module's top level that runs a program; covers TypeScript's TS2339 and TS2348 for a service member call and an Effect value call there | `runners.go` |
| ES1002 | The API, such as `Effect.gen`, when a `function*` is passed where a function that returns a lib `Generator` or `Iterator` is expected; covers the generator | `generators.go` |
| ES1003 | `async`, `await`, `for await`, and `await using` in a `.es` file; covers TypeScript's errors about them, such as an `await` outside an `async` function | `syntax.go` |
| ES1004 | `using` that no function encloses, including in a class static block | `syntax.go` |
| ES1005 | The callee of a runner in code a fiber runs | `runners.go` |
| ES2001 | The call or `throw` that adds an expected failure the function's contract does not allow | `contracts.go` |
| ES2002 | The call that adds a requirement the function's contract does not allow | `contracts.go` |
| ES2003 | A return annotation that leaves a contract channel out, or a plain return annotation on a function whose body runs an Effect | `contracts.go` |
| ES2004 | The `catch` keyword, when the `try` block can raise no expected failure (D37); covers TypeScript's errors about the `never` caught value | `failures.go` |
| ES2006 | A `return`, `break`, or `continue` that leaves a `finally` block, which in JavaScript would discard the failure in flight | `regions.go` |
| ES2102 | The name of an Effect function on a call cycle whose A TypeScript cannot infer because it depends on the function's own result, covering TS7022, TS7023, and TS7024 there; a `catch` variable, or a function, on a cycle that the checker typed before the cycle's E and R were known | `recursion.go` |
| ES4301 | A lambda that can pause where a plain function is expected, naming the first call that pauses and the function that calls it; an `.es` function passed by name that can pause; a method that implements a plain function member and pauses (D55). The fix names `Effect.forEach` | `callbacks.go` |
| ES4401 | The callee of a call to a function without a trust level, in code a fiber runs and outside a boundary | `trust.go` |
| ES4402 | Where code runs a platform API (D108): the callee of a call, `new`, or tagged template, or the property read, whose root is a binding imported from a platform module or a platform global; inside boundaries and at the top level too. It covers TypeScript's "cannot find name" for an unresolved platform global it reports | `platform.go` |
| ES4403 | A clock or randomness read, everywhere in a `.es` file | `platform.go` |

`compilerOptions.effectscript` (D34) sets the category of the two codes that are policy, not soundness, and the tracing mode:

```json
"effectscript": { "diagnostics": { "ES2004": "off", "ES4403": "warning" }, "tracing": "exported" }
```

ES2004 defaults to `"warning"` and ES4403 to `"error"`; every other ES code is always an error. An unknown key or invalid value is reported at its node in `tsconfig.json` with TypeScript's own messages (TS5023, TS6046). `CompilerOptions.EffectScript.Tracing` selects the functions that compile with spans (see "Tracing"). `tsc --showConfig` prints the block; `.tsbuildinfo` does not record it yet, so changing it does not invalidate an incremental build.

## Services and control operations

An abstract class in an `.es` file whose instance members are all abstract (methods and `abstract readonly` values), with no type parameters, no `implements` clause, and no `extends` clause, declares a service (D1). With `extends Control` instead, it declares a control operation (D2). Static members are allowed. Any other class is ordinary. The code is `services.go` (what the checker sees) and `services_lower.go` (what the lowering emits).

**What the checker sees.** The class means what this TypeScript declares (D45):

```ts
interface Users {
  find(id: string): Effect<User, UserNotFound, never>
  readonly "~esc/service"?: "app/users#Users"
}
class Users extends Context.Service<Users, Users> {}
```

The class base hook (`classBases`) gives the class no instance base other than the optional brand, and the static side of `Context.Service<Users, Users>` as its base constructor type. As a value, `Users` is a Context key: `Layer.succeed(Users, …)`, `Effect.provideService`, and `yield* Users` in TypeScript work. As a type, `Users` is the shape: `implements Users` and a parameter typed `Users` mean the members. The brand holds the key, so two services with the same members are different types: providing `Primary` leaves `Replica` in R, and a `Replica` implementation is not a `Primary`. The checker finds `Context.Service` in effect's own declarations (`effect/dist/Context.d.ts`), so a program that does not include effect keeps the class ordinary.

**Keys.** The key joins the nearest `package.json`'s `name` (its directory's name when it has none), the module path within that package without `.es`, and the class name: `@acme/app/src/users#Users`. A case directory without a name gives `es-services/main#Users`.

**What the lowering emits.** The class extends the runtime's key class, and TypeScript's own transform erases the abstract members:

```ts
// main.es                                       // main.js
abstract class Users {                           class Users extends runtime.$service("app/users#Users") {
  abstract find(id: string): Effect<User, …>       static layer = Layer.succeed(Users, …);
  static readonly layer = Layer.succeed(Users, …) }
}
```

A service with `static readonly default` extends `runtime.$reference(key, () => Flags.default)`, a `Context.Reference` (D40); its static side is `Context.Service<never, Flags>`, so using it adds nothing to R. A control operation extends `runtime.$service(key)` and then registers one kernel operation per abstract method, `runtime.$control(Approval, ["approve"])`; the runtime's `handle` provides an implementation whose methods perform those operations. Generated code imports `@effectscript/runtime` as a namespace under a free name, as it imports `effect/Effect`.

**Declaration files.** The declaration transform writes a service class as the abstract class it is in source, which TypeScript code could only extend. A declaration transformer hook (`services_declarations.go`) rewrites it to what the checker sees: the instance members move into an interface with the brand, a `Users_base` constant gets the runtime's key type, and the class extends it with only its static members:

```ts
export interface Users {
    find(id: string): Effect.Effect<User, UserNotFound, never>;
    readonly "~esc/service"?: "app/users#Users";
}
declare const Users_base: import("@effectscript/runtime").ServiceKey<Users>;
export declare abstract class Users extends Users_base {
    static readonly layer: Layer.Layer<Users, never, never>;
}
```

`ServiceKey<I>` is `(abstract new () => {}) & Context.Service<I, I>`, the type of `$service`; a service with a static default extends `ReferenceKey<I>`, the type of `$reference`, a `Context.Reference`. A default-exported service declares the interface and the class locally, because merged declarations must share their export status, and ends with `export default Users`.

**Member calls (D41).** In an `.es` function body, a call `S.member(…)` where `S` is a Context key whose service declares `member` looks the service up, then applies the call rule to the member. The member receiver hook (`memberReceiver`) makes the checker read `member` from the service's shape, so a member the service declares wins over the key's own `pipe`, `of`, and `use`. The rule is structural: it covers `.es` services, `Context.Service` classes written in TypeScript, and Effect's own services such as `FileSystem` and `Path`.

| Source | Generated | R gains |
| --- | --- | --- |
| `Users.find(id)`, a member that returns an Effect | `yield* (yield* Users).find(id)` | `Users`, and the member's R |
| `Path.join(a, b)`, a member that returns a plain value | `(yield* Path).join(a, b)` | `Path` |
| `UserRepo.all()`, a member that is an Effect value | `yield* (yield* UserRepo).all` | `UserRepo`, and the member's R |
| `Users()`, the service itself | `yield* Users` | `Users` |
| `FeatureFlags()`, a service with a default | `yield* FeatureFlags` | nothing |

The last three rows are one rule from Chapter 13: in an `.es` function body, a value whose type is an Effect, called with no arguments, runs (D24). That covers a Context key or a `Config`, whose variance the `Effect` interface declares; an `Exit`, a union of two Effects, which `outcome()` replays; and a tagged error instance, whose class declares the variance itself, which `error()` fails with. The call signatures hook (`effectValueCall`) gives such a callee the signature `() => Effect<A, E, R>`, built from effect's own `Effect` interface when the callee's type does not name it, and the call expression type hook then types the call as `A`, so `Settings().baseUrl` checks. A lookup makes the enclosing function an Effect even when the member returns a plain value, and `effectOf` adds the service to R.

A lookup also keeps a lambda from compiling to plain JavaScript: `canBePlain` asks `performs`, which is `runs` or a lookup, so `(part) => Path.join(root, part)` is an Effect function.

**Methods.** An object literal method or a class method in an `.es` file becomes an Effect when its body runs a call, by the rule that decides functions, or when it is expected to return one (`expectsEffect`): its object literal's contextual type, or the member it implements through its class's `implements` clause, is a function that returns an Effect. So `class UsersFake implements Users { find(id) { return { … } } }` is an implementation even though its body runs nothing. Its inferred return type is `Effect<A, E, R>`. It keeps its name and parameters, and its body runs as a generator bound to the method's `this` (`methods.go`):

```ts
find(id) { return Effect.gen({ self: this }, function* () { … }) }
```

A method's expectation is read once, from the contextual type the checker has when it first asks, whereas a lambda's form is decided per overload candidate (D56).
### Checks of service classes

`services_check.go` reports misuse of service and control classes:

| Code | Reported at | What it reports |
| --- | --- | --- |
| ES2101 | The return type of a service or control method, or its name | A method without a full contract: the annotation must be an `Effect` with all three channels written, such as `Effect<number, never, never>`, or an alias of one. `Effect<A>` leaves channels to defaults, so it is not one. An annotation TypeScript cannot resolve is left to TypeScript's error |
| ES3001 | The class name | Two service or control classes anywhere in the program share a key; a note names the other |
| ES3002 | The `extends` expression | A class extends a service; covers TypeScript's errors about the inherited abstract members |
| ES3003 | The `new` expression | `new Users()`; covers TS2511 |
| ES3004 | The `instanceof` expression | `x instanceof Users` |
| ES3005 | The class name | A fully abstract generic class, which no one key can represent |
| ES3006 | The static member's name | A static member with a service member's name, which `S.member(…)` could not reach |

### Providers

`providers_check.go` compares each implementation member with the service member it implements: every method and function-valued property of an object literal with the type its contextual type gives that member, and every method of a class with the member of the same name in its `implements` clause. When both are functions that return Effects, it reports a failure the contract does not allow (ES2001, at the member) and a requirement the contract does not declare (ES3101, at the call that looks the service up per call), and covers TypeScript's TS2322, TS2345, or TS2416 for the same mismatch at its error spans: the member's name, an arrow member's expression body, and the header of each function that encloses the object and is a call argument. A contract whose E or R holds a type parameter is not checked.

A provider is usually a program that returns an object literal whose members have untyped parameters, typed by the service, as in `Layer.effect(Users, () => { const db = Database(); return { find(id) { … } } })`. Such a lambda is context sensitive, so the first pass of overload resolution skips it and TypeScript types it as `anyFunctionType`, which an Effect parameter does not accept: the candidate failed before the second pass inferred its type arguments, and E and R came out `unknown` without an error. The skipped function type hook (`skipped.go`) gives a lambda with no required parameters, where only an Effect is expected, the non-inferrable `any` type instead; the parameter accepts it and inference ignores it, so the second pass infers E and R from the program.

### wire

`wire(…)` is the runtime's `wire`, imported or from the prelude (D27). `wire.go` reads each argument's `Layer<Out, E, In>` from its `~effect/Layer` variance property and builds the provider graph: an argument needs another when one of its inputs is one of the other's outputs. The runtime folds the arguments with `Layer.provideMerge` from the last to the first, so the lowering emits them consumers first, a topological order that keeps source order among independent arguments; the arguments can be written in any order.

| Code | Reported at | What it reports |
| --- | --- | --- |
| ES3202 | The later of two arguments | Two arguments provide the same service |
| ES3203 | The `wire` call | The arguments form a cycle, printed as `CacheLive → Database → DatabaseLive → Cache → CacheLive` |
| ES3204 | The argument | Its type does not name its outputs: `Out` is `any` or `unknown`, as in `Layer.Any` |
| ES3201 | The runner's program | A runner whose program starts with an empty Context (`isRootRunner` in `runners.go`: Effect's `run*` functions, but not their `*With` forms, and `NodeRuntime.runMain` and `BunRuntime.runMain`) runs a program whose R holds a service that a `wire` argument anywhere in the program needs and no argument of that `wire` provides; notes name the arguments that need it, and it covers TS2345 |

An input that no argument provides stays in the result's `In`, as with `Layer.provide`, so wiring part of a graph is not an error by itself.

## Failures and regions

`try`, `catch`, and `finally` in an Effect function compile to Effect's failure and cleanup frames, never to a JavaScript `try`, because Effect does not run a generator's JavaScript `finally` when a fiber fails or is interrupted (design chapter 26).

### Types

- **`throw`.** `throw e` in an Effect function lowers to `return yield* Effect.fail(e)` (D46); the `return` keeps TypeScript's narrowing after the throw in the generated code. The static type of `e` joins E, widened as TypeScript widens a return type, so `throw "bad"` adds `string` while a value declared with a literal type keeps it. A function is an Effect when a throw can end it: one that no `try` block of the function with a `catch` encloses (`hasEscapingThrow`).
- **E through a `try`.** A `catch` handles every expected failure of its `try` block (D13, D28), so what escapes `try { A } catch { B }` is what `B` raises. Requirements are never handled: R is the union of every block's. A `finally` block adds its own failures and requirements (D15). `failures.go` computes this from the syntax tree and the checker's types (`flowOf`, `tryFlow`).
- **The caught value.** In a function the lowering can turn into an Effect, the variable of a `catch` without a type annotation has the union of the `try` block's expected failures, `never` when there are none, so `instanceof` and `_tag` narrow it. The checker asks `catchVariableType` through the catch clause variable type hook. A `catch` at a module's top level keeps TypeScript's `unknown`.

### Regions

A region is a block that runs as an Effect of its own, so a runtime helper can wrap it: today the `try`, `catch`, and `finally` blocks of a `try` statement. Each compiles to a generator closure over the function's variables, wrapped in `Effect.fnUntraced` and named `<function>$try`, `$catch`, or `$finally` (D36):

```ts
// lookup.es                                 // lookup.js
function lookup(id: number) {                const lookup = Effect.fnUntraced(function* (id) {
  try {                                          const completion = yield* runtime.$finally(runtime.$catch(
    return find(id)                                Effect.fnUntraced(function* lookup$try() {
  } catch (e) {                                      return { kind: "return", value: yield* find(id) };
    return `guest (${e._tag})`                     })(),
  } finally {                                      Effect.fnUntraced(function* lookup$catch(e) {
    events.push("done")                              return { kind: "return", value: `guest (${e._tag})` };
  }                                                })),
}                                                  Effect.fnUntraced(function* lookup$finally() {
                                                     events.push("done");
                                                   })());
                                                 if (completion.kind === "return")
                                                   return completion.value;
                                             });
```

`$catch` is the kernel's `catchExpected` and `$finally` is `Effect.onExit`, from `@effectscript/runtime`, which the file imports as `* as runtime` under a free name. A `return`, `break`, or `continue` that leaves a region cannot jump out of its closure, so the closure returns a completion record: `{ kind: "return", value }`, `{ kind: "break", label }`, or `{ kind: "continue", label }`, with `""` as the label of an unlabeled jump, and the runtime's `$normal` when it reaches its end. The generator that runs the region dispatches each record that can arrive: it returns, breaks, or continues itself, or, when it is a region's closure too and the jump leaves it as well, returns the record to its own runner. A region that no jump leaves runs as `yield* …;` with no record. A closure whose block refers to `this` runs with the function's `this`. Control cannot leave a `finally` block (ES2006): in JavaScript a jump there replaces the block's exit and discards a failure in flight, and EffectScript never drops a failure silently (D15). A `var` in a region belongs to the whole function, so the lowering declares it in the Effect function and assigns it where the source declared it.

### Regions in plain functions

A function that compiles to plain JavaScript cannot pause, so its `try`, `catch`, and `finally` stay JavaScript's (D54). Each of its throws is caught by one of its own `catch` clauses, or a throw could end it and it would be an Effect. A throw there marks the thrown value as an expected failure, and each `catch` unwraps it or rethrows anything else, a defect, so D13 holds:

```ts
// clamp.es                                  // clamp.js
function clamp(n: number) {                  function clamp(n) {
  try {                                          try {
    if (n > 10) throw new Invalid({ n })             if (n > 10) throw runtime.$expected(new Invalid({ n }));
    return n                                         return n;
  } catch (e) {                                  } catch (e) {
    return -10                                       e = runtime.$caught(e);
  }                                                  return -10;
}                                                }
                                             }
```

`regions.go` holds the machinery as an API for every region kind: Scope regions ([Scope regions: using and fork](#scope-regions-using-and-fork)) use it, and so will loops over producers (F7):

| Piece | Role |
| --- | --- |
| `region` | Collects the jumps (`jump{kind, label}`) that leave a region while its closures are emitted |
| `frame` | One generator being emitted, the function's body or a region's closure, with the loops, switches, and labels open in it; `frame.lands(j)` says whether a jump stays inside |
| `regionStatements(r, block)` | Visits a block as the body of a closure of region `r` |
| `regionFunction(r, role, binding, block, statements)` | Makes `Effect.fnUntraced(function* f$role(binding) { … })`, appending `return $normal` when a jump leaves `r` and the block can reach its end |
| `runRegion(original, run, r)` | Emits `const completion = yield* run` and one dispatch per jump, or `yield* run;` when nothing leaves |
| `runtimeMember`, `runtimeCall` | Reference `@effectscript/runtime` helpers, importing the package on first use |

To add a region kind: create a `region`, emit each block with `regionStatements` and `regionFunction`, compose the closures with runtime helpers, and pass the result to `runRegion`. A consumer that handles some jumps itself, as a producer loop handles its own `break` and `continue`, filters `r.jumps` before dispatching.
## Contracts

The return type annotation of an `.es` function with a body is the function's contract, `Effect<A, E, R>`: the whole computation, as `Promise<string>` is the whole computation of an async function. `contracts.go` implements the checks of design chapter 14:

| Rule | How |
| --- | --- |
| A function with a contract is an Effect, whatever its body runs | `hasContract` in `analyzer.isEffect`; a lambda's form stays the lambda's position's to decide (D56), so a lambda with a contract in a plain position is plain and returns A (D113) |
| What it returns is checked against A | The unwrapped return type hook gives `checkReturnStatement` and an arrow function's expression body the A of the signature's return type, where TypeScript would check against the whole Effect; a mismatch is TypeScript's own error, such as TS2322. F2's contextual return type hook types the returned expressions by A |
| An expected failure that escapes the body must be in E | ES2001 at the call or `throw` that adds it, naming the failure and the contract, with a fix that widens E. `effectInfo` keeps, for each failure and requirement, the call or `throw` in the function's own body that adds it |
| A requirement must be in R | ES2002 at the call that needs it |
| A channel written `infer Name` comes from the body (D57) | The annotated return type hook completes the signature's return type: A as TypeScript infers a return type (`GetReturnTypeFromBody`), E and R from `effectOf`. The infer type hook keeps `checkInferType` from reporting TS1338 and the unused name; the binder already declares such a name in no scope |
| The declaration file carries the full contract | An annotation with an `infer` channel is not copied: the declaration type annotation hook makes the emitter serialize the signature's return type. A full contract is copied, and the prelude's declaration-file imports make its `Effect` portable |
| A contract that leaves a channel out is ES2003 | `Effect<User>` and `Effect<User, UserNotFound>`, with both fixes: the missing channels as `never`, or as `infer` |
| A plain return annotation keeps its type on a function that compiles to plain JavaScript (D67) | On a function whose body runs an Effect, throws past its own `catch`, or makes a trusted call whose failures escape, it is ES2003. Methods are exempt: a method that implements a plain-function member is declared with a plain type (D55) |
| A caller sees the contract, not the body | `addCall` reads a callee's E and R from its contract when the contract infers neither; a callee whose contract infers a channel is read through its body, which never asks for a return type that a call cycle may be computing |

```text
main.es(14,20): error ES2001: UserNotFound can escape greeting, but its contract says E = never.
  Catch it inside greeting, or change the contract to Effect<string, UserNotFound, never>.
main.es(15,3): error ES2002: welcome needs Mailer, but its contract says R = Users.
  Provide Mailer inside welcome, or add it: Effect<string, MailError, Mailer | Users>.
main.es(6,35): error ES2003: Effect<string> leaves contract channels unspecified. Did you mean never or inferred?
  Write Effect<string, never, never> to check the missing channels as never.
  Or write Effect<string, infer E, infer R> to infer them from the body.
```

### Recursion (D52)

E and R of a function on a call cycle are a fixpoint over the cycle's strongly connected component, computed inside the component only (`recursion.go`). `effectOf` solves a function's component with Tarjan's algorithm over the call graph whose edges are the `.es` callees a body reads (`bodyCallees`: a callee whose contract states E and R, or a generic callee the call instantiates, is no edge). A component is solved after every component it calls. Its members start with empty E and R and are read again, with the E and R found so far, until none grows; the rows are finite and only grow, so the iteration ends. While a component is being solved, `effectOf` returns a member's approximation, and `tryBlockFlow` reads a member's `try` blocks again in each round. In `recursion`, `isOdd` and `isEven` each learn the other's failure only through the cycle, and both get `Effect<boolean, Even | Odd, never>`.

A stays TypeScript's to infer. When it depends on the function's own result, TypeScript reports TS7023 and types the function `any`; on an Effect function that is ES2102 instead, at the function's name, whose fix writes the contract with its success type and `infer` for E and R. A pure recursive function compiles to plain JavaScript, so TS7023 and a plain annotation stay TypeScript's (D67).

The checker caches types, so one it computes while a component is being solved cannot change afterwards. Two such types are checked once the component is solved, and a mismatch is ES2102, because a written contract on the cycle breaks the dependency: a `catch` variable whose `try` block calls into the cycle, and a member's own return type. `es2102-cycle-catch.fail` shows the first; writing the contract of the function the `try` block calls makes it compile.

A provider's members are checked against the service they implement by `providers_check.go`, which reports ES2001 and ES3101 at the member; a member with its own contract is also checked against that contract here. The two checks compare different contracts, so one mismatch is reported once.

## Scope regions: using and fork

A block of an Effect function that declares `using` or calls `fork` is a Scope region (D14, D19, D47). It runs under a Scope of its own, which only its `using` acquisitions and the fibers it forks receive, and which closes when the block exits by any path: success, failure, or interruption. Closing runs the releases and interrupts the forked fibers in reverse order, and waits for each. `scopes.go` decides what the checker sees, and `scopes_lower.go` emits the regions:

```ts
// serve.es
function serve(port: number) {
  using server = Http.listen(port)    // Effect<Server, never, Scope>
  fork(() => heartbeat())             // stopped before the server closes
  return server.awaitShutdown()
}
// Effect<string, never, Http>

// serve.js
const serve = Effect.fnUntraced(function* (port) {
  return yield* runtime.$region(Effect.fnUntraced(function* serve$using(scope) {
    const server = yield* runtime.$using(scope, (yield* Http).listen(port));
    yield* runtime.$fork(Effect.gen(function* () { return yield* heartbeat(); }), scope);
    return yield* server.awaitShutdown();
  }));
});
```

`$region` is `Effect.scopedWith`: it makes the Scope, does not add it to the Context, and closes it with the closure's exit. Everything else in the block runs under the Scope its caller supplies, so a resource acquired without `using` belongs to the caller, and `Scope` stays in R (D14).

### Regions

| Block | Lowers to |
| --- | --- |
| A function's body | `return yield* runtime.$region(…)`: a return from the closure is a return from the function, so it needs no completion record |
| Any other block | `const completion = yield* runtime.$region(…)` and its dispatch, as [Regions](#regions) describes, inside a block so that it stays one statement |
| A `try`, `catch`, or `finally` block | A Scope region inside that region's closure (`regionStatements`), so a release runs before the `catch` |
| `for (using x of values)` | A Scope region around each iteration's body that first takes the value with `using`; the body's own `using` and `fork` join it |

A closure is named `<function>$using` (D36), for a block that only forks too, and runs with the function's `this` when its block refers to `this`.

### using

`using x = init` decides by its initializer (`isAcquisition`):

- **An acquisition**, a call that runs an Effect, lowers to `const x = yield* runtime.$using(scope, f(a))`: the call's Effect, not run, runs with the region's Scope, so `Effect.acquireRelease` registers its release there. The using initializer hook (`usingInitializer`) tells the checker not to require a Disposable (TS2850), and `x` has the Effect's success type.
- **A value**, any other initializer, lowers to `runtime.$using(scope, Effect.succeed(v))`. TypeScript checks that it is a Disposable, `null`, or `undefined`.

`$using` releases a Disposable value of either kind with `[Symbol.dispose]()` when the region closes, and registers that release with interruption masked. It adds no masking to the acquisition itself (D25): `acquireRelease` masks acquisition by default, so a timeout waits for it and then the release runs, and `{ interruptible: true }` lets cancellation reach it.

A function that declares `using` compiles to frames (D54), even when it runs nothing else, so it never compiles to plain JavaScript. A lambda with `using` passed where a plain function is expected, such as to `Array.prototype.map` or `Effect.sync`, is ES4301: `this callback can pause (using t)`.

### fork

`fork(program, options)` in a Scope region lowers to `yield* runtime.$fork(program, scope, options)`, which is `Effect.forkIn`: the region interrupts the fiber when it closes and waits for its cleanup, so a fiber forked after a resource was acquired stops before the resource is released. `forkOwner` finds the block that owns a call: the call names the runtime's `fork` (`isRuntimeExport`), runs, as `fork(program)` does, and sits in a block of an Effect function's body. A `fork` that no block encloses, in an arrow function whose body is an expression, stays the prelude's `fork`, which is `Effect.forkScoped`: the caller's Scope owns the fiber, and `Scope` joins R. `Effect.forkScoped`, `Effect.forkChild`, and `Effect.forkDetach` keep Effect's meaning, and `Fiber.join` waits for a fiber and fails with its failure.

A block-owned fiber cannot outlive its block: `function start() { return fork(() => work()) }` returns a fiber that is interrupted as `start` returns. Code that hands a fiber to its caller uses `Effect.forkScoped` or `Effect.forkDetach`.

### Types

`flowOf` asks `getsScope` about each call, and `addScoped` adds what a call that receives a region's Scope contributes:

| Call | Adds to E | Adds to R |
| --- | --- | --- |
| A `using` acquisition | The acquisition's E | The acquisition's R without `Scope`, and the service its callee looks up |
| A `fork` its block owns | Nothing: the fiber's failures reach the parent through `Fiber.join` | The program's R, including a `Scope` that the program needs itself |
| Any other call | Its E | Its R, so an acquisition without `using` keeps `Scope` |

### One copy of effect

The runtime and the program must load the same copy of `effect`. Each copy numbers its fibers from 1, and `Effect.forkIn` registers a finalizer that skips the interruption when the fiber closing the Scope has the forked fiber's id, so with two copies a Scope can fail to stop a fiber. `smoke.sh` links the runtime's `effect` to the cases' copy; a project gets one copy because the runtime declares `effect` as a peer dependency.

### Commits

| Change | Subject |
| --- | --- |
| `utpqokps` | effectscript: count only using declarations as using |
| `ssqqsmwt` | checker: add a using initializer hook |
| `rmkrouvo` | runtime: register a Disposable's release with interruption masked |
| `xvsrqxyz` | effectscript: lower using to Scope regions |
| `twvqsuom` | effectscript: give each fork to the block that calls it |
| `nzuukwkz` | effectscript: take each for...of value with using |
| `qsytkrtw` | effectscript: document Scope regions, using, and fork in EFFECTSCRIPT.md |
| `rkxpuynr` | effectscript: keep this in a lowered arrow function |

## The prelude

Every `.es` file sees these names without an import (D4, D48): `Effect`, `Schema`, `Layer`, `TaggedError`, `Control`, `Scope`, `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, and `abandon`. They are the exports of `@effectscript/runtime`, whose `prelude.ts` gives `Effect` and `Scope` two meanings each: the effect module, for `Effect.gen` and `Scope.make`, and a type, so `Effect<A, E, R>` means `Effect.Effect<A, E, R>` and `Scope` means `Scope.Scope`.

| Step | How | Where |
| --- | --- | --- |
| Load the runtime | A `.es` file with an identifier that has a prelude name, and no top-level declaration or import of that name, resolves `@effectscript/runtime` as an implicit import, as a file with `importHelpers` resolves `tslib`. A file that uses no prelude name compiles exactly as TypeScript would | `implicitRuntimeImport`, through `compiler.RegisterImplicitImportHook` |
| Resolve a name | When no scope declares a prelude name, the checker resolves it to the runtime's export before it searches the global scope. A local declaration or import therefore shadows it, and `.ts` files never see it | `resolvePreludeName`, through `checker.RegisterNameResolutionHook` |
| Import it | The checker records each prelude name a file uses as a value outside a type. The lowering adds `import * as Effect from "effect/Effect"` for the effect modules (`Effect`, `Layer`, `Schema`, `Scope`) and one `import { TaggedError, pipe, … } from "@effectscript/runtime"` for the rest | `preludeUses`, `preludeImports`, `lowering.visitSourceFile` |

A name used only in types, such as `Scope` in `Effect<void, never, Scope>`, is not imported. The emitted imports are ES module imports: `.es` files compile to ES modules. When `@effectscript/runtime` is not installed, a prelude name is TypeScript's TS2304 (cannot find name).

A declaration file is ordinary TypeScript, so the `.d.ts` of a `.es` file imports the prelude names its declarations write: `import type { Effect, Scope } from "@effectscript/runtime"`. Declaration emit asks the entity name visibility hook whether each name it writes is visible; `preludeNameVisible` says yes for a prelude name and records it, and `transformDeclarations`, a declaration transformer hook, adds the import (`declarations.go`, case `es-prelude-dts`).

## Imports between files

A `.es` file imports another module by its source name, and the emitted JavaScript imports the output (D31):

| Source | Emitted JavaScript | Resolves to |
| --- | --- | --- |
| `import { welcome } from "./welcome.es"` in `.es` | `from "./welcome.js"` | `welcome.es` |
| `import { formatName } from "./format.ts"` in `.es` | `from "./format.js"` | `format.ts` |
| `import { welcome } from "./welcome.js"` in `.ts` | unchanged | `welcome.es`, as `./format.js` resolves to `format.ts` |

- **Resolution.** The module resolver tries `welcome.es` for `./welcome.es`, and after `.ts` and `.tsx` for `./welcome.js` and an extensionless specifier, so `.ts` beats `.es` when both exist. An `.es` resolution is always allowed, like a `.ts` one (`module/resolver.go`, `module/util.go`).
- **Extensions in specifiers.** A `.es` file may name a `.ts` or `.es` source without `allowImportingTsExtensions` (no TS5097), because its emitted JavaScript never keeps the source name (`core.AllowImportingTsExtensionsFrom`). A `.ts` file follows TypeScript's rules.
- **Emit.** The lowering rewrites the specifier of every import and export declaration in a `.es` file that names a TypeScript or EffectScript source, with or without `rewriteRelativeImportExtensions` (`imports.go`). The `.d.ts` of a `.es` file names outputs too, including in the `import("./users.js")` types the checker writes for inferred types, so stock TypeScript can read it (`declarations.go`); and `./users.es` resolves to `users.d.ts` when only the declaration file exists, as `./users.ts` does. Dynamic `import()` is not rewritten: it returns a Promise, which `.es` code does not await.

Case `es-imports` covers an `.es` file calling an `.es` Effect function and a pure helper, and a `.ts` file calling an `.es` function that `.es` code then runs; `es-imports-norewrite` covers a project without `rewriteRelativeImportExtensions`.

## The runtime

Generated JavaScript imports only `effect` and `@effectscript/runtime` (D30): the prelude names it uses, `effect/Effect` for `Effect.fnUntraced` under a free name (`Effect`, else `Effect_1`, …), and the helpers as one namespace under a free name (`runtime`, else `runtime_1`, …), called as `runtime.$catch(…)`. The runtime's exports are the ABI between compiler and runtime, so a compiler change that needs a new helper adds it to `packages/runtime` in the same commit; `smoke.sh` builds the runtime from source and every case links it, so the commit is tested as one. Case `es-runtime-name` declares `runtime` and `Effect_1` itself and still runs.

## Code that is not Effect

A call to a function that returns no `Effect` needs a trust level (D5, D22, D23). `trust.go` classifies the callee by the declaration its signature resolves to:

| Callee | Trust | A bare call |
| --- | --- | --- |
| An `.es` function, or a parameter of an `.es` function | EffectScript's own | Allowed |
| A declaration in TypeScript's lib files: `String`, `Array`, `Math`, `JSON`, `Map`, `RegExp`, `URL`, … | A built-in with a shipped contract (D5) | Allowed; the contract's exceptions join E |
| An export of `effect`, `@effect/*`, or `@effectscript/runtime`, and the functions and values those return | Trusted `sync` (D22); exceptions are defects | Allowed |
| A declaration tagged `/** @effectscript trusted */` | Trusted `sync` (D23); each `@throws {T}` joins E | Allowed |
| A declaration in a platform package (`@types/node`, `bun-types`) | Platform | ES4402 reports the import or global instead |
| Anything else | None | ES4401 |

A package's name comes from the nearest `package.json` (`packageOf` in `origins.go`), so `@effectscript/runtime` installed through a `file:` link, outside `node_modules`, is still recognized.

An Effect package may re-export another package's functions: `@effect/vitest` re-exports vitest, so `assert` and `expect` are declared in vitest's files. A call whose callee a declaration file declares is still Effect's when the callee is reached through an import from an Effect package, or through a value that a trusted call to one returned (`reachedThroughEffect`): `assert.strictEqual(n, 1)` and `expect(n).toBe(1)` are trusted calls whose exceptions are defects, so a failed assertion fails the test. The program's own TypeScript keeps its rule even when an Effect function returned the value.

**The tag.** `/** @effectscript trusted */` may sit on a function or method declaration, on any of its overloads, or on the statement of a `const` that a function initializes, which is how a library function is trusted:

```ts
/**
 * @effectscript trusted
 * @throws {SyntaxError} when the text is not JSON
 */
export const parseJson: (text: string) => unknown = JSON.parse
```

Each `@throws {T}` names a class in the declaring file's scope.

**Where ES4401 applies.** In code a fiber runs, outside a boundary. A boundary is a callback of `Effect.try`, `Effect.tryPromise`, `Effect.sync`, `Effect.promise`, or `Effect.callback` (D23, D43), known by effect's declarations, including the `try` and `catch` of an options object. Code that no fiber runs is JavaScript (D51): the module's top level, and a plain callback that the top level passes to a TypeScript function, such as a `describe` body or a framework's route handler (`outsideFibers`, the places where D38 lets a runner start). A call that returns a Promise gets ES4401's Promise text and an `Effect.tryPromise` fix.

**Expected failures.** A call whose contract lists exceptions compiles to a runtime helper that tests `instanceof` (D51):

```ts
// main.es                              // main.js
function port(text: string) {           const port = Effect.fnUntraced(function* port(text) {
  return parsePort(text)                  return yield* runtime.$trusted(() => parsePort(text), [PortError]);
}                                       });
```

The listed classes join E (`addFailures`, from `flowOf`), and a call whose failures can end a function makes it an Effect (`hasEscapingFailure`, which D10's fixpoint asks too). Inside a `try` with a `catch` in a function that compiles to plain JavaScript (D54), the helper is `runtime.$trustedPlain`, which throws a listed exception as `$expected`, so the `catch` receives it. In a callback that compiles to plain JavaScript, such as one `Array.prototype.map` calls, a trusted call stays an ordinary call and its exceptions are defects. When an operand runs an Effect, the operands are evaluated first, in order, and passed to the thunk: `((receiver, args) => () => receiver.m(...args))(yield* r(), [yield* x()])`.

The generated code imports each class from a module the `.es` file imports that exports it, usually the one it imports the trusted function from: `import { PortError } from "./parsers.ts"`. A global class such as `SyntaxError` needs no import. A class that no imported module exports is ES4401 (`reachClass`). The code is `trust_lower.go`.

The shipped contracts (`builtinContracts`) list only built-ins that parse text from outside the program: `JSON.parse` fails with `SyntaxError`, and `decodeURI` and `decodeURIComponent` with `URIError`. Every other built-in is trusted, and its exceptions are defects.

## Platform APIs and clock reads

`platform.go` reports where `.es` code runs a Node.js or browser API (ES4402), each with the Effect module that replaces it (Chapter 11's table, D6), and clock and randomness reads (ES4403).

**ES4402 fires where the code runs the API (D108), not where it names it.** A platform API is a binding imported from a Node.js built-in module, with or without `node:`, or from a `bun:` module, or a global in `platformGlobals`: `fetch`, `XMLHttpRequest`, `WebSocket`, the timers, `process` (with `process.env`, `argv`, and the standard streams named apart), `console`, `crypto`, `performance`, `localStorage`, `sessionStorage`, `indexedDB`, `Worker`, the DOM's entry points, `require`, `module`, `__dirname`, `__filename`, `Buffer`, `Bun`, and `Deno`. A global's name must resolve to the global that a lib file or a platform package declares, or not resolve at all, as `process` does without `@types/node`. The code runs the API when it calls it, constructs it with `new`, tags a template with it, or reads one of its properties, which can run a getter, directly, by destructuring, or by spreading. Importing it, re-exporting it, or passing it as a value runs nothing, so Effect's own setup compiles in `.es`:

```ts
import { createServer } from "node:http"
const Server = NodeHttpServer.layer(createServer, { port: 3000 })   // allowed: a value
export function log(message: string) { console.log(message) }       // ES4402: a call
```

A namespace import's members are values too, until the code runs one: `os.EOL` is allowed, and `os.cpus()` is ES4402. Web standards that touch nothing outside the program, such as `URL`, `TextEncoder`, `AbortController`, `structuredClone`, `Headers`, and `Response`, are not platform APIs. ES4401 leaves a call that runs a platform API to ES4402 (`isPlatformCallee`).

**ES4403.** A call that resolves to `Date.now`, `Date()`, `new Date()` without arguments, `Math.random`, `crypto.randomUUID`, `crypto.getRandomValues`, `performance.now`, or one of `Temporal.Now`'s functions is a clock or randomness read, reported with the `Clock`, `DateTime`, `Random`, or `Crypto` call that replaces it. A project that sets ES4403 to `"warning"` or `"off"` (D34) gets that, and the read compiles as a plain call.

Both apply everywhere in a `.es` file, inside boundaries (D23) and at the top level.

## Where programs start

`runners.go` holds D38's runner registry and two checks. `runnerOf` knows a runner by the declaration its call resolves to: Effect's `runPromise`, `runPromiseExit`, `runSync`, `runSyncExit`, `runFork`, and `runCallback`, and their `*With` forms; `ManagedRuntime`'s `run*` methods; `NodeRuntime.runMain` and `BunRuntime.runMain`; and `@effect/vitest`'s `it.effect` and `it.live`, their modifiers, `.each`, and `.prop`, and `layer(…)`.

| Code | Rule |
| --- | --- |
| ES1001 | At a module's top level, a call that would run a program inside a function body: a call that runs an Effect or an `.es` Effect function, a service member call, or a call of an Effect value such as `Users()` or `Port()`. Only the outermost such call is reported. A program passed straight to a runner, as in `Effect.runPromise(main())`, gets the fix `() => main()` |
| ES1005 | A runner anywhere but the module's top level and the plain callbacks that the top level passes to TypeScript functions (`outsideFibers`). The fix forks the program, `fork(() => refresh(id))`; a test runner's fix registers the test at the top level or in a `describe` callback |

```ts
NodeRuntime.runMain(() => main())          // allowed: a runner at the top level, given a program
describe("users", () => {
  it.effect("finds Ada", () => find("1"))  // allowed: a plain callback from the top level
})
export const config = loadConfig()         // ES1001
```

## Debugging

`esc --sourceMap` maps generated JavaScript to `.es` lines and columns. The lowering keeps the position of each statement and expression it moves, inside generators and region closures too, so `node --enable-source-maps` prints `.es` frames (case `debug-stack`):

```text
TypeError: Cannot read properties of undefined (reading 'name')
    at ownerName (main.es:12:25)
```

Generated functions carry their source's names (D36, `names.go`). A compiled function's generator is named after the function, or the variable or property that names it in the source, and region closures are `<function>$try`, `$catch`, and `$finally`. A named function expression binds its name inside its body, so when the body refers to that name, the generator is a method of an object literal instead, which has the name and binds nothing: `Effect.fnUntraced({ *fact(n) { … } }.fact)`. A lambda without a name stays anonymous. Clauses and producers, still to come in lane F7, can name their generators with `generatorExpression`.

## Tracing

`compilerOptions.effectscript.tracing` (D34) selects the functions that compile with spans (`tracing.go`):

| Mode | Spans |
| --- | --- |
| `"none"`, the default | None: functions compile to `Effect.fnUntraced` |
| `"exported"` | Each exported function declaration, and each exported `const` that a function initializes: `Effect.fn("greet")(function* greet(…) { … })` |
| `"all"` | Every named function that compiles to frames, and every lowered method, whose Effect runs as `Effect.withSpan(Effect.gen(…), "Users.find")` |

A span is named after its function; a class method's span names its class too. A lambda without a name, such as a callback, runs in its caller's span. Spans give the logical call stack that a JavaScript debugger cannot show (D36), through Effect's `DevTools` layer or any tracer Effect supports.

## The editor and the esc command

The binary that `tsc/cmd/tsc` builds is `esc`, EffectScript's compiler, under any file name (`identity.go`, through `tsc.SetIdentity`):

```text
$ esc --version
EffectScript 0.0.1 (TypeScript 7.1.0-dev)
$ esc --help
esc: The EffectScript Compiler - Version 0.0.1 (TypeScript 7.1.0-dev)
```

`esc --lsp --stdio` is TypeScript 7's language server running the EffectScript checker, so TypeScript's features work on `.es` files with EffectScript's types: a call that runs has its success type, and an `.es` function has its inferred contract. Hooks in `hover.go` and `completions.go` add what TypeScript cannot know:

| Request | In an `.es` file |
| --- | --- |
| Hover on a function | Its inferred contract: `function lookup(id: number): Effect.Effect<string, NotFound, never>` |
| Hover on the callee of a call that runs | The callee's contract, then "Runs here and returns `string`.", the success value the call evaluates to. It asks the analyzer the same questions as the lowering, so a call in a `.ts` file or one the lowering leaves plain shows nothing |
| Hover on a plain function or a call to it | `function double(n: number): number`, then "Plain JavaScript: EffectScript code calls it directly. TypeScript callers see `Effect<number, never, never>`." A `.ts` file keeps TypeScript's view |
| Completions | The prelude names, as names in scope that the file's own declarations and imports shadow; a file that uses none yet takes them from the runtime another `.es` file imports. After `Users.` in a function body, the service's members (D41), not the Context key's |
| Diagnostics | Pulled with `textDocument/diagnostic` for a document opened as `effectscript` or `typescript`. ES diagnostics have source `ES` and their number as the code, which VS Code shows as `ES(1003)` |

Upstream code needed four fixes for `.es` files in the server: auto-import panicked with "Extension .es is unsupported" (`module.TryGetJSExtensionForFile`), a new `.es` file on disk did not join its project (the wildcard globs in `tsoptions`), renaming an `.es` file did not update its importers (the `willRenameFiles` filter), and the server exited with status 1 after `shutdown` and `exit`, which Neovim reports on every restart. Per-checker state lives on the checker (`ExtensionState`), because the server replaces its checkers on every edit: with a package-level map, 300 edits of one `.es` file grew the server from 448 MiB to 914 MiB; now it stays at 305 MiB, as the same file as `.ts` does.

`effectscript/lsp-smoke.sh` builds `esc` and the runtime, checks `esc --version` and `esc --help`, and runs `lsp-smoke.mjs`, an LSP client without dependencies, on `effectscript/lsp-fixture`: 37 checks of diagnostics, hovers, definitions, references, renames, completions, file watching, and shutdown across `.es` and `.ts` files. The editor setups and their tests are in `../editors/` (start with `../editors/README.md`).

### Commits

| Change | Subject |
| --- | --- |
| `lutmtlns` | checker: let in-tree extensions keep state per checker |
| `ssqltlzy` | effectscript: keep the analyzer on its checker |
| `xosuknwx` | module: map .es files to .js output |
| `kowtxrmz` | tsoptions: match .es files in wildcard include globs |
| `mwpkwpsm` | lsp: send willRenameFiles for .es files |
| `knrotrus` | effectscript: add a headless language server test |
| `uwpykqxu` | ls: add a quick info hook |
| `otxrukzp` | effectscript: show a running call's success value on hover |
| `qlpsqxln` | execute: let a compiler built on tsc name itself |
| `txvnzpqn` | effectscript: identify the compiler as esc |
| `trpvstxy` | editors: derive an EffectScript grammar from VS Code's TypeScript grammar |
| `svkttyrl` | editors: add the VS Code extension |
| `ryxzxutw` | ls: let a quick info hook replace the quick info line |
| `unqyxosy` | checker: add a signature with another return type |
| `xumoqrzt` | effectscript: show a plain function the way EffectScript calls it |
| `xpqmlqkm` | ls: add completion hooks for scope names and members |
| `lqzlumyp` | effectscript: complete prelude names and service members |
| `vrzttyov` | lsp: exit with status 0 after shutdown and exit |
| `nzvnqlxu` | editors: check completions and plain hovers in the VS Code test |
| `wuyspvlk` | editors: set up Neovim and Zed |
| `pzsnyzmw` | effectscript: document the editor and esc in EFFECTSCRIPT.md |
| `yppyvsqw` | editors: run esc through the compiler package's launcher in editor tests |

## What it cannot do yet

- `arguments` inside a region's closure is the closure's, not the function's, and a function whose body declares `using` or calls `fork` runs its whole body in such a closure.
- A `using` in the head of a C-style `for` loop, `for (using x = init; …; …)`, stays TypeScript's own `using`, which Effect does not run on failure or interruption, and an acquisition there is TS2850.
- Handlers have no meaning yet: the prelude names them, and their lane gives them meaning. A control operation in a forked fiber is not ES4201 yet.
- ES3201 is reported at runners only, not at a written contract whose R excludes the service a `wire` leaves unprovided.
- A trusted call's expected failures are defects inside a callback that compiles to plain JavaScript, such as one `Array.prototype.map` calls, where D9 and D10 read strictly would make the callback ES4301.
- Runners are known by their declarations in `effect`, `@effect/platform-node`, `@effect/platform-bun`, and `@effect/vitest`; a function that wraps one is not a runner. ES4402's globals are a fixed list (`platformGlobals`), so a platform global outside it, such as `postMessage` or `BroadcastChannel`, is not reported.
- ES4402 follows a platform API by its import or global name. A platform function reached through a local alias, a destructured namespace member, a parameter, or an object that a platform call returned is not reported, and as a call to a platform package it is not ES4401 either.
- Declaration files carry no JSDoc metadata for `$direct` variants (Chapter 27), so code compiled against a `.d.ts` runs a plain function's Effect form, which is correct and slower.
- A callback parameter without a contract returns an Effect (D12), so a function cannot call it inside a callback that JavaScript calls synchronously, such as a `sort` comparator: that is ES4301.
- TypeScript types a context-sensitive lambda's parameters from the first candidate that types them, so an overload whose parameter types differ cannot retype them.
- Accessors, constructors, async functions, and generators are never lowered. `arguments` inside a lowered arrow function is the generator's, and `arguments` and `super` inside a lowered method do not reach the method.
- A lowered function declaration is a `const`, so a call above it at the module top level hits the temporal dead zone.
- A call cycle through a lambda, such as `Effect.forEach(children, (child) => walk(child))` where `walk` calls back, is solved through the checker's type of the call, which the checker computes once; a cycle whose E grows through it is ES2102. A union of Effects is not treated as an Effect.
- Hover shows no control class (D33): the compiler does not yet tell `pure` from `sync` or find `captures`. The language server has no EffectScript quick fixes or refactors (D33), completions in a `catch` do not narrow to the error classes its `try` can raise, and the server's `serverInfo` still says `typescript`.
- The synthesized wrappers, such as `yield*` and `Effect.fnUntraced(…)`, have no source positions of their own; the statements and expressions inside them do, so stack traces and breakpoints map to `.es` lines.

## Differential check against the TypeScript-API prototype

`../prototype/esc` compiled copies of `es-wrap`, `es-lower`, and `es-infer`, with plain return annotations removed because the prototype rejects them (ES2003). Both compilers put `yield*` on the same call sites (2, 12, and 6) and leave the same calls plain, including `Config.Port(...)` and `[1, 2, 3].map(double)`. Both infer `lookup: Effect<string, NotFound, never>` and `main: Effect<void, NotFound, never>`.

The prototype reports ES1001 on the top-level `Effect.runPromise(Effect.catch(main(), …))`, because the design (Chapter 11, "Where programs start") forbids running a program at the module top level. This fork reports it too, so `es-infer` passes the runner a program: `Effect.runPromise(() => Effect.catch(() => main(), …))`.

## Next hardest risk

Position-dependent lowering now happens inside overload resolution, in one checker pass ([Programs, plain functions, and callbacks](#programs-plain-functions-and-callbacks)): the overload probes behind the single-pass experiment get the experiment's answers (case `lambda-forms`), and no construct needed a second pass. What remains of that risk is E and R through a call cycle: a recursive component needs its rows iterated to a fixed point (D52), inside a checker that resolves return types lazily and reports cycles as TS7023.
