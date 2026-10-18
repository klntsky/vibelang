# Effeck fork of the TypeScript compiler

This directory is microsoft/TypeScript with a short stack of commits that teach the Go compiler (`tsc/`) Effeck: TypeScript syntax in which every function is an Effect (effect@4.0.5). A call to a function that returns an Effect runs it, and the compiler infers each function's `Effect<A, E, R>`.

It is a first prototype. It lowers `.effeck` files with checker types, runs the output on Effect 4, and emits `.d.ts` files with the inferred Effect types. Services, control operations, handlers, producers, failures and regions, Scope regions (`using` and `fork`), the prelude, trust levels at TypeScript boundaries, platform APIs, runners, and debugging and tracing work (see their sections); the acceptance table below says which lane builds each remaining feature. The design is the specification in `../docs/design/src/`; the syntax decisions are `../docs/design/DECISIONS.md`.

## Base

| Item | Value |
| --- | --- |
| Upstream commit | `257e87406c6a8be277cb8cd5ef8d7b458a650390`, "[api] Fix named pipe / UDS creation (#64818)", 2026-10-14 |
| Why that commit | It is the `gitHead` of npm `typescript@next` 7.1.0-dev.20261015.1 (`npm view typescript@next gitHead version`) |
| Go | `tsc/go.mod` says `go 1.27`; with Go 1.26 installed, `GOTOOLCHAIN=auto` downloads go1.27.0 |
| History | `typescript/` is upstream's tree at that commit without `tsc/testdata` (61,721 test baselines, 349 MB), imported by one commit of the Effeck repository. The compiler commits below follow it. To move to a newer upstream commit, replace `typescript/` with that tree, again without `tsc/testdata`, and rebase the compiler commits onto it. |

## Build and test

Run these from `typescript/`:

```sh
cd tsc && CGO_ENABLED=0 go build -trimpath -gcflags=all=-dwarf=false -o ../built/local/tsc ./cmd/tsc
effeck/smoke.sh                 # builds tsc and the runtime, then runs every case
effeck/smoke.sh --status        # one line per case: name, owner, and status
effeck/smoke.sh ch04 ref2       # the cases whose names contain ch04 or ref2
effeck/smoke.sh -v effeck-infer     # one case, with what tsc printed
effeck/lsp-smoke.sh             # builds effeck and the runtime, then runs the language server test
./built/local/tsc -p effeck/cases/ts-hello
```

The binary is Effeck's compiler, `effeck`, under any file name; `effeck/lsp-smoke.sh` builds it as `built/local/effeck`, and `../scripts/build-effeck.sh` as `../packages/compiler/native/effeck`.

The build embeds the `lib.*.d.ts` files; Herebyfile's `-tags=noembed` build copies them next to the binary instead. A cold build takes about 75 s and fills about 1.7 GB of Go build cache. Always pass `-trimpath`: without it, Go keys each package's cache entry by its absolute directory, so every jj workspace cold-builds and adds its own copy. `-gcflags=all=-dwarf=false` shrinks the entries; panics still print stack traces.

### Cases

`effeck/smoke.sh` builds `tsc`, builds `../packages/runtime` with that `tsc`, and runs `effeck/harness.mjs`, which compiles the cases in parallel and runs them. Each directory in `effeck/cases` that has a `tsconfig.json` is a case, and its sources decide its kind:

| Kind | Marked by | Passes when |
| --- | --- | --- |
| Failing | A source whose first line is `// expect: EK2001 EK2002` | The error codes `tsc -p` reports are exactly those codes. Warnings and message text do not count |
| Runnable | A source with a line `// expect-result: <json>` that exports `main()` | `tsc -p` reports no errors, and `out/run.mjs`, which the harness writes, prints `@@result <json>`: the JSON of the success value of the Effect that `main()` returns. What the program logs does not count |
| Plain | Neither | `tsc -p` reports no errors, every file in `expected/` other than `stdout.txt`, `diagnostics.txt`, and `messages.txt` equals the file at the same path under `out/`, and `run.mjs`, or else `out/main.js`, prints `expected/stdout.txt` |

A case of any kind may also have `expected/diagnostics.txt`, which lists the diagnostics `tsc` must report, exactly, one per line in `tsc`'s order and without message text: `main.effeck(5,23): error EK1001`. Use it where a diagnostic's position or count matters, such as a rule that must report one call and not its neighbor, or a warning that a runnable case must still print. A case may also have `expected/messages.txt`, whose every line must appear in what `tsc` prints: use it where a diagnostic's text matters, such as the fix a note suggests.

A case with a file named `pending` is pending. The file's first word names the lane that owns the missing feature and the rest says what is missing, for example `F5 wire (D27)`. A pending case runs but never fails the suite. When it passes, the harness reports `XPASS`, and its owner deletes `pending` in the commit that makes it pass. The suite fails when a case that is not pending fails.

A case's sources are also inputs outside the smoke suite: the (B) projector's forms table and its Go tests read them. A commit that edits a case also runs `bash scripts/check-forms-table.sh` and `sh packages/ts-mapper/test/parity/run.sh <edited cases>` from the repository root, the second keeping the type column (b) at 0, and `go test ./internal/effeckmapper/...` from `typescript/tsc`. A decision that moves in the forms table is expected after a case edit, but `scripts/check-forms-table.sh --write` regenerates the snapshot only after `tsc/internal/effeckmapper/forms_decisions_approved.txt` names each move, one line with the site, the move and why it is right.

Each case gets two symlinks in its `node_modules`: `effect`, from `$EFFECT_NODE_MODULES` (default `../experiments/node_modules`, else the main checkout's `~/effectscript/experiments/node_modules`), and `effeck-runtime`, from `$EFFECK_RUNTIME` (default `../packages/runtime`). `smoke.sh` builds the runtime's `dist/` from source with the fork's `tsc`, so a commit that changes the compiler and the runtime is tested as one. It links `effect` into the runtime's own `node_modules` when that directory has none, so a case loads one copy of `effect`.

| Case | Proves |
| --- | --- |
| `ts-hello` | The stock compiler still compiles and runs plain TypeScript |
| `effeck-parse` | An `.effeck` file parses and checks as TypeScript and emits `main.js` and `main.d.ts`; JavaScript prints its value, because `console` is a platform API |
| `effeck-wrap` | Exported functions and arrow functions lower to `Effect.fnUntraced`, and JavaScript runs them with `Effect.runSync` |
| `effeck-lower` | Type-directed `yield*`: pure helpers stay plain, Effect-returning TypeScript calls and `.effeck` calls run, operands run left to right, a `Config` stays data |
| `effeck-infer` | Unannotated functions get `Effect<A, E, R>`: a typed failure flows from `lookup` into `main`, the `.d.ts` declares it, and the module's top level passes a program to a runner |
| `effeck-service-keys` | Service classes compile to Context key classes that JavaScript uses as Effect keys; an `implements` class is an implementation; two services with the same members are different types |
| `effeck-services` | `Users.find(id)` and `Users()` look the service up, value members read from `Settings()`, a provider captures another service, and object literal and class methods run Effects |
| `effeck-services-ts` | A TypeScript `Context.Service` class and Effect's `Path` are used as services: an Effect-valued member runs when called, and the service's own `pipe` wins over the key's |
| `effeck-services-default` | `static readonly default` makes a service a `Context.Reference` that never joins R; `static make` and `static readonly layer` stay on the class |
| `effeck-control` | `extends Control` declares a control operation; `Approval.approve(…)` joins R until the runtime's `handle` answers it |
| `effeck-service-misuse.fail` | EK3002-EK3006: a service that is extended, constructed, tested with `instanceof`, declared generic, or given a static member with a service member's name |
| `effeck-service-contract.fail` | EK2101: a service method with a bare return type, a partial `Effect<A>`, or no annotation; a value member takes any type |
| `effeck-service-keys-shared.fail` | EK3001: two services in one module share a key |
| `effeck-layer-programs` | A `Layer.effect` program whose object has untyped methods still infers `Layer<Users, never, Database>` |
| `effeck-wire` | `wire` takes its arguments in any order and emits them consumers first, so one `Database` serves `Users` and `Orders` |
| `effeck-wire.fail` | EK3202, EK3203, EK3204: a duplicate provider, a cycle, and an argument typed `Layer.Any` |
| `effeck-wire-missing.fail` | EK3201 at a runner: `MailerLive needs Smtp` |
| `ek3201-runner-services.fail` | EK3201 for a service nothing provides, at `NodeRuntime.runMain`, at `Effect.runPromise`, and at `it.effect`, in place of TypeScript's argument error; none for a test inside `layer(…)` or one that provides the service itself; a type error inside the test's body stays |
| `effeck-wire-discard.fail` | A layer that provides nothing, `Layer.effectDiscard`'s, keeps its E and In through `wire`: EK2001 at a contract and EK3201 at the runner |
| `effeck-provider-escapes.fail` | EK2001 names the provider member that a failure escapes, in an object literal and in an `implements` class |
| `effeck-provider-audited.fail` | EK3101 at a per-call lookup, in an object literal and in an `implements` class |
| `effeck-effect-value-calls` | Calling an `Exit` replays it and calling a tagged error instance fails with it (D24): `Effect<A, E, never>` and `Effect<never, EmptyDocument, never>` |
| `effeck-concise-comments` | A lowered concise arrow keeps the comment after it outside its block, and a lowered file keeps its last comment |
| `effeck-service-dts` | A service's `.d.ts` is an interface plus a key class: TypeScript code compiled against it with `skipLibCheck` off implements `Users`, provides it with `Layer.succeed`, and yields `Users` and a default service in `Effect.gen` |
| `failures-throw` | `throw` is `Effect.fail`: a throw alone makes a function an Effect, the thrown type joins E (`throw "bad"` adds `string`), a `catch` narrows the caught union by `instanceof` and `_tag` and rethrows, and the `.d.ts` declares each E |
| `failures-causes` | `catch` skips a defect, a mixed cause, and an interruption (D13); `finally` runs on each, and a failing `finally` keeps both failures (D15) |
| `failures-plain` | A function that runs no Effect and catches its own throws stays plain JavaScript, with JavaScript's `try`; its `catch` receives expected failures and rethrows a `TypeError` defect (D54) |
| `ek2004-dead-catch.fail` | A `catch` over a `try` that raises no expected failure is EK2004, which `compilerOptions.effeck` makes an error here; a live `catch` is not reported |
| `contracts` | Written contracts: returns checked against A, a contract wider than the body, a contract on a body that runs nothing, `infer` in each channel (D57), and the `.d.ts` with the inferred channels filled in |
| `contract-wider-callers` | A caller sees a contract wider than a body that runs nothing: callers of `Effect<number, Gone, Needed>` and `number!Gone!Needed` take `Gone` and `Needed`, and `number!!` keeps the plain path (Chapter 15) |
| `contract-wider-callers.fail` | Calling such a function under a narrower contract is EK2002 for its requirement and EK2001 for its failure |
| `contracts-success.fail` | A returned value that A does not accept is TypeScript's TS2322 |
| `ek2001-escaping-failure.fail` | A failure from a call and one from a `throw` escape their contracts; a `catch` keeps a third function within its contract |
| `ek2002-escaping-requirement.fail` | A service the body needs escapes the contract's R |
| `ek2003-bare-return.fail` | A plain return annotation is EK2003 on a function that runs an Effect or throws, and allowed on one that compiles to plain JavaScript (D67) |
| `ek2003-missing-channels.fail` | `Effect<string>` and `Effect<string, UserNotFound>` leave channels out |
| `recursion` | `isOdd` and `isEven` learn each other's failures through their cycle (D52), a caller outside reads the solved contract, and a self-recursive `totalSize` needs no contract |
| `ek2102-recursive-success.fail` | Effect functions whose A depends on their own result need a contract; a pure recursive function keeps TypeScript's rules |
| `ek2102-cycle-catch.fail` | A `catch` whose `try` block calls into a cycle with inferred contracts needs a written contract on the cycle |
| `ek2006-finally-jump.fail` | A `return` and a `continue` that leave `finally` blocks are EK2006; a `break` that stays inside one is not |
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
| `plain-imports` | Another `.effeck` module's plain functions run inline through named, renamed, and namespace imports |
| `plain-methods` | `[Equal.symbol]` and `[Hash.symbol]` compile to plain methods that `Equal.equals` and `HashSet` call (D55) |
| `callbacks-pausing.fail` | EK4301 for a lambda and for a function value that can pause where `Array.prototype.map` and `forEach` call them |
| `plain-methods-pausing.fail` | EK4301 for an `[Equal.symbol]` that pauses (D55) |
| `callback-contracts` | A callback parameter without a contract accepts any contract (D12): `mapAll` and `withRetry` infer E and R from each call, and the `.d.ts` writes `<A, B, E, R>` |
| `generic-callees` | A call to a generic `.effeck` function adds the callee's E and R as the call instantiates them |
| `d39-unions` | Where a parameter takes a function or an Effect (D39), a lambda with a parameter is a function, and one without is a program when it can pause and a function when it is plain |
| `callbacks-result-unions` | A callback declared to return a plain value or an Effect, as `Arbitrary.checkEffect`'s property is, is plain when D10 allows it and an Effect function when it can pause |
| `contract-lambdas-plain` | A lambda with a contract where only a plain function fits is plain and returns A: `Schema.suspend` and `SchemaTransformation.transform`. Pending on F3's D57 return check |
| `trust-levels` | `JSON.parse`'s `SyntaxError` and a trusted function's `@throws` class join E and reach a `catch`, in Effect functions and in plain ones (D54); an unlisted exception is a defect; operands that run Effects run before the trusted call; Effect's exports, a boundary, a trusted re-export, a trusted method, and a parameter's callee need nothing |
| `ek4401-plain-call.fail` | EK4401 for a plain TypeScript call, one that returns a Promise, one in a plain callback, and a trusted contract whose `@throws` class the file cannot import; none for a trusted call, a boundary, or the top level |
| `ek4401-boundary-operands.fail` | EK4401's note for a call whose argument or receiver runs an Effect says to pass it to `try` as an input, since `try`'s function cannot run one; a call with plain operands gets `try((titles) => sortTitles(titles), titles)`, with the bindings it reads as inputs |
| `ek4401-effect-reexports.fail` | D22 through re-exports: `assert` and `expect` imported from `@effect/vitest`, which re-exports vitest's, are trusted, and so is `.toBe` on what `expect` returned; the same `expect` imported from `vitest`, and the program's own method on a value `Option.getOrThrow` returned, keep EK4401 |
| `ek4402-platform.fail` | EK4402 where code runs a platform API (D108): imported functions called inside boundaries, a namespace member called, a property read on a default import and on a namespace member, `fetch`, `setTimeout`, `console.log`, destructuring `process`, and `new WebSocket`; importing, passing `createServer` and `os.EOL` as values, a type-only import, `URL`, and `TextEncoder` are allowed |
| `platform-values` | Importing `createServer` from `node:http` and `os` from `node:os`, and passing them as values, compiles and runs |
| `ek4403-clock.fail` | EK4403 for `Date.now()`, `new Date()`, `Math.random()`, `crypto.randomUUID()`, `performance.now()`, and `Date()`, at the top level too; `new Date(text)`, `Date.parse`, and `Date.UTC` are allowed |
| `ek4403-warning` | With `"EK4403": "warning"`, the reads are warnings and run as plain calls |
| `ek1001-top-level.fail` | EK1001 for an `.effeck` program, an Effect, a program passed straight to a runner, a service member call, a service, and an Effect value at the top level; none for values, lambdas, or a runner given a program |
| `ek1005-runners.fail` | EK1005 for runners in code a fiber runs, `ManagedRuntime` and `*With` forms included, and for `it.effect` in a test body; none at the top level or in a plain callback the top level passes to TypeScript (`describe`, a route handler, `layer(…)`); `stubs/` holds the real declarations of `NodeRuntime.runMain` and `@effect/vitest` |
| `debug-stack` | With `sourceMap`, Node's stack traces point at `.effeck` lines and name the function, a region closure (`audited$try`), and a recursive function whose generator is a method (D36) |
| `tracing-exported` | `"tracing": "exported"` gives exported functions `Effect.fn` spans named after them |
| `tracing-all` | `"tracing": "all"` gives every named Effect function a span, a method an `Effect.withSpan` span named `Class.method`, and a callback none |
| `plain-const` | A function that declares `const` and `let` and runs no Effect compiles to plain JavaScript, so `Array.prototype.map` calls it; only `using` needs frames (D54) |
| `scopes-using` | `using` releases in reverse order on success, failure, and interruption; once per loop iteration through `continue`, a labeled `break`, and `return`; before a `catch` runs; in a method and an arrow function; a TypeScript Disposable and `null`; an acquisition without `using` stays the caller's (D14); a masked acquisition finishes before a timeout takes effect and an interruptible one does not start (D25); `for (using x of values)` |
| `scopes-fork` | A block interrupts the fiber it forked and waits for its cleanup before releasing what it acquired earlier: in a function body, an inner block, a loop body, and a block that fails; `Fiber.join` returns the child's result or its failure; a `fork` that no block encloses uses the caller's Scope; `Effect.forkScoped` and `Effect.forkDetach` keep Effect's meaning |
| `scopes-types` | `Scope` leaves R only for `using` acquisitions and forks a block owns; `contracts.ts` checks each contract exactly |
| `scopes-callback.fail` | EK4301 for a callback that declares `using`, of a Disposable or an acquisition, passed to `Array.prototype.map` or `Effect.sync` (D54) |
| `lambda-this` | A lowered arrow function keeps its enclosing function's `this`: a program passed to `Effect.retry`, an Effect function passed to `Effect.forEach`, and a forked program in a method |
| `handlers` | Two handlers for one program, a clause that returns without resuming and runs the program's `finally`, `abandon` in a clause, `resumeWith` raising a declared error the program catches, `onReturn`, a second `resume` that is a defect, a clause that holds a `using` resource until the resumed program finishes, and 100,000 tail resumes under 50 frames (D18) |
| `ek2005.fail` | EK2005 for a `resumeWith` program that needs a service and for one that fails with an undeclared error; a program that fails with the declared error, or returns a value read in the clause, is allowed |
| `ek4201.fail` | EK4201 for a control operation under `Effect.forEach` with `concurrency: 4`, `Effect.race`, and `Effect.timeout`, and for a producer that performs one passed to `Stream.runCollect`; none without concurrency, with `concurrency: 1`, with a handler inside each branch, or for a `for...of` over the producer |
| `ek4202.fail` | EK4202 for a TypeScript function whose declared requirements hold a control operation; none for a generic TypeScript function that passes its argument's requirements through |
| `producers` | Producers (D26): laziness; cleanup before `break`, `return`, and `continue` to an outer label complete; one pass per run; service calls between yields; `yield*`; `Stream.take` without running ahead; a failure reaching the loop's `catch`; pattern, existing-variable, and `var` loop targets; a labeled JavaScript loop around a producer loop; `for...of` over a Stream; 100,000 elements under 50 frames; and `Stream<number, Broken, never>` in the `.d.ts` |

### Acceptance cases

The prototype's 46 examples (`../prototype/esc/examples/`) are cases named after their files, such as `ch04-wire` and `ref2-cycle.fail`. Each keeps its `.effeck` source and `.ts` helpers as the prototype has them, except that kernel imports name `effeck-runtime` and `effeck-runtime/kernel`. They compile with `module: nodenext` and `rewriteRelativeImportExtensions`, as the app template does. Every compiler lane turns its cases green and never turns a green case red. Each case is pending on the lane that owns its feature; a case that mixes features belongs to the lane that owns its first missing feature, and its `pending` file names the other lanes it needs:

| Owner | Feature | Cases |
| --- | --- | --- |
| F1 | Diagnostics, prelude, imports between files | `ch10-effect-gen.fail` |
| F2 | Programs as values, plain functions, callbacks | `ch05-programs`, `ch10-map-pausing-callback.fail`, `ch13-plain`, `ch14-callbacks`, `ref4-callbacks`, `ref4-pausing-function-value.fail`, `ref5-programs-as-data` |
| F3 | `throw`, regions, contracts | `ch02-contract-narrower.fail`, `ch02-functions`, `ch03-errors`, `ch03-safelookup-no-try.fail`, `ch12-contracts`, `ch12-welcome-missing-mailer.fail`, `ch15-failures`, `nominal-services.fail`, `regions` |
| F5 | Services, layers, `wire` | `ch01-first-look`, `ch04-provider-escapes.fail`, `ch04-services`, `ch04-users-audited.fail`, `ch04-wire`, `ch12-rates-bare-return.fail`, `ref1-existing-service`, `ref2-ambiguous-provider.fail`, `ref2-cycle.fail`, `ref2-missing-provider.fail`, `ref2-opaque-layer.fail`, `ref2-provider-graph` |
| F6 | `using`, `fork`, Scope regions | `ch06-disposable`, `ch06-resources`, `ch07-concurrency`, `ref3-resourceful-retry` |
| F7 | Handlers, producers, streams | None pending: `ch08-producers`, `ch08-streams`, `ch09-approve-all-forked.fail`, `ch09-handlers`, `ref6-fork.fail`, `ref6-handlers`, and `stack-depth` pass |
| F8 | Boundaries, runners | None pending: `ch02-plain-ts-call.fail`, `ch10-fetch.fail`, `ch10-typescript`, `ch11-top-level-run.fail`, `ch24-profile`, and `d5-clock-read.fail` pass |

The `pending` files are the source of truth: `effeck/smoke.sh --status` prints each case's current owner.

## The stack

The following history includes every commit from the TypeScript import through
this table's update that touches `typescript/`, in ancestor order. Change IDs
are jj's and survive rewrites. Area is the commit subject's prefix; commits that
only change the runtime or examples remain in the relevant lane's prose below.
Regenerate with `jj log -r 'unpwqpnz::main' --reversed -- typescript/`.

| Change | Subject | Area |
| --- | --- | --- |
| `unpwqpnz` | typescript: import microsoft/TypeScript at 257e874 | typescript |
| `pzxkrxsm` | effeck: add a build-and-run smoke test | effeck |
| `ltlztyux` | tspath: accept .effeck files as TypeScript sources | tspath |
| `xvqlzzvl` | compiler: add a script transformer registration hook | compiler |
| `wzsmttko` | effeck: lower .effeck functions to Effect.fnUntraced generators | effeck |
| `uvwpnsyr` | checker: let emit-time extensions query the checker | checker |
| `wzmxpyyu` | effeck: run Effect-typed calls with yield* | effeck |
| `wnyqnmso` | checker: add a call expression type hook | checker |
| `vlkoxlmy` | effeck: type a call that runs as the Effect's success value | effeck |
| `xnptkzuz` | checker: add an inferred return type hook | checker |
| `ukynlwvn` | effeck: infer Effect<A, E, R> for .effeck functions | effeck |
| `wzntprnu` | effeck: describe the fork in EFFECK.md | effeck |
| `zwwnumoq` | effeck: build with -trimpath in smoke.sh | effeck |
| `mzuwnztz` | effeck: run cases with a parallel harness | effeck |
| `ylzmpsoz` | effeck: port the prototype's 46 examples as cases | effeck |
| `oozozxtw` | checker: add overload candidate hooks | checker |
| `vvpuorns` | checker: add a function expression type hook | checker |
| `kptnxtrk` | checker: add a contextual return type hook | checker |
| `smmzorwv` | effeck: decide a lambda's form per overload candidate | effeck |
| `rokrkspy` | effeck: compile a lambda where an Effect is expected as a program | effeck |
| `lutmtlns` | checker: let in-tree extensions keep state per checker | checker |
| `ssqltlzy` | effeck: keep the analyzer on its checker | effeck |
| `wskpkmqv` | checker: add hooks for file checks and diagnostic filters | checker |
| `wvstzskn` | compiler: exempt registered diagnostic sources from comment directives | compiler |
| `yuprxynn` | tsoptions: add the effeck compiler option | tsoptions |
| `wuupxsnv` | effeck: add the EK diagnostics registry and reporting path | effeck |
| `qqszuvwv` | effeck: report EK1003 and EK1004 | effeck |
| `msunyvzt` | effeck: report EK1002 for generator-based APIs | effeck |
| `rkproqwz` | checker: add a class base hook | checker |
| `mxvtplyt` | effeck: compile abstract classes to service keys | effeck |
| `zympzwnv` | checker: add member receiver and call signatures hooks | checker |
| `ssslmlqz` | effeck: call service members through the current Context | effeck |
| `ptlsrptn` | effeck: lower methods to Effect functions | effeck |
| `sxoqtkxo` | effeck: give a service with a static default a reference key | effeck |
| `qlnyylqp` | effeck: declare control operations | effeck |
| `ozrknmym` | effeck: document services in EFFECK.md | effeck |
| `xosuknwx` | module: map .effeck files to .js output | module |
| `kowtxrmz` | tsoptions: match .effeck files in wildcard include globs | tsoptions |
| `mwpkwpsm` | lsp: send willRenameFiles for .effeck files | lsp |
| `knrotrus` | effeck: add a headless language server test | effeck |
| `uwpykqxu` | ls: add a quick info hook | ls |
| `otxrukzp` | effeck: show a running call's success value on hover | effeck |
| `qlpsqxln` | execute: let a compiler built on tsc name itself | execute |
| `txvnzpqn` | effeck: identify the compiler as effeck | effeck |
| `rnpxuunq` | checker: add an expression type hook | checker |
| `vkznolxv` | effeck: convert function values where Effects are expected | effeck |
| `xkssspqy` | effeck: add a case for calling Effect values | effeck |
| `xuttxlwl` | effeck: report EK4301 for callbacks that can pause | effeck |
| `ykwlmytt` | checker: add a catch clause variable type hook | checker |
| `yullvuom` | checker: expose an expression's type before literal widening | checker |
| `mssmulnm` | effeck: lower try, catch, and finally to Effect regions | effeck |
| `qsxrpnuo` | effeck: lower throw to Effect.fail and add its type to E | effeck |
| `vlzoymlw` | effeck: keep JavaScript's try in plain functions (D54) | effeck |
| `ywvpypzw` | checker: add a name resolution hook before the global scope | checker |
| `zlxmxpnz` | compiler: add an implicit import hook | compiler |
| `swxmkoyv` | effeck: point the runtime at the cases' effect in smoke.sh | effeck |
| `urznvokp` | effeck: resolve and import the prelude in .effeck files | effeck |
| `uwozskvx` | effeck: compare a runnable case's result, not its logs | effeck |
| `oywqrtxx` | module: resolve .effeck source files | module |
| `urkxzymx` | core: let .effeck files import TypeScript sources by extension | core |
| `ntslxkmz` | effeck: emit .js specifiers for imports of sources | effeck |
| `rplwrxxw` | effeck: record a prelude use only at a source reference | effeck |
| `nvukvnsq` | effeck: test the runtime import's free name | effeck |
| `kmonzlpz` | effeck: decide plain functions as a greatest fixpoint | effeck |
| `nmlrmkpo` | effeck: give plain functions a $direct variant and an Effect form | effeck |
| `tlvxnnyq` | effeck: let pipe take the program first | effeck |
| `srwuqnot` | effeck: report misused service classes | effeck |
| `yvlmtqkl` | effeck: lower a method's body as an Effect body only | effeck |
| `truwtzrw` | checker: add a skipped function type hook | checker |
| `qzmttpum` | effeck: infer a program whose result is context sensitive | effeck |
| `zvvqmzyy` | effeck: order wire's arguments and check its provider graph | effeck |
| `vwtxzvnt` | effeck: report a service that wire leaves unprovided at a runner | effeck |
| `ktslyskx` | effeck: check providers against their service's contracts | effeck |
| `xorurrnt` | effeck: document service checks and wire in EFFECK.md | effeck |
| `rwmokwmv` | checker: add an entity name visibility hook | checker |
| `knrysqlq` | compiler: add a declaration transformer hook | compiler |
| `psvzumxw` | module: resolve an .effeck specifier to its declaration file | module |
| `nxmqvpsu` | effeck: write portable declaration files for .effeck files | effeck |
| `okvzuusr` | effeck: hand ch13-plain and ref4-callbacks to the lanes they wait on | effeck |
| `pylwouuv` | effeck: report EK4301 for a method that implements a plain member | effeck |
| `llryttxs` | effeck: declare a service as an interface and a key class | effeck |
| `zummuosy` | effeck: document service declaration files in EFFECK.md | effeck |
| `nwnutlmz` | effeck: read a generic .effeck callee's E and R from the call | effeck |
| `uuwrwppk` | checker: add type parameter and parameter type hooks | checker |
| `tmmqumtk` | effeck: let a callback parameter without a contract accept any contract | effeck |
| `wnqqlkmm` | effeck: document programs, plain functions, and callbacks | effeck |
| `vumssupn` | effeck: add the returned-lambda probe to lambda-forms | effeck |
| `uqswlnul` | effeck: add a case for a parameter that takes a function or an Effect | effeck |
| `ywyktuko` | effeck: compare a case's diagnostics with expected/diagnostics.txt | effeck |
| `rvpuznxt` | effeck: report EK4401 for calls without a trust level | effeck |
| `uspovlno` | effeck: lower the expected failures of trusted calls | effeck |
| `nuwqloyu` | effeck: report platform APIs (EK4402) and clock reads (EK4403) | effeck |
| `olsmqqqy` | effeck: report programs started outside a runner (EK1001, EK1005) | effeck |
| `zunmxlrk` | effeck: name generators after their source (D36) | effeck |
| `xnmknson` | effeck: trace functions with Effect.fn spans (D34) | effeck |
| `stmorqmo` | effeck: document boundaries, runners, debugging, and tracing | effeck |
| `ooknlorp` | effeck: report EK4402 where code runs a platform API (D108) | effeck |
| `okrmuzvz` | effeck: add a pending case for calling Exit and error values | effeck |
| `rvysxzyx` | effeck: run a call to an Exit or an error value | effeck |
| `ysknkpmu` | effeck: keep the comment after a concise arrow out of its block | effeck |
| `vsvzswzz` | effeck: keep the last comment of a lowered file | effeck |
| `twymwmrr` | effeck: document calls to Exit and error values in EFFECK.md | effeck |
| `ztwuloyu` | effeck: keep a lambda that must return a Config plain | effeck |
| `wpukknuy` | effeck: look ahead in pipe without checking the first stage | effeck |
| `lmsyqkzu` | checker: export the iterated type of an iterable | checker |
| `lzqulkkl` | effeck: convert any iterable of programs for Effect.all | effeck |
| `tsvpvvxs` | effeck: let a callback that may return an Effect take either form | effeck |
| `pqtovvuk` | effeck: name the collection and callee in EK4301 notes correctly | effeck |
| `nzkrouwn` | checker: add an annotated return inference hook | checker |
| `nzplpwps` | effeck: type a plain lambda with a contract as returning A | effeck |
| `uqxznpul` | effeck: document the F2 follow-up fixes in EFFECK.md | effeck |
| `oqrpqyrk` | effeck: report EK2004 for a catch that can never run | effeck |
| `rtqqsqrv` | declarations: add a hook to declare a type from the checker instead of its annotation | declarations |
| `kxzmxurw` | checker: add hooks for checked return types and infer outside conditional types | checker |
| `stuypkuq` | effeck: check written contracts (EK2001, EK2002, EK2003, D57) | effeck |
| `utpqokps` | effeck: count only using declarations as using | effeck |
| `ssqqsmwt` | checker: add a using initializer hook | checker |
| `xvsrqxyz` | effeck: lower using to Scope regions | effeck |
| `twvqsuom` | effeck: give each fork to the block that calls it | effeck |
| `nzuukwkz` | effeck: take each for...of value with using | effeck |
| `qsytkrtw` | effeck: document Scope regions, using, and fork in EFFECK.md | effeck |
| `ovsoxmyu` | effeck: solve E and R of call cycles as a fixpoint (D52, EK2102) | effeck |
| `rkxpuynr` | effeck: keep this in a lowered arrow function | effeck |
| `ryxzxutw` | ls: let a quick info hook replace the quick info line | ls |
| `unqyxosy` | checker: add a signature with another return type | checker |
| `xumoqrzt` | effeck: show a plain function the way Effeck calls it | effeck |
| `xpqmlqkm` | ls: add completion hooks for scope names and members | ls |
| `lqzlumyp` | effeck: complete prelude names and service members | effeck |
| `vrzttyov` | lsp: exit with status 0 after shutdown and exit | lsp |
| `pzsnyzmw` | effeck: document the editor and effeck in EFFECK.md | effeck |
| `uvuursyx` | effeck: report EK2006 for a jump out of a finally block | effeck |
| `yppyvsqw` | editors: run effeck through the compiler package's launcher in editor tests | editors |
| `lqwnpvkm` | effeck: trust what Effect's packages re-export (D22) | effeck |
| `npwvluvx` | effeck: suggest a trusted re-export for a package's function | effeck |
| `nmnxqxrr` | runtime: keep the errors and inputs of a layer that provides nothing | runtime |
| `xvumwqrx` | effeck: alias a prelude type the declaration file already binds | effeck |
| `lnsqowzl` | effeck: let a case check the text of its diagnostics | effeck |
| `zlrnlrmv` | effeck: tell EK4401 to bind operands that run Effects first | effeck |
| `suvtvsls` | effeck: report EK3201 for any service a runner's program lacks | effeck |
| `zootqspn` | effeck: name the runner check's reader runnerRequirements | effeck |
| `xqrnzlyn` | effeck: compile a clause's tail resume to the kernel's request | effeck |
| `pnvzvzmo` | effeck: report EK2005 for resumeWith programs outside the operation's contract | effeck |
| `susnvvyx` | effeck: report control operations that cannot reach their handler | effeck |
| `lxxwsyuu` | checker: add an iterated element type hook | checker |
| `tnnzwswk` | effeck: compile function* to producers and loop over Streams | effeck |
| `moutqkun` | effeck: document handlers and producers in EFFECK.md | effeck |
| `oxmyvyxm` | docs: refresh the compiler commit table | docs |

## Short forms

`.effeck` files add short forms that the parser expands into the TypeScript they abbreviate (`tsc/internal/parser/effeck.go`), so the binder, checker, emitter, declaration emit, and language service see ordinary TypeScript nodes, and the long forms still compile. A node with no source text of its own is synthesized: it takes the location of the source token it stands for, `NodeFlagsReparsed` keeps the language service from treating it as source, and `NodeFlagsReparserTransformedLiteral` makes the printer use an identifier's own text, or a string literal's source text in quotes. That is the mechanism the JSDoc reparser uses for `@typedef` names. A node that only wraps source nodes spans them, so navigation still reaches the source inside.

A declaration form starts with a word that an identifier follows on the same line. Two identifiers in a row are a TypeScript syntax error, so the words stay ordinary identifiers everywhere else: `catch (error)` and `error.id` parse as before.

| Form | Expands to | Decision |
| --- | --- | --- |
| `error NotFound { id: string }` | `class NotFound extends TaggedError("NotFound")<{ readonly id: string }> {}` | D126 |
| `User!NotFound!Db`, `User!!`, `User!NotFound` | `Effect.Effect<User, NotFound, Db>`; an empty slot is `never`; left-out dependencies are `infer R` on a function with a body (`parseReturnType`), else `never` | D127 |
| `service Users { find(id: string): User!NotFound }`, `control Approval { ... }` | `abstract class Users { abstract find(...) }`, with `extends Control` for `control`; bodiless instance members become abstract, value members also readonly (`effeckAbstractMembers`, cleared inside nested classes); an implemented member is EK2103. The older spelling with `class` (`service class Users`) parses the same way and draws the warning EK2120, whose quick fix removes `class` (`declaration_forms.go`) | D128, D223 |
| `layer UsersLive: Users { setup; find(id) { ... }; readonly region = "eu" }`, the old spelling of `Layer.effect(Users) { setup; return { ... } }` (D221) | `const UsersLive = Layer.effect(Users, () => { setup; return { find(id) { ... }, region: "eu" } })`, or `Layer.succeed` without setup; setup after a member is EK2104 (`layers_check.go`). It draws the warning EK2121, whose quick fix rewrites the declaration into the call from its source text (`layerDeclarationText` in `declaration_forms.go`). The current form needs nothing from the parser beyond block params (D188) | D129, D221 |
| `provide AppLive { ... }` | A `Block` flagged `NodeFlagsEffeck` whose first statement holds the layer; the analyzer (`provideFlow`) and the lowering (`visitProvideBlock`, a region under `Effect.provide`) recognize it (`provide.go`). The old spelling `with AppLive { ... }` parses the same way and draws the warning EK1009, whose quick fix writes `provide`. Providers separated by commas become an array of layers, and `Key = value` becomes `$provideService(Key, "Key", value)` from `effeck-runtime` (`effeckProviders` in the parser); `providerChannelsOf` reads the array element by element | D130, D172-D175 |
| `catch (e: NotFound) { A } catch (e) { B }` (retired: warning EK2112; `catch (cause: Cause)` draws EK2113; `retiredcatch.go` finds the chain and its quick fix writes `match (e) { when NotFound: …; default: throw e; }`, an `if (e is …)` chain, or `match (exit(…))`; D216, D217) | One `catch ($caught)` whose block is `if ($isFailure<NotFound>($caught)) { const e = $caught; A } else { const e = $caught; B }`, ending in `throw $caught` without an untyped clause; the lowering adds the tags (`catches.go`) | D131 |
| `interface Cart { id: string; mutable items: Array<Item> }` | `interface Cart { readonly id: string; items: Array<Item> }`: every property and index signature of an interface or object type gets a synthesized `readonly` unless written `mutable` (`parseEffeckMutable`, read in `scanTypeMemberStart` and `parseTypeMember`) | D134 |
| `schema User { id: string; home?: Address }` | `const User = Schema.Struct({ id: Schema.String, home: Schema.optional(Address) })` and, through `p.reparseList`, `interface User extends Schema.Schema.Type<typeof User> {}` before it; a field type with no schema form becomes `Schema.Unknown` flagged `NodeFlagsEffeck`, which is EK2109, as is a named value that is not a schema (`schemas.go`) | D133 |
| `try { B } handle Approval.approve(request) { C }` | `try { B } catch ($performed) { if ($performed) { const [$k, request] = $clause(Approval.approve); C } }`, flagged `NodeFlagsEffeck`, inside `try { ... }` when catch clauses or `finally` follow; in C, `resume(v)` passes `$k`, and a tail resume returns; the checker reads `Approval.approve` from the control's shape (`memberReceiver`), and the analyzer (`handleFlow`) and the lowering (`visitHandleStatement`, a region under `handle`) recognize the statement (`handles.go`) | D132 |

An entry module, one that exports a parameterless `function main`, gets `if (import.meta.main) NodeRuntime.runMain(main())` at the end of its JavaScript, or `runtime.$runMain(main())` when `@effect/platform-node` does not resolve from it; `implicitRuntimeImport` adds the package as an implicit import so the plan can tell (`entry.go`, D135). A module that calls a runner at its top level gets none.

In `.effeck` files, `parsePostfixTypeOrHigher` leaves `!` to `parseType`, which reads the slots of an Effect type; JSDoc's postfix non-null `T!` does not parse there. Hovers rewrite Effect types into the `!` form through `ls.RegisterQuickInfoTextHook` (`effeck/bang.go`), and contract messages write them the same way.

The VS Code grammar adds these forms in `editors/vscode/scripts/effeck-grammar.mjs`, which `derive-grammar.mjs` applies whenever it rederives the grammar from TypeScript's.

## Seams in upstream files

Effeck code lives in `tsc/internal/effeck/`, which may import any internal package because it sits under `tsc/internal/`. It registers its hooks from `init`. Upstream code changes in three ways: one-line call sites in existing files, new files that hold the hooks, and the `effeck` compiler option, which is added by hand to the generated option tables because the generator's type model has no nested objects.

| Upstream file | Change | Lines (+/-) |
| --- | --- | --- |
| `tsc/internal/parser/parser.go` | Hand an identifier that starts a short form to `effeck.go` in `parseStatement`, `parseDeclarationWorker`, and `scanStartOfDeclaration`; add a readonly modifier in `parsePropertyOrMethodSignature` while `effeckReadonlyMembers` is set; read Effect types after `parseType`, stop `parsePostfixTypeOrHigher` at `!`, and infer left-out dependencies in `parseReturnType`; make bodiless members of a service or control body abstract in `parseMethodDeclaration` and `parsePropertyDeclaration`, and clear that inside `parseClassDeclarationOrExpression`; parse `provide` (and its old spelling `with`) as a provide block in `parseStatement`; read typed and repeated catch clauses, and handle clauses, in `parseTryStatement`; read `mutable` in `scanTypeMemberStart` and `parseTypeMember`, and make other property and index signatures readonly | +64 / -1 |
| `tsc/internal/ls/extensions.go` | `RegisterQuickInfoTextHook` rewrites a hover's quick info line after the quick info hooks; `RegisterCodeFixProvider` adds a quick fix, such as EK1009's | +22 / -0 |
| `tsc/internal/ls/codeactions.go` | `CodeFixProvider.Source` lets a provider fix diagnostics from a source other than `ts`, such as `EK` | +7 / -1 |
| `tsc/internal/ast/nodeflags.go` | `NodeFlagsEffeck` marks a node that stands for an Effeck form the checker reads as ordinary code | +1 / -0 |
| `tsc/internal/tspath/extension.go` | `ExtensionEffeck` in the TypeScript extension lists; `.effeck` declares to `.d.ts` | +9 / -8 |
| `tsc/internal/core/core.go` | `.effeck` is `ScriptKindTS` | +1 / -1 |
| `tsc/internal/compiler/emitter.go` | Run hook transformers first in `getScriptTransformers` | +2 / -0 |
| `tsc/cmd/tsc/main.go` | Blank import of `internal/effeck` | +2 / -0 |
| `tsc/internal/checker/checker.go` | Apply call-type hooks where `checkCallExpression` returns and on the `getQuickTypeOfExpression` call shortcut; wrap `getReturnTypeFromBody`; hold extension state; run file check hooks at the end of `checkSourceFile`; filter what `getDiagnostics` returns; ask class base hooks in `getBaseConstructorTypeOfClass` and `resolveBaseTypesOfClass`; ask member receiver hooks before `checkPropertyAccessExpressionOrQualifiedName` looks up a member; ask call signatures hooks in `resolveCallExpression` when the callee has none; set `ResolveBeforeGlobals`; ask skipped function type hooks where `checkFunctionExpressionOrObjectLiteralMethod` skips a context-sensitive function | +23 / -5 |
| `tsc/internal/checker/checker.go` | Ask the catch clause variable type hook in `getTypeForVariableLikeDeclaration`; a catch variable is always initialized | +4 / -1 |
| `tsc/internal/checker/checker.go` | Ask missing property hooks in `checkPropertyAccessExpressionOrQualifiedName` when a property access finds no member (`datamethods.go`) | +3 |
| `tsc/internal/checker/checker.go` | A pipe has its body's type in `checkBinaryLikeExpression`, and its body has the pipe's contextual type in `getContextualTypeForBinaryOperand` (`extensions_pipes.go`) | +7 |
| `tsc/internal/format/rulecontext.go` | The two characters of `\|>` are no binary operator context in `isBinaryOpContext`, so the formatter keeps them together | +3 |
| `tsc/internal/checker/checker.go` | Ask using initializer hooks before `checkVariableLikeDeclaration` requires a `using` initializer to be a Disposable | +1 / -1 |
| `tsc/internal/checker/checker.go` | Ask iterated element type hooks in `checkRightHandSideOfForOf` and, for `yield*`, in `getYieldedTypeOfYieldExpression`, `checkYieldExpression`, and `checkAndAggregateYieldOperandTypes` | +14 / -3 |
| `tsc/internal/checker/checker.go` | Check what a function returns against `unwrapReturnTypeOf` at the four places that check it against `unwrapReturnType`; complete an annotated return type through annotated return type hooks in `getReturnTypeOfSignature`; ask infer type hooks in `checkInferType` | +9 / -7 |
| `tsc/internal/transformers/declarations/transform.go` | `ensureType` serializes a type from the checker when a type annotation hook asks | +1 / -1 |
| `tsc/internal/checker/checker.go` | Tell call candidate hooks which candidate `inferTypeArguments` and `isSignatureApplicable` check, and let candidate filter hooks reject it; pass the type of a function expression, a contextual return type (`getContextualReturnType` wraps the old body as `getContextualReturnTypeWorker`), and every checked expression through hooks; append hook type parameters in `getSignatureFromDeclaration` and `getOuterTypeParameters`; let parameter type hooks replace an annotation's type in `getTypeForVariableLikeDeclaration`; let annotated return inference hooks replace the return annotation's type in `inferFromAnnotatedParametersAndReturn` | +15 / -5 |
| `tsc/internal/compiler/program.go` | Comment directives skip diagnostics from exempt sources | +1 / -1 |
| `tsc/internal/compiler/fileloader.go` | Resolve each module an implicit import hook names for a file | +4 |
| `tsc/internal/binder/nameresolver.go` | `NameResolver.ResolveBeforeGlobals`, consulted before the global scope | +6 |
| `tsc/internal/module/resolver.go` | Try `.effeck`, then `.d.ts`, for `./welcome.effeck`, and `.effeck` after `.tsx` for `./welcome.js` | +19 |
| `tsc/internal/module/util.go` | An `.effeck` resolution is always allowed | +2 / -1 |
| `tsc/internal/module/util.go` | An `.effeck` file's output extension is `.js`, so auto-import can name it | +1 / -1 |
| `tsc/internal/parser/parser.go` | `try(` starts an expression in an `.effeck` file: the statement switch, `isStartOfLeftHandSideExpression`, `parsePrimaryExpression` (`effeck_hatches.go`) | +9 / -1 |
| `tsc/internal/parser/parser.go` | `\|>` ends `parseBinaryExpressionRest` and follows `parseConditionalExpressionRest` in `parseAssignmentExpressionOrHigherWorker`; `%` starts an expression in `parsePrimaryExpression` and `isStartOfLeftHandSideExpression` (`effeck_pipes.go`) | +9 / -3 |
| `tsc/internal/parser/parser.go` | `finally (` reads a binding before the finally block in `parseTryStatement`; an identifier `uninterruptible`, `restore`, or `interruptible` before `{` on its line starts an interruption block in `parseStatement` (`effeck_primitives.go`) | +8 / -1 |
| `tsc/internal/tsoptions/parsedcommandline.go` | `.effeck` in a project's wildcard include globs, so a new `.effeck` file joins the project | +4 / -4 |
| `tsc/internal/lsp/server.go` | `.effeck` in the `willRenameFiles` filter; exit with status 0 after `shutdown` and `exit` | +3 / -3 |
| `tsc/internal/ls/hover.go` | Apply quick info hooks to each hover | +1 |
| `tsc/internal/astnav/tokens.go` | In an `.effeck` file, an identifier that `getTokenAtPosition` finds between a node's children, where TypeScript code has only trivia, goes to `effeck.go` instead of panicking, both at the position and as the token ending there | +23 / -3 |
| `tsc/internal/astnav/effeck.go` (new) | The source token a desugared node holds out of order or outside its range, such as `for concurrent`'s binding and items; otherwise the word of a form: a class or const keyword for a declaration that parses into one, else the form's own node | +77 |
| `tsc/internal/ls/completions.go` | Add completion scope hooks' symbols to global completions; let member completion hooks replace the type left of the dot | +2 |
| `tsc/internal/execute/tsc/help.go` | The `--version` line, the `--help` title, its examples' command, and its icon come from the identity | +6 / -6 |
| `tsc/internal/core/compileroptions.go` | A `.effeck` file may import a TypeScript source by its extension | +5 / -1 |
| `tsc/internal/checker/emitsupport.go` | Ask entity name visibility hooks in `isEntityNameVisible` | +4 |
| `tsc/internal/compiler/emitter.go` | Run declaration transformer hooks after TypeScript's | +2 / -2 |
| `tsc/internal/core/options_generated.go` (generated) | `CompilerOptions.Effeck`, in `Clone` and `Equals` | +6 |
| `tsc/internal/tsoptions/declarations_generated.go` (generated) | The `effeck` declaration | +8 |
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
| `tsc/internal/ls/extensions.go` (new) | `RegisterQuickInfoHook`, `RegisterCompletionScopeHook`, `RegisterMemberCompletionHook`; in an `.effeck` hover the hooks' notes come first, then a `---` rule, then the JSDoc | +101 |
| `tsc/internal/execute/tsc/extensions.go` (new) | `SetIdentity` | +56 |
| `tsc/internal/checker/extensions_signatures.go` (new) | `SignatureWithReturnType` | +8 |
| `tsc/internal/checker/extensions_loops.go` (new) | `RegisterIteratedElementTypeHook` | +29 |
| `tsc/internal/checker/extensions_methods.go` (new) | `RegisterMissingPropertyHook`, `NewPropertySymbol`, `SignatureWithThisFromFirstParameter`, `NewFunctionType`, `GetIntersectionType` | +78 |
| `tsc/internal/checker/extensions_pipes.go` (new) | `isEffeckPipe` | +13 |
| `tsc/internal/core/effeck.go` (new) | `EffeckOptions` | +22 |
| `tsc/internal/tsoptions/effeck.go` (new) | Parse, validate, and serialize `compilerOptions.effeck` | +149 |

A rebase onto a newer nightly can conflict only in the existing files above. If upstream regenerates the option tables, add the `effeck` lines again; `tsoptions/effeck.go` lists where they go.

## What the lowering does

The transform runs on each `.effeck` file before the TypeScript transforms. It holds the checker lock, plans the file, then rewrites it:

```ts
// main.effeck                                   // main.js
function lookup(id: number) {                const lookup = Effect_1.fnUntraced(function* (id) {
  if (id >= names.length)                        if (id >= names.length)
    Effect.fail(new NotFound({ id }))                yield* Effect.fail(new NotFound({ id }));
  return names[id]                               return names[id];
}                                            });
```

- **Which calls run.** A call runs, as `yield* f(x)`, when its resolved signature returns effect's own `Effect` interface, or when it calls an `.effeck` function that is an Effect. Types that extend `Effect` (`Config`, `Exit`, Context keys, `TaggedError` instances) are data (design D24). Only calls in a lowered function's own body run; calls at the module top level, in parameter initializers, and in nested plain functions stay plain. A call to a function that compiles to plain JavaScript runs inline, as `double$direct(x)`, anywhere in the file.
- **Which functions become Effects.** A function declaration becomes `Effect.fnUntraced(function* …)` when it performs an Effect operation or can fail; otherwise it compiles to plain JavaScript (D10), as a `$direct` variant and an Effect form. A lambda takes the form its position asks for: a plain function, an Effect function, or a program. [Programs, plain functions, and callbacks](#programs-plain-functions-and-callbacks) gives the rules.
- **Output shape.** A declaration becomes a `const` (with `export` or `export default`), and the file imports `* as Effect from "effect/Effect"` under a free name. Operands keep JavaScript's order, for example `(yield* greet("a")) + (yield* greet("bc"))`.
- **Types.** Inside a lowered body, a call that runs has the Effect's success type, so `findName(2).toUpperCase()` checks. An unannotated `.effeck` Effect function has return type `Effect<A, E, R>`. A comes from its returns; E and R are the unions of the error and requirement channels of every Effect it runs. The `.d.ts` declares `main(): Effect.Effect<void, NotFound, never>`, and a callback such as `(e) => Console.log(…)` passed to `Effect.catch` checks and runs as an Effect function.

Checking takes one pass. The analyzer decides from function bodies and callee types, and the hooks type each call and lambda inside TypeScript's own overload resolution.

## Programs, plain functions, and callbacks

A function value compiles in the form its position asks for (Chapter 13, "Function values"):

```ts
// main.effeck                                       // main.js
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
| A plain function | An `.effeck` function that compiles to plain JavaScript | `f$direct` (D9) |
| A plain function | An `.effeck` function that can pause | rejected, then EK4301 |

A syntactic array or object literal of programs converts element by element, so `Effect.all([job, () => fetch(id)])` needs no helper. Calling an Effect value runs it (`Clock.currentTimeMillis()`, a cached program, a `Config`): the call signatures hook (`effectValueCall`, `services.go`) gives such a callee the signature `() => Effect<A, E, R>`.

### By-name arguments (D147)

Where a parameter expects an Effect, an argument expression that calls something TypeScript sees as an Effect, an `.effeck` function (plain ones too), a function that returns an Effect, or a service member, is passed by name: as a program that evaluates it each time it runs.

```ts
// main.effeck                                        // main.js
const fiber = fork(fetchReport())                     const fiber = yield* runtime.$fork(Effect.gen(function* () { return yield* fetchReport(); }), scope_1)
const total = Effect.runSync(subtotal())              const total = Effect.runSync(Effect.gen(function* () { return subtotal$direct(); }))
Console.log(fast())                                   yield* Console.log(yield* fast())
```

- **Where.** The expression type hook (`expressionType`, `conversions.go`) decides per overload candidate, like the conversions above: `convertByName` when the candidate's declared parameter expects a program (`classify`), the expression calls an Effect (`pauses`), and its value is not itself an Effect. Arguments, array elements, and record fields qualify, in lowered function bodies and at a module's top level (`mayRunByName`). A position that accepts any value, such as `Console.log`'s, runs the call where it stands.
- **Type.** `byNameType` builds `Effect<A, E, R>` from the expression's value type and what `flowOf` collects from it. The value type follows each check: an inference pass that skips a generic call gives the placeholder `never`, so `valueTypeOf` reads the expression's own type there; otherwise a type parameter with a default, as in `Effect.forkScoped`'s `<Arg extends Effect | Options = Options>`, would take the default.
- **Channels.** `flowOf` skips an expression passed by name (`isByName`), so its errors and requirements join the callee's Effect, not the enclosing function's.
- **Lowering.** `visit` checks `convertByName` first and emits `Effect.gen(function* () { return … })` with a fresh frame (`byNameProgram`), or `Effect.gen({ self: this }, …)` when the expression uses `this`; a plain function's call inside it runs through its `$direct` variant.
- **The top level.** EK1001 does not apply to a call passed by name (`passedAsProgram`), so `NodeRuntime.runMain(main())` and `Effect.runPromise(loadConfig())` are allowed. A service member call or an Effect value call there still needs `() =>`: the checker types those only inside function bodies.
- **Hover.** A hover on the callee says "Passed by name to `Effect.timeout` as a program: it runs when that program runs, not here".

Case `effeck-by-name` covers retry, race, timeout inside `try`, `fork`, array elements, `Console.log`, and a top-level runner; `lsp-smoke.sh` checks the hover.

### Plain JavaScript (D9, D10, D55)

```ts
// math.effeck                                       // math.js
export function double(n: number) {             export function double$direct(n) { return n * 2; }
  return n * 2                                   export function double(...args) { return Effect.sync(() => double$direct(...args)); }
}
// main.effeck
import { double } from "./math.effeck"               import { double } from "./math.js";
                                                 import { double$direct } from "./math.js";
const quad = (n: number) => double(double(n))    const quad = (n) => double$direct(double$direct(n));
```

- **Eligibility.** A named function declaration compiles to plain JavaScript when it performs no Effect operation, no throw can end it, and it declares no `using` (`plain.go`). The decision is a greatest fixpoint over the declarations its calls reach (D56): each is plain when it would be with the calls inside the group counted as plain, so `isEven` and `isOdd` calling each other are plain, and so is a function that calls them.
- **Two forms, one calling convention.** A plain function without a return annotation, or with an Effect contract, compiles to `double$direct` and `double`, whose type is `(n: number) => Effect<number, never, never>` for TypeScript and in the `.d.ts` (Chapter 15). A function annotated with a plain return type, such as `function double(n: number): number`, keeps that type and compiles to one plain function.
- **Calls.** A call from an `.effeck` file runs inline through `$direct`, at a module's top level too: it computes a value and runs no program. Another module reaches the variant through a named or namespace import, and the lowering adds `import { double$direct } from "./math.js"` next to the original import. A reference that cannot reach it, such as a default import, uses the Effect form, which is always correct.
- **Function values.** Passed where a plain function is expected, `double` is `double$direct`; where Effect expects an Effect function, it is `double`.
- **Methods.** A method that implements a plain function member, such as `Equal.Equal`'s `[Equal.symbol]`, compiles to a plain method; if its body pauses it is EK4301 (D55).

EK4301 (`callbacks.go`) reports a lambda in plain form that pauses, an `.effeck` function value that cannot be plain, and such a method. The candidate that rejected them is TypeScript's argument error with no TypeScript message, so EK4301 is the only diagnostic.

### Callbacks without contracts (D12)

```ts
// mapAll.effeck                                      // mapAll.d.ts
function mapAll<A, B>(                           export declare function mapAll<A, B, E, R>(
  items: ReadonlyArray<A>,                         items: ReadonlyArray<A>,
  f: (a: A) => B) { … }                            f: (a: A) => Effect<B, E, R>): Effect<B[], E, R>;
```

A parameter whose annotation is a function type returning something other than an Effect accepts any contract (`callbackcontracts.go`). The type parameters hook adds `E` and `R` to the function's signature and to the outer type parameters of what it declares, and the parameter type hook gives the parameter the type `(a: A) => Effect<B, E, R>`, so a call to the callback runs it, and each call of the function infers `E` and `R` from its argument. A lambda passed as an argument keeps the types its call gives it. A declaration transformer hook writes the type parameters and the contracts into the `.d.ts`, before the prelude transform imports `Effect`. A call to a generic `.effeck` function, such as `mapAll`, adds the callee's E and R as the call instantiates them.

### Commits

| Change | Subject |
| --- | --- |
| `oozozxtw` | checker: add overload candidate hooks |
| `vvpuorns` | checker: add a function expression type hook |
| `kptnxtrk` | checker: add a contextual return type hook |
| `smmzorwv` | effeck: decide a lambda's form per overload candidate |
| `rokrkspy` | effeck: compile a lambda where an Effect is expected as a program |
| `rnpxuunq` | checker: add an expression type hook |
| `vkznolxv` | effeck: convert function values where Effects are expected |
| `xkssspqy` | effeck: add a case for calling Effect values |
| `xuttxlwl` | effeck: report EK4301 for callbacks that can pause |
| `kmonzlpz` | effeck: decide plain functions as a greatest fixpoint |
| `nmlrmkpo` | effeck: give plain functions a $direct variant and an Effect form |
| `tlvxnnyq` | effeck: let pipe take the program first |
| `okvzuusr` | effeck: hand ch13-plain and ref4-callbacks to the lanes they wait on |
| `pylwouuv` | effeck: report EK4301 for a method that implements a plain member |
| `nwnutlmz` | effeck: read a generic .effeck callee's E and R from the call |
| `uuwrwppk` | checker: add type parameter and parameter type hooks |
| `tmmqumtk` | effeck: let a callback parameter without a contract accept any contract |
| `ztwuloyu` | effeck: keep a lambda that must return a Config plain |
| `wpukknuy` | effeck: look ahead in pipe without checking the first stage |
| `lmsyqkzu` | checker: export the iterated type of an iterable |
| `lzqulkkl` | effeck: convert any iterable of programs for Effect.all |
| `tsvpvvxs` | effeck: let a callback that may return an Effect take either form |
| `pqtovvuk` | effeck: name the collection and callee in EK4301 notes correctly |
| `nzkrouwn` | checker: add an annotated return inference hook |
| `nzplpwps` | effeck: type a plain lambda with a contract as returning A |
| `uqxznpul` | effeck: document the F2 follow-up fixes in EFFECK.md |

## Diagnostics

EK diagnostics are TypeScript diagnostics whose source is `ES`. `tsc` prints them as `main.effeck(7,13): error EK1003: …`, with each fix line indented under the headline, and the language server publishes them with `source: "ES"` and the numeric code, which VS Code shows as `EK(1003)`. The checker reports them while it checks a file, so `tsc`, `tsc --noEmit`, the language server, and `tsc --api` see the same diagnostics. `@ts-ignore` and `@ts-expect-error` do not apply to them, so an `@ts-expect-error` above one is itself reported as unused (TS2578).

`tsc/internal/effeck/diagnostics.go` is the registry: one `Message` for each code in Chapter 28, `EK1001` through `EK4403`, with its default category and the headline the spec prints. A Go test keeps the registry equal to Chapter 28's table. A check is a function in its own file:

```go
func init() { fileChecks = append(fileChecks, checkContracts) }

func checkContracts(c *checker.Checker, file *ast.SourceFile) {
	d := report(c, ret, EK2001, "UserNotFound", "greeting", "its contract says E = never")
	note(d, "Catch it inside greeting, or change the contract to {0}.", contract)
	cover(c, ret) // EK2001 explains TypeScript's TS2322 at this return
}
```

| Name | What it does |
| --- | --- |
| `fileChecks` | Functions that run once for each `.effeck` file a checker checks, after TypeScript's own checks of that file |
| `report(c, node, m, args...)` | Adds a diagnostic with message `m` at `node`'s error span, in the category the project gives `m`'s code, and returns it; returns nil when the project turns the code off |
| `reportAt(c, file, loc, m, args...)` | `report` at a span that is not a node's error span, such as one keyword |
| `note(d, text, args...)` | Adds an indented line, such as a fix, under `d` |
| `cover(c, node)` | Hides TypeScript's own diagnostics inside `node`, because an EK diagnostic there explains them |
| `m.With(text)` | A second headline for `m`'s code, for a situation the registry's text does not fit |

Report only from a file check, and only at nodes of the file being checked: the program reads each file's diagnostics from the checker that checks that file. A check that finds a problem while the checker types another file records it and reports it from the file check of the file that holds the node.

The checks so far:

| Code | Reported at | File |
| --- | --- | --- |
| EK1001 | The outermost call at a module's top level that runs a program; covers TypeScript's TS2339 and TS2348 for a service member call and an Effect value call there | `runners.go` |
| EK1002 | The API, such as `Effect.gen`, when a `function*` is passed where a function that returns a lib `Generator` or `Iterator` is expected; covers the generator, and a producer passed by name with its body | `generators.go` |
| EK1003 | `async`, `await`, `for await`, and `await using` in a `.effeck` file; covers TypeScript's errors about them, such as an `await` outside an `async` function | `syntax.go` |
| EK1004 | `using` that no function encloses, including in a class static block | `syntax.go` |
| EK1005 | The callee of a runner in code a fiber runs | `runners.go` |
| EK2001 | The call or `throw` that adds an expected failure the function's contract does not allow | `contracts.go` |
| EK2002 | The call that adds a requirement the function's contract does not allow | `contracts.go` |
| EK2003 | A return annotation that leaves a contract channel out, or a plain return annotation on a function whose body runs an Effect | `contracts.go` |
| EK2004 | The `catch` keyword, when the `try` block can raise no expected failure (D37); covers TypeScript's errors about the `never` caught value | `failures.go` |
| EK2005 | The program passed to `resumeWith` when it needs a service or fails with an error the operation does not declare (D17); covers TypeScript's TS2345 there | `handlers.go` |
| EK2006 | A `return`, `break`, or `continue` that leaves a `finally` block, which in JavaScript would discard the failure in flight | `regions.go` |
| EK2102 | The name of an Effect function on a call cycle whose A TypeScript cannot infer because it depends on the function's own result, covering TS7022, TS7023, and TS7024 there; a `catch` variable, or a function, on a cycle that the checker typed before the cycle's E and R were known | `recursion.go` |
| EK4201 | A control operation that a combinator runs on another fiber: at its perform site in the argument, else at the argument; and a producer that performs one, passed to a function that runs it as a Stream | `reach.go` |
| EK4202 | The callee, when a function declared outside Effeck declares a control operation in its requirements (D20) | `reach.go` |
| EK4301 | A lambda that can pause where a plain function is expected, naming the first call that pauses and the function that calls it; an `.effeck` function passed by name that can pause; a method that implements a plain function member and pauses (D55). The fix names `Effect.forEach` | `callbacks.go` |
| EK4401 | The callee of a call to a function without a trust level, in code a fiber runs and outside a boundary | `trust.go` |
| EK4402 | Where code runs a platform API (D108): the callee of a call, `new`, or tagged template, or the property read, whose root is a binding imported from a platform module or a platform global; inside boundaries and at the top level too. It covers TypeScript's "cannot find name" for an unresolved platform global it reports | `platform.go` |
| EK4403 | A clock or randomness read, everywhere in a `.effeck` file | `platform.go` |
| EK4404 | A hatch passed Effect's options object | `hatches.go` |
| EK4405 | A hatch callback reads a binding outside the hatch | `hatches.go` |
| EK4406 | A call whose D23-tagged declaration affects emit, in code a fiber runs and outside a boundary; warns that tags cannot survive the emit switch and asks for a `trusted(impl, [Errors])` value (D249) | `trust.go` |
| EK4801-EK4818, EK4821-EK4825 | The rules of Effect's language service (D240-D244): an Effect value a statement discards, an Effect returned from an Effect function, a global `Error` failure, `Layer.mergeAll` dependencies, nested `provide` blocks, an Effect 3 name in place of TS2339, and the rest of Chapter 28's EK48xx rows. One walk per file; quick fixes in `lsrules_fixes.go`, the Effect 3 table in `outdated.go`, and the style rules EK4821-EK4825, off by default, in `lsrules_style.go`. The refactors (`refactors.go`) use `ls.RegisterRefactorProvider`, and the layer hover and the Duration, brand, and directive completions (`lsrules_editor.go`) use `ls.RegisterCompletionHook` | `lsrules.go` |
| EK4801, EK4819, EK4820 | In a `.ts` file outside `node_modules`: a discarded Effect, `yield` without `*` in an Effect generator, `yield*` of a never-succeeding Effect without `return` (D242). `typeScriptFileChecks` runs them, as `fileChecks` runs the `.effeck` checks | `lsrules_ts.go` |

`compilerOptions.effeck` (D34) sets adjustable diagnostic categories (including the EK4406 migration setting) and the tracing mode:

```json
"effeck": { "diagnostics": { "EK2004": "off", "EK4403": "warning" }, "tracing": "exported" }
```

EK2004 and EK4406 default to `"warning"` and EK4403 to `"error"`; the EK48xx rules have their own defaults, and every one but EK4813 is adjustable too (D241), from `core.EffeckAdjustableCodes`, which both `tsoptions` and the registry read. A `Message` with `Off` set reports nothing until a project or a comment turns it on. `directives.go` reads `// @effeck-diagnostics EK4801:off` (to the end of the file) and `// @effeck-diagnostics-next-line EK4801:off` comments for the adjustable codes. Every other EK code is always an error. An unknown key or invalid value is reported at its node in `tsconfig.json` with TypeScript's own messages (TS5023, TS6046). `CompilerOptions.Effeck.Tracing` selects the functions that compile with spans (see "Tracing"). `tsc --showConfig` prints the block; `.tsbuildinfo` does not record it yet, so changing it does not invalidate an incremental build.

## Services and control operations

An abstract class in an `.effeck` file whose instance members are all abstract (methods and `abstract readonly` values), with no type parameters, no `implements` clause, and no `extends` clause, declares a service (D1). With `extends Control` instead, it declares a control operation (D2). Static members are allowed. Any other class is ordinary. The code is `services.go` (what the checker sees) and `services_lower.go` (what the lowering emits).

**What the checker sees.** The class means what this TypeScript declares (D45):

```ts
interface Users {
  find(id: string): Effect<User, UserNotFound, never>
  readonly "~effeck/service"?: "app/users#Users"
}
class Users extends Context.Service<Users, Users> {}
```

The class base hook (`classBases`) gives the class no instance base other than the optional brand, and the static side of `Context.Service<Users, Users>` as its base constructor type. As a value, `Users` is a Context key: `Layer.succeed(Users, …)`, `Effect.provideService`, and `yield* Users` in TypeScript work. As a type, `Users` is the shape: `implements Users` and a parameter typed `Users` mean the members. The brand holds the key, so two services with the same members are different types: providing `Primary` leaves `Replica` in R, and a `Replica` implementation is not a `Primary`. The checker finds `Context.Service` in effect's own declarations (`effect/dist/Context.d.ts`), so a program that does not include effect keeps the class ordinary.

**Keys.** The key joins the nearest `package.json`'s `name` (its directory's name when it has none), the module path within that package without `.effeck`, and the class name: `@acme/app/src/users#Users`. A case directory without a name gives `effeck-services/main#Users`.

**What the lowering emits.** The class extends the runtime's key class, and TypeScript's own transform erases the abstract members:

```ts
// main.effeck                                       // main.js
abstract class Users {                           class Users extends runtime.$service("app/users#Users") {
  abstract find(id: string): Effect<User, …>       static layer = Layer.succeed(Users, …);
  static readonly layer = Layer.succeed(Users, …) }
}
```

A service with `static readonly default` extends `runtime.$reference(key, () => Flags.default)`, a `Context.Reference` (D40); its static side is `Context.Service<never, Flags>`, so using it adds nothing to R. A control operation extends `runtime.$service(key)` and then registers one kernel operation per abstract method with its declared parameter count, `runtime.$control(Approval, ["approve"], [1])`; the runtime's `handle` provides an implementation whose methods perform those operations, and pads a call's arguments to that count, so a clause receives its continuation after every declared parameter even when the call leaves an optional argument out (-1, for a rest parameter, keeps the arguments as given). Generated code imports `effeck-runtime` as a namespace under a free name, as it imports `effect/Effect`.

**Declaration files.** The declaration transform writes a service class as the abstract class it is in source, which TypeScript code could only extend. A declaration transformer hook (`services_declarations.go`) rewrites it to what the checker sees: the instance members move into an interface with the brand, a `Users_base` constant gets the runtime's key type, and the class extends it with only its static members:

```ts
export interface Users {
    find(id: string): Effect.Effect<User, UserNotFound, never>;
    readonly "~effeck/service"?: "app/users#Users";
}
declare const Users_base: import("effeck-runtime").ServiceKey<Users>;
export declare abstract class Users extends Users_base {
    static readonly layer: Layer.Layer<Users, never, never>;
}
```

`ServiceKey<I>` is `(abstract new () => {}) & Context.Service<I, I>`, the type of `$service`; a service with a static default extends `ReferenceKey<I>`, the type of `$reference`, a `Context.Reference`. A default-exported service declares the interface and the class locally, because merged declarations must share their export status, and ends with `export default Users`.

**Member calls (D41).** In an `.effeck` function body, a call `S.member(…)` where `S` is a Context key whose service declares `member` looks the service up, then applies the call rule to the member. The member receiver hook (`memberReceiver`) makes the checker read `member` from the service's shape, so a member the service declares wins over the key's own `pipe`, `of`, and `use`. The rule is structural: it covers `.effeck` services, `Context.Service` classes written in TypeScript, and Effect's own services such as `FileSystem` and `Path`.

| Source | Generated | R gains |
| --- | --- | --- |
| `Users.find(id)`, a member that returns an Effect | `yield* (yield* Users).find(id)` | `Users`, and the member's R |
| `Path.join(a, b)`, a member that returns a plain value | `(yield* Path).join(a, b)` | `Path` |
| `UserRepo.all()`, a member that is an Effect value | `yield* (yield* UserRepo).all` | `UserRepo`, and the member's R |
| `Users()`, the service itself | `yield* Users` | `Users` |
| `FeatureFlags()`, a service with a default | `yield* FeatureFlags` | nothing |

The last three rows are one rule from Chapter 13: in an `.effeck` function body, a value whose type is an Effect, called with no arguments, runs (D24). That covers a Context key or a `Config`, whose variance the `Effect` interface declares; an `Exit`, a union of two Effects, which `outcome()` replays; and a tagged error instance, whose class declares the variance itself, which `error()` fails with. The call signatures hook (`effectValueCall`) gives such a callee the signature `() => Effect<A, E, R>`, built from effect's own `Effect` interface when the callee's type does not name it, and the call expression type hook then types the call as `A`, so `Settings().baseUrl` checks. A lookup makes the enclosing function an Effect even when the member returns a plain value, and `effectOf` adds the service to R.

A lookup also keeps a lambda from compiling to plain JavaScript: `canBePlain` asks `performs`, which is `runs` or a lookup, so `(part) => Path.join(root, part)` is an Effect function.

**Methods.** An object literal method or a class method in an `.effeck` file becomes an Effect when its body runs a call, by the rule that decides functions, or when it is expected to return one (`expectsEffect`): its object literal's contextual type, or the member it implements through its class's `implements` clause, is a function that returns an Effect. So `class UsersFake implements Users { find(id) { return { … } } }` is an implementation even though its body runs nothing. Its inferred return type is `Effect<A, E, R>`. It keeps its name and parameters, and its body runs as a generator bound to the method's `this` (`methods.go`):

```ts
find(id) { return Effect.gen({ self: this }, function* () { … }) }
```

A method's expectation is read once, from the contextual type the checker has when it first asks, whereas a lambda's form is decided per overload candidate (D56).
### Checks of service classes

`services_check.go` reports misuse of service and control classes:

| Code | Reported at | What it reports |
| --- | --- | --- |
| EK2101 | The return type of a service or control method, or its name | A method without a full contract: the annotation must be an `Effect` with all three channels written, such as `Effect<number, never, never>`, or an alias of one. `Effect<A>` leaves channels to defaults, so it is not one. An annotation TypeScript cannot resolve is left to TypeScript's error |
| EK3001 | The class name | Two service or control classes anywhere in the program share a key; a note names the other |
| EK3002 | The `extends` expression | A class extends a service; covers TypeScript's errors about the inherited abstract members |
| EK3003 | The `new` expression | `new Users()`; covers TS2511 |
| EK3004 | The `instanceof` expression | `x instanceof Users` |
| EK3005 | The class name | A fully abstract generic class, which no one key can represent |
| EK3006 | The static member's name | A static member with a service member's name, which `S.member(…)` could not reach |

### Providers

`providers_check.go` compares each implementation member with the service member it implements: every method and function-valued property of an object literal with the type its contextual type gives that member, and every method of a class with the member of the same name in its `implements` clause. When both are functions that return Effects, it reports a failure the contract does not allow (EK2001, at the member) and a requirement the contract does not declare (EK3101, at the call that looks the service up per call), and covers TypeScript's TS2322, TS2345, or TS2416 for the same mismatch at its error spans: the member's name, an arrow member's expression body, and the header of each function that encloses the object and is a call argument. A contract whose E or R holds a type parameter is not checked.

A provider is usually a program that returns an object literal whose members have untyped parameters, typed by the service, as in `Layer.effect(Users, () => { const db = Database(); return { find(id) { … } } })`. Such a lambda is context sensitive, so the first pass of overload resolution skips it and TypeScript types it as `anyFunctionType`, which an Effect parameter does not accept: the candidate failed before the second pass inferred its type arguments, and E and R came out `unknown` without an error. The skipped function type hook (`skipped.go`) gives a lambda with no required parameters, where only an Effect is expected, the non-inferrable `any` type instead; the parameter accepts it and inference ignores it, so the second pass infers E and R from the program.

### wire

`wire(…)` is the runtime's `wire`, imported or from the prelude (D27). `wire.go` reads each argument's `Layer<Out, E, In>` from its `~effect/Layer` variance property and builds the provider graph: an argument needs another when one of its inputs is one of the other's outputs. The runtime folds the arguments with `Layer.provideMerge` from the last to the first, so the lowering emits them consumers first, a topological order that keeps source order among independent arguments; the arguments can be written in any order.

| Code | Reported at | What it reports |
| --- | --- | --- |
| EK3202 | The later of two arguments | Two arguments provide the same service |
| EK3203 | The `wire` call | The arguments form a cycle, printed as `CacheLive → Database → DatabaseLive → Cache → CacheLive` |
| EK3204 | The argument | Its type does not name its outputs: `Out` is `any` or `unknown`, as in `Layer.Any` |
| EK3201 | The runner's program | A runner's program needs a service that the runner does not provide. A root runner (`isRootRunner` in `runners.go`: Effect's `run*` functions, but not their `*With` forms, and `NodeRuntime.runMain` and `BunRuntime.runMain`) starts with an empty Context; `it.effect` provides `Scope` and the services of an enclosing `layer(…)`, which its parameter's type says. Notes name the `wire` arguments that need the service when one does, and otherwise say how to provide it. It hides TypeScript's TS2345 or TS2769 about the argument's type (`coverArgument`), and keeps the errors inside the program |

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
// lookup.effeck                                 // lookup.js
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

`$catch` is the kernel's `catchExpected` and `$finally` is `Effect.onExit`, from `effeck-runtime`, which the file imports as `* as runtime` under a free name. A `return`, `break`, or `continue` that leaves a region cannot jump out of its closure, so the closure returns a completion record: `{ kind: "return", value }`, `{ kind: "break", label }`, or `{ kind: "continue", label }`, with `""` as the label of an unlabeled jump, and the runtime's `$normal` when it reaches its end. The generator that runs the region dispatches each record that can arrive: it returns, breaks, or continues itself, or, when it is a region's closure too and the jump leaves it as well, returns the record to its own runner. A region that no jump leaves runs as `yield* …;` with no record. A closure whose block refers to `this` runs with the function's `this`. Control cannot leave a `finally` block (EK2006): in JavaScript a jump there replaces the block's exit and discards a failure in flight, and Effeck never drops a failure silently (D15). A `var` in a region belongs to the whole function, so the lowering declares it in the Effect function and assigns it where the source declared it.

### Catch expressions (D236-D239)

`parser/effeck_catch.go` reads `catch` after an expression, in `parseAssignmentExpressionOrHigherWorker` before the conditional and in a pipe's body, and writes a call of the runtime's `$catchExpression` whose callee is anchored on the keyword:

| Source | Parses as |
| --- | --- |
| `x catch fallback` | `$catchExpression(() => x, () => fallback)` |
| `x catch (e) => h`, `x catch e => h` | `$catchExpression(() => x, (e) => h)` |

The arrows are Effeck lambdas whose parameter types expect Effects, so they lower to `Effect.fnUntraced` generators and the checker types the call as any runtime call: its value is `A | B` and its E is the handler's. `$catchExpression` runs the left side with `Effect.suspend` under the kernel's `catchExpected`, as `$catch` does for the statement. `effeck/catch_expressions.go` reports EK2114 in a hatch body and EK2004 when `flowOf` finds no expected failure in the left side, gives the keyword its hover, and `runners.go` names the whole expression in EK1001. The hover reaches the keyword, which has no symbol, because `ls/hover.go` runs the quick info hooks before it drops an empty quick info.

### Regions in plain functions

A function that compiles to plain JavaScript cannot pause, so its `try`, `catch`, and `finally` stay JavaScript's (D54). Each of its throws is caught by one of its own `catch` clauses, or a throw could end it and it would be an Effect. A throw there marks the thrown value as an expected failure, and each `catch` unwraps it or rethrows anything else, a defect, so D13 holds:

```ts
// clamp.effeck                                  // clamp.js
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
| `runtimeMember`, `runtimeCall` | Reference `effeck-runtime` helpers, importing the package on first use |

To add a region kind: create a `region`, emit each block with `regionStatements` and `regionFunction`, compose the closures with runtime helpers, and pass the result to `runRegion`. A consumer that handles some jumps itself, as a producer loop handles its own `break` and `continue`, filters `r.jumps` before dispatching.
## Contracts

The return type annotation of an `.effeck` function with a body is the function's contract, `Effect<A, E, R>`: the whole computation, as `Promise<string>` is the whole computation of an async function. `contracts.go` implements the checks of design chapter 14:

| Rule | How |
| --- | --- |
| A function with a contract is an Effect, whatever its body runs | `hasContract` in `analyzer.isEffect`; a lambda's form stays the lambda's position's to decide (D56), so a lambda with a contract in a plain position is plain and returns A (D113) |
| What it returns is checked against A | The unwrapped return type hook gives `checkReturnStatement` and an arrow function's expression body the A of the signature's return type, where TypeScript would check against the whole Effect; a mismatch is TypeScript's own error, such as TS2322. F2's contextual return type hook types the returned expressions by A |
| An expected failure that escapes the body must be in E | EK2001 at the call or `throw` that adds it, naming the failure and the contract, with a fix that widens E. `effectInfo` keeps, for each failure and requirement, the call or `throw` in the function's own body that adds it |
| A requirement must be in R | EK2002 at the call that needs it |
| A channel written `infer Name` comes from the body (D57) | The annotated return type hook completes the signature's return type: A as TypeScript infers a return type (`GetReturnTypeFromBody`), E and R from `effectOf`. The infer type hook keeps `checkInferType` from reporting TS1338 and the unused name; the binder already declares such a name in no scope |
| The declaration file carries the full contract | An annotation with an `infer` channel is not copied: the declaration type annotation hook makes the emitter serialize the signature's return type. A full contract is copied, and the prelude's declaration-file imports make its `Effect` portable |
| A contract that leaves a channel out is EK2003 | `Effect<User>` and `Effect<User, UserNotFound>`, with both fixes: the missing channels as `never`, or as `infer` |
| A plain return annotation keeps its type on a function that compiles to plain JavaScript (D67) | On a function whose body runs an Effect, throws past its own `catch`, or makes a trusted call whose failures escape, it is EK2003. Methods are exempt: a method that implements a plain-function member is declared with a plain type (D55) |
| A caller sees the contract, not the body | `addCall` reads a callee's E and R from its contract when the contract infers neither; a callee whose contract infers a channel is read through its body, which never asks for a return type that a call cycle may be computing |

```text
main.effeck(14,20): error EK2001: UserNotFound can escape greeting, but its contract says E = never.
  Catch it inside greeting, or change the contract to Effect<string, UserNotFound, never>.
main.effeck(15,3): error EK2002: welcome needs Mailer, but its contract says R = Users.
  Provide Mailer inside welcome, or add it: Effect<string, MailError, Mailer | Users>.
main.effeck(6,35): error EK2003: Effect<string> leaves contract channels unspecified. Did you mean never or inferred?
  Write Effect<string, never, never> to check the missing channels as never.
  Or write Effect<string, infer E, infer R> to infer them from the body.
```

### Recursion (D52)

E and R of a function on a call cycle are a fixpoint over the cycle's strongly connected component, computed inside the component only (`recursion.go`). `effectOf` solves a function's component with Tarjan's algorithm over the call graph whose edges are the `.effeck` callees a body reads (`bodyCallees`: a callee whose contract states E and R, or a generic callee the call instantiates, is no edge). A component is solved after every component it calls. Its members start with empty E and R and are read again, with the E and R found so far, until none grows; the rows are finite and only grow, so the iteration ends. While a component is being solved, `effectOf` returns a member's approximation, and `tryBlockFlow` reads a member's `try` blocks again in each round. In `recursion`, `isOdd` and `isEven` each learn the other's failure only through the cycle, and both get `Effect<boolean, Even | Odd, never>`.

A stays TypeScript's to infer. When it depends on the function's own result, TypeScript reports TS7023 and types the function `any`; on an Effect function that is EK2102 instead, at the function's name, whose fix writes the contract with its success type and `infer` for E and R. A pure recursive function compiles to plain JavaScript, so TS7023 and a plain annotation stay TypeScript's (D67).

The checker caches types, so one it computes while a component is being solved cannot change afterwards. Two such types are checked once the component is solved, and a mismatch is EK2102, because a written contract on the cycle breaks the dependency: a `catch` variable whose `try` block calls into the cycle, and a member's own return type. `ek2102-cycle-catch.fail` shows the first; writing the contract of the function the `try` block calls makes it compile.

A provider's members are checked against the service they implement by `providers_check.go`, which reports EK2001 and EK3101 at the member; a member with its own contract is also checked against that contract here. The two checks compare different contracts, so one mismatch is reported once.

## Scope regions: using and fork

A block of an Effect function that declares `using` or calls `fork` is a Scope region (D14, D19, D47). It runs under a Scope of its own, which only its `using` acquisitions and the fibers it forks receive, and which closes when the block exits by any path: success, failure, or interruption. Closing runs the releases and interrupts the forked fibers in reverse order, and waits for each. `scopes.go` decides what the checker sees, and `scopes_lower.go` emits the regions:

```ts
// serve.effeck
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

A function that declares `using` compiles to frames (D54), even when it runs nothing else, so it never compiles to plain JavaScript. A lambda with `using` passed where a plain function is expected, such as to `Array.prototype.map` or `Effect.sync`, is EK4301: `this callback can pause (using t)`.

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
| `utpqokps` | effeck: count only using declarations as using |
| `ssqqsmwt` | checker: add a using initializer hook |
| `rmkrouvo` | runtime: register a Disposable's release with interruption masked |
| `xvsrqxyz` | effeck: lower using to Scope regions |
| `twvqsuom` | effeck: give each fork to the block that calls it |
| `nzuukwkz` | effeck: take each for...of value with using |
| `qsytkrtw` | effeck: document Scope regions, using, and fork in EFFECK.md |
| `rkxpuynr` | effeck: keep this in a lowered arrow function |

## Handlers and producers

Control operations handled with one-shot delimited continuations, and producers built on them, are the part of Effeck that Effect does not have (design chapters 8, 9, and 26). `handlers.go` and `reach.go` hold the handler rules, `producers.go` holds producers and loops over Streams, and `packages/runtime/src/handlers.ts` and `producers.ts` are their runtime.

### Handlers (D16-D18, D50)

`handle`, `resume`, `resumeWith`, and `abandon` are the runtime's functions, and their TypeScript signatures type a handler. The compiler adds one lowering rule, the tail resume, and three checks:

```ts
// approval.effeck                                  // approval.js
function approved() {                           const approved = Effect.fnUntraced(function* approved() {
  return handle(() => checkout("c-42"), Approval, {   return yield* handle(Effect.gen(function* () { return yield* checkout("c-42"); }), Approval, {
    approve(request, k) {                               approve(request, k) {
      const decision = Reviewers.decide(request)          return Effect.gen(function* () {
      return resume(k, decision)                            const decision = yield* (yield* Reviewers).decide(request);
    }                                                       return runtime.$tailResume(k, decision);
  })                                                      });
}                                                       }
                                                      });
                                                    });
```

- **Types.** `handle(program, Control, clauses, onReturn?)` takes the program as an `Effect<A, E, R>`, so a lambda there is a program (D7). It needs a clause for every operation of the control, because it removes the whole control from R. A clause's `k` is a `Continuation<V, E, H>` (D50): V is what the perform site receives, E the operation's declared errors, and H what `resume` returns. The `handle` call is `Effect<A | each clause's A, E | each clause's E, Exclude<R, Control> | each clause's R>`, with `onReturn`'s result in place of A when it is given. H is the program's own A, or `never` with `onReturn`, because TypeScript fixes H before it checks `onReturn` (D97).
- **Clauses** compile as any method or lambda expected to return an Effect: an object literal method becomes an Effect method (`expectsEffect`), and a lambda an Effect function (D56). The runtime calls a clause with the operation's arguments and `k`.
- **Tail resume (D18).** In a clause's own frame, `return resume(k, v)` and `return resumeWith(k, p)` return the kernel's request, `runtime.$tailResume(k, v)` and `runtime.$tailResumeWith(k, p)`, and so does a lambda clause whose expression body is the call. The clause's generator finishes, and then its Clause frame resumes the program, so a clause that resumes 100,000 times stays under 50 frames (cases `handlers` and `stack-depth`). The plan records each clause (`planClause`) and each call of `resume` or `resumeWith` (`planResume`); the lowering emits the request only where `frame.clause` holds (`resumesInTail`). A region's closure is a frame of its own, so inside a `try`, `catch`, `finally`, or Scope region of the clause, `resume` runs as any call does (`yield*`), and the region's cleanup runs after the resumed program. A clause whose body is itself a Scope region, because it declares `using` or forks, has no tail resume either: its body's closure keeps the clause's frame, but what it holds must be released after the resumed program.

The checks:

| Code | Reported at | Rule |
| --- | --- | --- |
| EK2005 | The program passed to `resumeWith` | It runs at the perform site, inside the handled program, so it may need no service and may fail only with the operation's declared errors (D17). The runtime's signature rejects the same program as TS2345, which EK2005 covers (`checkResumeWith`) |
| EK4201 | The perform site in an argument, such as `Approval.approve(id)`, or the argument | A combinator that runs the argument on another fiber, where this fiber's handler cannot be reached: effect's `forkChild`, `forkDetach`, `forkIn`, `forkScoped`, `race`, `raceAll`, `raceAllFirst`, `raceFirst`, `timeout`, `timeoutOption`, and `timeoutOrElse`; any call of the `Effect` or `Stream` module with a `concurrency` option other than `1`; `zip` and `zipWith` with `concurrent: true`; and the prelude's `fork`. It reads the requirements of each argument, an Effect, a function that returns one, or an array or record of them, so a branch that handles the operation inside itself passes (`checkForkedArguments`). A producer whose requirements hold a control operation, passed to any function, is EK4201 too, because run as a Stream it runs on a fiber of its own (`checkProducerArguments`) |
| EK4202 | The callee | A function declared outside Effeck whose declared return type requires a control operation, because its body can fork where no table sees it (D20). Only the declared type counts, so a generic combinator such as `Effect.forEach`, whose R is a type parameter, is left to EK4201 (`checkTypeScriptCallee`) |

A control type is the type of a class that `classKindOf` calls a control class; `runtimeExport` knows `handle`, `resume`, `resumeWith`, and `fork` by their declarations in the runtime package, through an import, a namespace, or the prelude.

### Producers (D26)

A `function*` declaration or function expression in an `.effeck` file is a producer (`isProducer`). Its body is an Effect body, as any `.effeck` function's is, so its calls run; `yield v` performs the built-in Yield operation, and `yield* p` yields every element of another producer, a Stream, or an iterable. Calling a producer runs none of its body: it returns a Stream that carries the body. Generator methods, `*items() { … }`, stay JavaScript's generators.

```ts
// lines.effeck                                 // lines.js
function* lines(path: string) {             function lines(path) {
  const file = Files.open(path)                 return runtime.$produce(Effect.fnUntraced(function* lines() {
  while (true) {                                    const file = yield* (yield* Files).open(path);
    const line = file.readLine()                    while (true) {
    if (line === undefined) return                      const line = yield* file.readLine();
    yield line                                          if (line === undefined) return;
  }                                                     yield* runtime.$yield(line);
}                                                   }
                                                }));
                                            }
```

- **Types.** A producer returns `Stream<T, E, R>` (`producerType`, through the inferred return type hook): T is the type of its yields, widened as a return type is, so `yield 1` yields `number`, and E and R are what its body can fail with and needs. To TypeScript callers and `.d.ts` readers it is a function that returns a Stream, and Stream's own functions take it directly. A call to a producer does not run: it constructs the Stream.
- **Lowering.** The producer stays a plain function with its parameters, and its body becomes a generator named after it (`visitProducer`, `generatorExpression`). `yield v` becomes `yield* runtime.$yield(v)`, and `yield* p` becomes `yield* runtime.$yieldAll(p)` (`visitYield`). The body's `try`, `finally`, and Scope regions lower as in any Effect function.
- **Loops.** `for...of` over a value whose type is a Stream, a producer or any other, in an `.effeck` function's body is a loop over its elements (`isStreamLoop`); over any other iterable it is JavaScript's loop. The function can pause, so it compiles to an Effect (`hasStreamLoop`, which `isEffect`, D10's fixpoint, and `canBePlain` ask), and the Stream's E and R join the function's (`addStreamLoop`, from `flowOf`). The loop body is a region (`visitStreamLoop`) whose closure, `<function>$for`, takes the element:

```ts
for (const line of lines(path)) {      const completion = yield* runtime.$forOf(lines(path), Effect.fnUntraced(function* first$for(line) {
  if (line.startsWith("ERROR"))            if (line.startsWith("ERROR"))
    return line                                return { kind: "return", value: line };
}                                      }));
                                       if (completion.kind === "return") return completion.value;
```

`runtime.$forOf(iterable, body, label?)` handles the body's `break` and `continue` of its own loop, unlabeled or with the loop's label, and returns every other jump for the enclosing generator to dispatch; a body that no jump leaves returns nothing, which counts as normal completion. A binding pattern, an existing variable, and a `var` are each a loop target. A labeled loop passes its label to `$forOf` and loses the label statement.

- **How the runtime iterates.** Over a producer, `$forOf` runs the producer's body under a Yield handler on the loop's fiber, and the loop body is the clause, which resumes the producer in tail position, so 100,000 elements stay under 50 frames. `break`, `return`, a jump to an outer label, and a failure in the body return from the clause without resuming, so the producer's cleanup runs before the loop exits. Over any other Stream, `$forOf` pulls it with `Stream.toPull` and closes it when the loop exits.
- **The Stream a producer is.** Anything else that runs a producer as a Stream, such as `Stream.take`, starts its body on a fiber of its own on the first pull. Its Yield clause completes the pending pull and waits for the next before it resumes the body: a handoff with no buffer, so the producer never runs ahead of its consumer, and a pull from any fiber works. Closing the stream interrupts that fiber, which abandons the producer and runs its cleanup. Case `producers` shows `Stream.take(numbers(100), 2)` logging two yields and the close, nothing more.
- **Checker seam.** A Stream has no `[Symbol.iterator]`, so the iterated element type hook (`iteratedElementType`) gives a `for...of` over a Stream, and a producer's `yield*` of one, the Stream's element type before TypeScript asks for an iterator. EK1002 also covers TypeScript's errors in a producer passed by name to `Effect.gen` or another generator-based API, whose body was written for that API.

### Commits

| Change | Subject |
| --- | --- |
| `lzvlmlou` | runtime: type continuations as Continuation<V, E, H> |
| `xqrnzlyn` | effeck: compile a clause's tail resume to the kernel's request |
| `pnvzvzmo` | effeck: report EK2005 for resumeWith programs outside the operation's contract |
| `susnvvyx` | effeck: report control operations that cannot reach their handler |
| `lxxwsyuu` | checker: add an iterated element type hook |
| `znpynsvy` | runtime: make a producer a Stream with a zero-capacity handoff |
| `tnnzwswk` | effeck: compile function* to producers and loop over Streams |
| `yxpkoupr` | examples: close the pipeline's producer after its last alert |
| `lynmrvwz` | docs: remove the stream page's producer compiler gap |
| `moutqkun` | effeck: document handlers and producers in EFFECK.md |

## The prelude

Every `.effeck` file sees these names without an import (D4, D48): `Effect`, `Schema`, `Layer`, `TaggedError`, `Control`, `Scope`, `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, and `abandon`. They are the exports of `effeck-runtime`, whose `prelude.ts` gives `Effect` and `Scope` two meanings each: the effect module, for `Effect.gen` and `Scope.make`, and a type, so `Effect<A, E, R>` means `Effect.Effect<A, E, R>` and `Scope` means `Scope.Scope`.

| Step | How | Where |
| --- | --- | --- |
| Load the runtime | A `.effeck` file with an identifier that has a prelude name, and no top-level declaration or import of that name, resolves `effeck-runtime` as an implicit import, as a file with `importHelpers` resolves `tslib`. A file that uses no prelude name compiles exactly as TypeScript would | `implicitRuntimeImport`, through `compiler.RegisterImplicitImportHook` |
| Resolve a name | When no scope declares a prelude name, the checker resolves it to the runtime's export before it searches the global scope. A local declaration or import therefore shadows it, and `.ts` files never see it | `resolvePreludeName`, through `checker.RegisterNameResolutionHook` |
| Import it | The checker records each prelude name a file uses as a value outside a type. The lowering adds `import * as Effect from "effect/Effect"` for the effect modules (`Effect`, `Layer`, `Schema`, `Scope`) and one `import { TaggedError, pipe, … } from "effeck-runtime"` for the rest | `preludeUses`, `preludeImports`, `lowering.visitSourceFile` |

A name used only in types, such as `Scope` in `Effect<void, never, Scope>`, is not imported.

Every other export of effect's root is in scope too (D146, `effectroot.go`): its other 135 namespaces, such as `Console`, `Option`, `Stream`, and `Schedule`, and its functions `absurd`, `cast`, `flow`, `hole`, and `identity`. A file that may use one resolves `effect` as an implicit import, the checker resolves the name to the root's export after the prelude, and the lowering imports it from its own module (`import * as Console from "effect/Console"`, `import { flow } from "effect/Function"`), never from the root, which would load every module. A global value that an ECMAScript lib file declares wins (`isBuiltinGlobal`), so `Array`, `String`, `Number`, `Boolean`, `Symbol`, `BigInt`, `Function`, and `RegExp` stay JavaScript's; a host global such as the DOM's `Option` does not. Declaration files import root names from effect: `import type { Chunk, Option } from "effect"`. Fourteen functions of the Effect module are in scope without the prefix too (D151, `effectFunctionNames`): `all`, `die`, `forEach`, `log`, `logDebug`, `logError`, `logInfo`, `logWarning`, `race`, `repeat`, `retry`, `sleep`, `timeout`, and `withSpan`, resolved through the root's `Effect` and imported from `effect/Effect` (case `effeck-bare-functions`). `TestEffectRootNamesMatchEffect` checks the table against the installed package, and case `effeck-root-names` covers the rules. The emitted imports are ES module imports: `.effeck` files compile to ES modules. When `effeck-runtime` is not installed, a prelude name is TypeScript's TS2304 (cannot find name).

A declaration file is ordinary TypeScript, so the `.d.ts` of a `.effeck` file imports the prelude names its declarations write: `import type { Effect, Scope } from "effeck-runtime"`. Declaration emit asks the entity name visibility hook whether each name it writes is visible; `preludeNameVisible` says yes for a prelude name and records it, and `transformDeclarations`, a declaration transformer hook, adds the import (`declarations.go`, case `effeck-prelude-dts`). A file that also binds the name itself, as an Effect user's `import { Effect } from "effect"` does, keeps that import in its declaration file, so the prelude's type is imported under a free name, `import type { Effect as Effect_1 } from "effeck-runtime"`, and only the references the hook allowed are renamed (case `effeck-prelude-dts-shadow`, whose TypeScript consumer compiles with `skipLibCheck` off).

## Imports between files

A `.effeck` file imports another module by its source name, and the emitted JavaScript imports the output (D31):

| Source | Emitted JavaScript | Resolves to |
| --- | --- | --- |
| `import { welcome } from "./welcome.effeck"` in `.effeck` | `from "./welcome.js"` | `welcome.effeck` |
| `import { formatName } from "./format.ts"` in `.effeck` | `from "./format.js"` | `format.ts` |
| `import { welcome } from "./welcome.js"` in `.ts` | unchanged | `welcome.effeck`, as `./format.js` resolves to `format.ts` |

- **Resolution.** The module resolver tries `welcome.effeck` for `./welcome.effeck`, and after `.ts` and `.tsx` for `./welcome.js` and an extensionless specifier, so `.ts` beats `.effeck` when both exist. An `.effeck` resolution is always allowed, like a `.ts` one (`module/resolver.go`, `module/util.go`).
- **Extensions in specifiers.** A `.effeck` file may name a `.ts` or `.effeck` source without `allowImportingTsExtensions` (no TS5097), because its emitted JavaScript never keeps the source name (`core.AllowImportingTsExtensionsFrom`). A `.ts` file follows TypeScript's rules.
- **Emit.** The lowering rewrites the specifier of every import and export declaration in a `.effeck` file that names a TypeScript or Effeck source, with or without `rewriteRelativeImportExtensions` (`imports.go`). The `.d.ts` of a `.effeck` file names outputs too, including in the `import("./users.js")` types the checker writes for inferred types, so stock TypeScript can read it (`declarations.go`); and `./users.effeck` resolves to `users.d.ts` when only the declaration file exists, as `./users.ts` does. Dynamic `import()` is not rewritten: it returns a Promise, which `.effeck` code does not await.

Case `effeck-imports` covers an `.effeck` file calling an `.effeck` Effect function and a pure helper, and a `.ts` file calling an `.effeck` function that `.effeck` code then runs; `effeck-imports-norewrite` covers a project without `rewriteRelativeImportExtensions`.

## The runtime

Generated JavaScript imports only `effect` and `effeck-runtime` (D30): the prelude names it uses, `effect/Effect` for `Effect.fnUntraced` under a free name (`Effect`, else `Effect_1`, …), and the helpers as one namespace under a free name (`runtime`, else `runtime_1`, …), called as `runtime.$catch(…)`. The runtime's exports are the ABI between compiler and runtime, so a compiler change that needs a new helper adds it to `packages/runtime` in the same commit; `smoke.sh` builds the runtime from source and every case links it, so the commit is tested as one. Case `effeck-runtime-name` declares `runtime` and `Effect_1` itself and still runs.

## Boundary hatches (D148-D150, D229-D232)

`sync`, `try`, and `callback` are prelude names whose first argument is TypeScript, not Effeck (`hatches.go`). That TypeScript is self-contained (D229): values of the `.effeck` file reach it as inputs, the arguments after it, which fill its first parameters (D230); `try`'s error mapper stays second (D231); the mapper and the inputs are Effeck (D232). A hatch is a call whose callee is one of those names with no declaration in any scope around it (`shadowed` reads the binder's locals); `try` is a keyword nothing can declare.

```ts
// main.effeck                                         // main.js
const id = sync(crypto.randomUUID())                   const id = yield* Effect.sync(() => crypto.randomUUID())
const page = sync(async () => (await load()).text())   const page = yield* Effect.promise(async () => (await load()).text())
const json = try((t) => JSON.parse(t), toBadJson, text)
                                                       const json = yield* Effect.try({ try: ((t) => JSON.parse(t)).bind(undefined, text), catch: toBadJson })
const body = try(async (id, signal) => fetchBody(id, signal), id)
                                                       const body = yield* Effect.tryPromise(((id, signal) => fetchBody(id, signal)).bind(undefined, id))
```

- **Parsing.** `parser/effeck_hatches.go`: in an `.effeck` file, `try` followed by `(` starts an expression; the callee parses as an identifier named `try`. Seams in `parser.go`: the statement switch, `isStartOfLeftHandSideExpression`, and `parsePrimaryExpression`.
- **Typing.** The runtime declares the overloads (`packages/runtime/src/hatches.ts`): the inputs are a rest parameter `...args: Args`, and the function's first parameters are `Args`, so TypeScript types each parameter from its input. Each overload names its first parameter for the compiler, `evaluate` or `value`, and a mapper overload names its second `mapError`. `hatchFormOf` reads the names from the resolved signature, and `isAsyncHatch` reads whether the function is `async` or returns a `then`able, so `sync(fetchText())`, whose value is a Promise, lowers to `Effect.promise`.
- **TypeScript inside.** `inHatch` makes a hatch's first argument TypeScript everywhere the compiler asks: `isLowerable` and `inLowerableBody` are false there, so calls do not run and an `.effeck` function's call is an inert Effect; `runs`, `directCallee`, and `flowOf` skip them; EK1003 (`async`, `await`), EK4401, EK4402, EK4403, and EK1005 do not apply. `forEachHatchEffeck` visits the mapper and the inputs, where they do.
- **Lowering.** `visitHatchCall` emits Effect's function under the module's `Effect` name, wraps a single expression in `() =>`, binds inputs with `.bind(undefined, …inputs)`, so Effect's signal lands after them, and turns a mapper into `{ try, catch }`. `callback` stays a call of the runtime's `callback`, which adapts `resume(value)`, `resume.fail`, and `resume.exit` to Effect's `resume(effect)` and runs a returned function on interruption. The emitted JavaScript never imports `sync` or `try`.
- **EK4404.** `try({ try, catch })` is an error: the object would be the hatch's value.
- **EK4405.** `outsideReferences` finds each identifier and shorthand property in the first argument whose value symbol the file declares outside it; imports, globals, prelude names, types, and `TaggedError` classes are exempt. The quick fix `hatchInputFix` adds the names as parameters after the existing inputs' parameters and passes them after the last argument. `hatchCall` writes the same form for EK4401's notes. Hover on a hatch's callee names its inputs.

Cases: `effeck-hatches` (every form, an inert Effect, `setTimeout` in a hatch, `runPromise` back in), `effeck-hatches-shadow` (a parameter named `callback`), `hatches-inputs` (inputs for every form, a call among them, a mapper with inputs, `Resume`), `ek1005-prelude-runner.fail`, `ek4404-try-options.fail`, `ek4405-self-contained.fail`, `hatch-inputs-effeck.fail` (EK4401 and EK4403 among inputs). The LSP smoke applies EK4405's quick fix.

## Method syntax on Effect data types (D152, D153)

A value of one of Effect's data types takes its module's data-first functions as methods (`datamethods.go`). `x.name(args)` checks and compiles as `Module.name(x, args)`:

```ts
// main.effeck                                    // main.js
const alerts = readings().filter(hot).take(2)      const alerts = Stream.take(Stream.filter(readings(), hot), 2)
const label = maybe.map(format).getOrElse(none)    const label = Option.getOrElse(Option.map(maybe, format), none)
const done = fiber.join()                          const done = yield* Fiber.join(fiber)
```

- **Lookup.** The checker asks missing property hooks only when a property access finds no member, so a type's own members always win: an Effect's `pipe`, an array's `map`. `moduleMethod` answers in `.effeck` files outside optional chains: `homeModule` finds the module whose declaration file, `effect/dist/<Module>.d.ts`, declares the receiver's apparent type, and every member of a union must come from the same module. Eighteen modules qualify (`methodModules`).
- **Typing.** The method is a property whose type has one call signature for each overload of `Module.name` that takes the value first: its first parameter is named `self`, or ends in `OrOptions`, as in `Stream.flatten`'s conditional dual. `SignatureWithThisFromFirstParameter` moves that parameter to `this`, so TypeScript checks the receiver against it and infers type parameters from it, and turns a type predicate on it into `this is …`, so `if (maybe.isSome())` narrows `maybe`. `declaredSignature` (`expected.go`) gives such a candidate the same shifted parameters, so lambda forms (D44) read the right parameter.
- **Lowering.** `plan.methods` records each call through a method; `visitModuleMethodCall` emits `Module.name(receiver, args)`, run with `yield*` where the call runs. `Module` is the prelude's import when the module's name means it at the call (`namesPreludeModule`, which records the prelude use); under a local declaration of that name, the lowering adds its own namespace import under a free name, such as `Option_1`.
- **Language service.** Hover on a method's name says which function the call reaches (`moduleMethodQuickInfo`); completion after a dot adds the module's methods to the receiver's members (`moduleMethodCompletions`, through `RegisterMemberCompletionHook`).
- **EK4501.** A method access that is not a call's callee is an error (`checkModuleMethods`): no function value exists to bind.

Cases: `effeck-methods` (Stream, Option, Duration, Chunk, Fiber, and Ref chains; `pipe` and arrays keep their members; `isSome` narrowing; a shadowed module name), `ek4501-method-value.fail`, `methods-optional-chain.fail`. The LSP smoke checks a hover and a completion.

## The pipe operator (D154-D156)

`x |> body` evaluates `body` with the topic `%` standing for `x` (`pipes.go`, `pipes_lower.go`):

```ts
// main.effeck                                      // main.js
const n = 9 |> addMargin(%) |> double(%)             const n = double$direct(addMargin$direct(9))
const r = flaky() |> retry(%, Schedule.recurs(5))    const r = yield* retry(Effect.gen(function* () { return yield* flaky() }), Schedule.recurs(5))
const l = user |> `${%.first} ${%.last}`             const l = (topic_1 = user, `${topic_1.first} ${topic_1.last}`)
```

- **Parsing.** `parser/effeck_pipes.go`: in an `.effeck` file, `|` directly followed by `>` ends a binary expression, and after a conditional expression starts a pipe, a `BinaryExpression` flagged `NodeFlagsEffeck` whose operator token is the `|`; the `>` belongs to the pipe as a token of no child, so every token node stays one scanner token for the formatter. The body is a conditional expression, and the loop makes pipes left-associative. `%` where an expression starts is an identifier named `%` flagged `NodeFlagsEffeck`; after an operand it is still the remainder operator.
- **Typing.** The checker gives a pipe its body's type and its body the pipe's contextual type (`extensions_pipes.go`). `resolveTopic`, a name resolution hook, resolves `%` to a property symbol whose type is the type of the left side of the innermost pipe whose body holds it, or to the error type outside every body (EK1007).
- **By name.** A topic is an argument like any other, so the conversion hook classifies its position; `byNameSubject` makes `mayRunByName`, `pauses`, `byNameType`, and `valueTypeOf` read the pipe's left side for it. A pipe whose topic is passed by name passes its left side: `isByName` and `byNameOf` say so for the left side, so `flowOf` skips it, EK1001 does not apply to it at a module's top level, and a hover on a call in it names the program's receiver (`byNameOwner`). Mixed uses are EK1008; a body without `%` is EK1006.
- **Lowering.** `planPipe` picks one of three forms. By name, the pipe compiles to its body, and `byNameProgram` turns each topic into a program that evaluates the left side. A topic used once, which `topicFirst` shows is evaluated unconditionally and after nothing with an effect, becomes the left side itself. Otherwise `visitPipe` assigns the left side to a hoisted `topic` variable in a comma expression, and `visitTopicClosure` lowers each function in the body that reads the topic inside `((topic_2) => function)(topic_1)`, so the function keeps its evaluation's value.
- **Language service.** Hover on `%` shows `(topic) %: T` and either "The value of `x`, which runs once" or "Passed by name to `retry` as a program" (`topicQuickInfo`). The formatter treats the two characters of `|>` as one operator (`format/rulecontext.go`).

Cases: `effeck-pipes` (values, by name, chains, closures, `% % 2`, nested pipes, a hatch, a top-level runner, a forked pipe), `pipe-missing-topic.fail`, `pipe-topic-outside.fail`, `pipe-mixed.fail`. The LSP smoke checks both hovers and that formatting keeps `|>`.

## Fiber forms (D166-D171)

`parser/effeck_fibers.go` writes each fiber form as calls the rest of the
compiler already understands, so the checker, the analyzer, and the lowering
need no new node kinds:

| Source | Parses as |
| --- | --- |
| `fork getUser(id)` | `fork(getUser(id))`: the prelude's fork, owned by the block (D95) |
| `fork { ... }` | `fork(() => { ... })` |
| `join x` | `$join(x)`, a call that runs: the fiber's value and its failures (D225) |
| `await x` (outside `async`) | `$await(x)`, kept only so that EK4609 and EK4610 name the replacement (D226) |
| `await.all x`, `await.allSettled x`, `await.race x`, `await.any x` | `$awaitAll(x)`, `$awaitAllSettled(x)`, `$awaitRace(x)`, `$awaitAny(x)` (D196); the callee stands on `await` alone |
| `select { case const v = a(): S ... default: D }` | `$selectN: { const $selected = $select(hasDefault, () => a(), ...); ... }`, kept only so that EK4607 names its replacement (D197) |
| `for concurrent(n) (const x of xs) S` | `$forConcurrent(xs, n, (x) => { S })` |

The synthesized parts are anchored on the keyword they stand for, flagged
`NodeFlagsReparsed`; the helper calls are known by their synthesized callee, not
by `NodeFlagsEffeck`, which marks provide blocks and schema structs. In a select, an
unlabeled break that would leave it gets the block's label; in a `for
concurrent` body, an unlabeled continue becomes a synthesized return and an
escaping break is flagged for EK4605. `effeck/fibers.go` reports EK4601-EK4612
(EK4607 on a select, EK4608 on a race over an object, EK4609 and EK4610 on a
plain `await`, whose quick fixes write `join x` and `x.await()`, EK4611 and
EK4612 on a `join`) and gives `join`, the await operations, and `for
concurrent` hover text; the runtime's `fibers.ts` implements `$join` (fibers
only) and `$await`, the await operations (`$awaitRace` races through
`$select`, forked with `startImmediately` so an element ready at start wins in
written order; `$awaitAll` and `$awaitAllSettled` put an object's keys back in
the order written), `$forConcurrent`, `detach`, and `forkIn`. An await
operation's elements are by-name programs because each helper's parameter
expects Effects (the union `AwaitElement` includes `Fiber` and `Deferred`, so
those pass as values). A numeric literal synthesized at another token prints its own text
(`scanner.GetSourceTextOfNodeFromSourceFile`), which the select's indices need.

Cases: `fibers-fork-join` (`join` on a fiber, an array, and an object; `fork { }`; the call form; `d.await()`; a failure at the join; fail fast; `detach`; `forkIn`), `fibers-join-identifiers` (a function named `join`, `words.join`, `join[0]`, and a member named `join` beside the keyword), `fibers-await.fail` (EK4601-EK4604 and EK4609-EK4612), `ek4602-await-promise.fail`, `fibers-for-concurrent`, `await-ops`.

## Primitives with their own syntax (D176-D180)

`parser/effeck_primitives.go` and `effeck/primitives.go`. A catch clause typed `Cause` puts the try statement's catch in cause mode: the parser makes the catch variable `$cause`, starts the catch block with `const $caught = $failureOf($cause)` so typed and untyped clauses before the Cause clause test the expected failure it holds (`$noFailure` when it holds none), and binds the Cause clause's name to `$cause`; the catch variable hook types `$cause` as `Cause<E>` of the try block, and the lowering wraps the try block with the runtime's `$catchCause` instead of `$catch`, which lets an abandonment's sentinel interrupt (`K.isAbortCause`) unwind through. `throw` of a value whose type has the `~effect/Cause` property lowers to `$failCause` (the plan's `causeThrows`), and `thrownType` adds the Cause's E. `finally (exit)` becomes the finally block with `const exit = $exit()` in front: the call type hook types `$exit()` as `Exit<void, E>` by instantiating the declared `Exit<void, unknown>`'s Success and Failure members with the try statement's E, and the lowering drops the declaration and passes the binding as the parameter of the finally closure under `$finallyExit` (Effect.onExit, success normalized to `Exit.void`). An interruption block is a Block whose first statement calls `$uninterruptibleBlock()`, `$restoreBlock()`, or `$interruptibleBlock()`; the lowering runs the rest as a region under the runtime's `uninterruptible`, `restore`, or `interruptible`, the same prelude functions a call with a program uses. `restore` reads the innermost region's restore from the `effeck/CurrentRestore` fiber reference that `uninterruptible` provides. `$exit` and the markers are recorded as prelude names for resolution only (`isRemovedPrimitiveMarker`), so no emitted file imports them. Any of these statements makes its function an Effect (`hasPrimitiveStatement`). Cases: `catch-retired`, `primitives-finally-exit`, `primitives-bare`, `primitives-masks`, `primitives.fail`.

## Code that is not Effect

A call to a function that returns no `Effect` needs a trust level (D5, D22, D23). `trust.go` classifies the callee by the declaration its signature resolves to:

| Callee | Trust | A bare call |
| --- | --- | --- |
| An `.effeck` function, or a parameter of an `.effeck` function | Effeck's own | Allowed |
| A declaration in TypeScript's lib files: `String`, `Array`, `Math`, `JSON`, `Map`, `RegExp`, `URL`, … | A built-in with a shipped contract (D5) | Allowed; the contract's exceptions join E |
| An export of `effect`, `@effect/*`, or `effeck-runtime`, and the functions and values those return | Trusted `sync` (D22); exceptions are defects | Allowed |
| A declaration tagged `/** @effeck trusted */` | Trusted `sync` (D23); each `@throws {T}` joins E | Allowed |
| A declaration in a platform package (`@types/node`, `bun-types`) | Platform | EK4402 reports the import or global instead |
| Anything else | None | EK4401 |

A package's name comes from the nearest `package.json` (`packageOf` in `origins.go`), so `effeck-runtime` installed through a `file:` link, outside `node_modules`, is still recognized.

An Effect package may re-export another package's functions: `@effect/vitest` re-exports vitest, so `assert` and `expect` are declared in vitest's files. A call whose callee a declaration file declares is still Effect's when the callee is reached through an import from an Effect package, or through a value that a trusted call to one returned (`reachedThroughEffect`): `assert.strictEqual(n, 1)` and `expect(n).toBe(1)` are trusted calls whose exceptions are defects, so a failed assertion fails the test. The program's own TypeScript keeps its rule even when an Effect function returned the value. For another package's function, EK4401's note suggests a tagged re-export from a `.ts` file (D23), because the package's own declaration cannot carry the tag.

**The migration (D249).** A D23-tagged call reports EK4406 where its contract changes the emitted failure handling: in code a fiber runs, outside a boundary, at `checkCall`'s gate. Top-level calls and plain or boundary callbacks remain plain and get no migration diagnostic. The quick fix imports `trusted` from `effeck-runtime`, writes `export const nameTrusted = trusted(name, [ActualClasses])` beside the TypeScript implementation, imports that value into Effeck, and rewrites the call. It reuses an existing matching wrapper and imports, preserves aliases, and aliases `trusted` only on a name collision. A barrel import is left for manual migration unless it resolves directly to the declaring file. Methods name their declaration (for example `Ports.offset`) and ask for a receiver adapter. Keep the original function for TypeScript callers. The value returns `Effect<A, E>`; listed instances fail and other throws die. Promise and Effect implementations are rejected by its type. `legacyTrustCategory` is the named switch point in `diagnostics.go`: it is warning for the legacy emitter and becomes error when (B) emit switches on. Projects can select `"EK4406": "error"` now through `compilerOptions.effeck.diagnostics`, or retain `"warning"` during migration.

**The tag.** `/** @effeck trusted */` may sit on a function or method declaration, on any of its overloads, or on the statement of a `const` that a function initializes, which is how a library function is trusted:

```ts
/**
 * @effeck trusted
 * @throws {SyntaxError} when the text is not JSON
 */
export const parseJson: (text: string) => unknown = JSON.parse
```

Each `@throws {T}` names a class in the declaring file's scope.

**Where EK4401 applies.** In code a fiber runs, outside a boundary. A boundary is a callback of `Effect.try`, `Effect.tryPromise`, `Effect.sync`, `Effect.promise`, or `Effect.callback` (D23, D43), known by effect's declarations, including the `try` and `catch` of an options object. Code that no fiber runs is JavaScript (D51): the module's top level, and a plain callback that the top level passes to a TypeScript function, such as a `describe` body or a framework's route handler (`outsideFibers`, the places where D38 lets a runner start). A call that returns a Promise gets EK4401's Promise text and an `Effect.tryPromise` fix.

**Expected failures.** A call whose contract lists exceptions compiles to a runtime helper that tests `instanceof` (D51):

```ts
// main.effeck                              // main.js
function port(text: string) {           const port = Effect.fnUntraced(function* port(text) {
  return parsePort(text)                  return yield* runtime.$trusted(() => parsePort(text), [PortError]);
}                                       });
```

The listed classes join E (`addFailures`, from `flowOf`), and a call whose failures can end a function makes it an Effect (`hasEscapingFailure`, which D10's fixpoint asks too). Inside a `try` with a `catch` in a function that compiles to plain JavaScript (D54), the helper is `runtime.$trustedPlain`, which throws a listed exception as `$expected`, so the `catch` receives it. In a callback that compiles to plain JavaScript, such as one `Array.prototype.map` calls, a trusted call stays an ordinary call and its exceptions are defects. When an operand runs an Effect, the operands are evaluated first, in order, and passed to the thunk: `((receiver, args) => () => receiver.m(...args))(yield* r(), [yield* x()])`.

The generated code imports each class from a module the `.effeck` file imports that exports it, usually the one it imports the trusted function from: `import { PortError } from "./parsers.ts"`. A global class such as `SyntaxError` needs no import. A class that no imported module exports is EK4401 (`reachClass`). The code is `trust_lower.go`.

The shipped contracts (`builtinContracts`) list only built-ins that parse text from outside the program: `JSON.parse` fails with `SyntaxError`, and `decodeURI` and `decodeURIComponent` with `URIError`. Every other built-in is trusted, and its exceptions are defects.

## Platform APIs and clock reads

`platform.go` reports where `.effeck` code runs a Node.js or browser API (EK4402), each with the Effect module that replaces it (Chapter 11's table, D6), and clock and randomness reads (EK4403).

**EK4402 fires where the code runs the API (D108), not where it names it.** A platform API is a binding imported from a Node.js built-in module, with or without `node:`, or from a `bun:` module, or a global in `platformGlobals`: `fetch`, `XMLHttpRequest`, `WebSocket`, the timers, `process` (with `process.env`, `argv`, and the standard streams named apart), `console`, `crypto`, `performance`, `localStorage`, `sessionStorage`, `indexedDB`, `Worker`, the DOM's entry points, `require`, `module`, `__dirname`, `__filename`, `Buffer`, `Bun`, and `Deno`. A global's name must resolve to the global that a lib file or a platform package declares, or not resolve at all, as `process` does without `@types/node`. The code runs the API when it calls it, constructs it with `new`, tags a template with it, or reads one of its properties, which can run a getter, directly, by destructuring, or by spreading. Importing it, re-exporting it, or passing it as a value runs nothing, so Effect's own setup compiles in `.effeck`:

```ts
import { createServer } from "node:http"
const Server = NodeHttpServer.layer(createServer, { port: 3000 })   // allowed: a value
export function log(message: string) { console.log(message) }       // EK4402: a call
```

A namespace import's members are values too, until the code runs one: `os.EOL` is allowed, and `os.cpus()` is EK4402. Web standards that touch nothing outside the program, such as `URL`, `TextEncoder`, `AbortController`, `structuredClone`, `Headers`, and `Response`, are not platform APIs. EK4401 leaves a call that runs a platform API to EK4402 (`isPlatformCallee`).

**EK4403.** A call that resolves to `Date.now`, `Date()`, `new Date()` without arguments, `Math.random`, `crypto.randomUUID`, `crypto.getRandomValues`, `performance.now`, or one of `Temporal.Now`'s functions is a clock or randomness read, reported with the `Clock`, `DateTime`, `Random`, or `Crypto` call that replaces it. A project that sets EK4403 to `"warning"` or `"off"` (D34) gets that, and the read compiles as a plain call.

Both apply everywhere in a `.effeck` file, inside boundaries (D23) and at the top level.

## Throw and do expressions

`parser/effeck_blocks.go` parses a throw expression (D186) as `(() => { throw operand })()` and a do expression (D187) as `(() => { ...; return last })()`, rewriting the block's last statement into the return of its completion value and reporting TS1109 or TS1108 where the proposal forbids an ending or a return. Nothing after the parser knows the forms exist: the immediately invoked arrow is an Effeck function, so its call runs where it stands, short-circuits under `??` and `?:`, and is taken by name where a parameter expects a program. `isStartOfLeftHandSideExpression` admits `do {` and `throw` so both start list elements.

## Block params

`parseCallExpressionRest` ends a call or a name followed on the same line by `{` with `parseEffeckBlockParam` (D188): `f { block }` becomes `f(() => { block })` and `f(args) { block }` becomes `f(args, () => { block })`, one call whose last argument is a parameterless arrow whose text starts with `{`, which is how `isBlockParamCall` (effeck/blockparams.go) recognizes it. `effeckNoBlockParams` keeps heritage clauses and a provide block's providers out. The checker seam `filterCallSignaturesByHooks` (checker/extensions_blockparams.go, called from `resolveCallExpression`) lets `blockParamSignatures` add two forms of each of the callee's signatures ahead of it: `RotateSignature(sig, n)`, a data-first dual with its Effect parameter moved last, and `FlattenSignature(sig, n, inner)`, a data-last dual `(args) => (self) => R` as `(args, self) => R` with the type parameters of both. Overload resolution picks the first that fits, so a dual infers every type parameter from the arguments and the block together. The plan records the form of each call that resolved to one in `blockForms`, and `visitBlockParamCall` emits `f(block, args)` or `f(args)(block)`, keeping the call's `yield*`.

## Pattern matching (D206-D215)

`parser/effeck_match.go` parses `match (x) { when P: v; ... default: d; }` and `x is P` from the TC39 [pattern matching](https://github.com/tc39/proposal-pattern-matching) proposal into conditionals TypeScript already narrows:

```ts
// main.effeck                                   // parses as
return match (e) {                               {
  when NotFound and { let id }: `no ${id}`;        if ($is(NotFound, e)) { if (typeof e === "object" && e !== null) { if ("id" in e) { const id = e.id; return `no ${id}` } } }
  when Timeout: "retry";                           if ($is(Timeout, e)) { return "retry" }
}                                                  return $noMatch(e) }
if (o is Option.Some(let v) && v > 0) S          { if (Option.isSome(o)) { const v = o.value; if (v > 0) { S } } }
```

- **Parsing.** `match (expr) {` on one line followed by `when`, `default`, or `}` starts a match in `parsePrimaryExpression`; anything else is a call of a function named `match`. Arms end with `parseSemicolon`, so ASI applies. `is` is parsed in `parseBinaryExpressionRest` at `instanceof`'s precedence when a pattern follows on the same line. Mixed `and`/`or`/`not` at one level, a binding inside `or` or `not`, and the forms listed in EK4704 become a `$patternError(code)` call that `effeck/match.go` reports (EK4702-EK4705).
- **Patterns.** `compilePattern` turns a pattern into steps, tests and `const` bindings, which `emitSteps` nests as ifs. A literal is `===` (`undefined` too); `Option.Some`/`None`, `Exit.Success`/`Failure`, `Result.Success`/`Failure` call the module's guard and read `value`, `cause`, `success`, or `failure` (`knownMatchers`); `String`, `Number`, `Boolean`, `BigInt`, and `Symbol` test `typeof`. Any other matcher is `$is(M, x)`, and an extractor with an argument list is `const $extractedN = $extract(M, x)` tested against `false` and then indexed. Arrays test `Array.isArray` and the length (`>=` with a rest, which is `.slice(n)`); objects test `typeof x === "object" && x !== null` and `"k" in x`, and a rest is `$rest(x, keys)`. `or` and `not` are inline boolean expressions; a guard is its parenthesized expression.
- **Positions.** The subject is used directly when it is a name or `this` followed by property accesses and literal element accesses (`cloneIdentExpression`), so TypeScript narrows it; each test reads it again, so a getter there runs once per read. Any other subject goes into `const $matchN`. A match after `return` becomes a block whose arms return (`matchReturnStatement`); a match statement becomes a labeled block whose arms `break` (`matchStatement`); anywhere else it is an immediately invoked arrow function (`matchFunctionCall`), an Effeck function, so its call runs where it stands. Without a `default` arm the arms end in `$noMatch(subject)`.
- **is with bindings.** In an `if` or `while` condition, `restructureEffeckIf` and `restructureEffeckWhile` split the `&&` chain and nest the steps; an if with an else becomes a labeled block that breaks after the then branch, and a while becomes `while (true) { steps; body; continue } break`. Elsewhere, an `is` with bindings is EK4703.
- **Exhaustiveness.** `checkExhaustive` reads the subject's type at each synthesized `$noMatch` call, which is what no arm took, and reports EK4701 when it is not `never` and the declared type is a finite union: literals, `null`, `undefined`, `void`, and objects with a string literal `_tag` (`isFiniteMatchType`).
- **Runtime.** `match.ts` in `effeck-runtime` holds `$is` (a custom matcher first, then `typeof` for primitive classes, `instanceof` for classes, a call for other functions, and SameValue with `0` matching `-0`), `$extract`, `$rest`, `$noMatch` (a `TypeError`, as the proposal throws), `$patternError`, and `customMatcher` (`Symbol.customMatcher` when the engine has it, else `Symbol.for("Symbol.customMatcher")`). `TaggedError`, and so every `error` class, has a static custom matcher that tests `instanceof` and extracts the instance.
- **Tooling.** A binding is a variable declaration of the source name, so hover and go to definition treat it as one. The TextMate grammar colors `match`, `when`, `is`, `and`, `or`, and `not`. A synthesized string literal prints its own text (`scanner/utilities.go`), which `typeof` tests need.

Cases: `match-patterns` (every pattern kind, every position, `is` in if and while, effectful arms, nested matches) and `match-patterns.fail` (EK4701-EK4705).

## Where programs start

`runners.go` holds D38's runner registry and two checks. `runnerOf` knows a runner by the declaration its call resolves to: Effect's `runPromise`, `runPromiseExit`, `runSync`, `runSyncExit`, `runFork`, and `runCallback`, and their `*With` forms; `ManagedRuntime`'s `run*` methods; `NodeRuntime.runMain` and `BunRuntime.runMain`; and `@effect/vitest`'s `it.effect` and `it.live`, their modifiers, `.each`, and `.prop`, and `layer(…)`.

| Code | Rule |
| --- | --- |
| EK1001 | At a module's top level, a call that would run a program inside a function body: a call that runs an Effect or an `.effeck` Effect function, a service member call, or a call of an Effect value such as `Users()` or `Port()`. Only the outermost such call is reported. A program passed straight to a runner, as in `Effect.runPromise(main())`, gets the fix `() => main()` |
| EK1005 | A runner anywhere but the module's top level and the plain callbacks that the top level passes to TypeScript functions (`outsideFibers`). The fix forks the program, `fork(() => refresh(id))`; a test runner's fix registers the test at the top level or in a `describe` callback |

```ts
NodeRuntime.runMain(() => main())          // allowed: a runner at the top level, given a program
describe("users", () => {
  it.effect("finds Ada", () => find("1"))  // allowed: a plain callback from the top level
})
export const config = loadConfig()         // EK1001
```

## Debugging

`effeck --sourceMap` maps generated JavaScript to `.effeck` lines and columns. The lowering keeps the position of each statement and expression it moves, inside generators and region closures too, so `node --enable-source-maps` prints `.effeck` frames (case `debug-stack`):

```text
TypeError: Cannot read properties of undefined (reading 'name')
    at ownerName (main.effeck:12:25)
```

Generated functions carry their source's names (D36, `names.go`). A compiled function's generator is named after the function, or the variable or property that names it in the source, and region closures are `<function>$try`, `$catch`, and `$finally`. A named function expression binds its name inside its body, so when the body refers to that name, the generator is a method of an object literal instead, which has the name and binds nothing: `Effect.fnUntraced({ *fact(n) { … } }.fact)`. A lambda without a name stays anonymous. A clause is named after its operation member, a producer's body generator after the producer, and the closure of a loop over a Stream is `<function>$for`.

## Tracing

`compilerOptions.effeck.tracing` (D34) selects the functions that compile with spans (`tracing.go`):

| Mode | Spans |
| --- | --- |
| `"none"`, the default | None: functions compile to `Effect.fnUntraced` |
| `"exported"` | Each exported function declaration, and each exported `const` that a function initializes: `Effect.fn("greet")(function* greet(…) { … })` |
| `"all"` | Every named function that compiles to frames, and every lowered method, whose Effect runs as `Effect.withSpan(Effect.gen(…), "Users.find")` |

A span is named after its function; a class method's span names its class too. A lambda without a name, such as a callback, runs in its caller's span. Spans give the logical call stack that a JavaScript debugger cannot show (D36), through Effect's `DevTools` layer or any tracer Effect supports.

## The editor and the effeck command

The binary that `tsc/cmd/tsc` builds is `effeck`, Effeck's compiler, under any file name (`identity.go`, through `tsc.SetIdentity`):

```text
$ effeck --version
Effeck 0.0.1 (TypeScript 7.1.0-dev)
$ effeck --help
effeck: The Effeck Compiler - Version 0.0.1 (TypeScript 7.1.0-dev)
```

`effeck --lsp --stdio` is TypeScript 7's language server running the Effeck checker, so TypeScript's features work on `.effeck` files with Effeck's types: a call that runs has its success type, and an `.effeck` function has its inferred contract. Hooks in `hover.go` and `completions.go` add what TypeScript cannot know:

| Request | In an `.effeck` file |
| --- | --- |
| Hover on a function | Its inferred contract: `function lookup(id: number): Effect.Effect<string, NotFound, never>` |
| Hover on the callee of a call that runs | The callee's contract, then "Runs here and returns `string`.", the success value the call evaluates to. It asks the analyzer the same questions as the lowering, so a call in a `.ts` file or one the lowering leaves plain shows nothing |
| Hover on a plain function or a call to it | `function double(n: number): number`, then "Plain JavaScript: Effeck code calls it directly. TypeScript callers see `Effect<number, never, never>`." A `.ts` file keeps TypeScript's view |
| Completions | The prelude names, as names in scope that the file's own declarations and imports shadow; a file that uses none yet takes them from the runtime another `.effeck` file imports. After `Users.` in a function body, the service's members (D41), not the Context key's |
| Diagnostics | Pulled with `textDocument/diagnostic` for a document opened as `effeck` or `typescript`. EK diagnostics have source `EK` and their number as the code, which VS Code shows as `EK(1003)` |

Upstream code needed four fixes for `.effeck` files in the server: auto-import panicked with "Extension .effeck is unsupported" (`module.TryGetJSExtensionForFile`), a new `.effeck` file on disk did not join its project (the wildcard globs in `tsoptions`), renaming an `.effeck` file did not update its importers (the `willRenameFiles` filter), and the server exited with status 1 after `shutdown` and `exit`, which Neovim reports on every restart. Per-checker state lives on the checker (`ExtensionState`), because the server replaces its checkers on every edit: with a package-level map, 300 edits of one `.effeck` file grew the server from 448 MiB to 914 MiB; now it stays at 305 MiB, as the same file as `.ts` does.

`effeck/lsp-smoke.sh` builds `effeck` and the runtime, checks `effeck --version` and `effeck --help`, and runs `lsp-smoke.mjs`, an LSP client without dependencies, on `effeck/lsp-fixture`: 37 checks of diagnostics, hovers, definitions, references, renames, completions, file watching, and shutdown across `.effeck` and `.ts` files. The editor setups and their tests are in `../editors/` (start with `../editors/README.md`).

### Commits

| Change | Subject |
| --- | --- |
| `lutmtlns` | checker: let in-tree extensions keep state per checker |
| `ssqltlzy` | effeck: keep the analyzer on its checker |
| `xosuknwx` | module: map .effeck files to .js output |
| `kowtxrmz` | tsoptions: match .effeck files in wildcard include globs |
| `mwpkwpsm` | lsp: send willRenameFiles for .effeck files |
| `knrotrus` | effeck: add a headless language server test |
| `uwpykqxu` | ls: add a quick info hook |
| `otxrukzp` | effeck: show a running call's success value on hover |
| `qlpsqxln` | execute: let a compiler built on tsc name itself |
| `txvnzpqn` | effeck: identify the compiler as effeck |
| `trpvstxy` | editors: derive an Effeck grammar from VS Code's TypeScript grammar |
| `svkttyrl` | editors: add the VS Code extension |
| `ryxzxutw` | ls: let a quick info hook replace the quick info line |
| `unqyxosy` | checker: add a signature with another return type |
| `xumoqrzt` | effeck: show a plain function the way Effeck calls it |
| `xpqmlqkm` | ls: add completion hooks for scope names and members |
| `lqzlumyp` | effeck: complete prelude names and service members |
| `vrzttyov` | lsp: exit with status 0 after shutdown and exit |
| `nzvnqlxu` | editors: check completions and plain hovers in the VS Code test |
| `wuyspvlk` | editors: set up Neovim and Zed |
| `pzsnyzmw` | effeck: document the editor and effeck in EFFECK.md |
| `yppyvsqw` | editors: run effeck through the compiler package's launcher in editor tests |

## Performance

Run the benchmarks on the build host, which is quieter and larger than a laptop; other suites running there at the same time add noise, so the tables report medians:

```sh
# Upstream tsgo for the baseline: build the import commit's compiler once.
jj workspace add ~/es-ws/upstream --name upstream -r 0c804b2f
(cd ~/es-ws/upstream && sh ~/effectscript/scripts/remote.sh sh -c 'cd typescript/tsc && go build -o ../built/local/tsc ./cmd/tsc')
# The compiler benchmark, then the language server benchmark.
scripts/remote.sh sh -c 'cd typescript/tsc && go build -o ../built/local/tsc ./cmd/tsc && cd ../.. && sh scripts/bench/compiler.sh --upstream $HOME/es-remote/upstream/typescript/built/local/tsc --runs 5'
scripts/remote.sh sh -c 'node scripts/bench/gen-synthetic.mjs /tmp/lsp 400 && node scripts/bench/lsp.mjs typescript/built/local/tsc /tmp/lsp/effeck'
```

`scripts/bench/gen-synthetic.mjs` writes the same N modules twice, in Effeck and in hand-written Effect 4 TypeScript. Each module declares two errors, a schema, a service and its layer, a plain helper, and functions that call into the previous module, catch with `match`, fork and join two fibers, provide the layer, and pipe the result. Its shapes are `chain` (small contracts), `dense` (each module's `deep` function also loads four earlier modules' services and calls the previous `deep`, so requirement and error unions grow with depth), and `single` (every module in one file). `compiler.sh` compiles the synthetic projects, the examples, the template, and the docs snippet corpus as one program with `--extendedDiagnostics`, and measures wall time and peak RSS with `scripts/bench/measure.py`.

Measured 2026-10-17 on msb-lat-nyc-01, Effeck at `main` plus the plain-function fix below, against upstream tsgo 7.1.0-dev at 257e874 (medians of 5 runs; seconds and MiB):

| Project | Effeck wall | Effeck check | Effect TS wall (upstream) | Effect TS check | Peak RSS, Effeck / Effect TS |
| --- | ---: | ---: | ---: | ---: | ---: |
| chain, 200 modules | 0.39 | 0.13 | 0.31 | 0.15 | 281 / 221 |
| chain, 400 modules | 0.62 | 0.29 | 0.50 | 0.30 | 348 / 313 |
| dense, 200 modules | 0.53 | 0.26 | 1.98 | 1.75 | 336 / 440 |
| dense, 400 modules | 1.15 | 0.71 | 12.34 | 11.95 | 549 / 977 |
| single file, 400 modules | 1.59 | 1.01 | 2.96 | 2.25 | 254 / 286 |

- **No Effeck pass is superlinear.** Chain and single-file time grow linearly with module count. Dense time roughly doubles per doubling, as its contracts grow. Hand-written Effect TypeScript is the one that blows up on the dense shape: checking `yield*` over requirement and error unions that grow with depth takes 12 s at 400 modules, against Effeck's 1.2 s, because Effeck infers each function's contract once instead of through generator types.
- **Effeck's own passes** take 19% of the CPU samples on the 400-module chain and 27% on the dense shape (`go tool pprof -focus='internal/effeck\.'` over `--pprofDir` profiles); TypeScript's checker does the rest. The largest are contract inference (`effectTypeOf`, `solve`) and expected-failure and trust analysis (`expectedFailures`, `callTrust`).
- **The hooks cost nothing on plain TypeScript:** this compiler checks the Effect TS project 3% slower than upstream tsgo.
- **The fixed cost is the effect root.** A file that uses any effect root name (D146), even `log`, makes the program import `effect` itself, whose index loads every module: about 515 more declaration files. The examples load 738 files, spend about 0.10 s parsing them and 2 ms checking, and peak at 220 MiB; the single-file shape, which uses no root name, loads 241. Importing each used module implicitly (`effect/Console`, `effect/Effect` for the bare functions), as the emitted JavaScript already does, would remove most of that. Name resolution (`preludeSymbol`), completions (`completions.go`), and method syntax (`effectModuleNamespace`) read the root module's symbol today and would need the per-module symbols instead.
- **The docs corpus** (684 Effeck and TypeScript files as one program, 1594 files with dependencies) compiles in 0.57 s.

Language server, `scripts/bench/lsp.mjs` on the chain shape (open the last module, pull its diagnostics, hover the call into the previous module):

| | Effeck, 200 modules | Effeck, 400 modules | Effect TS (upstream), 200 or 400 |
| --- | ---: | ---: | ---: |
| First diagnostics, before the fix | 270 ms | 380 ms | 80 to 90 ms |
| First diagnostics, after | 150 ms | 175 ms | |
| First hover, before the fix | 105 ms | 200 ms | 3 to 6 ms |
| First hover, after | 11 ms | 12 ms | |
| Later hovers | 0.2 ms | 0.2 ms | 0.2 ms |
| Peak RSS, after | 260 MiB | 277 MiB | 130 to 145 MiB |

**The fix:** `isPlainFunction` (plain.go) explored the transitive closure of a function's callees before deciding, even when the function's own body ruled it out, such as a `provide` block or a `throw` outside a `try`. The CLI pays that once per program, but the language server's checkers start empty, so the first hover in a module walked every function the call graph reached, through each module's `run` into the one before it. It now stops at a body that rules itself out and stops walking a body at the first call that rules it out; verdicts are unchanged. `TestPlainFunctionStopsAtItsOwnBody` pins it: deciding the last of 50 throwing functions decides only that one (before the fix, all 50). What remains of the first-diagnostics gap is parsing the effect root's declaration files (55% of the server's CPU in a 400-module session).

## What it cannot do yet

- `arguments` inside a region's closure is the closure's, not the function's, and a function whose body declares `using` or calls `fork` runs its whole body in such a closure.
- A `using` in the head of a C-style `for` loop, `for (using x = init; …; …)`, stays TypeScript's own `using`, which Effect does not run on failure or interruption, and an acquisition there is TS2850.
- Inside a clause, `resume(k, v)` has the program's own result type, or `never` with `onReturn` (D97), so a clause that uses the resumed result while another clause returns early with another type sees a narrower type than the value. The rule that `k` stays inside its clause is not checked.
- EK4201 knows effect's forking combinators by name and any call by its `concurrency` option; a TypeScript wrapper around a fork is EK4202 only when its declared requirements name the control operation. A producer stored in a variable and then passed to a Stream function is not EK4201.
- `this` in a producer is the function's, but `arguments` is the body generator's. A `for (using x of producer)` loop takes JavaScript's `for...of`, so it cannot iterate a producer.
- EK3201 is reported at runners only, not at a written contract whose R excludes the service a `wire` leaves unprovided.
- A trusted call's expected failures are defects inside a callback that compiles to plain JavaScript, such as one `Array.prototype.map` calls, where D9 and D10 read strictly would make the callback EK4301.
- Runners are known by their declarations in `effect`, `@effect/platform-node`, `@effect/platform-bun`, and `@effect/vitest`; a function that wraps one is not a runner. EK4402's globals are a fixed list (`platformGlobals`), so a platform global outside it, such as `postMessage` or `BroadcastChannel`, is not reported.
- EK4402 follows a platform API by its import or global name. A platform function reached through a local alias, a destructured namespace member, a parameter, or an object that a platform call returned is not reported, and as a call to a platform package it is not EK4401 either.
- Declaration files carry no JSDoc metadata for `$direct` variants (Chapter 27), so code compiled against a `.d.ts` runs a plain function's Effect form, which is correct and slower.
- A callback parameter without a contract returns an Effect (D12), so a function cannot call it inside a callback that JavaScript calls synchronously, such as a `sort` comparator: that is EK4301.
- TypeScript types a context-sensitive lambda's parameters from the first candidate that types them, so an overload whose parameter types differ cannot retype them.
- Accessors, constructors, async functions, and generator methods are never lowered. `arguments` inside a lowered arrow function is the generator's, and `arguments` and `super` inside a lowered method do not reach the method.
- A lowered function declaration is a `const`, so a call above it at the module top level hits the temporal dead zone.
- A call cycle through a lambda, such as `Effect.forEach(children, (child) => walk(child))` where `walk` calls back, is solved through the checker's type of the call, which the checker computes once; a cycle whose E grows through it is EK2102. A union of Effects is not treated as an Effect.
- Hover shows no control class (D33): the compiler does not yet tell `pure` from `sync` or find `captures`. The language server has no Effeck quick fixes or refactors (D33), completions in a `catch` do not narrow to the error classes its `try` can raise, and the server's `serverInfo` still says `typescript`.
- Pattern matching (D207, D214): array patterns match arrays only, not other iterables; object patterns do not match primitives; an `or` or `not` pattern cannot bind (EK4702); `is` bindings are not in scope in an `else` branch or after the statement; regular expression and relational patterns, computed keys other than names and literals, and `async match` are EK4704 or unparsed; an extractor with a custom matcher types its bindings from the matcher's return type only loosely; a subject such as `user.role` is read once per test rather than once, which a getter can observe; a match in expression position runs its arms in an arrow function, so an arm there cannot `yield`. The editor tests do not exercise hover on bindings.
- The synthesized wrappers, such as `yield*` and `Effect.fnUntraced(…)`, have no source positions of their own; the statements and expressions inside them do, so stack traces and breakpoints map to `.effeck` lines.

## Differential check against the TypeScript-API prototype

`../prototype/esc` compiled copies of `effeck-wrap`, `effeck-lower`, and `effeck-infer`, with plain return annotations removed because the prototype rejects them (EK2003). Both compilers put `yield*` on the same call sites (2, 12, and 6) and leave the same calls plain, including `Config.Port(...)` and `[1, 2, 3].map(double)`. Both infer `lookup: Effect<string, NotFound, never>` and `main: Effect<void, NotFound, never>`.

The prototype reports EK1001 on the top-level `Effect.runPromise(Effect.catch(main(), …))`, because the design (Chapter 11, "Where programs start") forbids running a program at the module top level. This fork reports it too, so `effeck-infer` passes the runner a program: `Effect.runPromise(() => Effect.catch(() => main(), …))`.

## Next hardest risk

Position-dependent lowering now happens inside overload resolution, in one checker pass ([Programs, plain functions, and callbacks](#programs-plain-functions-and-callbacks)): the overload probes behind the single-pass experiment get the experiment's answers (case `lambda-forms`), and no construct needed a second pass. What remains of that risk is E and R through a call cycle: a recursive component needs its rows iterated to a fixed point (D52), inside a checker that resolves return types lazily and reports cycles as TS7023.
