# Devtools

Use compiler diagnostics to inspect Effeck contracts and catch failures or requirements that escape a function's promise. The `effeck` compiler and its language server check `.effeck` and `.ts` files together; Oxlint supplies lint findings.

## Install language tooling

```ts
// example.effeck
function sample() {
  Effect.sleep("5 millis")
  return "sample"
}
// string!!
```

Install `effeck` as a dev dependency. Its `effeck --lsp --stdio` command starts the language server from the project's compiler, with one server for `.effeck` and `.ts` files.

For VS Code and Cursor, install the extension that ships in `effeck`, then open
the application directory:

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
# Or, for Cursor:
cursor --install-extension node_modules/effeck/editor/effeck.vsix
```

The extension is not on the VS Code Marketplace or Open VSX yet. Install it
again from `node_modules` after you update `effeck`.

Other editors configure their LSP client to run `effeck --lsp --stdio` for `*.effeck`
and `*.ts`, with `tsconfig.json` as the root marker. Turn off the other TypeScript
server for this project to avoid duplicate diagnostics. JetBrains IDEs can use
LSP4IJ.

For GitHub highlighting, add `*.effeck linguist-language=TypeScript` to `.gitattributes`; otherwise GitHub shows `.effeck` files without highlighting. The starter's `.gitattributes` already has this line. Effect's own `@effect/tsgo` tooling remains for `.ts` files and does not check Effeck semantics.

## Inspect inferred information

```ts
error Rejected

function accepted(value: number) {
  if (value < 0) throw new Rejected()
  return value + 1
}
// number!Rejected!
```

Inspect `A`, `E`, and `R` when changing a function. A local call returns the success value, while the function's complete contract also records escaping failures and unmet services. Generated declarations expose that complete Effect contract to TypeScript consumers.

Hover over a function to see its inferred `Effect<A, E, R>`; hover over a call to see its success value's type. Separate control-class hover text is a D33 design target, not a shipped feature.

## Diagnostics, completions, and refactors

```ts
// expect-error: EK2001
// Rejected escapes a contract promising no failures.
function promised(value: number): number!! {
  return accepted(value)
}
```

The language design diagnoses this as `EK2001`. Handle `Rejected` inside the function or widen its contract. Related checks include unmet services (`EK2002`), running a program at module top level (`EK1001`), and passing a suspending callback to synchronous JavaScript (`EK4301`).

Inside functions, plain foreign calls need an explicit boundary such as `Effect.try` or `Effect.tryPromise`, or a trusted declaration (`EK4401`). Effect's own packages are trusted. Running a platform API is rejected with an Effect module suggestion (`EK4402`, D108); importing or passing its binding as a value is allowed, and uncontrolled time or randomness reads have their own diagnostic (`EK4403`). A runner inside code a fiber runs is `EK1005`, and a `catch` that can never receive an expected failure is `EK2004`, a warning by default.

The server exposes TypeScript completions through the Effeck checker,
including service member types. D33 also calls for catch-specific completions,
mechanical ES quick fixes, and migration refactors. Those features are not
implemented by the current compiler. Diagnostic messages include suggested
repairs as text. Do not apply a TypeScript refactor that introduces `Effect.gen`
or `yield*` inside `.effeck`.

## Configure compiler checks

```ts
function checked(value: number): number!Rejected! {
  return accepted(value)
}
```

A contract gives the compiler a promise to check. To infer selected channels, write `infer Name` in those slots: `Effect<number, Rejected, infer R>` checks the success and failure channels and infers requirements. All three slots are required in a return annotation with a body. For omitted slots, EK2003 suggests explicit `never` or `infer` contracts in its diagnostic text. Configure the compiler in `tsconfig.json`:

```json
{
  "compilerOptions": {
    "strict": true,
    "sourceMap": true,
    "effeck": {
      "diagnostics": { "EK2004": "warning", "EK4403": "error" },
      "tracing": "exported"
    }
  }
}
```

Only EK2004 and EK4403 can be set to `"error"`, `"warning"`, or `"off"`; the values shown are their defaults. When EK4403 is lowered, those reads compile as trusted synchronous calls that tests cannot control. All other EK diagnostics are errors whose severity cannot change, and `@ts-expect-error` does not suppress them. `effeck` exits non-zero on any error and reports locations in the source. `tracing` accepts `"none"` (the default), `"exported"`, or `"all"` to add function spans.

## Lint and build integration

Run `effeck --noEmit` for type checking. The compiler owns type and language
diagnostics in the terminal, editor, and Vite overlay. The local
`effeck-vite` package supports Vite 7 and 8 and Vitest 5. It runs one
whole-project compiler subprocess on the first request and after edits, serves
JavaScript and source maps, and invalidates loaded `.effeck` modules and their
importers. It does not use `effeck --api`.

```ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import effeck from "effeck-vite"

export default defineConfig({
  plugins: [effeck()],
  test: { include: ["src/**/*.test.effeck"] }
})
```

The starter already includes this configuration. `bun run test` checks and
compiles source tests without a prior build. See
[TestClock](../testing/063-testclock.md) for tests with virtual time.

D35 assigns syntactic linting to Oxlint and reserves type-aware rules for `.ts`
files. `effeck-oxlint-config` and a tested Vite Plus integration are not
shipped. Keep type-aware linting off `.effeck`: a stock TypeScript checker models
different call semantics.

## Debugging in editors

```ts
function tracedSample() {
  return Effect.withSpan(sample(), "sample")
}
```

Add spans where you need an execution boundary to appear in telemetry. Effeck functions are untraced by default; `tracing` can add spans to exported functions or all functions that compile to frames. Effect's `DevTools` layer from `effect/devtools`, with its VS Code extension, shows fibers and logical span stacks across suspensions.

`effeck` with `sourceMap` enabled and the Vite plugin produce maps from emitted JavaScript directly to `.effeck` positions. Debuggers that read standard source maps map breakpoints and paused frames to `.effeck`: VS Code's JavaScript debugger, Chrome and Edge DevTools, and JetBrains IDEs.

Compiled functions and producers retain their source names; clauses use the operation member's name, and region closures use names such as `sample$try`, `$catch`, `$finally`, and `$using`. The JavaScript stack shows the active frame above Effect's run loop. A paused continuation is a heap value, so its frames are absent while a clause runs and become active again when it resumes. Stepping over a pausing call can enter the run loop; set a breakpoint on the next source line. Use spans for the logical call chain, and include the Effect version, inferred contract, and diagnostic when reporting a problem.

## Coming from Effect

Library debugging concepts still apply. Effeck's call execution, service declarations, and source mapping need language-aware tooling beyond the upstream TypeScript plugin.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/devtools)
