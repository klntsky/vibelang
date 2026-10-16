# EffectScript fork of the TypeScript compiler

This directory is microsoft/TypeScript with a short stack of commits that teach the Go compiler (`tsc/`) EffectScript: TypeScript syntax in which every function is an Effect (effect@4.0.5). A call to a function that returns an Effect runs it, and the compiler infers each function's `Effect<A, E, R>`.

It is a first prototype. It lowers `.es` files with checker types, runs the output on Effect 4, and emits `.d.ts` files with the inferred Effect types. Services and control operations work (see "Services and control operations"); errors (`throw`, `try`), the prelude, and programs as values are not built yet. The design is `../docs/design/effectscript-design.md`; the syntax decisions are `../docs/design/DECISIONS.md`.

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
| Runnable | A source with a line `// expect-result: <json>` that exports `main()` | `tsc -p` reports no errors, and `out/run.mjs`, which the harness writes, prints `<json>`: the JSON of the success value of the Effect that `main()` returns |
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
| `regions-jumps` | `try`, `catch`, and `finally` run as regions: labeled and unlabeled `break` and `continue`, `return` from nested regions, a labeled `try`, a `try` as an `if` body, `switch` inside a region, and `var` in a region give JavaScript's results and `finally` order |

### Acceptance cases

The prototype's 46 examples (`../prototype/esc/examples/`) are cases named after their files, such as `ch04-wire` and `ref2-cycle.fail`. Each keeps its `.es` source and `.ts` helpers as the prototype has them, except that kernel imports name `@effectscript/runtime` and `@effectscript/runtime/kernel`. They compile with `module: nodenext` and `rewriteRelativeImportExtensions`, as the app template does. Every compiler lane turns its cases green and never turns a green case red. Each case is pending on the lane that owns its feature; a case that mixes features belongs to the lane that owns its first missing feature, and its `pending` file names the other lanes it needs:

| Owner | Feature | Cases |
| --- | --- | --- |
| F1 | Diagnostics, prelude, imports between files | `ch10-effect-gen.fail` |
| F2 | Programs as values, plain functions, callbacks | `ch05-programs`, `ch10-map-pausing-callback.fail`, `ch13-plain`, `ch14-callbacks`, `ref4-callbacks`, `ref4-pausing-function-value.fail`, `ref5-programs-as-data` |
| F3 | `throw`, regions, contracts | `ch02-contract-narrower.fail`, `ch02-functions`, `ch03-errors`, `ch03-safelookup-no-try.fail`, `ch12-contracts`, `ch15-failures`, `regions` |
| F5 | Services, layers, `wire` | `ch01-first-look`, `ch04-provider-escapes.fail`, `ch04-services`, `ch04-users-audited.fail`, `ch04-wire`, `ch12-rates-bare-return.fail`, `ch12-welcome-missing-mailer.fail`, `nominal-services.fail`, `ref1-existing-service`, `ref2-ambiguous-provider.fail`, `ref2-cycle.fail`, `ref2-missing-provider.fail`, `ref2-opaque-layer.fail`, `ref2-provider-graph` |
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

## Seams in upstream files

EffectScript code lives in `tsc/internal/effectscript/`, which may import any internal package because it sits under `tsc/internal/`. It registers its hooks from `init`. Upstream code changes in three ways: one-line call sites in existing files, new files that hold the hooks, and the `effectscript` compiler option, which is added by hand to the generated option tables because the generator's type model has no nested objects.

| Upstream file | Change | Lines (+/-) |
| --- | --- | --- |
| `tsc/internal/tspath/extension.go` | `ExtensionEs` in the TypeScript extension lists; `.es` declares to `.d.ts` | +9 / -8 |
| `tsc/internal/core/core.go` | `.es` is `ScriptKindTS` | +1 / -1 |
| `tsc/internal/compiler/emitter.go` | Run hook transformers first in `getScriptTransformers` | +2 / -0 |
| `tsc/cmd/tsc/main.go` | Blank import of `internal/effectscript` | +2 / -0 |
| `tsc/internal/checker/checker.go` | Apply call-type hooks where `checkCallExpression` returns and on the `getQuickTypeOfExpression` call shortcut; wrap `getReturnTypeFromBody`; hold extension state; run file check hooks at the end of `checkSourceFile`; filter what `getDiagnostics` returns; ask class base hooks in `getBaseConstructorTypeOfClass` and `resolveBaseTypesOfClass`; ask member receiver hooks before `checkPropertyAccessExpressionOrQualifiedName` looks up a member; ask call signatures hooks in `resolveCallExpression` when the callee has none | +21 / -4 |
| `tsc/internal/checker/checker.go` | Ask the catch clause variable type hook in `getTypeForVariableLikeDeclaration`; a catch variable is always initialized | +4 / -1 |
| `tsc/internal/compiler/program.go` | Comment directives skip diagnostics from exempt sources | +1 / -1 |
| `tsc/internal/core/options_generated.go` (generated) | `CompilerOptions.EffectScript`, in `Clone` and `Equals` | +6 |
| `tsc/internal/tsoptions/declarations_generated.go` (generated) | The `effectscript` declaration | +8 |
| `tsc/internal/tsoptions/options_generated.go` (generated) | Parse, merge for `extends`, compare for semantic diagnostics and emit, and serialize for `--showConfig` | +15 / -1 |
| `tsc/internal/tsoptions/commandlineoption.go`, `tsconfigparsing.go` | The option's extra validation | +4 |
| `tsc/internal/compiler/extensions.go` (new) | `RegisterScriptTransformerHook`, `RegisterDirectiveExemptSource` | +47 |
| `tsc/internal/checker/extensions.go` (new) | `WithChecker`, `RegisterCallExpressionTypeHook`, `RegisterInferredReturnTypeHook`, `NewTypeReference`, `ExtensionState`, `CompilerOptions`, `AddDiagnostic`, `RegisterSourceFileCheckHook`, `RegisterDiagnosticsFilterHook`, `RegisterCatchVariableTypeHook`, `GetTypeOfExpressionBeforeWidening` | +168 |
| `tsc/internal/checker/extensions_classes.go` (new) | `RegisterClassBaseHook`, `Program`, `GetStringLiteralType`, `NewOptionalReadonlyPropertyType` | +48 |
| `tsc/internal/checker/extensions_calls.go` (new) | `RegisterMemberReceiverHook`, `RegisterCallSignaturesHook`, `NewCallSignature` | +52 |
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

- **Which calls run.** A call runs, as `yield* f(x)`, when its resolved signature returns effect's own `Effect` interface, or when it calls an `.es` function that is an Effect. Types that extend `Effect` (`Config`, `Exit`, Context keys, `TaggedError` instances) are data (design D24). Only calls in a lowered function's own body run; calls at the module top level, in parameter initializers, and in nested plain functions stay plain.
- **Which functions become Effects.** A function declaration, function expression, or arrow function in an `.es` file becomes `Effect.fnUntraced(function* …)` when its own body runs a call. Every other function stays plain JavaScript (D10), so `[1, 2, 3].map(double)` works. A function on a call cycle counts as an Effect, which is always correct and only slower.
- **Output shape.** A declaration becomes a `const` (with `export` or `export default`), and the file imports `* as Effect from "effect/Effect"` under a free name. Operands keep JavaScript's order, for example `(yield* greet("a")) + (yield* greet("bc"))`.
- **Types.** Inside a lowered body, a call that runs has the Effect's success type, so `findName(2).toUpperCase()` checks. An unannotated `.es` Effect function has return type `Effect<A, E, R>`. A comes from its returns; E and R are the unions of the error and requirement channels of every Effect it runs. The `.d.ts` declares `main(): Effect.Effect<void, NotFound, never>`, and a callback such as `(e) => Console.log(…)` passed to `Effect.catch` checks and runs as an Effect function.

Checking takes one pass. The analyzer decides from function bodies and callee types, and the hooks type each call and lambda inside TypeScript's own overload resolution.

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
## Failures and regions

`try`, `catch`, and `finally` in an Effect function compile to Effect's failure and cleanup frames, never to a JavaScript `try`, because Effect does not run a generator's JavaScript `finally` when a fiber fails or is interrupted (design chapter 26).

### Types

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
## What it cannot do yet

- `throw` and `using` are plain JavaScript. A `throw` is a defect.
- `arguments` inside a region's closure is the closure's, not the function's. A `return`, `break`, or `continue` that leaves a `finally` block is not dispatched.
- No prelude globals (D4), `pipe`, `wire`, `fork`, or handlers. Services and control operations exist, but none of their diagnostics (ES2101, ES3001-ES3006, ES3101, ES3201-ES3204) do, and a service's `.d.ts` declares an abstract class, not the key class TypeScript callers need.
- A provider's object literal is checked against the service's shape only as TypeScript checks any assignment, so an escaping error is TS2322, not ES2001 or ES3101 naming the member.
- No programs as values (D7). `Effect.retry(() => sync(store), policy)` passes a plain function where an Effect is expected.
- Most ES diagnostics do not exist yet. A top-level `main()` is an Effect value, not ES1001. An Effect lambda passed to `Array.prototype.forEach` silently does nothing, not ES4301. A plain TypeScript call is not ES4401.
- An annotated function keeps its annotated type. `function f(): number` with Effect calls is `() => number` to TypeScript but returns an Effect at run time. An Effect contract annotation is checked against the body as a TypeScript type, not as a contract.
- No `$direct` variants or JSDoc metadata. A pure `.es` function exports a plain function type.
- Accessors, constructors, async functions, and generators are never lowered. `this` and `arguments` inside a lowered arrow function rebind to the generator, and `arguments` and `super` inside a lowered method do not reach the method.
- A lowered function declaration is a `const`, so a call above it at the module top level hits the temporal dead zone.
- Module resolution does not probe for `.es`, so one `.es` file cannot import another.
- E and R miss contributions that arrive through a call cycle (no fixed point). A union of Effects is not treated as an Effect.
- Synthesized nodes have no source positions, and the language service is untested.

## Differential check against the TypeScript-API prototype

`../prototype/esc` compiled copies of `es-wrap`, `es-lower`, and `es-infer`, with plain return annotations removed because the prototype rejects them (ES2003). Both compilers put `yield*` on the same call sites (2, 12, and 6) and leave the same calls plain, including `Config.Port(...)` and `[1, 2, 3].map(double)`. Both infer `lookup: Effect<string, NotFound, never>` and `main: Effect<void, NotFound, never>`.

The prototype reports ES1001 on the top-level `Effect.runPromise(Effect.catch(main(), …))`, because the design (Chapter 11, "Where programs start") forbids running a program at the module top level. This fork has no ES1001 yet and treats `main()` there as an Effect value.

## Next hardest risk

Position-dependent lowering inside overload resolution. D7 (a zero-argument function is a program where an Effect is expected) and D9 (a function value in a plain JavaScript position passes its plain variant, or else ES4301) decide how a lambda lowers from the overload that resolution picks. The prototype needed up to 5 whole-program check passes for this, and found that only the declared, uninstantiated parameter type of the resolved overload works as the expected type. This fork avoids the loop only because it ignores position.

The same machinery must compute E and R as a fixed point per strongly connected component of the call graph, inside a checker that resolves return types lazily and reports cycles as TS7023.

Experiment: add a hook in `chooseOverload` that classifies each lambda argument against each candidate's declared parameter type. Then run the prototype's `ch05-programs.es` and `ch14-callbacks.es`, ported to explicit imports, and compare its decisions with the prototype's. If any construct needs a second check pass to match, single-pass checking is falsified for that construct.
