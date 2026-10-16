### 28. Tooling

```sh
# From the local EffectScript checkout:
sh scripts/setup.sh
bun create effectscript my-app --no-git
cd my-app
bun run build
bun run start
bun run test
bun run dev
bun x --no-install esc --noEmit
```

This private preview installs from the repository. The `@effectscript` packages
are not published to npm, and the extension is not on the marketplace (D58,
D60). D30's per-platform package distribution remains a release target.

Setup requires Go, Bun, and Node on PATH. The tested versions are Go 1.26
(with Go 1.27 downloaded automatically), Bun 1.4.1, and Node 26.10. Setup builds
`packages/compiler/native/esc`, installs and builds the runtime, and copies
`templates/app` into `~/.bun-create/effectscript`. The installed template uses
absolute `file:` dependencies on this checkout's compiler, runtime, and Vite
packages. Keep the checkout available; after updating it, rerun setup and
`bun install --force` in existing projects to refresh package copies.

Setup links `~/.local/bin/esc` when that directory is already on PATH and no
unrelated executable occupies the path. Project scripts find their own compiler
without that global link. `ESC_BINARY` overrides the launcher's native binary.
Setup preserves unrelated templates and never edits shell configuration.

#### Packages and versions

| Package | Current implementation |
| --- | --- |
| `@effectscript/compiler` | Node launcher forwarding arguments, streams, signals, and exit status to its package-relative `native/esc` |
| `@effectscript/runtime` | Continuation kernel, prelude, and generated-code helpers; see its README for the ABI |
| `@effectscript/vite` | Whole-project compiler subprocess integration for Vite 7 and 8 and Vitest 5 |
| VS Code extension | Local `.vsix` built by setup when `editors/vscode` is available |

The starter pins `effect`, `@effect/platform-node`, and `@effect/vitest` to
4.0.5, and Vitest to 5.0.5. The runtime checks Effect's version on load. The
compiler is the TypeScript 7.1 checker, with ordinary TypeScript semantics for
`.ts` files (D30). Its emitted declarations are ordinary `.d.ts` files.

`esc` accepts TypeScript compiler flags: bare `esc` checks and emits,
`--noEmit` checks, `--watch` watches, and `-b` builds references. The same native
binary exposes `--lsp --stdio` and the upstream `--api` transport. The current
Vite plugin uses subprocess compilation instead of the API transport (D105,
amending D31).

#### Building and testing

The starter's configuration emits Node ESM into `dist/`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "sourceMap": true,
    "rootDir": "src",
    "outDir": "dist",
    "noEmitOnError": true,
    "rewriteRelativeImportExtensions": true,
    "skipLibCheck": true,
    "types": []
  },
  "include": ["src/**/*.es"]
}
```

`build` runs `esc`. `start` builds and runs Node with `--enable-source-maps`.
`dev` builds once, then runs the compiler watcher and Node output watcher;
Ctrl+C stops both. Libraries can add `declaration` and `declarationMap` to
emit declarations. The starter does not enable them.

Import local sources as `./welcome.es` or `./format.ts`. The compiler emits
`.js` specifiers and declarations that name `.js` outputs (D31). In `.ts`,
`./welcome.js` resolves to `welcome.es`. Only `.es` sources are selected by the
starter's include glob; imported `.ts` integrations also join the program.

The starter tests source files through the Vite plugin:

```ts ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import effectscript from "@effectscript/vite"

export default defineConfig({
  plugins: [effectscript()],
  test: { include: ["src/**/*.test.es"] }
})
```

Register `it.effect` from `@effect/vitest` at the top level or in `describe`
(D38). Keep foreign assertions inside a boundary such as
`Effect.sync(() => assert.strictEqual(actual, expected))`. `bun run test`
needs no prior build and maps assertion failures to `.es` lines.

For a Vite application, add the same plugin to `vite.config.ts`. The plugin
runs one compiler subprocess for the project on the first request and after
source changes. Requests share compilation. It emits JavaScript and source
maps into its own directory under `node_modules/.effectscript/`, reports errors
in the terminal and overlay, and invalidates all loaded `.es` modules and their
importers after an edit. It does not compare exported contracts or use the
API transport yet. Sources must stay under Vite's root so emit paths and source maps agree (D107). Its README documents `project` and `compiler` overrides.

#### The editor

The server is the fork's own, `esc --lsp --stdio`, from the project's compiler
(D32). The same binary identifies as `EffectScript 0.0.1 (TypeScript 7.1.0-dev)`
with `--version`; there is no separate compiler entry point (D75). Setup builds a local `.vsix` when the editor package is present and
prints install commands:

```sh
code --install-extension /path/printed/by/setup/effectscript-0.0.1.vsix
# Or use cursor --install-extension with the same path.
```

Setup does not install the extension automatically. If it reports the editor
package is absent, update the checkout and rerun setup. Open the scaffolded
project directory so the extension finds its local compiler.

The extension finds `esc` from `effectscript.server.path`, then the nearest
`node_modules/.bin/esc`, then PATH. It requires a trusted workspace (D79).
Its grammar reuses VS Code's TypeScript grammar under `source.effectscript`,
with inner `.ts` scopes so existing themes apply.

For `.ts` ownership, disable VS Code's built-in TypeScript extension, or use
Cursor's `useTsgo` setting (D76). The extension serves `.es` while the built-in
TypeScript extension remains active, but leaves `.ts` to that extension.
Do not set `typescript.validate.enable` to false: this server reads that
setting too and would hide its EffectScript diagnostics.

Other editors can configure their LSP client for `.es` and `.ts`, with
`tsconfig.json` as the root marker, and disable their other TypeScript server
for this project. Add `*.es linguist-language=TypeScript` to `.gitattributes`
for GitHub highlighting.

Hover shows a running call's success value as "Runs here and returns `T`",
and a plain helper's direct signature alongside its TypeScript-facing type
(D77). Prelude names and service members complete in scope (D78). TypeScript
files retain TypeScript hover behavior. D33's control-class hover text,
catch-specific completions, mechanical ES code actions, and migration
refactors remain design targets. Diagnostic messages include repair suggestions
as text; the current compiler does not implement those editor refactors.

#### Diagnostics

Every ES diagnostic is an error, except two. Each error marks code that has no meaning, or code whose generated output would break a rule of Parts II and III, so none can be lowered and `// @ts-expect-error` does not apply to them; it keeps its TypeScript meaning for TypeScript's own codes. The two exceptions are policy, not soundness: ES2004, a `catch` that can never run, is a warning by default, and ES4403, a clock or randomness read, is an error by default. A project may set either to `"error"`, `"warning"`, or `"off"` in `compilerOptions.effectscript.diagnostics`; an ES4403 read then compiles as a trusted `sync` call that tests cannot control.

| Option in `compilerOptions.effectscript` | Values | Default |
| --- | --- | --- |
| `diagnostics` | `"ES2004"` and `"ES4403"`, each `"error"`, `"warning"`, or `"off"` | ES2004 `"warning"`, ES4403 `"error"` |
| `tracing` | `"none"`: no spans. `"exported"`: exported functions compile with `Effect.fn` spans. `"all"`: every named lowered function and method gets a span; anonymous lambdas do not (D104) | `"none"` |

`esc` exits non-zero when any error is reported. The CLI prints `error ES1003:`; LSP sends source `ES` and numeric code `1003`, which VS Code displays as `ES(1003)` (D61). Diagnostics print at `.es` spans, in the same form as the examples in this document, in the terminal, the editor, and Vite's overlay.

The diagnostic registry reserves these codes. This table describes each rule
and the intended repair (D33, D53). A registry entry does not prove that its
check or editor code action is implemented; the acceptance suite and compiler
README track coverage. Repairs below are suggestions, not shipped code actions:

| Code | What it reports | Suggested repair | Chapter |
| --- | --- | --- | --- |
| ES1001 | A module's top level runs a program | Export a function instead | 13 |
| ES1002 | A `function*` passed to an API whose parameter returns a library Generator or Iterator, such as `Effect.gen` (D64) | Convert the function | 10 |
| ES1003 | `async`, `await`, `for await`, or `await using`: a call already waits | Remove them | 10 |
| ES1004 | `using` at a module's top level or in a class static block | Move it into a function | 6 |
| ES1005 | A runner inside code that a fiber runs | Call the program, or `fork` it | 13 |
| ES2001 | An expected failure escapes the written contract | Catch it, widen the contract, or `Effect.orDie` | 2, 14 |
| ES2002 | A requirement escapes the written contract | Widen the contract, or provide the service | 14 |
| ES2003 | An Effect return annotation with missing channel slots, or a plain annotation on a non-method function whose body runs an Effect (D67, D74) | For `Effect<T>`, offer both `Effect<T, never, never>` and `Effect<T, infer E, infer R>`; for a bare `T`, write a contract | 14 |
| ES2004 | A `catch` that can never run (a warning by default) | Use the form that can fail, or `Effect.try` | 3 |
| ES2005 | A `resumeWith` program that needs a service or fails outside the operation's contract | Read the service in the clause first | 9 |
| ES2006 | A `return`, `break`, or `continue` that leaves a `finally` block, which in JavaScript discards the failure in flight | Move it after the `try` statement | 17 |
| ES2101 | A declaration without a body that has no full contract | Write `Effect<T, never, never>` | 14 |
| ES2102 | A recursive Effect function whose success, catch-variable, or return type needs a contract before its cycle's channels are known (D71, D72) | Write the contract | 14 |
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
| ES4401 | A plain TypeScript call in code a fiber runs, with no boundary or trusted contract (D100) | Wrap it in `Effect.try`, or tag the declaration `@effectscript trusted` | 10 |
| ES4402 | Running a Node.js or browser API; importing and passing values is allowed (D108) | Use the Effect module the error names | 11 |
| ES4403 | A clock or randomness read (an error by default) | Use `Clock`, `DateTime`, or `Random` | 10 |

#### Linting

D35 assigns type and language diagnostics to `esc` and reserves type-aware
linting for `.ts` files. A stock TypeScript checker models different call
semantics and cannot type-check EffectScript correctly. Run `esc --noEmit`
for `.es` checking. `@effectscript/oxlint-config` is not shipped, and Vite Plus
integration has no acceptance coverage yet. Do not install an unpublished
lint-config package from the registry.

#### Debugging

`esc` writes source maps when `sourceMap` is on, and the Vite plugin always serves them. Each map records original `.es` positions on the lowered AST through TypeScript emit, so any debugger that reads standard source maps maps breakpoints and paused frames to `.es` lines: VS Code's JavaScript debugger for Node.js, Bun, and Chrome targets; Chrome and Edge DevTools; and JetBrains IDEs.

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
