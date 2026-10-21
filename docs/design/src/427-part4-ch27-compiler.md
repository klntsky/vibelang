## Part IV. The compiler

### 27. Compiler architecture

```ts dts
import type { Effect } from "vibelang-runtime"
export declare function double(n: number): number!!
```

The compiler writes ordinary TypeScript declarations with Effect contracts.
TypeScript consumers call an Effect-returning function; VibeLang consumers
run that Effect through an ordinary call. The implementation is in
`typescript/tsc/internal/vibelang/`; `typescript/VIBELANG.md` lists its hooks,
acceptance cases, and remaining limitations.

#### The Go fork

The repository holds `microsoft/TypeScript` at `257e874`, the source of
`typescript@next` 7.1.0-dev.20261015.1, imported without upstream test baselines
(D60). The Go module requires Go 1.27. Build from the repository root:

```sh
cd typescript/tsc
CGO_ENABLED=0 GOFLAGS=-trimpath go build -o ../built/local/tsc ./cmd/tsc
```

`sh scripts/build-vibelang.sh` instead puts the native executable in
`packages/compiler/native/vibelang` for the package launcher. `-trimpath` lets jj
workspaces share Go build-cache entries. The compiler embeds its library
`.d.ts` files, so the executable needs no adjacent library directory.

VibeLang lives in-tree, uses no unsafe struct mirrors or `go:linkname`
bindings, and registers hooks through a blank import in `cmd/tsc/main.go`.
Upstream seams handle `.vibe` recognition, module resolution, checker queries
and extension state, diagnostics, compiler options, and transform registration.
They are small commits in the repository's jj history, not a separately
applied patch series.

#### Pipeline

```text
.vibe and .ts sources
  -> TypeScript parser and binder
  -> TypeScript checker with VibeLang hooks
       prelude and service keys
       lambda forms inside overload resolution (D56)
       contracts, failures, requirements, provider graphs
       trust levels and diagnostics
  -> VibeLang AST transform under the checker lock
       Effect.fnUntraced generators and Effect regions
       runtime helpers and plain $direct variants
  -> TypeScript emit
       JavaScript, source maps, and ordinary .d.ts files
```

The parser keeps TypeScript's grammar. The checker resolves prelude names as
implicit imports from `vibelang-runtime`, and `effect`'s other root exports as
implicit imports from `effect`, unless a local declaration or import shadows
them (D4, D48, D146). Class hooks give fully abstract service classes an Effect
Context key's static side and their abstract members' instance shape (D1, D45).

Lowering runs before TypeScript's type erasure, queries the checker while
holding the checker lock, and rewrites the TypeScript AST directly. The
machine chapters' computation IR is a semantic model; the compiler builds no
separate IR.

A call that returns Effect itself runs; structured Effect subtypes such as
Config stay data until called (D24). An unannotated or Effect-contract pure
function gets a `$direct` variant for inline calls and an Effect form for the
public ABI (D9, D10); a plain return annotation keeps one plain function (D67).
Calls and operands keep source order. `throw` adds an expected failure; `try`,
`catch`, and `finally` become regions with completion records for jumps (D13,
D15, D46).

#### One checker pass

Lambda hooks join TypeScript's own overload resolution. Each candidate asks for
a plain function, an Effect function, or a program; the first that accepts
selects the form (D44, D56). A return in a program is checked against the
Effect's success type. `pipe` looks ahead to its first stage (D8); function
values and collections convert where the parameter expects programs (D7, D49).
D10 eligibility is a greatest fixpoint over the callee group. D12 callback
parameters carry inferred E and R type parameters.

D52 requires recursive E and R inference inside a call-graph component; the
acceptance cases and `typescript/VIBELANG.md` record current coverage. No
whole-program iterative lowering pass runs.

The earlier `prototype/esc`, an executable reference, uses TypeScript 5.9.3 and
repeated checks; its 46 examples are now native-compiler acceptance cases under
`typescript/vibelang/cases/`. `prototype/single-pass/RESULTS.md` records the
experiments behind D56. The native implementation doesn't duplicate source
text per lambda form.

#### Runtime boundary

Generated JavaScript imports only `effect` and `vibelang-runtime` (D30). The
runtime holds the continuation kernel and the helpers the compiler emits; it
runs on Effect's fiber, Context, and Scope and adds no scheduler. It checks for
`effect@4.0.5` at load time and rejects other versions.

| Runtime helper | Purpose |
| --- | --- |
| `$programs` | Convert collections of program functions |
| `$catch`, `$finally` | Expected-failure handling and cleanup |
| `$expected`, `$caught` | Legacy helpers for pre-D260 output; new plain contracts keep JavaScript exceptions |
| `$region`, `$using`, `$fork` | Block-owned scopes, resources, and children |
| `$service`, `$reference`, `$control` | Service keys, defaults, and control operations |
| `$trusted`, `$trustedPlain` | Classify listed foreign exceptions |
| `handle`, `resume`, `$tailResume` | Continuation handling |
| `$produce`, `$yield`, `$forOf`, `$forStream`, `$toStream` | Producers and Stream adaptation |

The runtime's README lists the full ABI. Compiler and helper changes belong in
one commit. The smoke harness builds the runtime and links every case to that
workspace's package, so incompatible versions can't pass unnoticed.

#### Declarations and metadata

Services emit as runtime key classes. Declaration emit writes the instance
shape as an interface plus a class whose base has type `ServiceKey<Users>`
(D45), so ordinary TypeScript can implement and provide the service:

```ts dts
export interface Users {
  find(id: string): User!UserNotFound!
  readonly "~vibelang/service"?: "@acme/app/src/users#Users"
}
declare const Users_base: import("vibelang-runtime").ServiceKey<Users>
export declare abstract class Users extends Users_base {}
```

A default service uses `ReferenceKey`. Declaration transforms add type imports
for prelude names, preserve D12's callback parameters, and rewrite source
specifiers to `.js` (D31). TypeScript consumers need only the `.js`, `.d.ts`,
`effect`, and runtime packages.

Chapter 15's control-class and `$direct` JSDoc metadata is a design target; the
compiler neither emits nor reads it. A consumer compiled against a declaration
uses the Effect form: correct, but without the direct-call optimization.
Trusted TypeScript declarations do support `/** @vibelang trusted */` and
`@throws {T}` (D23): each listed exception class joins E, and generated code
tests it with `instanceof` (D51).

#### Acceptance checks

```sh
cd typescript
sh vibelang/smoke.sh --status
```

The harness compiles cases in parallel, runs successful programs on Node, and
checks expected failure codes and optional diagnostic positions. A `pending`
case reports its owning compiler lane and doesn't fail the suite. Source-map
and tracing cases check original `.vibe` frames and generated spans (D34, D36).
The docs checker, `bun docs/tools/check-snippets.ts` run from the repository
root, checks user-guide snippets independently.
