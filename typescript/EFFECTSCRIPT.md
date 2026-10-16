# EffectScript fork of the TypeScript compiler

This directory is microsoft/TypeScript with a short stack of commits that teach the Go compiler (`tsc/`) EffectScript: TypeScript syntax in which every function is an Effect (effect@4.0.5). A call to a function that returns an Effect runs it, and the compiler infers each function's `Effect<A, E, R>`.

It is a first prototype. It lowers `.es` files with checker types, runs the output on Effect 4, and emits `.d.ts` files with the inferred Effect types. Services, control operations, failures and regions, and the prelude work (see their sections); the acceptance table below says which lane builds each remaining feature. The design is `../docs/design/effectscript-design.md`; the syntax decisions are `../docs/design/DECISIONS.md`.

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
./built/local/tsc -p effectscript/cases/ts-hello
```

The build embeds the `lib.*.d.ts` files; Herebyfile's `-tags=noembed` build copies them next to the binary instead. A cold build takes about 75 s and fills about 1.7 GB of Go build cache. Always pass `-trimpath`: without it, Go keys each package's cache entry by its absolute directory, so every jj workspace cold-builds and adds its own copy. `-gcflags=all=-dwarf=false` shrinks the entries; panics still print stack traces.

### Cases

`effectscript/smoke.sh` builds `tsc`, builds `../packages/runtime` with that `tsc`, and runs `effectscript/harness.mjs`, which compiles the cases in parallel and runs them. Each directory in `effectscript/cases` that has a `tsconfig.json` is a case, and its sources decide its kind:

| Kind | Marked by | Passes when |
| --- | --- | --- |
| Failing | A source whose first line is `// expect: ES2001 ES2002` | The error codes `tsc -p` reports are exactly those codes. Warnings and message text do not count |
| Runnable | A source with a line `// expect-result: <json>` that exports `main()` | `tsc -p` reports no errors, and `out/run.mjs`, which the harness writes, prints `@@result <json>`: the JSON of the success value of the Effect that `main()` returns. What the program logs does not count |
| Plain | Neither | `tsc -p` reports no errors, every file in `expected/` other than `stdout.txt` equals the file at the same path under `out/`, and `run.mjs`, or else `out/main.js`, prints `expected/stdout.txt` |

A case with a file named `pending` is pending. The file's first word names the lane that owns the missing feature and the rest says what is missing, for example `F5 wire (D27)`. A pending case runs but never fails the suite. When it passes, the harness reports `XPASS`, and its owner deletes `pending` in the commit that makes it pass. The suite fails when a case that is not pending fails.

Each case gets two symlinks in its `node_modules`: `effect`, from `$EFFECT_NODE_MODULES` (default `../experiments/node_modules`, else the main checkout's `~/effectscript/experiments/node_modules`), and `@effectscript/runtime`, from `$EFFECTSCRIPT_RUNTIME` (default `../packages/runtime`). `smoke.sh` builds the runtime's `dist/` from source with the fork's `tsc`, so a commit that changes the compiler and the runtime is tested as one. It links `effect` into the runtime's own `node_modules` when that directory has none, so a case loads one copy of `effect`.

| Case | Proves |
| --- | --- |
| `ts-hello` | The stock compiler still compiles and runs plain TypeScript |
| `es-parse` | An `.es` file parses and checks as TypeScript and emits `main.js` and `main.d.ts` |
| `es-wrap` | Exported functions and arrow functions lower to `Effect.fnUntraced`, and JavaScript runs them with `Effect.runSync` |
| `es-lower` | Type-directed `yield*`: pure helpers stay plain, Effect-returning TypeScript calls and `.es` calls run, operands run left to right, a `Config` stays data |
| `es-infer` | Unannotated functions get `Effect<A, E, R>`: a typed failure flows from `lookup` into `main`, the `.d.ts` declares it, and the module runs its own program |
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
| `es-service-dts` | A service's `.d.ts` is an interface plus a key class: TypeScript code compiled against it with `skipLibCheck` off implements `Users`, provides it with `Layer.succeed`, and yields `Users` and a default service in `Effect.gen` |
| `failures-throw` | `throw` is `Effect.fail`: a throw alone makes a function an Effect, the thrown type joins E (`throw "bad"` adds `string`), a `catch` narrows the caught union by `instanceof` and `_tag` and rethrows, and the `.d.ts` declares each E |
| `failures-causes` | `catch` skips a defect, a mixed cause, and an interruption (D13); `finally` runs on each, and a failing `finally` keeps both failures (D15) |
| `failures-plain` | A function that runs no Effect and catches its own throws stays plain JavaScript, with JavaScript's `try`; its `catch` receives expected failures and rethrows a `TypeError` defect (D54) |
| `regions-jumps` | `try`, `catch`, and `finally` run as regions: labeled and unlabeled `break` and `continue`, `return` from nested regions, a labeled `try`, a `try` as an `if` body, `switch` inside a region, and `var` in a region give JavaScript's results and `finally` order |
| `lambda-forms` | A lambda takes the form the overload candidate under check declares, and the first candidate that accepts it in that form wins: the overload probes behind `../prototype/single-pass/RESULTS.md` |
| `programs` | A lambda with no parameters is a program where Effect expects an Effect: `retry`, `race`, `timeout`, `all` over an array and a record, `provide`, a stored lambda that reruns, and an object property typed through `Layer.succeed`'s `NoInfer<S>` |
| `program-values` | A function value converts where it is passed: `Effect.retry(flaky, …)`, array and record literals of programs, and arrays and records built at run time through `$programs` |
| `effect-values` | Calling an Effect value runs it: a clock read, a cached program, a `Config`, and a Context key |
| `pipe-programs` | `pipe` takes a program first when its first stage takes an Effect (D8), and stays effect's `pipe` on plain data |
| `plain-functions` | A pure function compiles to a `$direct` variant and an Effect form; calls run inline in source order; a pure recursive pair is plain (a greatest fixpoint); `xs.map(double)` passes `double$direct` |
| `plain-imports` | Another `.es` module's plain functions run inline through named, renamed, and namespace imports |
| `plain-methods` | `[Equal.symbol]` and `[Hash.symbol]` compile to plain methods that `Equal.equals` and `HashSet` call (D55) |
| `callbacks-pausing.fail` | ES4301 for a lambda and for a function value that can pause where `Array.prototype.map` and `forEach` call them |
| `plain-methods-pausing.fail` | ES4301 for an `[Equal.symbol]` that pauses (D55) |
| `callback-contracts` | A callback parameter without a contract accepts any contract (D12): `mapAll` and `withRetry` infer E and R from each call, and the `.d.ts` writes `<A, B, E, R>` |
| `generic-callees` | A call to a generic `.es` function adds the callee's E and R as the call instantiates them |

### Acceptance cases

The prototype's 46 examples (`../prototype/esc/examples/`) are cases named after their files, such as `ch04-wire` and `ref2-cycle.fail`. Each keeps its `.es` source and `.ts` helpers as the prototype has them, except that kernel imports name `@effectscript/runtime` and `@effectscript/runtime/kernel`. They compile with `module: nodenext` and `rewriteRelativeImportExtensions`, as the app template does. Every compiler lane turns its cases green and never turns a green case red. Each case is pending on the lane that owns its feature; a case that mixes features belongs to the lane that owns its first missing feature, and its `pending` file names the other lanes it needs:

| Owner | Feature | Cases |
| --- | --- | --- |
| F1 | Diagnostics, prelude, imports between files | `ch10-effect-gen.fail` |
| F2 | Programs as values, plain functions, callbacks | `ch05-programs`, `ch10-map-pausing-callback.fail`, `ch13-plain`, `ch14-callbacks`, `ref4-callbacks`, `ref4-pausing-function-value.fail`, `ref5-programs-as-data` |
| F3 | `throw`, regions, contracts | `ch02-contract-narrower.fail`, `ch02-functions`, `ch03-errors`, `ch03-safelookup-no-try.fail`, `ch12-contracts`, `ch12-welcome-missing-mailer.fail`, `ch15-failures`, `nominal-services.fail`, `regions` |
| F5 | Services, layers, `wire` | `ch01-first-look`, `ch04-provider-escapes.fail`, `ch04-services`, `ch04-users-audited.fail`, `ch04-wire`, `ch12-rates-bare-return.fail`, `ref1-existing-service`, `ref2-ambiguous-provider.fail`, `ref2-cycle.fail`, `ref2-missing-provider.fail`, `ref2-opaque-layer.fail`, `ref2-provider-graph` |
| F6 | `using`, `fork`, Scope regions | `ch06-disposable`, `ch06-resources`, `ch07-concurrency`, `ref3-resourceful-retry`, `ref6-fork.fail` |
| F7 | Handlers, producers, streams | `ch08-producers`, `ch08-streams`, `ch09-approve-all-forked.fail`, `ch09-handlers`, `ref6-handlers`, `stack-depth` |
| F8 | Boundaries, runners | `ch02-plain-ts-call.fail`, `ch10-fetch.fail`, `ch10-typescript`, `ch11-top-level-run.fail`, `ch24-profile`, `d5-clock-read.fail` |

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

Lane F2's commits, which landed among these, are listed in [Programs, plain functions, and callbacks](#programs-plain-functions-and-callbacks).

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
| `tsc/internal/checker/checker.go` | Tell call candidate hooks which candidate `inferTypeArguments` and `isSignatureApplicable` check, and let candidate filter hooks reject it; pass the type of a function expression, a contextual return type (`getContextualReturnType` wraps the old body as `getContextualReturnTypeWorker`), and every checked expression through hooks; append hook type parameters in `getSignatureFromDeclaration` and `getOuterTypeParameters`; let parameter type hooks replace an annotation's type in `getTypeForVariableLikeDeclaration` | +14 / -4 |
| `tsc/internal/compiler/program.go` | Comment directives skip diagnostics from exempt sources | +1 / -1 |
| `tsc/internal/compiler/fileloader.go` | Resolve each module an implicit import hook names for a file | +4 |
| `tsc/internal/binder/nameresolver.go` | `NameResolver.ResolveBeforeGlobals`, consulted before the global scope | +6 |
| `tsc/internal/module/resolver.go` | Try `.es`, then `.d.ts`, for `./welcome.es`, and `.es` after `.tsx` for `./welcome.js` | +19 |
| `tsc/internal/module/util.go` | An `.es` resolution is always allowed | +2 / -1 |
| `tsc/internal/core/compileroptions.go` | A `.es` file may import a TypeScript source by its extension | +5 / -1 |
| `tsc/internal/checker/emitsupport.go` | Ask entity name visibility hooks in `isEntityNameVisible` | +4 |
| `tsc/internal/compiler/emitter.go` | Run declaration transformer hooks after TypeScript's | +2 / -2 |
| `tsc/internal/core/options_generated.go` (generated) | `CompilerOptions.EffectScript`, in `Clone` and `Equals` | +6 |
| `tsc/internal/tsoptions/declarations_generated.go` (generated) | The `effectscript` declaration | +8 |
| `tsc/internal/tsoptions/options_generated.go` (generated) | Parse, merge for `extends`, compare for semantic diagnostics and emit, and serialize for `--showConfig` | +15 / -1 |
| `tsc/internal/tsoptions/commandlineoption.go`, `tsconfigparsing.go` | The option's extra validation | +4 |
| `tsc/internal/compiler/extensions.go` (new) | `RegisterScriptTransformerHook`, `RegisterDirectiveExemptSource`, `RegisterImplicitImportHook`, `RegisterDeclarationTransformerHook` | +106 |
| `tsc/internal/checker/extensions.go` (new) | `WithChecker`, `RegisterCallExpressionTypeHook`, `RegisterInferredReturnTypeHook`, `NewTypeReference`, `ExtensionState`, `CompilerOptions`, `AddDiagnostic`, `RegisterSourceFileCheckHook`, `RegisterDiagnosticsFilterHook`, `RegisterCatchVariableTypeHook`, `GetTypeOfExpressionBeforeWidening`, `RegisterNameResolutionHook`, `RegisterEntityNameVisibilityHook` | +217 |
| `tsc/internal/checker/extensions_classes.go` (new) | `RegisterClassBaseHook`, `Program`, `GetStringLiteralType`, `NewOptionalReadonlyPropertyType` | +48 |
| `tsc/internal/checker/extensions_calls.go` (new) | `RegisterMemberReceiverHook`, `RegisterCallSignaturesHook`, `NewCallSignature` | +52 |
| `tsc/internal/checker/extensions_functions.go` (new) | `RegisterSkippedFunctionTypeHook`, `GetNonInferrableAnyType` | +35 |
| `tsc/internal/checker/extensions_forms.go` (new) | `RegisterCallCandidateHook`, `RegisterCandidateFilterHook`, `RegisterFunctionExpressionTypeHook`, `RegisterContextualReturnTypeHook`, `NewFunctionTypeWithReturnType` | +118 |
| `tsc/internal/checker/extensions_values.go` (new) | `RegisterExpressionTypeHook`, `CreateArrayType`, `MapObjectType` | +45 |
| `tsc/internal/checker/extensions_callbacks.go` (new) | `RegisterTypeParametersHook`, `RegisterParameterTypeHook`, `NewTypeParameter` | +61 |
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

The position decides, inside TypeScript's own overload resolution (`forms.go`, `expected.go`):

1. `inferTypeArguments` and `isSignatureApplicable` tell the call candidate hook (`callCandidate`) which candidate they check a call's arguments against.
2. When the checker types a lambda among the arguments, also inside an array literal or as an object literal property, the function expression type hook (`functionExpressionType`) reads that candidate's declared, uninstantiated parameter type (D44) and follows the path to the lambda through element and property types. A bare type parameter says nothing, so the lambda falls back to its contextual type, instantiated with the inferences made so far, as `Layer.succeed`'s `NoInfer<S>` needs.
3. The expected type decides the form: a function type that returns an Effect asks for an Effect function, another function type for a plain function, and an Effect for a program. Where a parameter accepts a function or an Effect (D39), a lambda with parameters is a function and one without is a program when it can pause. Without an expected type, as for a returned lambda, a lambda is plain when D10 allows it and an Effect function otherwise.
4. A plain function cannot run an Effect, so when a candidate asks a lambda that pauses for the plain form, the candidate filter hook (`filterCandidate`) rejects the candidate and resolution tries the next one. The first candidate that accepts the lambda in the form it asks for wins, as TypeScript's first-applicable rule says.
5. Each candidate's first answer stands. The lambda compiles to the form the candidate resolution selected asked for, read back from the call's resolved signature.

A return statement in an Effect function or program is contextually typed by the success type of the Effect expected of it (the contextual return type hook, `contextualReturnType`), so the object a `Layer.effect` program returns gets the service's shape. `pipe` looks ahead (D8): when its first argument is a function with no parameters and its first stage takes an Effect, that argument is a program (`pipe.go`).

### Programs as values (D7, D49)

A function value converts where it is passed, as a lambda takes its form there. The expression type hook (`expressionType`, `conversions.go`) gives an argument, array element, or property value the type of its conversion against the candidate under check:

| Expected | Value | Lowers to |
| --- | --- | --- |
| An Effect | A function with no required parameters that returns an Effect | `Effect.suspend(value)` |
| An iterable or record of Effects | An array or record of such functions, built at run time | `runtime.$programs(value)`, which keeps a record's keys |
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
| ES1002 | The API, such as `Effect.gen`, when a `function*` is passed where a function that returns a lib `Generator` or `Iterator` is expected; covers the generator | `generators.go` |
| ES1003 | `async`, `await`, `for await`, and `await using` in a `.es` file; covers TypeScript's errors about them, such as an `await` outside an `async` function | `syntax.go` |
| ES1004 | `using` that no function encloses, including in a class static block | `syntax.go` |
| ES4301 | A lambda that can pause where a plain function is expected, naming the first call that pauses and the function that calls it; an `.es` function passed by name that can pause; a method that implements a plain function member and pauses (D55). The fix names `Effect.forEach` | `callbacks.go` |

`compilerOptions.effectscript` (D34) sets the category of the two codes that are policy, not soundness, and the tracing mode:

```json
"effectscript": { "diagnostics": { "ES2004": "off", "ES4403": "warning" }, "tracing": "exported" }
```

ES2004 defaults to `"warning"` and ES4403 to `"error"`; every other ES code is always an error. An unknown key or invalid value is reported at its node in `tsconfig.json` with TypeScript's own messages (TS5023, TS6046). `CompilerOptions.EffectScript.Tracing` holds the tracing mode for lane F8. `tsc --showConfig` prints the block; `.tsbuildinfo` does not record it yet, so changing it does not invalidate an incremental build.

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

The last three rows are one rule from Chapter 13: in an `.es` function body, a value whose type is an Effect, called with no arguments, runs. The call signatures hook (`effectValueCall`) gives such a callee the signature `() => Effect<A, E, R>`, and the call expression type hook then types the call as `A`, so `Settings().baseUrl` checks. A lookup makes the enclosing function an Effect even when the member returns a plain value, and `effectOf` adds the service to R.

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
| ES3201 | The runner's program | A runner (`Effect.runFork`, `runPromise`, `runSync`, their `Exit` forms, `runCallback`, `NodeRuntime.runMain`, `BunRuntime.runMain`) runs a program whose R holds a service that a `wire` argument anywhere in the program needs and no argument of that `wire` provides; notes name the arguments that need it, and it covers TS2345 |

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

`$catch` is the kernel's `catchExpected` and `$finally` is `Effect.onExit`, from `@effectscript/runtime`, which the file imports as `* as runtime` under a free name. A `return`, `break`, or `continue` that leaves a region cannot jump out of its closure, so the closure returns a completion record: `{ kind: "return", value }`, `{ kind: "break", label }`, or `{ kind: "continue", label }`, with `""` as the label of an unlabeled jump, and the runtime's `$normal` when it reaches its end. The generator that runs the region dispatches each record that can arrive: it returns, breaks, or continues itself, or, when it is a region's closure too and the jump leaves it as well, returns the record to its own runner. A region that no jump leaves runs as `yield* …;` with no record. A closure whose block refers to `this` runs with the function's `this`. A `var` in a region belongs to the whole function, so the lowering declares it in the Effect function and assigns it where the source declared it.

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

`regions.go` holds the machinery as an API for the region kinds still to come, `using` and `fork` blocks (lane F6) and loops over producers (F7):

| Piece | Role |
| --- | --- |
| `region` | Collects the jumps (`jump{kind, label}`) that leave a region while its closures are emitted |
| `frame` | One generator being emitted, the function's body or a region's closure, with the loops, switches, and labels open in it; `frame.lands(j)` says whether a jump stays inside |
| `regionStatements(r, block)` | Visits a block as the body of a closure of region `r` |
| `regionFunction(r, role, binding, block, statements)` | Makes `Effect.fnUntraced(function* f$role(binding) { … })`, appending `return $normal` when a jump leaves `r` and the block can reach its end |
| `runRegion(original, run, r)` | Emits `const completion = yield* run` and one dispatch per jump, or `yield* run;` when nothing leaves |
| `runtimeMember`, `runtimeCall` | Reference `@effectscript/runtime` helpers, importing the package on first use |

To add a region kind: create a `region`, emit each block with `regionStatements` and `regionFunction`, compose the closures with runtime helpers, and pass the result to `runRegion`. A consumer that handles some jumps itself, as a producer loop handles its own `break` and `continue`, filters `r.jumps` before dispatching.
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

## What it cannot do yet

- `using` is plain JavaScript.
- `arguments` inside a region's closure is the closure's, not the function's. A `return`, `break`, or `continue` that leaves a `finally` block is not dispatched.
- `fork` and handlers have no meaning yet: the prelude names them, and their lanes give them meaning.
- ES3201 is reported at runners only, not at a written contract whose R excludes the service a `wire` leaves unprovided.
- Most ES diagnostics do not exist yet. A top-level `main()` is an Effect value, not ES1001. A plain TypeScript call is not ES4401.
- An annotated function keeps its annotated type. `function f(): number` with Effect calls is `() => number` to TypeScript but returns an Effect at run time. An Effect contract annotation is checked against the body as a TypeScript type, not as a contract.
- Declaration files carry no JSDoc metadata for `$direct` variants (Chapter 27), so code compiled against a `.d.ts` runs a plain function's Effect form, which is correct and slower.
- A callback parameter without a contract returns an Effect (D12), so a function cannot call it inside a callback that JavaScript calls synchronously, such as a `sort` comparator: that is ES4301.
- TypeScript types a context-sensitive lambda's parameters from the first candidate that types them, so an overload whose parameter types differ cannot retype them.
- Accessors, constructors, async functions, and generators are never lowered. `this` and `arguments` inside a lowered arrow function rebind to the generator, and `arguments` and `super` inside a lowered method do not reach the method.
- A lowered function declaration is a `const`, so a call above it at the module top level hits the temporal dead zone.
- E and R miss contributions that arrive through a call cycle (no fixed point). A union of Effects is not treated as an Effect.
- Synthesized nodes have no source positions, and the language service is untested.

## Differential check against the TypeScript-API prototype

`../prototype/esc` compiled copies of `es-wrap`, `es-lower`, and `es-infer`, with plain return annotations removed because the prototype rejects them (ES2003). Both compilers put `yield*` on the same call sites (2, 12, and 6) and leave the same calls plain, including `Config.Port(...)` and `[1, 2, 3].map(double)`. Both infer `lookup: Effect<string, NotFound, never>` and `main: Effect<void, NotFound, never>`.

The prototype reports ES1001 on the top-level `Effect.runPromise(Effect.catch(main(), …))`, because the design (Chapter 11, "Where programs start") forbids running a program at the module top level. This fork has no ES1001 yet and treats `main()` there as an Effect value.

## Next hardest risk

Position-dependent lowering now happens inside overload resolution, in one checker pass ([Programs, plain functions, and callbacks](#programs-plain-functions-and-callbacks)): the overload probes behind the single-pass experiment get the experiment's answers (case `lambda-forms`), and no construct needed a second pass. What remains of that risk is E and R through a call cycle: a recursive component needs its rows iterated to a fixed point (D52), inside a checker that resolves return types lazily and reports cycles as TS7023.
