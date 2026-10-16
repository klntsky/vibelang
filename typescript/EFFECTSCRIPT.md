# EffectScript fork of the TypeScript compiler

This directory is microsoft/TypeScript with a short stack of commits that teach the Go compiler (`tsc/`) EffectScript: TypeScript syntax in which every function is an Effect (effect@4.0.5). A call to a function that returns an Effect runs it, and the compiler infers each function's `Effect<A, E, R>`.

It is a first prototype. It lowers `.es` files with checker types, runs the output on Effect 4, and emits `.d.ts` files with the inferred Effect types. Errors (`throw`, `try`), services, the prelude, and programs as values are not built yet. The design is `../docs/design/effectscript-design.md`; the syntax decisions are `../docs/design/DECISIONS.md`.

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

## Seams in upstream files

EffectScript code lives in `tsc/internal/effectscript/`, which may import any internal package because it sits under `tsc/internal/`. It registers its hooks from `init`. Upstream code changes in three ways: one-line call sites in existing files, new files that hold the hooks, and the `effectscript` compiler option, which is added by hand to the generated option tables because the generator's type model has no nested objects.

| Upstream file | Change | Lines (+/-) |
| --- | --- | --- |
| `tsc/internal/tspath/extension.go` | `ExtensionEs` in the TypeScript extension lists; `.es` declares to `.d.ts` | +9 / -8 |
| `tsc/internal/core/core.go` | `.es` is `ScriptKindTS` | +1 / -1 |
| `tsc/internal/compiler/emitter.go` | Run hook transformers first in `getScriptTransformers` | +2 / -0 |
| `tsc/cmd/tsc/main.go` | Blank import of `internal/effectscript` | +2 / -0 |
| `tsc/internal/checker/checker.go` | Apply call-type hooks where `checkCallExpression` returns and on the `getQuickTypeOfExpression` call shortcut; wrap `getReturnTypeFromBody`; hold extension state; run file check hooks at the end of `checkSourceFile`; filter what `getDiagnostics` returns | +9 / -4 |
| `tsc/internal/compiler/program.go` | Comment directives skip diagnostics from exempt sources | +1 / -1 |
| `tsc/internal/core/options_generated.go` (generated) | `CompilerOptions.EffectScript`, in `Clone` and `Equals` | +6 |
| `tsc/internal/tsoptions/declarations_generated.go` (generated) | The `effectscript` declaration | +8 |
| `tsc/internal/tsoptions/options_generated.go` (generated) | Parse, merge for `extends`, compare for semantic diagnostics and emit, and serialize for `--showConfig` | +15 / -1 |
| `tsc/internal/tsoptions/commandlineoption.go`, `tsconfigparsing.go` | The option's extra validation | +4 |
| `tsc/internal/compiler/extensions.go` (new) | `RegisterScriptTransformerHook`, `RegisterDirectiveExemptSource` | +47 |
| `tsc/internal/checker/extensions.go` (new) | `WithChecker`, `RegisterCallExpressionTypeHook`, `RegisterInferredReturnTypeHook`, `NewTypeReference`, `ExtensionState`, `CompilerOptions`, `AddDiagnostic`, `RegisterSourceFileCheckHook`, `RegisterDiagnosticsFilterHook` | +136 |
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
| ES1003 | `async`, `await`, `for await`, and `await using` in a `.es` file; covers TypeScript's errors about them, such as an `await` outside an `async` function | `syntax.go` |
| ES1004 | `using` that no function encloses, including in a class static block | `syntax.go` |

`compilerOptions.effectscript` (D34) sets the category of the two codes that are policy, not soundness, and the tracing mode:

```json
"effectscript": { "diagnostics": { "ES2004": "off", "ES4403": "warning" }, "tracing": "exported" }
```

ES2004 defaults to `"warning"` and ES4403 to `"error"`; every other ES code is always an error. An unknown key or invalid value is reported at its node in `tsconfig.json` with TypeScript's own messages (TS5023, TS6046). `CompilerOptions.EffectScript.Tracing` holds the tracing mode for lane F8. `tsc --showConfig` prints the block; `.tsbuildinfo` does not record it yet, so changing it does not invalidate an incremental build.

## What it cannot do yet

- `throw`, `try`/`catch`/`finally`, and `using` are plain JavaScript. A `throw` is a defect, and `catch` does not see a failed Effect.
- No services (D1), control operations (D2), prelude globals (D4), `pipe`, `wire`, `fork`, or handlers.
- No programs as values (D7). `Effect.retry(() => sync(store), policy)` passes a plain function where an Effect is expected.
- Most ES diagnostics do not exist yet. A top-level `main()` is an Effect value, not ES1001. An Effect lambda passed to `Array.prototype.forEach` silently does nothing, not ES4301. A plain TypeScript call is not ES4401.
- An annotated function keeps its annotated type. `function f(): number` with Effect calls is `() => number` to TypeScript but returns an Effect at run time. An Effect contract annotation is checked against the body as a TypeScript type, not as a contract.
- No `$direct` variants or JSDoc metadata. A pure `.es` function exports a plain function type.
- Methods, accessors, constructors, async functions, and generators are never lowered. `this` and `arguments` inside a lowered arrow function rebind to the generator.
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
