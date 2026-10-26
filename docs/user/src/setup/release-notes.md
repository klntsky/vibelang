# Release notes

VibeLang is v0, so a new version can change the language in breaking ways.
Before you update, read the notes for every version between yours and the new
one, and update `vibelang`, `vibelang-runtime`, and `vibelang-vite` together.

## 0.0.10 (unreleased)

This release has no breaking changes. Everything that compiles on 0.0.9
compiles the same.

- The terse contract syntax is back as an alias of `Effect<A, E, R>`, and
  the guides, the skill and the starter use it again:

  | Terse | Means |
  | --- | --- |
  | `User!NotFound!Db` | `Effect<User, NotFound, Db>` |
  | `User!NotFound!` | `Effect<User, NotFound>`: needs nothing |
  | `User!!Db` | `Effect<User, never, Db>`: cannot fail |
  | `User!!` | `Effect<User>`: cannot fail and needs nothing |
  | `User!NotFound` on a function with a body | `Effect<User, NotFound, infer R>` |

  Both spellings are valid and can be mixed in one file. `Effect<User>`
  keeps the meaning it has had since 0.0.8, `Effect<User, never, never>`.
- Code written for 0.0.6 with the shorthand compiles again without changes.
  The error EK1014 is retired, and `vibelang --fix` no longer rewrites the
  shorthand.
- EK4833 still reports only `Effect.Effect<A, E, R>`; its quick fix and
  `vibelang --fix` write the terse form. Hovers, messages and quick fixes
  show Effect types in the terse form again.
- EK4402 names the Effect service that replaces a platform API, the import
  that brings it in, and the layer that provides it: `FileSystem` with
  `NodeServices.layer`, `HttpClient` with `FetchHttpClient.layer`, `Path`
  for `process.cwd()`, `FileSystem.makeTempDirectory()` for `os.tmpdir()`.
  For an API that Effect lacks, such as `os.hostname()`, it shows the call in
  a `sync` hatch instead of suggesting a wrapper module.

## 0.0.9

This release has no breaking changes.

- `vibelang --fix` writes only the project's own files. It leaves workspace
  dependencies and referenced projects unchanged, while still reporting their
  diagnostics.
- When a round of fixes would add an error, `--fix` narrows down the refused
  fixes without rechecking the whole project once per file. Stopping it with
  SIGTERM exits cleanly, reports the interruption and writes no changes.
- Emitted declaration files import prelude names used in `typeof` queries
  and in inferred types copied from libraries. TypeScript consumers now
  resolve those names and retain their types.
- The register loader works with tools that read inline source maps, including
  Playwright's stack-trace support. They no longer mistake the loader's
  source map template for its own map.

## 0.0.8

0.0.7 was not published: 0.0.8 replaces it, and these notes include its changes.

### Changes that can break code since 0.0.6

- A contract is written `Effect<A, E, R>`, as in TypeScript. The shorthand
  `Result!Error!Dependencies` is retired: each use is the error EK1014,
  whose quick fix writes the Effect type it stood for. Run
  `vibelang -p tsconfig.json --fix` to rewrite a whole project:

  | 0.0.7 | 0.0.8 |
  | --- | --- |
  | `User!NotFound!Db` | `Effect<User, NotFound, Db>` |
  | `User!NotFound!` | `Effect<User, NotFound>` |
  | `User!!Db` | `Effect<User, never, Db>` |
  | `User!!` | `Effect<User>` |
  | `User!NotFound` on a function with a body | `Effect<User, NotFound, infer R>` |
  | `User!NotFound` anywhere else | `Effect<User, NotFound>` |

- A type argument left out of an Effect type means what it means in
  TypeScript: `Effect<A>` is `Effect<A, never, never>` and `Effect<A, E>`
  is `Effect<A, E, never>`. A return annotation such as `Effect<User>` on a
  function with a body was EK2003; it now checks that the function cannot
  fail and needs nothing. A service or control member may write
  `Effect<User>` or `Effect<User, NotFound>`, which EK2101 rejected.

- The starter treats EK3205 as an error: an entry module's `main` that
  needs services without providers stops the build and emits no JavaScript.
  Provide those services before running the entry program.
- Members returned by `Layer.effect(Service) { ... }` must fit the
  service's contract, as in `Layer.succeed`. EK3101 rejects a member that
  looks up services its contract does not allow at call time; failures
  outside the member's contract are EK2001.
- A recursive call whose contract leaves failures or requirements inferred
  uses the written success type in an operand or initializer. Where typing
  the call needs the whole contract, including an argument, a logical
  operand or a generic self-call, EK2102 asks for every channel instead of
  recursing through inference or reporting a circular return type.
- A `for...of` whose body runs an Effect calls its iterator's `return()`
  when the body fails or is interrupted, as it does on `break` or
  `return`. Iterator cleanup that was previously skipped now runs.
- EK4201 also rejects unhandled control operations passed by name across a
  fork, an await operation or a concurrent loop. Install the handler inside
  the new fiber, or run the operation on the current fiber.
- A comma between fields of an `error`, `schema error`, `service` or
  `control` declaration is EK1013. Separate fields with semicolons or new
  lines.

### Compiler

- A `data` declaration declares a tagged union with Effect's own
  `Data.TaggedEnum` type and its `Data.taggedEnum` constructors:
  `data Delivery { Waiting; Sent { tracking: string } }`.
  `Delivery.Sent({ tracking })` builds a value, and
  `when Delivery.Sent { let tracking }:` matches one. A `match` without
  `default` over a data type must handle every variant (EK4701). See
  [Data](../data-types/073-data.md).

- CommonJS output binds the names a file takes from the prelude, including
  `TaggedError`, bare Effect functions, `pipe` and `flow`. These names
  could previously fail with a `ReferenceError` when the module ran.
- Invalid jumps without a target, a Stream loop without a binding and
  incomplete Effect type arguments report diagnostics instead of crashing
  the compiler.
- A handler's tail `resume` inside a `for concurrent` body no longer
  reports EK4606.
- Hovering over a by-name argument no longer leaves the call inferred as
  `unknown`. The language server identifies itself as `vibelang`.

### New diagnostics

- EK1014 reports the retired contract shorthand and offers its
  `Effect<...>` replacement in the editor and through `vibelang --fix`.
- EK4833 reports only `Effect.Effect<A, E, R>`, and its fix writes
  `Effect<A, E, R>`. Hovers and messages show Effect types as `Effect<…>`.
- EK1013 names the field separator error, and its quick fix replaces the
  comma with a semicolon. Misspelled names in a `.vibe` file can suggest
  names from the prelude.
- EK1001 identifies the plain JavaScript context that cannot run a program,
  including class initializers, constructors, accessors, static blocks and
  default parameter values. EK4301's hatch suggestion includes the callback
  parameter where the call needs it.
- EK3201 explains that a program typed `any` has unknown service
  requirements. EK4401 suggests a `try` call that passes inputs into the
  hatch and the `trusted()` wrapper, and EK2003 explains that an exception inside a hatch
  becomes a defect with `sync` or an expected failure with `try`.
- With `--pretty`, a file with both TypeScript and VibeLang errors is
  counted once in the error summary.

### Starter and packages

- The starter builds standalone TypeScript sources, checks their errors and
  keeps source tests out of build output. New apps start at version
  `0.0.0`, and their README describes an installed app.
- The starter pins `@effect/platform-node-shared` to `4.0.5` so its
  transitive peer dependency agrees with Effect. Example projects install
  the published VibeLang packages and use the same platform override.
- `vibelang-vite` preserves fractional compile start timestamps when it
  compares file changes and provides compile and invalidation traces through
  `VIBELANG_VITE_DEBUG=1`.
- The release publisher skips an existing registry version only when its
  tarball checksum matches the local tarball. The internal TypeScript mapper
  is marked private.

### Docs

- The guides and packaged skill use `Effect<...>` contracts and teach
  `data` declarations and exhaustive matches.
- Setup describes npm and Bun separately, makes Bun optional for npm users
  and links TypeScript 7's removed-option migration notes. The Vite guide
  explains source checking and import extensions.
- The guides teach the current `trusted()` wrapper, the starter's provider
  check and Effect 4 APIs. They name `Cause.UnknownError`, explain that
  plain JavaScript recursion can overflow the stack, and link known
  limitations and the pending-interruption resource workaround.
- Examples omit harness directives and unexplained decision references.
  Token comparisons state the measured corpus, and agent setup starts with
  the packaged skill.
- GitHub is available from the site's top navigation on every page.

### Known limitations

- Stock `tsc`, `vue-tsc` and `svelte-check` do not check `.vibe` imports.
  The TypeScript 7.1 content mapper is experimental and unpublished, and
  does not lower `data` declarations; see
  [the current mapper limitation](known-limitations.md).
- Native Windows binaries are not available; Windows needs WSL.
- Effect remains pinned to `4.0.5` exactly. Effect `4.0.6` changes cleanup
  failure handling in a way VibeLang does not yet support; see
  [the pin's explanation](existing-project.md).
- A JavaScript Disposable acquired at the end of a masked `using`
  initializer can leak if interruption is pending before its release is
  registered. Use `Effect.acquireRelease` with protected acquisition; see
  [the safe pattern](../resource-management/030-scope.md#known-limitation-pending-interruption-and-javascript-disposables).

## 0.0.6

0.0.5 was not published: 0.0.6 replaces it, and these notes include its
changes. 0.0.4 was not published either, so from 0.0.3 read the 0.0.4 notes
too.

### Changes that can break code since 0.0.3

These fixes are included in the changes since the last published version,
0.0.3. The comparisons below describe the unreleased 0.0.4 compiler.

- An optional call of a callback without a contract, `f?.(x)`, runs the
  program the callback returns, as `f(x)` does. On 0.0.4 it returned the
  program unrun. The callback's failures and requirements now reach the
  caller's contract, so a contract that leaves them out is EK2001 or EK2002.
- A file whose functions are Effects only because they throw, produce,
  declare `using`, or call `JSON.parse`, `decodeURI` or `decodeURIComponent`
  loads effect. On 0.0.4 such a function was typed and declared plain while
  its JavaScript returned an Effect. Where effect does not resolve, each such
  function is the new error EK1011.

### Compiler

- A comment before an expression that the compiler writes after a `return`,
  `throw` or `yield` stays a comment. A `match` arm whose value follows a
  comment, or whose pattern begins with a comment on its line, as in
  `when [ // first`, returned `undefined`.
- `uninterruptible { ... }` stays uninterruptible when it declares a `using`
  resource, and releases the resource when the block ends. On 0.0.4 such a
  block compiled without its mask, and an interruption reached its
  statements.
- The compiler no longer overflows the stack on a function that passes a
  call of itself by name, as `guard(healthy(index - 1))` does. It reports
  EK2102 at the function: write its failures and requirements in its
  contract.
- The failures and requirements of a callback without a contract default to
  `never`, so a call that omits an optional callback adds none.
- A call that runs no longer takes type arguments from its contextual type,
  which types its success value, not the Effect: `return late()` under
  `Late!!`, where `late` returns a `Late` error instance, compiles.

### New warnings

None of these warnings changes the emitted JavaScript.

- EK4409: a call of an `any`-typed value is a program's value or what an
  Effect function returns. The call returns its value, so a program the callee
  returns never runs. Give the callee a type. EK4409 is adjustable, like
  EK2004.
- EK4802 also reports an Effect value in a returned conditional's arm, in an
  operand of `??` or `||`, or on the right side of `&&`. Where the success
  type names no value (`void`, `unknown`, `any`, or a type parameter the call
  infers), it reports a returned Effect value or program literal, a program
  returned by name (a `const`, field or parameter), and a returned value whose
  type is a union with an Effect value or a program among its members. Its
  quick fixes call the value to run it, or write its type as the success type
  to return it; a union with a member that is no program gets only the second.
- EK2004 also reports where a plain function's `catch` lets an exception
  through: the first call in the `try` block of an untrusted function, a
  declaration without `@throws`, or a trusted package's export whose name
  ends in `Sync` or contains `OrThrow`. A plain `catch` receives expected
  failures only; `Result.try` or a `module` block catches the exception.
- EK1012: a call that never succeeds, stored by a variable's initializer, an
  object literal's field, an array element or an assignment. The call runs
  there, so `const missing = Effect.fail(new Missing())` fails at the
  `const`. To keep the program, write a function or the declared Effect
  type. EK1012 is adjustable, like EK4409.

### Tools and integrations

- Coverage: V8 coverage tools such as `@vitest/coverage-v8` and c8 count the
  functions and statements the author wrote, not the thunks and wrappers the
  compiler adds. Stack traces and breakpoints are unchanged.
- `vibelang-vite`: a Vitest rerun recompiles a `.vibe` module after its
  imports or the compiler change. The plugin keeps `.vibe` modules out of
  Vitest's on-disk module cache.
- Formatter: a `module` block formats to itself under a dprint host without a
  TypeScript plugin, where each pass added a blank line, and a statement that
  starts with a parenthesis formats with dprint's `;` guard instead of
  failing.
- The setup page's Bun check expects Bun 1.4.0 or later again.

### Known limitations

- A returned Effect value, program literal or program named by reference
  where the success type names no value is the success value, not a program
  to run. EK4802 warns.
- A stored call that never succeeds runs where it stands. EK1012 warns.
- A JavaScript Disposable acquired in a masked section at the end of a
  `using` initializer can leak when an interruption is pending: it is delivered
  as the mask ends, before `using` registers the release. The pending
  `scopes-using-pending-interrupt` case records `["acquired"]` instead of
  `["acquired","disposed"]`. Use `Effect.acquireRelease` with its default
  protected acquisition and an explicit disposal callback, or
  `using h = Effect.acquireRelease(...)` for a lexical lifetime.
  See the [safe pattern](../resource-management/030-scope.md#known-limitation-pending-interruption-and-javascript-disposables).
- A class method annotated with a plain return type whose body runs an
  Effect keeps its plain type; called through an interface, it returns the
  Effect instead of running it.

## 0.0.4

### Changes that can break code that compiled on 0.0.3

Each one rejects, or changes, a program that compiled on 0.0.3 but ran
differently from what its types said.

- A `throw` that can escape a constructor, a getter or setter, a class
  `static` block, or a module's top level is a compile error: EK2003, or
  EK1001 at the top level. These run as plain JavaScript, so the throw ended
  the program with a defect its type said could not happen. A throw that a
  `catch` in the same code handles still compiles. Put code that must throw a
  JavaScript exception, such as a test's throwing getter, in a `module`
  declaration.
- A `match` arm that tests a class runs only for an instance of that class.
  A match that relied on a class pattern to cover a type that is neither that
  class nor a subclass of it is a compile error; on 0.0.3 it could run the
  wrong arm.
- A subclass in a schema error family that declares a `_tag` field must
  include its own name in that field's type.
- A `const` declared with an Effect type holds its initializer through
  `as`, `satisfies`, `!`, a conditional, and a pipe, as it already did through
  parentheses. The initializer runs each time the const is run, not where the
  const is declared.
- A direct `sync` or `try` of an async function passes the function exactly
  its inputs, and an `AbortSignal` only where its overload passes one. On
  0.0.3 a rest parameter received a stray `undefined` and an optional
  parameter of another type could receive the signal.
- Every package requires Node 26 or later.

### Compiler

- A plain function is called as a plain function from every import shape:
  a default import beside named or namespace imports, CommonJS output, and
  `export default f`.
- `super` and `arguments` work in Effect methods, Effect arrows, and `try`
  blocks. On 0.0.3, `super` in an Effect method emitted a module that failed
  to load.
- A bundle runs only its entry module's `main`, in ES module and CommonJS
  output, and a CommonJS file that exports `main` loads and runs it when Node
  runs the file. The compiler warns
  (EK3205) when an entry module's `main` needs services that nothing
  provides.
- `module` declarations: emitted names never collide with your names, global
  names, or the file's imports; CommonJS output keeps their re-exports; their
  declaration files use the `.vibe` file's imports.
- A schema error family's base schema encodes and decodes its subclasses.
- A non-tail `resume` has a type that covers every value its handler can end
  with.
- `using` awaits `Symbol.asyncDispose` of a value that only async-disposes.
  `sync` and `try` called as values pass an async function its `AbortSignal`.
- The compiler no longer crashes on `service X: typeof X` or on a pipe with a
  missing left operand, and the formatter no longer crashes on a file with a
  `match`.
- Lambdas and pipes keep their meaning in more places: a lambda for an
  optional member takes the member's contract; a factory lambda keeps
  TypeScript's type in a branch of a conditional argument; either arm of a
  returned conditional can make a factory; an annotated lambda can call the
  variable it initializes; a returned pipe that ends in a call returns what
  the call returns; a function literal on a pipe's left side stays a program
  for a curried stage; a pipe in an Effect-typed field keeps its function
  plain for callers.
- A `provide` block keeps `Exclude<R, I>` of a generic requirement, and a
  `for...of` over a Stream inside one no longer crashes the compiler.
- A `restore` block inside an uninterruptible expression is inside it, and a
  comment before a by-name program no longer ends its return early.
- A destructuring property name inside a hatch is not a capture.
- Declaration files: a function expression's written contract is its
  declared type, a `.vibe` file's header comment stays first, and schema
  declarations keep their comments.
- Diagnostics: a contract suggestion for a `never` success writes `never`,
  and a diagnostics comment can give its reason after punctuation.

### Tools and integrations

- The install guides install the current version and say that Windows needs
  WSL. The starter's `AGENTS.md`, `llms.txt`, and the agent skill teach the
  forms the compiler accepts.
- npm packages and the VS Code extension ship complete license and notice
  files.
- Projects: a project compiles sources above its directory; a program without
  a `tsconfig.json` checks each package's `.vibe` files with that package's
  settings; `VIBELANG_PROJECT` compiles the modules in its directory, and a
  workspace dependency's source compiles with its own `tsconfig.json`; a
  check-only project that imports `.ts` files compiles in the loaders; a root
  project compiles a nested package it reaches through `node_modules`.
- `vibelang-unplugin`: esbuild tree-shakes the `.vibe` modules of a package
  without side effects, and each entry's types import only its own bundler's
  types.

### Known limitations

- `using` does not release a value whose acquisition ends in an
  uninterruptible section when an interruption arrives during that section.
- A class method annotated with a plain return type whose body runs an
  Effect keeps its plain type; called through an interface, it returns the
  Effect instead of running it.

## 0.0.3

### Changes that can break code that compiled on 0.0.2

- `error`, `service`, and `control` declarations are classes. Error classes
  accept constructors, methods, and inheritance; each subclass has its own
  name as its tag. An error family's base can declare `_tag: string`;
  an abstract error has that tag type by default.
- `throw` always fails outside a TypeScript hatch or module. Plain functions
  and JavaScript generators cannot let a typed failure escape. Use an Effect
  contract for a function that can fail, or TypeScript for JavaScript throws.

### Language

- `schema error` declares a serializable error class. `schema Name = Type`
  declares a schema and its type; `service Name: Type` declares a service
  whose shape can be any type.
- Inline `module` declarations hold self-contained TypeScript. Import their
  exports by the module's name; diagnostics and editor navigation point into
  the body, and declaration emit writes a separate declaration file.
- Both sides of a catch expression receive its contextual type. An exhaustive
  match omits its final test. More data-first Effect functions support method
  syntax, including functions whose receiver is not named `self`.

### Tools and integrations

- Format `.vibe` with the dprint process plugin and lint it with
  `vibelang/eslint`. A tree-sitter grammar and Neovim support are available.
- TypeScript-style Effect forms warn with editor quick fixes where a
  VibeLang form preserves their meaning. `vibelang --fix` applies the fixes.
- The compiler package ships the VibeLang agent skill.
- Imported foreign-project sources use their own VibeLang settings.
  Declarations use `.js` specifiers for `.vibe` modules, and tools reuse
  successful whole-project compiles through an on-disk cache.
- `bun --preload vibelang/bun` runs and tests source modules.
  The Vite plugin's `register: true` lets child processes import `.vibe`;
  `vibelang-unplugin/deferred` loads the plugin when a bundler awaits it.

## 0.0.2

Nine published packages: `vibelang`, four native compilers
(`vibelang-{darwin,linux}-{arm64,x64}`), `vibelang-runtime`,
`vibelang-vite`, `vibelang-unplugin`, and `create-vibelang`.
`vibelang-unplugin` is new in this release. Native Windows binaries
are not available yet; use WSL and follow the Linux instructions.

### Changes that can break code that compiled on 0.0.1

Each is a compile error that names the problem; none silently changes what
compiled code does.

#### Platform globals through `globalThis` are EK4402

`process.env` was already EK4402. `globalThis.process.env`,
`globalThis.fetch`, `globalThis.crypto.subtle`, `globalThis.location`,
`globalThis.navigator`, and the other platform globals now are too.

```vibe
// expect-error: EK4402
function home() {
  return globalThis.process.env.HOME
}
```

Read configuration with `Config` from Effect:

```vibe
function homeDir() {
  return Config.String("HOME")()
}
```

Otherwise, read the global in a TypeScript module the project trusts, such as
`platformEnv.ts`:

```ts
// TypeScript (.ts)
/** @vibelang trusted */
export const processEnv = () => globalThis.process?.env
```

#### A call typed as a union of Effects runs where it stands

A call to a function that returns `Effect<A> | Effect<never, E>` now runs the
program, as `yield* decide(flag)` does in TypeScript. In 0.0.1 it silently held
the program.

```vibe
error Rejected

function decide(flag: boolean): Effect.Effect<number> | Effect.Effect<never, Rejected> {
  return flag ? Effect.succeed(1) : Effect.fail(new Rejected())
}
```

A 0.0.1 workaround that holds the program and then calls it no longer compiles:

```vibe
// expect-error: TS2349
function total(flag: boolean) {
  const held = decide(flag)
  return held() + 1
}
```

Delete the workaround and use the value:

```vibe
function total(flag: boolean) {
  const value = decide(flag)
  return value + 1
}
// number!Rejected!
```

A function annotated with a union of Effects whose branches return decided
Exits, such as `cond ? Exit.fail(e) : Exit.void`, runs the chosen Exit.

#### `Request` is the web-standard class

Web-standard globals keep their names, as JavaScript's built-ins do: `Request`,
`Response`, `Headers`, `URL`, `URLSearchParams`, `TextEncoder`, `TextDecoder`,
`AbortController`, `AbortSignal`, `Blob`, `FormData`, `structuredClone`, and
`fetch`. Of Effect's root names only `Request` collides, so `Request.tagged`
without an import is now TS2339 or TS2702. Import Effect's module:

```vibe
import { Request } from "effect"

interface Lookup extends Request.Request<string> {
  _tag: "Lookup"
  id: string
}

const Lookup = Request.tagged<Lookup>("Lookup")
```

#### A `const` with a declared Effect type holds the program

`const p: Effect.Effect<A, E> = make()` keeps the program `make()` returns cold
instead of running it, at any scope. When the call returns a program that
produces a program, as `Effect.cached(...)` does, the annotation must name the
outer program, `Effect.Effect<Effect.Effect<A, E>>`, or the compiler reports
TS2322 or TS2375. To run the call, drop the annotation.

#### A throw expression can't be an operator's left operand (EK1010)

In expression position, `throw cond ? a : b` threw `cond` and discarded the
rest; it is now EK1010. Write `throw (cond ? a : b)`. A statement that starts
with `throw` is unchanged.

#### A curried pipe stage after a call keeps the call

`x |> f(args)(%)` becomes `x.pipe(f(args))` only when `x` calls nothing. When a
call builds the left side, bind it first: `const s = Schema.NullOr(Schema.String)`,
then `s |> f(args)(%)`.

### Fixes

- An Effect subtype returned at an Effect boundary runs. A value the contract's
  written success type accepts is returned as data:
  `keep(command): ChildProcess.Command!never!Scope` returns the command.
- A discarded structured Effect call is diagnosed (EK4301). A program cast and
  piped with `|>` is passed by name.
- Callback cleanup waits for async and Effect finalizers on interrupt. Copied
  declaration types import prelude names.
- A `function*` annotated `Generator<...>` stays a JavaScript generator.
- Function-valued service implementations and contract-annotated arrows keep
  their programs.
- Nested Effect function declarations hoist, so a call above the declaration
  works.
- The checker no longer overflows the stack on a shorthand discriminant property
  under a union context.
- By-name tail programs, conditional by-name arms, and optional program fields
  run as written.
- Diagnostics: inline boundary callbacks, `Effect.fn` span advice, non-adjustable
  EK option policies, and `.d.ts` JSDoc that matches tsc.

### Known issues

- A throwing local arrow with no contract, inside a function with a plain return
  annotation, reports EK2003 on the outer annotation. Annotate the local arrow,
  such as `(value: number): void => { ... }`.
- EK4802 can warn on a local annotated `Effect<Effect<...>>` that returns its
  program unrun. The emitted code is correct.
- Programs that by-name lowering wraps spend extra Effect operations.
- A recursive `Schema.Class` whose `Schema.suspend` callback writes an Effect
  contract, `(): Schema.Codec<Crate, CrateWire>!! => Crate`, reports TS2506 and
  TS2740. Annotate the plain return type instead,
  `(): Schema.Codec<Crate, CrateWire> => Crate`. Both compile to the same
  JavaScript.
