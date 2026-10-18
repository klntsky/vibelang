### 28. Tooling

```sh
bun create effeck my-app        # or: npm create effeck@latest my-app
cd my-app
bun run build
bun run start
bun run test
bun run dev
bun x --no-install effeck --noEmit
```

Effeck 0.0.1 installs from npm as unscoped packages: `effeck`,
`effeck-runtime`, `effeck-vite`, `create-effeck`, and one `effeck-<os>-<cpu>`
package per platform. `create-effeck`
copies the starter, pins the Effeck packages to its own version, and installs
dependencies; `--no-install` skips installation. Projects need Node 26 or later
and Bun 1.4 or later. The compiler is a native binary: `effeck` lists one
package per platform as an optional dependency, and the package manager
installs the one that matches the machine, the per-platform distribution D30
planned:

| Platform | Package | Tested for 0.0.1 |
| --- | --- | --- |
| macOS on Apple silicon | `effeck-darwin-arm64` | Yes |
| macOS on Intel | `effeck-darwin-x64` | No |
| Linux on x64 | `effeck-linux-x64` | Yes |
| Linux on arm64 | `effeck-linux-arm64` | No |
| Windows on x64 | `effeck-win32-x64` | No |
| Windows on arm64 | `effeck-win32-arm64` | No |

To work on Effeck itself, install from a checkout instead. `sh scripts/setup.sh`
requires Go, Bun, and Node on PATH; the tested versions are Go 1.26 (with Go
1.27 downloaded automatically), Bun 1.4.1, and Node 26.10. Setup builds
`packages/compiler/native/effeck`, installs and builds the runtime, and copies
`templates/app` into `~/.bun-create/effeck`, a local template that
`bun create effeck` prefers over the npm package. That template uses absolute
`file:` dependencies on the checkout's compiler and Vite packages and on the
runtime tarball that setup packs (D120). Keep the checkout available; after
updating it, rerun setup and run
`bun update effeck-runtime effeck effeck-vite`
in projects created from it.

Setup links `~/.local/bin/effeck` when that directory is already on PATH and no
unrelated executable occupies the path. Project scripts find their own compiler
without that global link. `EFFECK_BINARY` overrides the launcher's native binary.
Setup preserves unrelated templates and never edits shell configuration.

#### Packages and versions

| Package | Current implementation |
| --- | --- |
| `effeck` | The compiler and language server: a Node launcher that forwards arguments, streams, signals, and exit status to the native binary of the matching platform package (`EFFECK_BINARY` overrides it), plus the VS Code extension at `editor/effeck.vsix` |
| `effeck-<os>-<cpu>` | The native `effeck` binary for one platform |
| `effeck-runtime` | Continuation kernel, prelude, and generated-code helpers; see its README for the ABI |
| `effeck-vite` | Whole-project compiler subprocess integration for Vite 7 and 8 and Vitest 5 |
| `create-effeck` | Scaffolds the starter for `bun create effeck` and `npm create effeck` |
| VS Code extension | Ships inside `effeck` as `editor/effeck.vsix`; not on the VS Code Marketplace or Open VSX |

The starter pins `effect`, `@effect/platform-node`, and `@effect/vitest` to
4.0.5, and Vitest to 5.0.5. The runtime checks Effect's version on load. The
compiler is the TypeScript 7.1 checker, with ordinary TypeScript semantics for
`.ts` files (D30). Its emitted declarations are ordinary `.d.ts` files.

`effeck` accepts TypeScript compiler flags: bare `effeck` checks and emits,
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
  "include": ["src/**/*.effeck"]
}
```

`build` runs `effeck`. `start` builds and runs Node with `--enable-source-maps`.
`dev` builds once, then runs the compiler watcher and Node output watcher;
Ctrl+C stops both. Libraries can add `declaration` and `declarationMap` to
emit declarations. The starter does not enable them.

Import local sources as `./welcome.effeck` or `./format.ts`. The compiler emits
`.js` specifiers and declarations that name `.js` outputs (D31). In `.ts`,
`./welcome.js` resolves to `welcome.effeck`. Only `.effeck` sources are selected by the
starter's include glob; imported `.ts` integrations also join the program.

The starter tests source files through the Vite plugin:

```ts ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import effeck from "effeck-vite"

export default defineConfig({
  plugins: [effeck()],
  test: { include: ["src/**/*.test.effeck"] }
})
```

Register `it.effect` from `@effect/vitest` at the top level or in `describe`
(D38). Effect's packages are trusted, so call `assert` from `@effect/vitest`
directly: `assert.strictEqual(actual, expected)`, whose failure is a defect
that fails the test. Keep another library's assertions inside a hatch, with
the values as inputs: `sync((actual) => check(actual), actual)`. `bun run test`
needs no prior build and maps assertion failures to `.effeck` lines.

For a Vite application, add the same plugin to `vite.config.ts`. The plugin
runs one compiler subprocess for the project on the first request and after
source changes. Requests share compilation. It emits JavaScript and source
maps into its own directory under `node_modules/.effeck/`, reports errors
in the terminal and overlay, and invalidates all loaded `.effeck` modules and their
importers after an edit. It does not compare exported contracts or use the
API transport yet. Sources must stay under Vite's root so emit paths and source maps agree (D107). Its README documents `project` and `compiler` overrides.

#### The editor

The server is the fork's own, `effeck --lsp --stdio`, from the project's compiler
(D32). The same binary identifies as `Effeck 0.0.1 (TypeScript 7.1.0-dev)`
with `--version`; there is no separate compiler entry point (D75). `effeck`
ships the extension as `editor/effeck.vsix`. Install it from a project:

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
# Or use cursor --install-extension with the same path.
```

Open the project directory so the extension finds its local compiler. Install
the extension again after updating `effeck`. In a checkout, setup also builds
the extension and prints its path.

The extension finds `effeck` from `effeck.server.path`, then the nearest
`node_modules/.bin/effeck`, then PATH. It requires a trusted workspace (D79).
Its grammar reuses VS Code's TypeScript grammar under `source.effeck`,
with inner `.ts` scopes so existing themes apply.

For `.ts` ownership, disable VS Code's built-in TypeScript extension, or use
Cursor's `useTsgo` setting (D76). The extension serves `.effeck` while the built-in
TypeScript extension remains active, but leaves `.ts` to that extension.
Do not set `typescript.validate.enable` to false: this server reads that
setting too and would hide its Effeck diagnostics.

Other editors can configure their LSP client for `.effeck` and `.ts`, with
`tsconfig.json` as the root marker, and disable their other TypeScript server
for this project. Add `*.effeck linguist-language=TypeScript` to `.gitattributes`
for GitHub highlighting.

Hover shows a running call's success value as "Runs here and returns `T`",
and a plain helper's direct signature alongside its TypeScript-facing type
(D77). Prelude names and service members complete in scope (D78). A method of
an Effect data type hovers as the function it calls, "Method syntax: calls
`Option.getOrElse(maybe, …)`", and completes after a dot (D152). A call passed
by name hovers as "Passed by name to `fork` as a program" (D147), and so does a
pipe's topic that passes its left side by name; any other topic hovers as its
value, "(topic) %: T" (D155). TypeScript
files retain TypeScript hover behavior. D33's control-class hover text,
catch-specific completions, mechanical EK code actions, and migration
refactors remain design targets. Diagnostic messages include repair suggestions
as text; the current compiler does not implement those editor refactors.

#### Diagnostics

Every EK diagnostic is an error, except seven. Each error marks code that has no meaning, or code whose generated output would break a rule of Parts II and III, so none can be lowered and `// @ts-expect-error` does not apply to them; it keeps its TypeScript meaning for TypeScript's own codes. Five warnings mark retired spellings that still compile the same: EK1009, the old `with` spelling of a `provide` block; EK2112, a catch clause that tests a failure's type; EK2113, a catch clause typed `Cause`; EK2120, a `service class` or `control class` declaration; and EK2121, a `layer` declaration. Each has a quick fix. The other two exceptions are policy, not soundness: EK2004, a `catch` statement or catch expression that can never run, is a warning by default, and EK4403, a clock or randomness read, is an error by default. A project may set either to `"error"`, `"warning"`, or `"off"` in `compilerOptions.effeck.diagnostics`; an EK4403 read then compiles as a trusted `sync` call that tests cannot control.

| Option in `compilerOptions.effeck` | Values | Default |
| --- | --- | --- |
| `diagnostics` | `"EK2004"` and `"EK4403"`, each `"error"`, `"warning"`, or `"off"` | EK2004 `"warning"`, EK4403 `"error"` |
| `tracing` | `"none"`: no spans. `"exported"`: exported functions compile with `Effect.fn` spans. `"all"`: every named lowered function and method gets a span; anonymous lambdas do not (D104) | `"none"` |

`effeck` exits non-zero when any error is reported. The CLI prints `error EK1003:`; LSP sends source `EK` and numeric code `1003`, which VS Code displays as `EK(1003)` (D61). Diagnostics print at `.effeck` spans, in the same form as the examples in this document, in the terminal, the editor, and Vite's overlay.

The diagnostic registry reserves these codes. This table describes each rule
and the intended repair (D33, D53). A registry entry does not prove that its
check or editor code action is implemented; the acceptance suite and compiler
README track coverage. Repairs below are suggestions, not shipped code actions:

| Code | What it reports | Suggested repair | Chapter |
| --- | --- | --- | --- |
| EK1001 | A module's top level runs a program | Export a function instead | 13 |
| EK1002 | A `function*` passed to an API whose parameter returns a library Generator or Iterator, such as `Effect.gen` (D64) | Convert the function | 10 |
| EK1003 | `async`, `for await`, or `await using`, and `await` inside an `async` function outside a hatch: a call already waits | Remove them; `await` in Effeck code waits for fibers (EK4601-EK4604) | 10 |
| EK1004 | `using` at a module's top level or in a class static block | Move it into a function | 6 |
| EK1005 | A runner inside code that a fiber runs | Call the program, or `fork` it | 13 |
| EK1006 | A pipe whose body does not use its topic, as in `name \|> "anonymous"` (D156) | Write `%` where the value goes: `name \|> greet(%)` | 5 |
| EK1007 | `%` outside every pipe's body (D156) | Use `%` only after `\|>`; `%` between two operands is still the remainder | 5 |
| EK1008 | A pipe that passes its left side by name to one program and also uses it as a value (D155) | Bind the value first with `const`, or pass it by name everywhere | 5, 13 |
| EK1009 | A `provide` block written with its old keyword, `with` (a warning; D172) | Write `provide`; the quick fix replaces the keyword | 4 |
| EK2001 | An expected failure escapes the written contract | Catch it, widen the contract, or `Effect.orDie` | 2, 14 |
| EK2002 | A requirement escapes the written contract | Widen the contract, or provide the service | 14 |
| EK2003 | An Effect return annotation with missing channel slots, or a plain annotation on a non-method function whose body runs an Effect (D67, D74) | For `Effect<T>`, suggest `T!!` or removing the annotation; for a bare `T`, suggest `T!!` or `T!Errors` | 14 |
| EK2004 | A `catch` that can never run, a statement or a catch expression (D238) (a warning by default) | Use the form that can fail, or `try` | 3 |
| EK2005 | A `resumeWith` program that needs a service or fails outside the operation's contract | Read the service in the clause first | 9 |
| EK2006 | A `return`, `break`, or `continue` that leaves a `finally` block, which in JavaScript discards the failure in flight | Move it after the `try` statement | 17 |
| EK2101 | A declaration without a body that has no full contract | Write `T!Errors!Dependencies`, or `T!!` | 14 |
| EK2102 | A recursive Effect function whose success, catch-variable, or return type needs a contract before its cycle's channels are known (D71, D72) | Write the contract | 14 |
| EK2103 | A member of a `service` or `control` declaration with a body or an initializer (D128) | Implement it in a layer, or make it static | 4 |
| EK2104 | A setup statement of an old `layer` declaration that follows one of its members (D129) | Move the statement above the members | 4 |
| EK2105 | A retired typed catch clause whose type has a member without a literal `_tag` (D131) | Catch a tagged error, or test the value in an untyped clause | 3 |
| EK2106 | A retired catch clause after one that catches every failure: an untyped clause or a `Cause` clause (D131, D176) | Move that clause last | 3 |
| EK2107 | A handle clause whose control is not a control declaration, such as a service (D132) | Provide a service with a layer; handle only a control's operations | 9 |
| EK2108 | A handle statement that leaves an operation of a control it handles without a clause, or gives one two clauses (D132) | Add the missing clause, or remove the second | 9 |
| EK2109 | A field of a `schema` declaration whose type has no schema form, or names a value that is not a schema (D133) | Declare the schema with `Schema.Struct`, or name a schema constant | 11 |
| EK2111 | A `restore` block that no `uninterruptible` block of the same function encloses (D180) | Put it inside `uninterruptible { … }`, or use `interruptible { … }` | 6 |
| EK2112 | Warning: a catch clause that tests a failure's type, `catch (e: NotFound)`, retired (D216) | `catch (e)` and test `e` with `match` or `is`; the quick fix rewrites the clauses | 3 |
| EK2113 | Warning: a catch clause typed `Cause`, retired (D217) | `match (exit(…))` with `Exit.Success` and `Exit.Failure` arms; the quick fix rewrites `try { return X } catch (c: Cause) { return Y }` | 3 |
| EK2114 | A catch expression, `x catch y` or `x catch (e) => …`, inside a hatch body, which is plain TypeScript (D239) | Move the catch out of the hatch, or use a try statement inside it | 3 |
| EK2120 | A warning: a `service class` or `control class` declaration, the older spelling (D223) | Write `service` or `control` without `class`; the quick fix removes `class` | 4 |
| EK2121 | A warning: a `layer` declaration, the old spelling of a layer (D221) | Write `Layer.effect(Service) { … }` or `Layer.succeed(Service, { … })`; the quick fix rewrites it | 4 |
| EK3001 | Two services share a key | Rename one | 4 |
| EK3002 | A class extends a service | Replace `extends` with `implements` | 4 |
| EK3003 | `new` on a service | Use `Users()` for the implementation | 4 |
| EK3004 | `instanceof` on a service | None: a service is a key, not a base class | 4 |
| EK3005 | A generic service class | Use an interface for the generic type | 4 |
| EK3006 | A static member with the same name as a service member | Rename one | 4 |
| EK3101 | A provider looks a service up per call where the contract allows no requirements | Capture the service when the layer is built, or declare it | 4 |
| EK3201 | No provider for a required service, with the path that needs it | Add the provider the program defines to `wire` | 4 |
| EK3202 | Two providers for one service | Remove one | 4 |
| EK3203 | Providers form a cycle | None | 4 |
| EK3204 | A `wire` argument whose type does not name its outputs | Compose it with `Layer.provide` | 4 |
| EK4201 | A control operation inside a combinator that runs it on another fiber | Handle the operation inside the branch | 9, 16 |
| EK4202 | A TypeScript function whose R holds a control operation | None in v0 | 9 |
| EK4301 | A callback that can pause or fail where a plain function is expected | Rewrite as `Effect.forEach` or `for...of` | 10, 16 |
| EK4401 | A plain TypeScript call in code a fiber runs, with no boundary or trusted contract (D100) | Wrap it in `try`, or tag the declaration `@effeck trusted` | 10 |
| EK4402 | Running a Node.js or browser API; importing and passing values is allowed (D108) | Use the Effect module the error names | 11 |
| EK4403 | A clock or randomness read (an error by default) | Use `Clock`, `DateTime`, or `Random` | 10 |
| EK4404 | `try` given Effect's `{ try, catch }` options object, which it would treat as a value | Pass the code and the error mapping as two arguments: `try((text) => parse(text), (error) => …, text)` | 10 |
| EK4405 | A hatch's TypeScript reads a binding the `.effeck` file declares outside the hatch: a local, a parameter, or a module declaration (D229) | Pass the value as an input, an argument after the function that fills its first parameter: `sync((text) => text.length, text)`; the quick fix does this | 10 |
| EK4501 | A method of an Effect data type named without a call, as in `const f = stream.filter` (D153) | Call it, or pass `Stream.filter` itself | 13 |
| EK4601 | `await` before a call that already runs (D226) | Remove `await`; to run the call concurrently, fork it and join the fiber | 7 |
| EK4602 | `await` on a promise in Effeck code (D226) | Wrap it in a hatch: `try(async () => …)` | 7 |
| EK4603 | `await` on any other value; Effeck code has only the await operations (D226) | Fork the program and join the fiber, or use `await.all`, `await.race`, `await.any`, or `await.allSettled` | 7 |
| EK4604 | `await` in a hatch body that is not `async` (D226) | Make the hatch's function `async` | 7 |
| EK4605 | `break` that would leave a `for concurrent` loop (D169) | Use `continue` to end one item, or `throw` to stop every item | 7 |
| EK4606 | `return` in a `for concurrent` body (D169) | Use `continue` to end one item | 7 |
| EK4607 | `select`, which `await.race` replaced (D197) | Write `await.race [a(), b()]`; for a `default:` clause, poll with `Queue.poll` and an `if` | 7 |
| EK4608 | `await.race` or `await.any` on an object (D196) | Write the elements in an array: `await.race [a(), b()]` | 7 |
| EK4609 | `await` on a fiber, or an array or object of fibers (D226) | Write `join` for `await` (quick fix) | 7 |
| EK4610 | `await` on a Deferred (D226) | Write `d.await()` (quick fix) | 7 |
| EK4611 | `join` on a value that is not a Fiber or an array or object of fibers (D225) | Join fibers; wait for a Deferred with `d.await()`; fork a program first | 7 |
| EK4612 | `join` in a hatch body, which is plain TypeScript (D225) | Move the `join` out of the hatch | 7 |
| EK4701 | A `match` without a `default` arm over a finite union (tagged errors, Option, Exit, Result, literals) that misses a case (D209) | Add an arm for each case left, or a `default` arm | 3 |
| EK4702 | A binding inside an `or` or `not` pattern (D210) | Bind outside the combinator, or match each alternative in its own arm | 3 |
| EK4703 | An `is` whose pattern binds names outside the condition of an `if` or `while` (D211) | Move the test into an `if` or `while` condition, or use `match` | 3 |
| EK4704 | A pattern form Effeck does not support yet: regular expression, relational, or a computed key other than a name or literal (D212) | Use a guard: `if (/re/.test(x))` or `if (x < 5)` | 3 |
| EK4705 | `and`, `or`, and `not` combined at one level without parentheses (D206) | Add parentheses: `(a and b) or c` | 3 |

#### Linting

D35 assigns type and language diagnostics to `effeck` and reserves type-aware
linting for `.ts` files. A stock TypeScript checker models different call
semantics and cannot type-check Effeck correctly. Run `effeck --noEmit`
for `.effeck` checking. `effeck-oxlint-config` is not shipped, and Vite Plus
integration has no acceptance coverage yet. Do not install an unpublished
lint-config package from the registry.

#### Debugging

`effeck` writes source maps when `sourceMap` is on, and the Vite plugin always serves them. Each map records original `.effeck` positions on the lowered AST through TypeScript emit, so any debugger that reads standard source maps maps breakpoints and paused frames to `.effeck` lines: VS Code's JavaScript debugger for Node.js, Bun, and Chrome targets; Chrome and Edge DevTools; and JetBrains IDEs.

```text
Paused on breakpoint, requestApproval  checkout.effeck:22
Call stack (JavaScript)
  requestApproval            checkout.effeck:22         the running Effeck function
  runLoop                    effect/dist/internal/effect.js
  evaluate                   effect/dist/internal/effect.js
```

What a debugger shows follows from the frame invariant (Chapter 26):

- **One Effeck frame at a time.** Compiled functions are generator functions named after their source, so the running function appears by name at its `.effeck` line. Its callers are frames on the fiber's `_stack`, not JavaScript activations, so the debugger's call stack shows Effect's run loop below it, not the Effeck call chain.
- **Regions** are closures named after their function and block: `profile$try`, `profile$catch`, `profile$finally`, `profile$using`.
- **Clauses and producers.** A clause is a generator function named after its operation member (`approve`), and a producer's body is one named after the producer (`lines`). A breakpoint in a clause shows the clause; the handled program's frames are inside `k`, a heap value, and appear again when the program resumes. A breakpoint in a producer shows the producer, and the loop body runs in the clause of the `for...of` that iterates it.
- **Stepping over a call that pauses** continues on a later run-loop step, so the debugger steps into Effect's run loop; set a breakpoint on the next line instead, as with `Effect.gen` code today.
- **Logical call stacks** across calls and suspensions come from spans: set `tracing` to `"exported"` or `"all"`, and use Effect's `DevTools` layer (`effect/devtools`) with its VS Code extension, which shows fibers and the span stack.
