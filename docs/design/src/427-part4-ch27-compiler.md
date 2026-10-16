## Part IV. The compiler

### 27. Compiler architecture

```ts dts
/** @effectscript control=pure direct=double$direct */
export declare const double: (n: number) => Effect.Effect<number>
```

That declaration is the compiler's whole contract with the outside world: an ordinary Effect type that any TypeScript code can use, plus a JSDoc tag that only EffectScript reads. This chapter covers how the compiler produces it.

#### Fork, with a content-mapper skeleton first

Lowering is type-directed: `f(x)` compiles differently when `f` is an EffectScript function, an Effect-returning TypeScript function, an `Effect` value, a service method, a control operation, a trusted function, or a value whose type extends `Effect` (Chapter 13). Plain JavaScript calls need control classes; contract checks need the checker. A TypeScript content mapper receives only a file name and text, and tsc emits no JavaScript for content-mapped files (`typescript-native-compiler.md` §1.7). The language therefore needs a compiler fork.

A content mapper can host a walking skeleton without any Go work:

- syntax-directed lowering;
- a runtime `bind` that runs a call's result when it is an `Effect`;
- every call on a heap frame;
- TypeScript inferring A, E, and R from the generated generators.

The skeleton validates the ABI, the declarations, and diagnostic mapping. Its runtime dispatch is wrong for the real language, because a call whose static type is `unknown` must not run an Effect it happens to return. It is a test rig, not a design. The prototype compiler below took the same path with TypeScript's compiler API in place of a content mapper, and with type-directed dispatch instead of a runtime `bind`.

#### Baseline

| Fact | Evidence |
| --- | --- |
| `microsoft/typescript-go` is archived (pushed 2026-08-31); its README at `89d5d5b` says "This Repo Is Closed" and points to `microsoft/TypeScript` | `typescript-native-compiler.md` §1.1 |
| The Go compiler lives in `microsoft/TypeScript` under `tsc/`, module `github.com/microsoft/TypeScript/tsc`, requiring Go 1.27; `main` was at `0e081d6` on 2026-10-15 | §1.2, §1.4 |
| typescript-go commit IDs do not exist in `microsoft/TypeScript`: history was rewritten on import | §1.2 |
| npm `typescript@latest` is 7.0.2; `typescript@next` is 7.1.0-dev.20261015.1, built from `main` at `257e874`; 7.1 stable is planned for 2026-11-24 | §1.3 |
| One binary serves the CLI (`tsc`), the language server (`--lsp --stdio`), and the API (`--api`) | §1.5 |
| No in-process plugins; the TypeScript team calls emit-side transforms "out of scope going forward" (#63771) | §1.7 |
| Go's `internal` rule compares import paths: only modules under `github.com/microsoft/TypeScript/tsc` may import `tsc/internal/*` | §1.6 (reproduced) |

Rules [Rule]:

- Fork `microsoft/TypeScript` at the `gitHead` of a published `typescript@next` nightly, and re-pin to the 7.1 stable tag when it ships. Pinning to a published build allows differential tests against the official binary.
- Put EffectScript in-tree under `tsc/internal/effectscript/`. Limit upstream edits to `.es` recognition and `Register*` hooks, wired by a blank import in `tsc/cmd/tsc/main.go`, carried as an ordered, digest-gated patch series (VibeLang's mechanics, `vibelang.md` §9).
- Do not build on shim modules. effect-tsgo carries 1,135 `//go:linkname` bindings and 24 unsafe struct mirrors, and an upstream `Symbol` layout change broke it (Effect-TS/tsgo#832).
- The fork needs Go 1.27.

#### Pipeline

```text
 .es source ──► parser: TypeScript's grammar, unchanged; function* in .es = producer
                  │
                  ▼
 binder + TypeScript checker: data types, generics, overloads, narrowing
   lambda forms per overload candidate, in the same pass (D56)
   classifies abstract classes: service (no extends) or control operation (extends Control)
   prelude names bound in every .es file
                  │                     ▲ .ts and .d.ts imports; Effect types via typeparser
                  ▼
 effect checker:  per function: contract (A, E, R), control class
                  fixed points inside call-graph cycles (D52); callback polymorphism
                  foreign calls: trusted contracts, boundaries, platform APIs (ES44xx)
                  fiber crossing (Chapter 16); second-class continuations (Chapter 9)
                  provider graphs (wire); service contract checks (ES3101)
                  │
                  ▼
 computation IR (checked, typed):
     call(kind, callee, args)   perform(op, args)   handle(region, clauses)
     catch(region, h)           final(region, fin)  scope(region)
     restore(region, Γ′)        mask(region, ι)     fork(program, owner)
     return(v)                  throw(e)            tailResume(k, v)
     direct(region)
                  │
        ┌─────────┴───────────────────────────┐
        ▼                                     ▼
 lowering to TypeScript:                declarations:
   generator frames, Effect regions,      .d.ts with the Effect ABI
   runtime calls, $direct variants        + metadata (control class, direct
        │                                    variant, producer form, keys)
        ▼                                  + declaration maps
 TypeScript emit: type erasure,
 module transforms → .js + composed source maps
```

EffectScript adds no syntax, so the parser is TypeScript's. What changes is meaning: the binder classifies fully abstract classes as services or control operations (Chapter 4, Chapter 9), and binds the prelude names in every `.es` file. The checked IR is where the machine meets the compiler. Every IR node is one transition of Chapter 20 or one region frame. Lowering is a mechanical map from IR to Effect and runtime calls, so the conformance tests can target the IR directly.

#### Runtime boundary

Generated code imports exactly two runtime packages: `effect`, pinned to the version the kernel supports, and `@effectscript/runtime`. The runtime package contains the kernel (Chapter 24), `suspend` and `handoff`, `catchExpected`, `tailResume`, `Control`, the key helpers `serviceKey`, `controlKey`, and `referenceKey`, the `Yield` operation, and the producer adapter. Everything else is ordinary Effect: services are `Context.Key`s, providers are Layers, and programs are `Effect` values. There is no EffectScript scheduler, no second Context, and no second fiber type. The kernel checks the Effect version at load time (`kernel/internals.ts`), so a version mismatch fails at startup instead of corrupting a fiber.

A service class compiles to an interface for its shape and a key class from the runtime, its static members kept (D45, Chapter 4):

```ts es
abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
  static readonly layer = UsersLive
}
```

```ts gen
export interface Users {
  find(id: string): Effect.Effect<User, UserNotFound>
  readonly "~esc/service"?: "@acme/app/src/users#Users"
}
export class Users extends serviceKey<Users>("@acme/app/src/users#Users") {
  static readonly layer = UsersLive
}
```

A control operation compiles the same way from `controlKey`, whose Context entry is handler evidence, and a service with `static readonly default` from `referenceKey`, which makes a `Context.Reference`:

```ts gen
export interface Approval {
  approve(request: string): Effect.Effect<boolean, ReviewBusy>
  readonly "~esc/service"?: "@acme/app/src/approval#Approval"
}
export class Approval extends controlKey<Approval>("@acme/app/src/approval#Approval") {}
```

The compiler checks a provider's implementation against the service's shape, taken from the `Effect<Users, …>` that `Layer.effect` expects of its program, not from the object literal the provider returns; that is what lets ES2001 and ES3101 name the method at fault (`prototype/esc/FINDINGS.md`, finding 9). Lowering also writes an explicit terminator after every generated statement, because a statement that begins with `(yield* …)` would otherwise join the previous line through automatic semicolon insertion (finding 28).

#### The prototype

`prototype/esc` is a prototype of this pipeline in TypeScript, about 3,560 lines on `typescript@5.9.3` and `effect@4.0.5`, with the experiments kernel as its runtime. It lowers by asking TypeScript's checker about the generated code and repeating until the answers stop changing. A second agent wrote its 46 example files from this document before the compiler existed: the 26 runnable ones produce their expected results, and the 20 that must fail report their codes. 25 of the document's 28 claimed contracts matched as first written; the other three led to D23's trusted contract for `profileCard`, D48's `Scope`, and D52's recursion fixpoint. Its findings settled D44-D53 (`prototype/esc/FINDINGS.md`).

#### One checker pass

The prototype re-checks the whole program until its decisions repeat, because three decisions form a loop: whether a call runs depends on the callee's type, a lambda's form depends on the overload that resolution picks, and resolution depends on the lambda's form. It needs 1 pass for 1 file, 2 for 16, 3 for 26, 4 for 2, and 5 for 1.

The compiler decides in one checker pass. The single-pass experiment (`prototype/single-pass/RESULTS.md`) tested that on the prototype's corpus: it patches TypeScript 5.9.3 so that `chooseOverload` classifies each lambda against each candidate's declared parameter type, emits every type-dependent choice as an intrinsic the checker resolves, and compares one check per file with the prototype's iteration.

```text
corpus   46 files: 817 call sites, 257 lambdas and methods
result   0 disagreements; the prototype's generated TypeScript, byte for byte, in 46 of 46 files
cost     46 checks in 2.1 s, against the prototype's 124 checks in 4.0 s (warm)
probes   2 synthetic files, 17 disagreements
           16  follow from 6 lambdas with two self-consistent lowerings: the prototype keeps
               the one its first guess reaches; one pass keeps the first accepting candidate's (D56)
            1  mutual recursion: E crosses the cycle only by iterating the component (D52)
```

Inside the checker, each link of the loop resolves when something first asks for it, so no link needs a pass of its own. The experiment turned into these rules for the checker:

| Rule | Why |
| --- | --- |
| Classify a lambda inside `chooseOverload`, per candidate, from the candidate's declared parameter type, and keep that first classification (D44, D56) | The lambda's own inferred type, or a deferred re-check that sees it, makes the answer depend on the first guess |
| Skip a lambda in TypeScript's first round only when the form the candidate selects is context sensitive | `Layer.effect(Users, () => {…})` selects a program, which is a call; skipping it by its plain form failed the only candidate (`ch04-services.es:75`) |
| Check a lambda at most once per form, with TypeScript's one-shot caches (contextual parameter types, resolved return types, inner calls' signatures) keyed by form | The harness duplicated each lambda's text per form instead: 2.03 times the final text |
| Apply the call rule to the call node | Narrowing must see the original call: `Exit.isFailure(x)` lost its narrowing through a wrapper (`ch15-failures.es:181`) |
| Type a call to a D10 function from plain code as the plain variant's value | Typed as an Effect, it rejected a plain-callback overload |
| Compute D10 on demand, as a greatest fixpoint over the callee's component | A D9 conversion, a lambda with no expected type, and a plain-code call need it before the caller is typed |
| Keep D8's look-ahead for `pipe` | `pipe`'s first argument takes its form from the next argument's parameter type |
| Iterate E and R inside a recursive component only (D52) | Mutual recursion is the one construct that needs a second check |

What remains is the last row, and holding the three forms inside the Go checker rather than in duplicated text. G19 tracks both (Chapter 30).

#### The Go fork

`~/effectscript-ts` is the fork the rules above call for, as a jj repository: 11 commits on `257e874`, the `gitHead` of `typescript@next` 7.1.0-dev.20261015.1. EffectScript lives in `tsc/internal/effectscript/`, 5 files and 523 lines, and registers three hooks: a script transformer that runs before type erasure, a call-expression type hook, and an inferred-return-type hook. The upstream diff is 20 added and 12 removed lines in 5 existing files, plus 93 lines in 2 new files that define the hooks, so a rebase onto a newer nightly can conflict only in those 5 files.

It already lowers calls by type and infers `Effect<A, E, R>` for `.es` functions, in one checker pass. A call runs when its resolved signature returns effect's own `Effect` or calls an `.es` Effect function, so a `Config` stays data (D24). A function whose body runs nothing stays plain JavaScript (D10). An unannotated function's E and R are the unions of what it runs, and the `.d.ts` declares them. On its three lowering cases it matches the prototype: the same `yield*` sites (2, 12, and 6), the same plain calls, including `Config.Port(…)` and `[1, 2, 3].map(double)`, and the same contracts, `lookup: Effect<string, NotFound, never>` and `main: Effect<void, NotFound, never>` (`~/effectscript-ts/EFFECTSCRIPT.md`).

It needs only one pass because it ignores position: it has no programs as values (D7) and no per-candidate forms (D56), and its E and R miss what arrives through a cycle (D52). Failure semantics (`throw`, `try`, `using`), services, control operations, the prelude, `pipe`, `wire`, `fork`, handlers, ES diagnostics, `$direct` metadata, methods, and imports between `.es` files are not built yet. The single-pass harness's decisions are the reference for porting D56 and D52 into it.

#### Declarations and metadata

The `.d.ts` file carries only the Effect ABI: functions as `(...) => Effect.Effect<A, E, R>`, producers as `(...) => Stream.Stream<A, E, R>`, service classes, Layers, and control operations. EffectScript-only facts travel as a JSDoc tag on the declaration, which TypeScript ignores and EffectScript reads, as in the example at the top of this chapter: the control class, the `$direct` variant, and a producer's internal form. A missing or unreadable tag means "control class `suspends`, no direct variant". That is always correct and only slower, so per-file bundling never fails closed (Chapter 15). VibeLang made "empty row means plain call" a second calling convention, and 21.5% of call sites in its multi-module programs then failed closed under per-file bundling (`vibelang.md` §6).

The same channel carries trusted contracts in the other direction. `/** @effectscript trusted */` and its `@throws` lines on a TypeScript declaration (Chapter 10) are read by the same code that reads the compiler's own tags.

#### What to borrow, and what not to

| From | Borrow | Do not inherit |
| --- | --- | --- |
| VibeLang checkpoint (`ebe89a2b`) | Fork mechanics: SHA pin, `go build -overlay`, digest-gated patches. The content mapper as the first seam. Effect-row state beside the checker (`Signature` fields or a `LinkStore`), never a `TypeFlags` bit. Lowering before `TypeEraserTransformer` through an extended `EmitResolver`. Intrinsics recognized by resolved symbol identity. Its one-shot and abandonment rules as conformance tests. Its foreign implicit-invocation predicate. | Eager functions with `Result` values. Two calling conventions chosen per call site. A private handler loop and scheduler beside the host. An `AsyncLocalStorage` bridge. Layers without acquisition or lifetime. |
| effect-tsgo (upstream 0.53.1) | `internal/typeparser`: recognizes Effect, Layer, and services, and extracts A/E/R from the `_A`/`_E`/`_R` members of `~effect/Effect`. Its diagnostics infrastructure, severities, and per-file overrides; its baseline harness with real Effect packages; its per-platform binary packaging. | Its after-check hook as a lowering seam: the hook can only append diagnostics and cannot change emit. |
| VibeLang, a measured gotcha | TypeScript's subtype reduction merged structurally identical classes and caused a runtime panic. Service classes are nominal for the same reason (Chapter 4; `experiments/scratch-abstract-services/probe.ts`). | |
