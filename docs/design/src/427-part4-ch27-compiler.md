## Part IV. The compiler

### 27. Compiler architecture

```ts dts
import type { Effect } from "@effectscript/runtime"
export declare function double(n: number): Effect<number, never, never>
```

The compiler writes ordinary TypeScript declarations with Effect contracts.
TypeScript consumers call an Effect-returning function; EffectScript consumers
run that Effect through an ordinary call. The current implementation is in
`typescript/tsc/internal/effectscript/`. `typescript/EFFECTSCRIPT.md` lists the
hooks, acceptance cases, and remaining limitations.

#### The Go fork

The repository contains `microsoft/TypeScript` at `257e874`, the source of
`typescript@next` 7.1.0-dev.20261015.1, imported without upstream test baselines
(D60). The Go module requires Go 1.27. Build from the repository root:

```sh
cd typescript/tsc
CGO_ENABLED=0 GOFLAGS=-trimpath go build -o ../built/local/tsc ./cmd/tsc
```

Use `sh scripts/build-esc.sh` instead to place the native executable in
`packages/compiler/native/esc` for the package launcher. `-trimpath` lets jj
workspaces share Go build-cache entries. The compiler embeds its library
`.d.ts` files, so the executable needs no adjacent library directory.

EffectScript lives in-tree and registers hooks through a blank import in
`cmd/tsc/main.go`. Upstream seams handle `.es` recognition, module resolution,
checker queries and extension state, diagnostics, compiler options, and
transform registration. The implementation uses no unsafe struct mirrors or
`go:linkname` bindings. The seams are small commits in the repository's jj
history, rather than a separately applied patch series.

#### Pipeline

```text
.es and .ts sources
  -> TypeScript parser and binder
  -> TypeScript checker with EffectScript hooks
       prelude and service keys
       lambda forms inside overload resolution (D56)
       contracts, failures, requirements, provider graphs
       trust levels and diagnostics
  -> EffectScript AST transform under the checker lock
       Effect.fnUntraced generators and Effect regions
       runtime helpers and plain $direct variants
  -> TypeScript emit
       JavaScript, source maps, and ordinary .d.ts files
```

The parser keeps TypeScript's grammar. The checker resolves prelude names as
implicit imports from `@effectscript/runtime`, unless a local declaration or
import shadows them (D4, D48). Class hooks give fully abstract service classes
the static side of an Effect Context key and the instance shape of their
abstract members (D1, D45).

Lowering runs before TypeScript's type erasure. It queries the checker while
holding the checker lock and rewrites the TypeScript AST directly. The
computation IR described by the machine chapters is a semantic model; the
current compiler does not materialize a separate IR.

A call that returns Effect itself runs; structured Effect subtypes such as
Config remain data until called (D24). A pure function gets a `$direct` variant
for inline calls and an Effect form for the public ABI (D9, D10). Calls and
operands retain source order. `throw` adds an expected failure; `try`, `catch`,
and `finally` become regions with completion records for jumps (D13, D15, D46).

#### One checker pass

Lambda hooks participate in TypeScript's own overload resolution. Each
candidate asks for a plain function, an Effect function, or a program; the
first accepting candidate selects the form (D44, D56). A return in a program
is checked against the Effect's success type. `pipe` looks ahead to its first
stage (D8), and function values and collections convert where the parameter
expects programs (D7, D49). D10 eligibility uses a greatest fixpoint over the
callee group. D12 callback parameters carry inferred E and R type parameters.

D52 requires recursive E and R inference inside a call-graph component. The
compiler's acceptance cases and `typescript/EFFECTSCRIPT.md` record its current
coverage; no whole-program iterative lowering pass runs.

The earlier `prototype/esc` uses TypeScript 5.9.3 and repeated checks as an
executable reference. Its 46 examples are now native-compiler acceptance cases
under `typescript/effectscript/cases/`. `prototype/single-pass/RESULTS.md`
records the experiments that established D56. The native implementation does
not duplicate source text per lambda form.

#### Runtime boundary

Generated JavaScript imports only `effect` and `@effectscript/runtime` (D30).
The runtime contains the continuation kernel and the helpers the compiler
emits. It runs on Effect's fiber, Context, and Scope; it adds no scheduler.
The package checks `effect@4.0.5` at load time and rejects other versions.

| Runtime helper | Purpose |
| --- | --- |
| `$programs` | Convert collections of program functions |
| `$catch`, `$finally` | Expected-failure handling and cleanup |
| `$expected`, `$caught` | Expected failures in plain JavaScript regions |
| `$region`, `$using`, `$fork` | Block-owned scopes, resources, and children |
| `$service`, `$reference`, `$control` | Service keys, defaults, and control operations |
| `$trusted`, `$trustedPlain` | Classify listed foreign exceptions |
| `handle`, `resume`, `$tailResume` | Continuation handling |
| `$produce`, `$yield`, `$forOf`, `$forStream`, `$toStream` | Producers and Stream adaptation |

The runtime's README lists the full ABI. Compiler and helper changes belong
in the same commit. The smoke harness builds the runtime and links every case
to that workspace's package, so incompatible versions cannot pass unnoticed.

#### Declarations and metadata

Services emit as runtime key classes. Declaration emit writes the instance
shape as an interface and a class whose base has the `ServiceKey<Users>` type
(D45), so ordinary TypeScript can implement and provide the service:

```ts dts
export interface Users {
  find(id: string): Effect.Effect<User, UserNotFound, never>
  readonly "~esc/service"?: "@acme/app/src/users#Users"
}
declare const Users_base: import("@effectscript/runtime").ServiceKey<Users>
export declare abstract class Users extends Users_base {}
```

A default service uses `ReferenceKey`. Declaration transforms add type imports
for prelude names, preserve D12's callback parameters, and rewrite source
specifiers to `.js` (D31). TypeScript consumers need only the `.js`, `.d.ts`,
`effect`, and runtime packages.

Chapter 15's control-class and `$direct` JSDoc metadata is a design target.
The current compiler does not emit or read that metadata. A consumer compiled
against a declaration uses the Effect form, which preserves correctness at
the cost of the direct-call optimization. Trusted TypeScript declarations do
support `/** @effectscript trusted */` and `@throws {T}` (D23): each listed
exception class joins E and generated code tests it with `instanceof` (D51).

#### Acceptance checks

```sh
cd typescript
sh effectscript/smoke.sh --status
```

The harness compiles cases in parallel, runs successful programs on Node, and
checks expected failure codes and optional diagnostic positions. Cases marked
`pending` report their owning compiler lane and do not fail the suite.
Source-map and tracing cases check original `.es` frames and generated spans
(D34, D36). The docs checker independently checks user-guide snippets with
`bun docs/tools/check-snippets.ts` from the repository root.
