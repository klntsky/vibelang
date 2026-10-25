# Devtools

Compiler diagnostics show VibeLang contracts and catch failures or requirements that escape a function's promise. The `vibelang` compiler and its language server check `.vibe` and `.ts` files together.

## Install language tooling

```vibe
// example.vibe
// run: Console.log(sample())
function sample() {
  sleep("5 millis")
  return "sample"
}
// string!!
// Output: sample
```

Install `vibelang` as a dev dependency. `vibelang --lsp --stdio` starts the language server from the project's compiler, one server for `.vibe` and `.ts` files.

For VS Code and Cursor, install the extension that ships in `vibelang`, then open the application directory:

```sh
code --install-extension node_modules/vibelang/editor/vibelang.vsix
# Or, for Cursor:
cursor --install-extension node_modules/vibelang/editor/vibelang.vsix
```

The extension is not on the VS Code Marketplace or Open VSX yet; reinstall it from `node_modules` after you update `vibelang`.

Other editors configure their LSP client to run `vibelang --lsp --stdio` for `*.vibe` and `*.ts`, with `tsconfig.json` as the root marker. Turn off the other TypeScript server for this project to avoid duplicate diagnostics. JetBrains IDEs can use LSP4IJ.

For GitHub highlighting, add `*.vibe linguist-language=TypeScript` to `.gitattributes`; the starter's `.gitattributes` already has this line. Effect's own `@effect/tsgo` tooling remains for `.ts` files and does not check VibeLang semantics.

## Inspect inferred information

```vibe
// run: Console.log(accepted(2))
error Rejected

function accepted(value: number) {
  if (value < 0) throw new Rejected()
  return value + 1
}
// number!Rejected!
// Output: 3
```

Inspect `A`, `E`, and `R` when changing a function. A local call returns the success value; the function's complete contract also records escaping failures and unmet services, and generated declarations expose it to TypeScript consumers.

Hover over a function to see its inferred `Effect<A, E, R>`, and over a call to see its success value's type. Separate control-class hover text is planned but not shipped.

## Diagnostics, completions, and refactors

```vibe
// expect-error: EK2001
// Rejected escapes a contract promising no failures.
function promised(value: number): number!! {
  return accepted(value)
}
```

This is `EK2001`: handle `Rejected` inside the function or widen its contract. Related checks: unmet services (`EK2002`), running a program at module top level without an explicit Effect type on a `const` (`EK1001`), and passing a suspending callback to synchronous JavaScript (`EK4301`).

Inside functions, plain foreign calls need a boundary hatch such as `try` or `sync`, or a trusted declaration (`EK4401`). Effect's own packages and packages matching the `vibelang` setting `trustedPackages` are trusted: plain calls are synchronous, their exceptions are defects, and Effect-returning calls run. Running a platform API is rejected with an Effect module suggestion (`EK4402`, D108); importing or passing its binding as a value is allowed, and uncontrolled time or randomness reads have their own diagnostic (`EK4403`). A runner inside code a fiber runs is `EK1005`, and a try block that runs Effects with E = `never` gets `EK2004` (defects pass through), a warning by default.

EK1010 refuses an unparenthesized throw expression on the left of a binary operator (except comma), as a conditional's condition, or before `as` or `satisfies`. Write `throw (…)` to throw the whole expression; `value ?? throw e` and throw statements keep their meaning.

The server exposes TypeScript completions through the VibeLang checker, service member types included. Quick fixes repair retired spellings and the mechanical cases of Effect's language service rules (below); refactors write a function's inferred contract, convert an `async` function or an interface, and convert between calls and pipes (D245). Catch-specific completions are not implemented. Other diagnostic messages suggest repairs as text. Do not apply a TypeScript refactor that introduces `Effect.gen` or `yield*` inside `.vibe`.

## Configure compiler checks

```vibe
// run: Console.log(checked(2))
function checked(value: number): number!Rejected! {
  return accepted(value)
}
// Output: 3
```

A contract gives the compiler a promise to check. To infer selected channels, write `infer Name` in those slots: `Effect<number, Rejected, infer R>` checks success and failure and infers requirements. A return annotation with a body requires all three slots; for omitted slots, EK2003's text suggests explicit `never` or `infer` contracts. Configure the compiler with a `vibelang` object at the top level of `tsconfig.json`:

```json
{
  "compilerOptions": {
    "strict": true,
    "sourceMap": true
  },
  "vibelang": {
    "diagnostics": { "EK2004": "warning", "EK4403": "error" },
    "tracing": "exported",
    "trustedPackages": ["@acme/*"]
  }
}
```

A config that `extends` another inherits its `vibelang` settings, as it inherits compiler options. TypeScript, typescript-eslint, and editors' TypeScript servers ignore the top-level object. `compilerOptions.vibelang` is also accepted, but those tools reject it as an unknown compiler option (TS5023), so keep the settings at the top level (D275).

Each `.vibe` file keeps the settings of the package that owns it (D297). When a project reads another workspace package's `.vibe` source, as a project that imports that package does, the checker applies that package's settings, from the nearest `tsconfig.json` above the file, to the file's trust, diagnostics, and tracing. The importing project's settings apply to the files of its own package. A package therefore names the packages it trusts once, in its own `tsconfig.json`, and every project that imports it checks its source as the package's own build does.

EK1012, EK2004, EK3205, EK4403, EK4406, EK4409, Effect's language service rules, EK4801-EK4825 except EK4813, and the TypeScript-style forms below, EK4830-EK4837, can be set to `"error"`, `"warning"`, or `"off"`; the values shown are EK2004's and EK4403's defaults. With EK4403 lowered, those reads compile as trusted synchronous calls that tests cannot control. Migration diagnostics such as EK2112 for retired typed catch clauses are warnings with fixed severity; core language errors have fixed severity too. `@ts-expect-error` does not suppress EK diagnostics. `vibelang` exits non-zero on any error and reports source locations. `tracing` accepts `"none"` (the default), `"exported"`, or `"all"` to add function spans.

`trustedPackages` accepts package-name globs matched against `package.json` names, workspace packages included. The whole name must match; `*`, `?`, and character classes match within each slash-delimited segment, so `"@acme/*"` trusts packages in that scope. Without the option, foreign calls keep EK4401. For expected failures on one function, use `trusted(impl, [Errors])` from `vibelang-runtime`, in TypeScript or VibeLang.

## TypeScript-style Effect code

Effect declarations and combinator calls written in TypeScript's style compile in a `.vibe` file, and draw a warning where VibeLang has a form of its own (D284-D290). Rewrite `Effect.gen` bodies as functions using ordinary calls instead of `yield*`. The warning's quick fix writes that form when it keeps what the program does; otherwise the warning's note says what the form would change, and there is no fix.

| Code | TypeScript style | VibeLang form | Quick fix |
| --- | --- | --- | --- |
| EK4830 | `class NotFound extends Data.TaggedError("NotFound")<{ readonly id: string }> {}` | `error NotFound { id: string }` | When the tag is the class name and the base is a TaggedError; a different tag or `Data.Error` keeps the warning without one |
| EK4831 | `class Users extends Context.Service<Users, { find(id: string): Effect.Effect<User> }>()("app/Users") {}` | `service Users { find(id: string): User!! }` | When the members are written in the type argument; the key becomes the one the compiler derives |
| EK4832 | `const User = Schema.Struct({ id: Schema.String })` | `schema User { id: string }` | When every field has a declaration form; `Schema.optionalKey` or a check keeps the constant, without a warning |
| EK4833 | `Effect.Effect<User, NotFound, Db>` | `User!NotFound!Db` | Always, except where EK2003 asks for the missing channels |
| EK4834 | `return Effect.succeed(user)`, `return Effect.fail(error)`, where the call runs | `return user`, `throw error` | When the function stays an Effect function |
| EK4835 | `Effect.catchTag(find(id), "NotFound", (e) => …)` | `find(id) catch (e) => match (e) { when NotFound: …; default: throw e; }` | When each handler is an arrow function with one value |
| EK4836 | `Effect.withSpan(load(), "load")`, `Effect.provide(main(), AppLive)` | `withSpan("load") { return load() }`, `provide AppLive { return main() }` | Where a block can stand |
| EK4837 | `class Failed extends Schema.TaggedError<Failed>()("Failed", { … }) {}` | `error Failed { … }`, which is not a Schema | None; turn the code off where errors are Schemas by design |

Apply every fix in a project, then compile as the rest of the command says:

```sh
npx vibelang -p tsconfig.json --fix --noEmit
```

`--fix` also applies the quick fixes of the retired spellings: EK1009, EK2112, EK2113, EK2120, and EK2121. It prints each file it changes, with its fixes by code, and writes the files only after it checks the fixed program: a fix that would add an error is listed as not fixed, and the file keeps its other fixes. It fixes only what the project reports, so the `diagnostics` setting and the comments below choose what it touches; `// @vibelang-diagnostics-next-line EK4833:off` keeps one line as written. A second run finds nothing to fix. `--fix` cannot be combined with `--build` or `--watch`.

## Effect language service rules

The table accounts for the 108 diagnostics and editor features in `@effect/language-service` 0.87.5 (D240). Many rules address forms VibeLang replaces: there is no `Effect.gen` to misuse, a call already runs, and `catch` catches failures. Existing EK diagnostics cover others. The remaining checks are EK4801-EK4825, with mechanical quick fixes, alongside refactors, hovers, and completions. The table also marks features not implemented.

EK4801-EK4825 are lint. Each has a default, error, warning, or off, and all but EK4813 can be set in the `vibelang` setting `diagnostics` or by a comment (D241). A later comment overrides an earlier one; within one comment, a code named on its own overrides `*`. Words after the codes are prose, and punctuation ends a category, so `// @vibelang-diagnostics-next-line EK4801:off: returned unrun` keeps its reason:

```vibe
// run: Console.log(discard(() => { Console.log("unused") ; return 42 }))
function discard(program: Effect.Effect<number>) {
  // @vibelang-diagnostics-next-line EK4801:off
  program
  return 0
}
// Output: 0
```

`// @vibelang-diagnostics EK4818:warning` applies from its line to the end of the file; `*` names every adjustable code. EK4801, EK4819, and EK4820 also report in the `.ts` files the program checks; for Effect's other rules in `.ts` files, use Effect's own TypeScript 7 server, `@effect/tsgo` (D242).

| Group | Effect's rule or feature | In VibeLang | Example |
| --- | --- | --- | --- |
| Correctness | `anyUnknownInErrorContext` | EK4811, off by default | `throw cause` where `cause: unknown` |
| Correctness | `classSelfMismatch` | Not needed: a `service` has no `Self` parameter | `service Users { find(id: string): User!NotFound }` |
| Correctness | `duplicatePackage` | EK4803, warning | two versions of `effect` in one program |
| Correctness | `effectFnImplicitAny` | Not needed: parameters are TypeScript's, so `strict` reports TS7006 | `function load(id) { … }` |
| Correctness | `floatingEffect` | EK4801, error, with a quick fix; also in `.ts` files | `program` alone on a line; the fix writes `program()` |
| Correctness | `genericEffectServices` | Covered: EK3005 | `service Repo<T> { … }` |
| Correctness | `missingEffectContext` | Covered: EK2002 at a contract, EK3201 at a runner | `function f(): User!NotFound! { return Users.find(id) }` is EK2002 |
| Correctness | `missingEffectError` | Covered: EK2001 | `function f(): User!! { return Users.find(id) }` |
| Correctness | `missingLayerContext` | Covered: EK2002 and EK3201; `wire` places providers | `provide UsersLive { … }` without the database layer is EK3201 at the runner |
| Correctness | `missingReturnYieldStar` | Not needed in `.vibe`: `throw` ends the function. EK4820 in `.ts` files | `throw new NotFound({ id })` |
| Correctness | `missingStarInYieldEffectGen` | Not needed in `.vibe`: a call runs, and `Effect.gen` with `function*` is EK1002. EK4819 in `.ts` files | `const user = Users.find(id)` |
| Correctness | `nonObjectEffectServiceType` | Effect 3 only, and a `service` declares members | `service Port { value: number }` |
| Correctness | `outdatedApi` | EK4813, error, in place of TS2339, with a quick fix; the same fix on TS2339 in `.ts` files | `Effect.catchAll`; the fix writes `Effect.catch` |
| Correctness | `outdatedEffectCodegen` | Not adopted: no codegen comments | None |
| Correctness | `overriddenSchemaConstructor` | EK4814, warning | `class Person extends Schema.Class<Person>("Person")({ … }) { constructor() { … } }` |
| Correctness | `unsupportedServiceAccessors` | Not needed: `Users.find(id)` calls a member through the key, generic or not | `Users.find(id)` |
| Anti-pattern | `catchUnfailableEffect` | Covered: EK2004, warning | `try { Effect.succeed(1) } catch (e) { … }`; a catch receives typed failures only, in plain functions too (D282) |
| Anti-pattern | `effectFnIife` | Not needed: no `Effect.fn`; an immediately called function is a call | `(() => load())()` |
| Anti-pattern | `effectGenUsesAdapter` | Not needed: no `Effect.gen` | None |
| Anti-pattern | `effectInFailure` | EK4815, warning | `throw program`; run it: `program()` |
| Anti-pattern | `effectInVoidSuccess` | EK4802, warning, with a quick fix; TS2322 under a `void!!` contract | `return program` from a function that runs Effects |
| Anti-pattern | `globalErrorInEffectCatch` | EK4804, warning | `try(code, (cause) => new Error(String(cause)))` |
| Anti-pattern | `globalErrorInEffectFailure` | EK4804, warning | `throw new Error("boom")`; declare `error Boom { }` |
| Anti-pattern | `layerMergeAllWithDependencies` | EK4807, warning, with a quick fix | `Layer.mergeAll(DbLive, UsersLive)`; the fix writes `wire` |
| Anti-pattern | `lazyPromiseInEffectSync` | Not needed: `sync(() => promise)` already compiles to `Effect.promise` (D148) | `sync(async () => fetch(url))` |
| Anti-pattern | `leakingRequirements` | EK4809, warning; `@vibelang leakable` exempts a service | `service Users { find(): User!!Db; all(): User[]!!Db }` |
| Anti-pattern | `multipleEffectProvide` | EK4808, warning, with a quick fix | `provide A { provide B { … } }`; the fix writes `provide A, B { … }` or `provide wire(A, B) { … }` |
| Anti-pattern | `returnEffectInGen` | EK4802, warning, with a quick fix | `return program`; the fix writes `return program()` |
| Anti-pattern | `runEffectInsideEffect` | Covered: EK1005 | `Effect.runPromise(load())` inside a function |
| Anti-pattern | `schemaSyncInEffect` | Effect 3 only | None |
| Anti-pattern | `scopeInLayerEffect` | Effect 3 only: Effect 4's `Layer.effect` owns its scope | `Layer.effect(Users) { using pool = open(); … }` |
| Anti-pattern | `strictEffectProvide` | EK4816, off by default | `provide AppLive { … }` outside `main` |
| Anti-pattern | `tryCatchInEffectGen` | Not needed: `catch` catches failures | `try { Users.find(id) } catch (e) { … }` |
| Anti-pattern | `unknownInEffectCatch` | EK4806, warning | `try(code, (cause) => cause)` |
| Effect-native | `asyncFunction` | Covered: EK1003 | `async function load() { … }` |
| Effect-native | `cryptoRandomUUID` | Covered: EK4403 | `crypto.randomUUID()` |
| Effect-native | `cryptoRandomUUIDInEffect` | Covered: EK4403 | `crypto.randomUUID()` |
| Effect-native | `extendsNativeError` | EK4805, warning | `class Boom extends Error {}`; declare `error Boom { }` |
| Effect-native | `globalConsole` | Covered: EK4402 | `console.log(x)` |
| Effect-native | `globalConsoleInEffect` | Covered: EK4402 | `console.log(x)` |
| Effect-native | `globalDate` | Covered: EK4403 | `Date.now()` |
| Effect-native | `globalDateInEffect` | Covered: EK4403 | `new Date()` |
| Effect-native | `globalFetch` | Covered: EK4402 | `fetch(url)` |
| Effect-native | `globalFetchInEffect` | Covered: EK4402 | `fetch(url)` |
| Effect-native | `globalRandom` | Covered: EK4403 | `Math.random()` |
| Effect-native | `globalRandomInEffect` | Covered: EK4403 | `Math.random()` |
| Effect-native | `globalTimers` | Covered: EK4402 | `setTimeout(f, 10)` |
| Effect-native | `globalTimersInEffect` | Covered: EK4402 | `setTimeout(f, 10)` |
| Effect-native | `instanceOfSchema` | EK4817, off by default, with a quick fix | `value instanceof Person`; the fix writes `Schema.is(Person)(value)` |
| Effect-native | `newPromise` | EK4812, warning | `new Promise((resolve) => …)`; use `callback((resume) => …)` |
| Effect-native | `nodeBuiltinImport` | Covered: EK4402 where the code runs the import; importing is allowed (D108) | `readFile(path)` from `node:fs/promises` |
| Effect-native | `preferSchemaOverJson` | EK4818, off by default | `JSON.parse(text)` |
| Effect-native | `processEnv` | Covered: EK4402 | `process.env.PORT`; use `Config` |
| Effect-native | `processEnvInEffect` | Covered: EK4402 | `process.env.PORT` |
| Effect-native | `unsafeEffectTypeAssertion` | EK4810, warning, with a quick fix | `program as Effect.Effect<number>` drops a failure |
| Style | `catchAllToMapError` | Not needed: a catch that rethrows is the one form | `catch (e) { throw new Failed({ cause: e }) }` |
| Style | `deterministicKeys` | Not needed: the compiler derives each service key from its package, module, and name | `@acme/app/src/users#Users` |
| Style | `effectDoNotation` | Not needed: `const` statements bind values | `const user = Users.find(id)` |
| Style | `effectFnOpportunity` | Not needed: every function is one | `function load(id: string) { … }` |
| Style | `effectMapFlatten` | Not needed: statements | `const a = f(); return g(a)` |
| Style | `effectMapVoid` | Not needed: statements | `f()` on its own line |
| Style | `effectSucceedWithVoid` | Not needed: a function that returns nothing | `function done() { }` |
| Style | `flatMapToMap` | Not needed: statements | `return g(f())` |
| Style | `importFromBarrel` | Not needed: the compiler imports effect's root names (D146) | `Option.some(1)` with no import |
| Style | `missedPipeableOpportunity` | Refactor: "Convert to a pipe" | `f(g(h(x)))` becomes `x \|> h(%) \|> g(%) \|> f(%)` |
| Style | `missingEffectServiceDependency` | Effect 3 only; `wire` computes dependencies | `wire(UsersLive, DbLive)` |
| Style | `nestedEffectGenYield` | Not needed: no `Effect.gen` | None |
| Style | `redundantSchemaTagIdentifier` | Not needed: `error` and `schema` declarations take their tag from their name | `error NotFound { id: string }` |
| Style | `schemaStructWithTag` | EK4821, off by default, with a quick fix to `Schema.TaggedStruct` | `Schema.Struct({ _tag: Schema.Literal("A"), … })` |
| Style | `schemaUnionOfLiterals` | Effect 3 only | None |
| Style | `serviceNotAsClass` | Not needed: `service` declarations | `service Users { … }` |
| Style | `strictBooleanExpressions` | EK4822, off by default | `if (count) { … }` |
| Style | `unnecessaryArrowBlock` | EK4823, off by default, with a quick fix | `(x) => { return x + 1 }` |
| Style | `unnecessaryEffectGen` | Not needed: no `Effect.gen` | None |
| Style | `unnecessaryFailYieldableError` | Not needed: `throw new NotFound({ id })` | `throw new NotFound({ id })` |
| Style | `unnecessaryPipe` | EK4824, off by default, with a quick fix | `pipe(x)` |
| Style | `unnecessaryPipeChain` | EK4825, off by default, with a quick fix | `x.pipe(f).pipe(g)` |
| Hover | Extended Effect type | Covered: a function's hover shows its contract, `A!E!R` (D77) | `string!UserNotFound!Users` |
| Hover | `yield*` hover | Not needed: a running call's hover shows its success value (D77) | Runs here and returns `User` |
| Hover | Layer service involvement | Hover: a layer lists what it provides, needs, and fails with | Hover on `UsersLive` |
| Hover | Layer graph | Hover: `wire` lists the order it builds its layers in, and what each uses | Hover on `wire(UsersLive, DbLive)` |
| Completion | `Self` in classes | Not needed: declarations have no `Self` | None |
| Completion | `Effect.gen` snippet | Not needed: no `Effect.gen` | None |
| Completion | `Effect.fn` span name | Not needed: `tracing` names spans after functions (D104) | `"tracing": "exported",
      "trustedPackages": ["@acme/*"]` |
| Completion | Duration strings | Completion: `"2 seconds"` and the other units where a `Duration.Input` goes | `Effect.sleep("1 second")` |
| Completion | Namespace import style | Not needed: effect's root names are in scope (D146) | `Option.some(1)` |
| Completion | `Schema.brand` name | Completion: the declared name in `Schema.brand("…")` | `schema UserId = Schema.String.pipe(Schema.brand("UserId"))` |
| Completion | Comment directives | `@vibelang-diagnostics` comments (D241), whose codes complete | `// @vibelang-diagnostics-next-line EK4801:off` |
| Refactor | async function to `Effect.gen` | Refactor: "Convert to a VibeLang function" | `await fetch(url)` becomes `try((url) => fetch(url), url)` |
| Refactor | async function to `Effect.gen`, with an error per promise | Refactor: "Convert to a VibeLang function with an error per await" | `try(fetch(url), (cause) => new FetchFailed({ cause }))` |
| Refactor | async function to `Effect.fn` | Refactor: "Convert to a VibeLang function"; VibeLang has one function form | As above |
| Refactor | async function to `Effect.fn`, with an error per promise | Refactor: "Convert to a VibeLang function with an error per await" | As above |
| Refactor | `Effect.gen` to `Effect.fn` | Not needed: one function form | None |
| Refactor | Implement service accessors | Not needed: `Users.find(id)` | None |
| Refactor | Function calls to pipe | Refactor: "Convert to a pipe" | `f(g(x))` becomes `x \|> g(%) \|> f(%)` |
| Refactor | Pipe to data-first | Refactor: "Convert the pipe to calls" | `x \|> g(%) \|> f(%)` becomes `f(g(x))` |
| Refactor | Toggle return type signature | Refactor: "Write the inferred contract" and "Remove the contract" | `function load(id: string): User!NotFound!Users` |
| Refactor | Remove an unnecessary `Effect.gen` | Not needed: no `Effect.gen` | None |
| Refactor | Wrap with `Effect.gen` | Not needed: every function is one | None |
| Refactor | Toggle pipe styles | Refactor: "Write pipe(x, …)" and "Write x.pipe(…)" | `x.pipe(f, g)` and `pipe(x, f, g)` |
| Refactor | Layer Magic | Not needed: `wire` composes layers in dependency order (D27) | `wire(AppLive, UsersLive, DbLive)` |
| Refactor | Structural type to Schema | Refactor: "Convert to a schema declaration" | `interface User { name: string }` becomes `schema User { name: string }` |
| Refactor | `Effect.Service` to `Context.Tag` | Not needed: `service` declarations | None |
| Codegen | `annotate` | Refactor: "Write the inferred contract" | `function load(id: string): User!NotFound!Users` |
| Codegen | `accessors` | Not needed: `Users.find(id)` | None |
| Codegen | `typeToSchema` | Refactor: "Convert to a schema declaration" | `schema User { name: string }` |
| Other | Renaming a tagged class renames its tag | Not needed: an `error` declaration's tag is its name, so a rename renames both | `error NotFound { }` renamed to `Missing` |
| Other | Go to definition for `RpcClient` | Not adopted yet: the language server has no definition hook | None |

## Lint and build integration

Type-check with `vibelang --noEmit`. The compiler owns type and language diagnostics in the terminal, editor, and Vite overlay. The local `vibelang-vite` package supports Vite 7 and 8 and Vitest 5. It runs one whole-project compiler subprocess on the first request and after edits, serves JavaScript and source maps, and invalidates loaded `.vibe` modules and their importers; it does not use `vibelang --api`.

```ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import vibelang from "vibelang-vite"

export default defineConfig({
  plugins: [vibelang()],
  test: { include: ["src/**/*.test.vibe"] }
})
```

The starter already includes this configuration. `bun run test` checks and compiles source tests without a prior build. See [TestClock](../testing/063-testclock.md) for tests with virtual time.

Lint `.vibe` files with ESLint through `vibelang/eslint`, the parser the `vibelang` package ships (D277). The compiler parses each file, so every VibeLang form is accepted, and ESLint gets the tree typescript-eslint gives it for TypeScript: typescript-eslint's rules and plugins such as `eslint-plugin-import`, `eslint-plugin-jsdoc`, and `eslint-plugin-unicorn` apply unchanged. Install `eslint` and `typescript-eslint` 8 beside `vibelang`; the parser uses typescript-eslint's scope analysis.

```js
// eslint.config.js
import tseslint from "typescript-eslint"
import vibelang from "vibelang/eslint"

export default tseslint.config({
  files: ["src/**/*.vibe"],
  extends: [tseslint.configs.recommended],
  languageOptions: { parser: vibelang }
})
```

Reports, fixes, and `// eslint-disable-next-line` comments land on the `.vibe` text. Rules that need type information, such as `@typescript-eslint/no-unnecessary-type-assertion`, cannot run on a `.vibe` file, since no TypeScript program loads one: enable them for `.ts` files only and check types with `vibelang --noEmit`. EK4801-EK4825 stay in the compiler and the language server. To see the tree a rule reads, run `vibelang --estree main.vibe < main.vibe`.

Forms whose TypeScript long form would mislead a rule have their own node types, which rules and `no-restricted-syntax` selectors can name:

| Form | ESTree node |
| --- | --- |
| `A!E!R` | `VibeEffectType` with `success`, `failure`, and `requirements` |
| `A!infer E`, outside a conditional type | `VibeInferType` with `name` |
| `x \|> f(%)` | `VibePipeExpression` with `left` and `right`; `%` is `TopicReference` |
| `x catch fallback`, `x catch (e) => recover(e)` | `VibeCatchExpression` with `expression` and `handler` |
| `provide AppLive, Random = seeded { ... }` | `VibeProvideStatement` with `providers` and `body`; `Random = seeded` is `VibeServiceProvider` |
| `uninterruptible { ... }`, `restore { ... }`, `interruptible { ... }` | `VibeMaskStatement` with `kind` and `body` |
| `module text { ... }` | `VibeModuleDeclaration` with `id` and `body`, a `TSModuleBlock` of the body's TypeScript; `from text` is an `Identifier` |

For example, `VibeCatchExpression > Literal.handler[value='']` selects a recovery to an empty string.

The other forms keep the TypeScript the checker reads: a block param is an arrow function argument; `do { ... }` and a throw expression are immediately invoked arrow functions; `error NotFound { id: string }` is a class with those fields; `service` and `control` are abstract classes; `schema User { ... }` is a `const User` typed by its fields; `fork`, `join`, and `await.all` are calls. A module declaration's body lints as its own TypeScript module: sharing a name with the file is not a redeclaration, and `no-undef` reports a file binding read from the body. The parser leaves the names it writes for the checker, such as a `match`'s temporaries, out of scope analysis, so no rule reports a name the author did not write. A Vite Plus integration and a VibeLang Oxlint configuration are not shipped.

## Debugging in editors

```vibe
// run: Console.log(tracedSample())
function tracedSample() {
  return withSpan(sample(), "sample")
}
// Output: sample
```

Add spans where an execution boundary should appear in telemetry. VibeLang functions are untraced by default; `tracing` can add spans to exported functions or named functions and methods that compile to frames. Anonymous functions get no automatic span. Effect's `DevTools` layer from `effect/devtools`, with its VS Code extension, shows fibers and logical span stacks across suspensions.

`vibelang` with `sourceMap` enabled, and the Vite plugin, map emitted JavaScript directly to `.vibe` positions. Debuggers that read standard source maps map breakpoints and paused frames to `.vibe`: VS Code's JavaScript debugger, Chrome and Edge DevTools, and JetBrains IDEs.

Compiled functions and producers keep their source names; clauses use the operation member's name, and region closures use names such as `sample$try`, `$catch`, `$finally`, and `$using`. The JavaScript stack shows the active frame above Effect's run loop. A paused continuation is a heap value: its frames are absent while a clause runs and active again when it resumes. Stepping over a pausing call can enter the run loop; set a breakpoint on the next source line. Use spans for the logical call chain. When reporting a problem, include the Effect version, inferred contract, and diagnostic.

## Coming from Effect

Library debugging concepts still apply; VibeLang's call execution, service declarations, and source mapping need language-aware tooling beyond the upstream TypeScript plugin.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/devtools)
