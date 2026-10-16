# Devtools

Use compiler diagnostics to inspect EffectScript contracts and catch failures or requirements that escape a function's promise. The `esc` compiler and its language server check `.es` and `.ts` files together; Oxlint supplies lint findings.

## Install language tooling

```ts
// example.es
function sample() {
  Effect.sleep("5 millis")
  return "sample"
}
// Effect<string, never, never>
```

Install `@effectscript/compiler` as a dev dependency. Its `esc --lsp --stdio` command starts the language server from the project's compiler, with one server for `.es` and `.ts` files.

In VS Code, install `effectscript.effectscript`. It registers `*.es` as the `effectscript` language, starts the project's server, and sets `typescript.validate.enable` to `false` in workspaces containing `.es` files so the built-in server does not duplicate diagnostics. Other editors configure their LSP client to run `esc --lsp --stdio` for `*.es` and `*.ts`, use `tsconfig.json` as the root marker, and turn off their other TypeScript server for the project. JetBrains IDEs can use LSP4IJ.

For GitHub highlighting, add `*.es linguist-language=TypeScript` to `.gitattributes`; otherwise GitHub highlights `.es` as JavaScript. Effect's own `@effect/tsgo` tooling remains for `.ts` files and does not check EffectScript semantics.

## Inspect inferred information

```ts
class Rejected extends TaggedError("Rejected")<{}> {}

function accepted(value: number) {
  if (value < 0) throw new Rejected()
  return value + 1
}
// Effect<number, Rejected, never>
```

Inspect `A`, `E`, and `R` when changing a function. A local call returns the success value, while the function's complete contract also records escaping failures and unmet services. Generated declarations expose that complete Effect contract to TypeScript consumers.

Hover over a function to see its inferred `Effect<A, E, R>` and control class; hover over a call to see its success value's type. The control class records whether the function computes directly, performs synchronous work, can pause, or can capture a continuation.

## Diagnostics, completions, and refactors

```ts
// Deliberately invalid: Rejected escapes a contract promising no failures.
function promised(value: number): Effect<number, never, never> {
  return accepted(value)
}
```

The language design diagnoses this as `ES2001`. Handle `Rejected` inside the function or widen its contract. Related checks include unmet services (`ES2002`), running a program at module top level (`ES1001`), and passing a suspending callback to synchronous JavaScript (`ES4301`).

Inside functions, plain foreign calls need an explicit boundary such as `Effect.try` or `Effect.tryPromise`, or a trusted declaration (`ES4401`). Effect's own packages are trusted. Platform calls are rejected with an Effect module suggestion (`ES4402`), and uncontrolled time or randomness reads have their own diagnostic (`ES4403`). A runner inside code a fiber runs is `ES1005`, and a `catch` that can never receive an expected failure is `ES2004`, a warning by default.

Completions include service members, prelude names, and the error classes a `catch` can receive. Quick fixes cover diagnostics with mechanical fixes, including converting generator code, adding a missing contract, wrapping an untrusted call, and replacing a platform call with its Effect module.

Refactors convert `Effect.gen` or `Effect.fn` functions into EffectScript functions, convert `Context.Service` classes to abstract service classes, scaffold service implementations as object literals or `implements` classes, and write inferred contracts as annotations. Whole-function conversion is offered only when every construct has an EffectScript form. Do not apply a TypeScript refactor that introduces `Effect.gen` and `yield*` inside `.es`.

## Configure compiler checks

```ts
function checked(value: number): Effect<number, Rejected, never> {
  return accepted(value)
}
```

Complete contracts give the compiler a concrete promise to check. Use all three channels or omit the annotation until partial-annotation spelling is chosen. Configure the compiler in `tsconfig.json`:

```json
{
  "compilerOptions": {
    "strict": true,
    "sourceMap": true,
    "effectscript": {
      "diagnostics": { "ES2004": "warning", "ES4403": "error" },
      "tracing": "exported"
    }
  }
}
```

Only ES2004 and ES4403 can be set to `"error"`, `"warning"`, or `"off"`; the values shown are their defaults. When ES4403 is lowered, those reads compile as trusted synchronous calls that tests cannot control. All other ES diagnostics are errors whose severity cannot change, and `@ts-expect-error` does not suppress them. `esc` exits non-zero on any error and reports locations in the source. `tracing` accepts `"none"` (the default), `"exported"`, or `"all"` to add function spans.

## Lint and build integration

The compiler owns type and language diagnostics in the terminal, editor, and Vite overlay. Oxlint parses `.es` as TypeScript and runs syntactic lint rules there. Its type-aware mode, tsgolint, skips `.es` because its embedded TypeScript checker models different call semantics.

Install `@effectscript/oxlint-config` as a dev dependency and extend it in `.oxlintrc.json`:

```json
{
  "extends": ["@effectscript/oxlint-config"]
}
```

This config disables type-aware rules on `.es` and syntactic rules whose premise changes, such as `require-yield`. Vite Plus uses `@effectscript/vite` for development, builds, and tests, and this Oxlint config for lint. Run `esc --noEmit` for type checking. Each diagnostic has one owner, so reports need no deduplication.

## Debugging in editors

```ts
function tracedSample() {
  return Effect.withSpan(() => sample(), "sample")
}
```

Add spans where you need an execution boundary to appear in telemetry. EffectScript functions are untraced by default; `tracing` can add spans to exported functions or all functions that compile to frames. Effect's `DevTools` layer from `effect/devtools`, with its VS Code extension, shows fibers and logical span stacks across suspensions.

`esc` with `sourceMap` enabled and the Vite plugin produce maps composed from `.es` through generated TypeScript to JavaScript. Debuggers that read standard source maps map breakpoints and paused frames to `.es`: VS Code's JavaScript debugger, Chrome and Edge DevTools, and JetBrains IDEs.

Compiled functions and producers retain their source names; clauses use the operation member's name, and region closures use names such as `sample$try`, `$catch`, `$finally`, and `$using`. The JavaScript stack shows the active frame above Effect's run loop. A paused continuation is a heap value, so its frames are absent while a clause runs and become active again when it resumes. Stepping over a pausing call can enter the run loop; set a breakpoint on the next source line. Use spans for the logical call chain, and include the Effect version, inferred contract, and diagnostic when reporting a problem.

## Coming from Effect

Library debugging concepts still apply. EffectScript's call execution, service declarations, and source mapping need language-aware tooling beyond the upstream TypeScript plugin.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/devtools)
