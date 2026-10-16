### 28. Tooling

```sh
npm install effect@4.0.5 @effectscript/runtime
npm install --save-dev @effectscript/compiler
npx esc --noEmit        # check every .ts and .es file in the program
npx esc                 # check, then emit .js, .d.ts, and source maps
npx esc --watch         # rebuild on change
```

```json
// tsconfig.json
{
  "compilerOptions": {
    "strict": true,
    "module": "nodenext",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "effectscript": {
      "diagnostics": { "ES4403": "warning" },
      "tracing": "exported"
    }
  },
  "include": ["src"]
}
```

The compiler is one native binary, `esc`, the fork of TypeScript's `tsc` (Chapter 27). It reads the same `tsconfig.json`, accepts every `tsc` flag, and checks `.ts` and `.es` files as one program. The name is EffectScript, the extension is `.es`, and package names use the `@effectscript` scope (D58). The compiler source is `typescript/` in the same private repository as the spec, runtime, and tools (D60).

#### Packages and versions

| Package | What it holds | Install as |
| --- | --- | --- |
| `@effectscript/compiler` | The `esc` binary, through per-platform optional dependencies (`@effectscript/compiler-darwin-arm64`, `-linux-x64`, and so on), as `typescript@7` ships `tsc` | A dev dependency |
| `@effectscript/runtime` | The kernel and the helpers generated code imports: `handle`, `tailResume`, `catchExpected`, `suspend`, `handoff`, `Control`, the key helpers `serviceKey`, `controlKey`, and `referenceKey`, `Yield`, and the producer adapter | A dependency |
| `@effectscript/vite` | The Vite plugin | A dev dependency, for Vite projects |
| `@effectscript/oxlint-config` | The Oxlint settings for `.es` files | A dev dependency, for Oxlint users |
| `effectscript.effectscript` | The VS Code extension | From the VS Code marketplace |

| Component | v0 supports |
| --- | --- |
| `effect` | 4.0.5 exactly. The runtime checks the version when it loads and refuses any other, because the kernel reads Effect internals (Chapter 25) |
| TypeScript | The compiler is TypeScript 7.1's checker (Chapter 27), so `.ts` files in the program check as TypeScript 7.1 checks them. Emitted declarations are ordinary `.d.ts` files that TypeScript 7 reads |
| JavaScript runtimes | Wherever `effect@4.0.5` and its platform packages run. The evidence in Part III was gathered on Node.js 26.10 and Bun 1.4.1 |
| Building the compiler | Go 1.27 |

`esc` adds no commands of its own. `esc` checks and emits, `esc --noEmit` checks, `esc --watch` and `esc -b` rebuild, `esc --lsp --stdio` serves editors, and `esc --api` serves build tools.

#### Building

Libraries and servers build with `esc` or `esc --watch` and run the emitted JavaScript. Applications built with Vite add the plugin:

```ts ts
// vite.config.ts
import { defineConfig } from "vite"
import effectscript from "@effectscript/vite"

export default defineConfig({
  plugins: [effectscript()]
})
```

- **Compilation.** The plugin starts one `esc --api` process for the whole program and asks it for each `.es` module Vite requests, so types and contracts come from the same checker as `esc`.
- **Source maps.** Each module is served with a map composed from `.es` through the generated TypeScript to JavaScript.
- **Diagnostics.** Compiler errors print in the terminal and appear in Vite's error overlay, at `.es` spans.
- **Hot reload.** An edit re-emits the module. If its exported contracts, control classes, or `$direct` variants changed, its importers are invalidated too, because their generated code depends on those facts (Chapter 15).

Module specifiers name source files. In `.es`, `import { welcome } from "./welcome.es"` and `import { formatName } from "./format.ts"`; the emitted JavaScript imports `./welcome.js` and `./format.js`. TypeScript files import an EffectScript module as `./welcome.js`, which the compiler resolves to `welcome.es` as it resolves `.js` to `.ts`. A published library ships `.js` and `.d.ts` files with the metadata tags of Chapter 27, so a consumer needs no compiler unless it writes `.es` itself.

#### The editor

The language server is the fork's own, from the same binary: `esc --lsp --stdio`, taken from the project's `@effectscript/compiler`. It serves the `.es` and `.ts` files of the program, so one server owns both.

| Editor | Setup |
| --- | --- |
| VS Code | Install `effectscript.effectscript`. It registers the `effectscript` language for `*.es`, starts `esc --lsp --stdio`, and sets `"typescript.validate.enable": false` in workspaces that contain `.es` files, so VS Code's built-in TypeScript server does not report the same `.ts` errors a second time |
| Neovim, Helix, Zed, Emacs, JetBrains IDEs (through LSP4IJ) | Configure the editor's LSP client to run `esc --lsp --stdio` for `*.es` and `*.ts`, with `tsconfig.json` as the root marker, and turn off the editor's other TypeScript server for the project |
| GitHub | Add `*.es linguist-language=TypeScript` to `.gitattributes`; GitHub otherwise highlights `.es` as JavaScript |

v0 ships these editor features:

| Feature | What it does |
| --- | --- |
| Hover | A function's inferred contract and control class; at a call, the success value's type |
| Completions | TypeScript's, plus service members after `Users.`, the prelude names, and, in a `catch`, the error classes its `try` block can raise |
| Quick fixes | Every diagnostic with a mechanical fix has one; the code table below lists them |
| Refactors | Convert an `Effect.gen` or `Effect.fn` function to an EffectScript function; convert a `Context.Service` class to an abstract service class; implement a service, scaffolding an object literal or an `implements` class with every member; write a function's inferred contract as its annotation |

The `Effect.gen` conversion removes `Effect.gen` or `Effect.fn`, `function*`, and `yield*`; turns `yield* Service` followed by a method call into `Service.method()`; turns `return yield* new E()` into `throw new E()`; turns a trailing `Effect.catchTag` or `Effect.ensuring` stage into `try`/`catch` or `finally`; and leaves every other combinator as a call with `() =>` programs. It is offered only when the whole function converts, so a function that keeps a construct with no EffectScript form stays in its `.ts` file.

#### Diagnostics

Every ES diagnostic is an error, except two. Each error marks code that has no meaning, or code whose generated output would break a rule of Parts II and III, so none can be lowered and `// @ts-expect-error` does not apply to them; it keeps its TypeScript meaning for TypeScript's own codes. The two exceptions are policy, not soundness: ES2004, a `catch` that can never run, is a warning by default, and ES4403, a clock or randomness read, is an error by default. A project may set either to `"error"`, `"warning"`, or `"off"` in `compilerOptions.effectscript.diagnostics`; an ES4403 read then compiles as a trusted `sync` call that tests cannot control.

| Option in `compilerOptions.effectscript` | Values | Default |
| --- | --- | --- |
| `diagnostics` | `"ES2004"` and `"ES4403"`, each `"error"`, `"warning"`, or `"off"` | ES2004 `"warning"`, ES4403 `"error"` |
| `tracing` | `"none"`: no spans. `"exported"`: exported functions compile with `Effect.fn` spans. `"all"`: every function that compiles to frames does | `"none"` |

`esc` exits non-zero when any error is reported. Diagnostics print at `.es` spans, in the same form as the examples in this document, in the terminal, the editor, and Vite's overlay.

Every code, with its quick fix:

| Code | What it reports | Quick fix | Chapter |
| --- | --- | --- | --- |
| ES1001 | A module's top level runs a program | Export a function instead | 13 |
| ES1002 | A generator-based API (`Effect.gen`) in a `.es` file, where `function*` declares a producer | Convert the function | 10 |
| ES1003 | `async`, `await`, `for await`, or `await using`: a call already waits | Remove them | 10 |
| ES1004 | `using` at a module's top level | Move it into a function | 6 |
| ES1005 | A runner inside code that a fiber runs | Call the program, or `fork` it | 13 |
| ES2001 | An expected failure escapes the written contract | Catch it, widen the contract, or `Effect.orDie` | 2, 14 |
| ES2002 | A requirement escapes the written contract | Widen the contract, or provide the service | 14 |
| ES2003 | A return annotation on a function with a body that is not `Effect<A, E, R>` with all three slots, each checked or written `infer Name` | For `Effect<T>`, offer both `Effect<T, never, never>` and `Effect<T, infer E, infer R>`; for a bare `T`, write a contract | 14 |
| ES2004 | A `catch` that can never run (a warning by default) | Use the form that can fail, or `Effect.try` | 3 |
| ES2005 | A `resumeWith` program that needs a service or fails outside the operation's contract | Read the service in the clause first | 9 |
| ES2101 | A declaration without a body that has no full contract | Write `Effect<T, never, never>` | 14 |
| ES2102 | A recursive function whose A depends on its own result | Write the contract | 14 |
| ES3001 | Two services share a key | Rename one | 4 |
| ES3002 | A class extends a service | Replace `extends` with `implements` | 4 |
| ES3003 | `new` on a service | Use `Users()` for the implementation | 4 |
| ES3004 | `instanceof` on a service | None: a service is a key, not a base class | 4 |
| ES3005 | A generic service class | Use an interface for the generic type | 4 |
| ES3006 | A static member with the same name as a service member | Rename one | 4 |
| ES3101 | A provider looks a service up per call where the contract allows no requirements | Capture the service when the layer is built, or declare it | 4 |
| ES3201 | No provider for a required service, with the path that needs it | Add the provider the program defines to `wire` | 4 |
| ES3202 | Two providers for one service | Remove one | 4 |
| ES3203 | Providers form a cycle | None | 4 |
| ES3204 | A `wire` argument whose type does not name its outputs | Compose it with `Layer.provide` | 4 |
| ES4201 | A control operation inside a combinator that runs it on another fiber | Handle the operation inside the branch | 9, 16 |
| ES4202 | A TypeScript function whose R holds a control operation | None in v0 | 9 |
| ES4301 | A callback that can pause or fail where a plain function is expected | Rewrite as `Effect.forEach` or `for...of` | 10, 16 |
| ES4401 | A plain TypeScript call with no boundary or trusted contract | Wrap it in `Effect.try`, or tag the declaration `@effectscript trusted` | 10 |
| ES4402 | A Node.js or browser API | Replace the import with the Effect module the error names | 11 |
| ES4403 | A clock or randomness read (an error by default) | Use `Clock`, `DateTime`, or `Random` | 10 |

#### Linting, Oxlint, and Vite Plus

Each diagnostic has exactly one owner, so nothing is reported twice:

| Diagnostics | Owner | Files |
| --- | --- | --- |
| Type errors (TypeScript's codes) and language errors (ES codes) | `esc`, through the CLI, the language server, and the Vite plugin | `.ts` and `.es` |
| Syntactic lint rules | Oxlint | `.ts` and `.es`: EffectScript adds no syntax, so Oxlint's TypeScript parser reads `.es` files |
| Type-aware lint rules, including Effect's `@effect/tsgo` rules through tsgolint | Oxlint | `.ts` only |

Oxlint's type-aware mode embeds TypeScript's checker, not EffectScript's, so on `.es` files it would analyze the wrong semantics: a rule that flags an unused `Effect` value, for example, would flag every `.es` call statement whose callee returns one. `@effectscript/oxlint-config` turns type-aware rules off for `**/*.es` and turns off the syntactic rules whose premise EffectScript changes, such as `require-yield` (a producer need not yield):

```json
// .oxlintrc.json
{
  "extends": ["@effectscript/oxlint-config"]
}
```

Vite Plus runs Vite, Vitest, and Oxlint from one configuration, and it picks EffectScript up through the same two pieces: `@effectscript/vite` in its Vite plugins, for development, builds, and tests, and `@effectscript/oxlint-config` in its lint settings. A type-check step runs `esc --noEmit` in place of `tsc --noEmit`. Effect's own TypeScript tooling (`@effect/tsgo`) keeps working on `.ts` files; it does not run on `.es` files, whose semantics it does not model.

#### Debugging

`esc` writes source maps when `sourceMap` is on, and the Vite plugin always serves them. Each map is composed from `.es` through the generated TypeScript to JavaScript, so any debugger that reads standard source maps maps breakpoints and paused frames to `.es` lines: VS Code's JavaScript debugger for Node.js, Bun, and Chrome targets; Chrome and Edge DevTools; and JetBrains IDEs.

```text
Paused on breakpoint, requestApproval  checkout.es:22
Call stack (JavaScript)
  requestApproval            checkout.es:22         the running EffectScript function
  runLoop                    effect/dist/internal/effect.js
  evaluate                   effect/dist/internal/effect.js
```

What a debugger shows follows from the frame invariant (Chapter 26):

- **One EffectScript frame at a time.** Compiled functions are generator functions named after their source, so the running function appears by name at its `.es` line. Its callers are frames on the fiber's `_stack`, not JavaScript activations, so the debugger's call stack shows Effect's run loop below it, not the EffectScript call chain.
- **Regions** are closures named after their function and block: `profile$try`, `profile$catch`, `profile$finally`, `profile$using`.
- **Clauses and producers.** A clause is a generator function named after its operation member (`approve`), and a producer's body is one named after the producer (`lines`). A breakpoint in a clause shows the clause; the handled program's frames are inside `k`, a heap value, and appear again when the program resumes. A breakpoint in a producer shows the producer, and the loop body runs in the clause of the `for...of` that iterates it.
- **Stepping over a call that pauses** continues on a later run-loop step, so the debugger steps into Effect's run loop; set a breakpoint on the next line instead, as with `Effect.gen` code today.
- **Logical call stacks** across calls and suspensions come from spans: set `tracing` to `"exported"` or `"all"`, and use Effect's `DevTools` layer (`effect/devtools`) with its VS Code extension, which shows fibers and the span stack.
