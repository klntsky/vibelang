### 28. Tooling

```sh
bun create vibelang my-app        # or: npm create vibelang@latest my-app
cd my-app
bun run build
bun run start
bun run test
bun run dev
bun x --no-install vibelang --noEmit
```

VibeLang 0.0.2 installs from npm as unscoped packages: `vibelang`,
`vibelang-runtime`, `vibelang-vite`, `vibelang-unplugin`, `create-vibelang`,
and one `vibelang-<os>-<cpu>` package per platform. `create-vibelang` copies
the starter, pins the VibeLang packages to its own version, and installs
dependencies (`--no-install` skips that). Projects need at least Node 26 and
Bun 1.4. The compiler is a native binary: `vibelang` lists each platform's package
as an optional dependency, and the package manager installs the one that
matches the machine (the per-platform distribution D30 planned):

| Platform | Package | Rehearsed for 0.0.2 |
| --- | --- | --- |
| macOS on Apple silicon | `vibelang-darwin-arm64` | No |
| macOS on Intel | `vibelang-darwin-x64` | No |
| Linux on x64 | `vibelang-linux-x64` | Yes |
| Linux on arm64 | `vibelang-linux-arm64` | No |
| Windows on x64 | `vibelang-win32-x64` | No |
| Windows on arm64 | `vibelang-win32-arm64` | No |

Version 0.0.2 ships all six binaries, cross-compiled; the end-to-end npm and
Bun rehearsal runs on Linux x64.

To work on VibeLang itself, install from a checkout. `sh scripts/setup.sh`
needs Go, Bun, and Node on PATH (tested: Go 1.26 with Go 1.27 downloaded
automatically, Bun 1.4.1, and Node 26.10). Setup builds
`packages/compiler/native/vibelang`, installs and builds the runtime, and copies
`templates/app` into `~/.bun-create/vibelang`, a local template that
`bun create vibelang` prefers over the npm package. That template has absolute
`file:` dependencies on the checkout's compiler and Vite packages and on the
runtime tarball setup packs (D120), so keep the checkout. After updating it,
rerun setup, then run `bun update vibelang-runtime vibelang vibelang-vite` in
projects created from it.

Setup links `~/.local/bin/vibelang` when that directory is already on PATH and
no unrelated executable occupies the path; project scripts find their own
compiler without it. Setup preserves unrelated templates and never edits shell
configuration.

#### Packages and versions

| Package | Current implementation |
| --- | --- |
| `vibelang` | Compiler and language server: a Node launcher that forwards arguments, streams, signals, and exit status to the matching platform package's native binary (`VIBELANG_BINARY` overrides it), plus the VS Code extension, the Node loader `vibelang/register`, and `vibelang/project`, the whole-project compile the loader and the bundler plugins share, with its on-disk cache (D299) |
| `vibelang-<os>-<cpu>` | The native `vibelang` binary for one platform |
| `vibelang-runtime` | Continuation kernel, prelude, and generated-code helpers; its README gives the ABI |
| `vibelang-vite` | Whole-project compiler subprocess integration for Vite 7 and 8 and Vitest 5 |
| `vibelang-unplugin` | Per-module compiler integration for webpack, Rspack, Rollup, Rolldown, esbuild, and Bun |
| `create-vibelang` | Scaffolds the starter for `bun create vibelang` and `npm create vibelang` |
| VS Code extension | Ships inside `vibelang` as `editor/vibelang.vsix`; not on the VS Code Marketplace or Open VSX |

The starter pins `effect`, `@effect/platform-node`, and `@effect/vitest` to
4.0.5, and Vitest to 5.0.5. The runtime checks Effect's version on load. The
compiler is the TypeScript 7.1 checker, with ordinary TypeScript semantics for
`.ts` files (D30), and emits ordinary `.d.ts` declarations.

`vibelang` accepts TypeScript compiler flags: bare `vibelang` checks and emits,
`--noEmit` checks, `--watch` watches, and `-b` builds references. The same
native binary exposes `--lsp --stdio` and the upstream `--api` transport. The
Vite plugin compiles in a subprocess, not yet through the API transport (D105,
amending D31).

A whole-project compile is cached on disk (D299). The next process that loads
the same project reuses the result while the compiler, the configs, the scanned
directories, the installed packages, and every file the compile read are
unchanged; `VIBELANG_CACHE_DIR` moves the cache and `VIBELANG_DISABLE_CACHE=1`
turns it off.

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
  "include": ["src/**/*.vibe"]
}
```

`build` runs `vibelang`. `start` builds and runs Node with `--enable-source-maps`.
`dev` builds once, then runs the compiler watcher and Node output watcher;
Ctrl+C stops both. To emit declarations, a library adds `declaration` and
`declarationMap`; the starter leaves them off.

Import local sources as `./welcome.vibe` or `./format.ts`; the compiler emits
`.js` specifiers and declarations that name `.js` outputs (D31). In `.ts`,
`./welcome.js` resolves to `welcome.vibe`. The starter's include glob selects
only `.vibe` sources; imported `.ts` integrations join the program too.

The starter tests source files through the Vite plugin:

```ts ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import vibelang from "vibelang-vite"

export default defineConfig({
  plugins: [vibelang()],
  test: { include: ["src/**/*.test.vibe"] }
})
```

Register `it.effect` from `@effect/vitest` at the top level or in `describe`
(D38). Effect's packages are trusted, so call `assert` from `@effect/vitest`
directly: a failed `assert.strictEqual(actual, expected)` is a defect that
fails the test. Put another library's assertions in a hatch, with the values
as inputs: `sync((actual) => check(actual), actual)`. `bun run test` needs no
prior build and maps assertion failures to `.vibe` lines.

For a Vite application, add the same plugin to `vite.config.ts`. It runs one
compiler subprocess for the project, shared by requests, on the first request
and after source changes. It emits JavaScript and source maps into its own
directory under `node_modules/.vibelang/`, reports errors in the terminal and
overlay, and after an edit invalidates every loaded `.vibe` module and its
importers. It doesn't compare exported contracts yet. Sources must stay under
Vite's root so emit paths and source maps agree (D107). Its README documents
the `project` and `compiler` overrides.

#### The editor

The server is the fork's own, `vibelang --lsp --stdio` from the project's
compiler (D32). With `--version` the same binary identifies as
`VibeLang 0.0.2 (TypeScript 7.1.0-dev)`; there is no separate compiler entry
point (D75). Install the extension, which `vibelang` ships as
`editor/vibelang.vsix`, from a project:

```sh
code --install-extension node_modules/vibelang/editor/vibelang.vsix
# Or use cursor --install-extension with the same path.
```

Open the project directory so the extension finds its local compiler, and
reinstall the extension after updating `vibelang`. In a checkout, setup also
builds the extension and prints its path.

The extension looks for `vibelang` in `vibelang.server.path`, then the nearest
`node_modules/.bin/vibelang`, then PATH, and requires a trusted workspace
(D79). Its grammar reuses VS Code's TypeScript grammar under `source.vibe`,
with inner `.ts` scopes so existing themes apply.

For the server to own `.ts`, disable VS Code's built-in TypeScript extension or
use Cursor's `useTsgo` setting (D76). While the built-in extension is active,
this one serves `.vibe` and leaves `.ts` to it. Don't set
`typescript.validate.enable` to false: this server reads that setting too and
would hide its VibeLang diagnostics.

In other editors, configure the LSP client for `.vibe` and `.ts` with
`tsconfig.json` as the root marker, and disable any other TypeScript server for
the project. For GitHub highlighting, add `*.vibe linguist-language=TypeScript`
to `.gitattributes`.

Editor features (TypeScript files keep TypeScript's hover behavior):

- Hover shows a running call's success value as "Runs here and returns `T`",
  and a plain helper's direct signature beside its TypeScript-facing type
  (D77).
- Prelude names and service members complete in scope (D78).
- A method of an Effect data type hovers as the function it calls, "Method
  syntax: calls `Option.getOrElse(maybe, …)`", and completes after a dot
  (D152).
- A call passed by name hovers as "Passed by name to `fork` as a program"
  (D147), as does a pipe topic that passes its left side by name; any other
  topic hovers as its value, "(topic) %: T" (D155).
- A layer's hover lists what it provides, needs, and fails with; `wire`'s
  hover, the order it builds its layers in. A `Duration.Input` string, the name
  in `Schema.brand("…")`, and the codes of an `@vibelang-diagnostics` comment
  complete (D246).
- Quick fixes cover retired spellings and the EK48xx rules whose repair is
  mechanical (D240).
- Refactors, code actions of kind `refactor.rewrite` (D245): write or remove a
  function's contract; convert an interface to a `schema` declaration; convert
  an `async` function to a VibeLang function, with or without an `error`
  declaration per `await`; convert nested calls to a `|>` pipe and back; toggle
  `x.pipe(f)` and `pipe(x, f)`.

D33's control-class hover text and catch-specific completions remain design
targets. Diagnostic messages include repair suggestions as text; the compiler
does not implement those editor refactors.

#### Diagnostics

Every EK diagnostic before EK4801 is an error, except seven. Each error marks code with no meaning, or whose generated output would break a rule of Parts II and III, so none can be lowered and `// @ts-expect-error` does not apply to them (it keeps its TypeScript meaning for TypeScript's own codes). Five warnings mark retired spellings that still compile the same, each with a quick fix: EK1009, EK2112, EK2113, EK2120, and EK2121. The other two exceptions, EK2004 and EK4403, are policy, not soundness: a project may set either to `"error"`, `"warning"`, or `"off"` in the `vibelang` setting `diagnostics`; an EK4403 read then compiles as a trusted `sync` call that tests cannot control.

EK4801-EK4825 carry the rules of Effect's language service that VibeLang's syntax does not already rule out (D240). They are lint, not soundness: each has a default category (error, warning, or off), and each except EK4813, which stands in for TypeScript's TS2339, can be set in the `vibelang` setting `diagnostics` or by a comment (D241):

```ts
// @vibelang-diagnostics EK4818:warning            from here to the end of the file
// @vibelang-diagnostics-next-line EK4801:off       the next line only
// @vibelang-diagnostics *:off                      every adjustable code
```

EK4830-EK4837 mark Effect code written the way Effect's TypeScript writes it where VibeLang has a form of its own (D284-D290): an error class (EK4830, or EK4837 for a Schema error class), a `Context.Service` class, a constant that holds a `Schema.Struct`, an `Effect.Effect<A, E, R>` type, a running `Effect.succeed` or `Effect.fail`, a catch combinator, and `Effect.withSpan`, `Effect.provide`, or `Effect.provideService`. They are lint too, warnings by default and adjustable the same way, and they never change emit or the exit status. Each has a quick fix wherever the VibeLang form keeps the program's meaning; where it would not, the warning says what would change and offers none. `vibelang -p tsconfig.json --fix` applies every such fix, and the quick fixes of the retired spellings, to a project's `.vibe` files and then compiles as the rest of the command line says; it writes a file only when the fixed program reports no error the original did not (D291).

A later comment overrides an earlier one; within one comment, a code named on its own overrides `*`. D266 supersedes D242: every EK diagnostic is restricted to `.vibe` files, and ordinary TypeScript sources keep TypeScript diagnostics whatever these settings say.

| Option in the tsconfig's top-level `vibelang` object, or `compilerOptions.vibelang` (D275) | Values | Default |
| --- | --- | --- |
| `diagnostics` | `"EK2004"`, `"EK4403"`, EK4801-EK4825 except `"EK4813"`, and EK4830-EK4837, each `"error"`, `"warning"`, or `"off"` | EK2004 `"warning"`, EK4403 `"error"`; EK48xx as the table below says |
| `tracing` | `"none"`: no spans. `"exported"`: exported functions get `Effect.fn` spans. `"all"`: every named lowered function and method gets a span; anonymous lambdas don't (D104) | `"none"` |

`vibelang` exits non-zero when it reports any error. The CLI prints `error EK1003:`; LSP sends source `EK` and numeric code `1003`, which VS Code displays as `EK(1003)` (D61). In the terminal, the editor, and Vite's overlay, diagnostics print at `.vibe` spans in the form this document's examples use.

The diagnostic registry reserves these codes; the table gives each rule and its
intended repair (D33, D53). A registry entry doesn't prove its check or editor
code action is implemented; the acceptance suite and compiler README track
coverage. Repairs are suggestions, not shipped code actions:

| Code | What it reports | Suggested repair | Chapter |
| --- | --- | --- | --- |
| EK1001 | Except for an explicitly Effect-typed top-level `const` initializer (D262), a module's top level runs a program | Export a function instead | 13 |
| EK1002 | A `function*` passed to an API whose parameter returns a library Generator or Iterator, such as `Effect.gen` (D64) | Convert the function | 10 |
| EK1003 | `async`, `for await`, or `await using`, and `await` inside an `async` function outside a hatch: a call already waits | Remove them; `await` in VibeLang code waits for fibers (EK4601-EK4604) | 10 |
| EK1004 | `using` at a module's top level or in a class static block | Move it into a function | 6 |
| EK1005 | A runner inside code that a fiber runs | Call the program, or `fork` it | 13 |
| EK1006 | A pipe whose body does not use its topic, as in `name \|> "anonymous"` (D156) | Write `%` where the value goes: `name \|> greet(%)` | 5 |
| EK1007 | `%` outside every pipe's body (D156) | Use `%` only after `\|>`; `%` between two operands is still the remainder | 5 |
| EK1008 | A pipe that passes its left side by name to one program and also uses it as a value (D155) | Bind the value first with `const`, or pass it by name everywhere | 5, 13 |
| EK1009 | Warning: a `provide` block written with its old keyword, `with` (D172) | Write `provide`; the quick fix replaces the keyword | 4 |
| EK1010 | An unparenthesized throw expression is a binary operator's left operand (except comma), a conditional's condition, or the operand of `as` or `satisfies` (D186) | Write `throw (…)` to throw the whole expression | 3 |
| EK2001 | An expected failure escapes the written contract | Catch it, widen the contract, or `Effect.orDie` | 2, 14 |
| EK2002 | A requirement escapes the written contract | Widen the contract, or provide the service | 14 |
| EK2003 | An Effect return annotation with missing channel slots, a plain annotation on a non-method function whose body runs an Effect (D67, D74), or a plain annotation on a function, or a JavaScript iterator annotation on a generator, whose throw can escape (D282, D296) | For `Effect<T>`: `T!!` or no annotation; for a bare `T`: `T!!` or `T!Errors`; for an escaping throw, a quick fix writes `T!E!`, or a hatch keeps JavaScript's throw | 14 |
| EK2004 | Warning by default: a catch whose try block has E = `never`, in an Effect or a plain function, or a catch expression with no expected errors; defects, JavaScript exceptions included, pass through (D238, D260, D282) | Use the form that can fail, or `try` | 3 |
| EK2005 | A `resumeWith` program that needs a service or fails outside the operation's contract | Read the service in the clause first | 9 |
| EK2006 | A `return`, `break`, or `continue` that leaves a `finally` block, which in JavaScript discards the failure in flight | Move it after the `try` statement | 17 |
| EK2101 | A declaration without a body that has no full contract | Write `T!Errors!Dependencies`, or `T!!` | 14 |
| EK2102 | A recursive Effect function whose success, catch-variable, or return type needs a contract before its cycle's channels are known (D71, D72) | Write the contract | 14 |
| EK2103 | A member of a `service` or `control` declaration with a body or an initializer (D128), or any instance member of `service Name: Type` (D281) | Implement it in a layer, declare it in the shape, or make it static | 4 |
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
| EK2115 | A factory body runs an Effect, looks up a service (D269), or throws (D282) | Move the work into `return () => { … }`; hold a forwarding call in an Effect-typed const before returning it | 5, 13 |
| EK2116 | A factory returns a program on one path and a value or nothing on another (D269) | Return `() => value` or `() => { throw error }` on every returning path | 5, 13 |
| EK2117 | An error whose tag does not fit: a subclass whose base's tag type does not include the subclass's name, a concrete error whose `_tag` field's type does not include its own name, an error that extends a class that is not an error, a `schema error` that extends an error that is not one, or an `error` that extends a `schema error` (D278, D279, D304) | Give the base the field `_tag: string`, or declare it `abstract`; extend an error of the same kind | 3 |
| EK2118 | A subclass that declares fields and no constructor, whose base's constructor takes something other than one object of fields (D278) | Write a constructor that calls `super` with what the base takes and sets the fields | 3 |
| EK2120 | Warning: a `service class` or `control class` declaration, the old spelling (D223) | Write `service` or `control` without `class`; the quick fix removes `class` | 4 |
| EK2121 | Warning: a `layer` declaration, the old spelling (D221) | Write `Layer.effect(Service) { … }` or `Layer.succeed(Service, { … })`; the quick fix rewrites it | 4 |
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
| EK4301 | A callback that can pause or fail where a plain function is expected | Rewrite as `Effect.forEach` or `for...of`; a visitor returning a program uses `() => () => call()` when its result is used | 10, 16 |
| EK4401 | Packages matching the `vibelang` setting `trustedPackages` are trusted (D261); otherwise, a plain TypeScript call in code a fiber runs, with no boundary or trusted contract (D100) | Wrap it in `try`, or tag the declaration `@vibelang trusted` | 10 |
| EK4402 | Running a Node.js or browser API; importing and passing values is allowed (D108) | Use the Effect module the error names | 11 |
| EK4403 | Error by default: a clock or randomness read | Use `Clock`, `DateTime`, or `Random` | 10 |
| EK4404 | `try` given Effect's `{ try, catch }` options object, which it would treat as a value | Pass the code and the error mapping as two arguments: `try((text) => parse(text), (error) => …, text)` | 10 |
| EK4405 | A hatch's TypeScript reads a binding the `.vibe` file declares outside the hatch: a local, a parameter, or a module declaration (D229) | Pass the value as an input, an argument after the function that fills its first parameter: `sync((text) => text.length, text)`; the quick fix does this | 10 |
| EK4406 | A call whose D23-tagged TypeScript declaration changes the emitted failure contract (warning until the emit switch, D249) | Export a `trusted(impl, [Errors])` value beside the implementation and call that value | 10 |
| EK4501 | A method of an Effect data type named without a call, as in `const f = stream.filter` (D153) | Call it, or pass `Stream.filter` itself | 13 |
| EK4601 | `await` before a call that already runs (D226) | Remove `await`; to run the call concurrently, fork it and join the fiber | 7 |
| EK4602 | `await` on a promise in VibeLang code (D226) | Wrap it in a hatch: `try(async () => …)` | 7 |
| EK4603 | `await` on any other value; VibeLang code has only the await operations (D226) | Fork the program and join the fiber, or use `await.all`, `await.race`, `await.any`, or `await.allSettled` | 7 |
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
| EK4704 | A pattern form VibeLang does not support yet: regular expression, relational, or a computed key other than a name or literal (D212) | Use a guard: `if (/re/.test(x))` or `if (x < 5)` | 3 |
| EK4705 | `and`, `or`, and `not` combined at one level without parentheses (D206) | Add parentheses: `(a and b) or c` | 3 |
| EK4801 | A statement that discards an Effect value, as `program` alone on a line; an error by default | Call it: `program()` (quick fix) | 2 |
| EK4802 | Warning: an Effect function without a contract accepting that Effect as A returns an Effect value, so its caller gets the program; declared binding and contextual contracts count (S7-3) | Call it: `return program()` (quick fix), or write the contract | 2 |
| EK4803 | Warning: the program loads two versions of `effect`, an `@effect/*` package, or `vibelang-runtime` | Install one version | 28 |
| EK4804 | Warning: a failure that is a global `Error`, thrown or returned by a `try` mapping | Declare it: `error Failed { message: string }` | 3 |
| EK4805 | Warning: a class that extends `Error` | Write an `error` declaration | 3 |
| EK4806 | Warning: a `try` mapping that returns `unknown` or `any` | Return an error declaration's instance | 3 |
| EK4807 | Warning: `Layer.mergeAll` of a layer that needs a service another argument provides | Write `wire` (quick fix) | 4 |
| EK4808 | Warning: a `provide` block that holds only another `provide` block | `provide A, B { … }`, or `provide wire(A, B) { … }` when one needs the other (quick fix) | 4 |
| EK4809 | Warning: a service whose every Effect member needs the same service | Read it where the layer is built, or mark it `@vibelang leakable` | 4 |
| EK4810 | Warning: an assertion from one Effect type to another that drops failures or services | Remove the assertion (quick fix) | 14 |
| EK4811 | Off by default: a throw of `any` or `unknown`, or a call that adds one to E or R | Throw an error declaration's instance; give the callee a contract | 3 |
| EK4812 | Warning: `new Promise` in VibeLang code | `callback((resume) => …)`, or `sync(promise)` | 10 |
| EK4813 | An Effect 3 name of the Effect module, such as `Effect.catchAll`; it replaces TypeScript's TS2339 | Write the Effect 4 name, `Effect.catch` (quick fix, also for TS2339 in `.ts` files) | 11 |
| EK4814 | Warning: a constructor in a class that extends a Schema class, which decoding does not call | Add a static function that makes an instance | 11 |
| EK4815 | Warning: a throw of an Effect value | Run it: `program()` | 3 |
| EK4816 | Off by default: a layer provided outside a function named `main` | Provide layers in the entry module's `main` | 4 |
| EK4817 | Off by default: `instanceof` a Schema class | `Schema.is(Person)(value)` (quick fix) | 11 |
| EK4818 | Off by default: `JSON.parse` or `JSON.stringify` in VibeLang code | Decode with `Schema.fromJsonString` | 11 |
| EK4819 | Reserved historical D242 rule: `yield` of an Effect in an Effect generator; suppressed in TypeScript by D266 | Write `yield*` (quick fix) | 10 |
| EK4820 | Reserved historical D242 rule: `yield*` of an Effect that never succeeds, as a statement; suppressed in TypeScript by D266 | Write `return yield*` (quick fix) | 10 |
| EK4821 | Off by default: `Schema.Struct` with a `_tag: Schema.Literal(…)` field | `Schema.TaggedStruct("Opened", { … })` (quick fix) | 11 |
| EK4822 | Off by default: a condition whose type is not boolean | Compare explicitly: `if (count > 0)` | 2 |
| EK4823 | Off by default: an arrow function whose block only returns a value | Write the value as the body (quick fix) | 2 |
| EK4824 | Off by default: a `pipe` with no functions | Remove the pipe (quick fix) | 5 |
| EK4825 | Off by default: a pipe of a pipe, `x.pipe(f).pipe(g)` | Pass every function to one pipe (quick fix) | 5 |
| EK4830 | A warning: a class that extends `Data.TaggedError`, `TaggedError`, `Data.Error`, or `Schema.TaggedError` with an empty body (D284) | `error NotFound { id: string }` (quick fix when the tag is the name and the base is a TaggedError) | 3 |
| EK4831 | A warning: a `Context.Service<Self, Shape>()(key)` class whose shape is an object type (D285) | `service Users { … }` (quick fix when the shape is written in place) | 4 |
| EK4832 | A warning: a `const` that holds a `Schema.Struct` a `schema` declaration writes (D286) | `schema User { … }` (quick fix) | 11 |
| EK4833 | A warning: `Effect.Effect<A, E, R>` or `Effect<A, E, R>` that the `!` form writes (D287) | `A!E!R` (quick fix) | 2 |
| EK4834 | A warning: `Effect.succeed(x)` or `Effect.fail(e)` where the call runs (D288) | `x`, or `throw e` (quick fix) | 2, 3 |
| EK4835 | A warning: `Effect.catchTag`, `Effect.catchTags`, or `Effect.catch` in a VibeLang function (D289) | `x catch (e) => match (e) { when NotFound: …; default: throw e; }` (quick fix) | 3 |
| EK4836 | A warning: `Effect.withSpan`, `Effect.provide`, or `Effect.provideService` in a VibeLang function (D290) | `withSpan("name") { … }`, `provide Layer { … }` (quick fix) | 4, 24 |
| EK4837 | A warning: a class that extends `Schema.TaggedError` with an empty body, which an error declaration replaces only by dropping its Schema (D284) | `error Failed { … }` where nothing needs the Schema; turn the code off where errors are Schemas by design | 3 |
| EK9100 | The text projector cannot decide a lambda or await element’s form (editor warning before the emit switch) | Annotate with the fork’s inferred form and channels where safe; program-form and await sites require an author’s annotation | 10 |

#### Linting

D35 assigns type and language diagnostics to `vibelang` and reserves type-aware
linting for `.ts` files: a stock TypeScript checker models different call
semantics and cannot type-check VibeLang correctly. Check `.vibe` files with
`vibelang --noEmit`.

Syntactic lint runs in ESLint through `vibelang/eslint` (D277). Its parser
runs `vibelang --estree`, which converts the VibeLang parser's tree to ESTree
(`internal/vibelangestree`): TypeScript forms as typescript-estree 8 converts
them, VibeLang forms whose long form would mislead a rule as `Vibe*` nodes,
ranges as UTF-16 offsets into the `.vibe` text, and the source's own tokens and
comments. `packages/compiler/test/eslint/run.sh` holds the conversion to
typescript-estree's output on every `.ts` file in the repository, and lints
every `.vibe` file. `vibelang-oxlint-config` is not shipped, and Vite Plus
integration has no acceptance coverage yet; don't install an unpublished
lint-config package from the registry.

#### Debugging

`vibelang` writes source maps when `sourceMap` is on; the Vite plugin always serves them. Each map carries original `.vibe` positions from the lowered AST through TypeScript emit, so any debugger that reads standard source maps maps breakpoints and paused frames to `.vibe` lines: VS Code's JavaScript debugger (Node.js, Bun, and Chrome targets), Chrome and Edge DevTools, and JetBrains IDEs.

```text
Paused on breakpoint, requestApproval  checkout.vibe:22
Call stack (JavaScript)
  requestApproval            checkout.vibe:22         the running VibeLang function
  runLoop                    effect/dist/internal/effect.js
  evaluate                   effect/dist/internal/effect.js
```

What a debugger shows follows from the frame invariant (Chapter 26):

- **One VibeLang frame at a time.** Compiled functions are generator functions named after their source, so the running function appears by name at its `.vibe` line. A function whose body is a single result returns its Effect without a generator, through a thunk named after it that a stack shows as `PrimitiveImpl.<name>` (D274). Its callers are frames on the fiber's `_stack`, not JavaScript activations, so the call stack shows Effect's run loop below it, not the VibeLang call chain.
- **Regions** are closures named after their function and block: `profile$try`, `profile$catch`, `profile$finally`, `profile$using`.
- **Clauses and producers.** A clause is a generator function named after its operation member (`approve`); a producer's body, one named after the producer (`lines`). A breakpoint in a clause shows the clause; the handled program's frames are inside `k`, a heap value, and reappear when the program resumes. A breakpoint in a producer shows the producer; the loop body runs in the clause of the `for...of` that iterates it.
- **Stepping over a call that pauses** continues on a later run-loop step, so the debugger steps into Effect's run loop; set a breakpoint on the next line instead, as with `Effect.gen` code today.
- **Logical call stacks** across calls and suspensions come from spans: set `tracing` to `"exported"` or `"all"`, and use Effect's `DevTools` layer (`effect/devtools`) with its VS Code extension, which shows fibers and the span stack.
