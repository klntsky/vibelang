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
cd tsc && CGO_ENABLED=0 go build -o ../built/local/tsc ./cmd/tsc   # about 75 s cold
effectscript/smoke.sh                                             # builds, then runs every case
effectscript/smoke.sh es-infer                                    # one case
./built/local/tsc -p effectscript/cases/ts-hello                  # the stock-compiler smoke test
```

The build embeds the `lib.*.d.ts` files; Herebyfile's `-tags=noembed` build copies them next to the binary instead. A cold build fills about 2.1 GB of Go build cache, or 1.7 GB with `GOFLAGS=-gcflags=all=-dwarf=false`.

Each directory in `effectscript/cases` is one case. `smoke.sh` compiles it with `tsc -p`, which must print nothing. It then compares every file in `expected/` with the same path under `out/`, and compares `expected/stdout.txt` with the output of `node run.mjs` or `node out/main.js`. A case whose `package.json` depends on `effect` gets a `node_modules` symlink to `$EFFECT_NODE_MODULES`, by default `../experiments/node_modules`.

| Case | Proves |
| --- | --- |
| `ts-hello` | The stock compiler still compiles and runs plain TypeScript |
| `es-parse` | An `.es` file parses and checks as TypeScript and emits `main.js` and `main.d.ts` |
| `es-wrap` | Exported functions and arrow functions lower to `Effect.fnUntraced`, and JavaScript runs them with `Effect.runSync` |
| `es-lower` | Type-directed `yield*`: pure helpers stay plain, Effect-returning TypeScript calls and `.es` calls run, operands run left to right, a `Config` stays data |
| `es-infer` | Unannotated functions get `Effect<A, E, R>`: a typed failure flows from `lookup` into `main`, the `.d.ts` declares it, and the module runs its own program |

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

## Seams in upstream files

EffectScript code lives in `tsc/internal/effectscript/` (5 files, 523 lines), which may import any internal package because it sits under `tsc/internal/`. It registers three hooks from `init`. Upstream code changes in two ways: one-line call sites in existing files, and new files that hold the hooks.

| Upstream file | Change | Lines (+/-) |
| --- | --- | --- |
| `tsc/internal/tspath/extension.go` | `ExtensionEs` in the TypeScript extension lists; `.es` declares to `.d.ts` | +9 / -8 |
| `tsc/internal/core/core.go` | `.es` is `ScriptKindTS` | +1 / -1 |
| `tsc/internal/compiler/emitter.go` | Run hook transformers first in `getScriptTransformers` | +2 / -0 |
| `tsc/cmd/tsc/main.go` | Blank import of `internal/effectscript` | +2 / -0 |
| `tsc/internal/checker/checker.go` | Apply call-type hooks where `checkCallExpression` returns and on the `getQuickTypeOfExpression` call shortcut; wrap `getReturnTypeFromBody` | +6 / -3 |
| `tsc/internal/compiler/extensions.go` (new) | `RegisterScriptTransformerHook` | +31 |
| `tsc/internal/checker/extensions.go` (new) | `WithChecker`, `RegisterCallExpressionTypeHook`, `RegisterInferredReturnTypeHook`, `NewTypeReference` | +62 |

That is 20 added and 12 removed lines in 5 existing upstream files, plus 93 lines in 2 new upstream-package files. A rebase onto a newer nightly can conflict only in those 5 files.

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

## What it cannot do yet

- `throw`, `try`/`catch`/`finally`, and `using` are plain JavaScript. A `throw` is a defect, and `catch` does not see a failed Effect.
- No services (D1), control operations (D2), prelude globals (D4), `pipe`, `wire`, `fork`, or handlers.
- No programs as values (D7). `Effect.retry(() => sync(store), policy)` passes a plain function where an Effect is expected.
- None of the ES diagnostics exist. A top-level `main()` is an Effect value, not ES1001. An Effect lambda passed to `Array.prototype.forEach` silently does nothing, not ES4301. A plain TypeScript call is not ES4401.
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
