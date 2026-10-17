# effeck-vite

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

Compile `.effeck` modules with the native Effeck compiler in Vite and Vitest.
The compiler owns type and language diagnostics. The plugin serves its JavaScript
and source maps, reports errors in the terminal and Vite overlay, and invalidates
source modules and their importers after edits.

## Install

```sh
npm install --save-dev effeck effeck-vite vite
# For tests:
npm install --save-dev vitest@5.0.5 @effect/vitest@4.0.5
```

The plugin runs the `effeck` compiler package, an optional peer dependency:
install it, or pass a `compiler` path. It supports Vite 7 and 8 on Node 22 or
later; the acceptance tests use Vite 8 and Vitest 5. Apps made with
`create-effeck` already include the plugin and a Vitest configuration.

## Configure Vite

```ts
// vite.config.ts
import { defineConfig } from "vite"
import effeck from "effeck-vite"

export default defineConfig({ plugins: [effeck()] })
```

Keep your normal `tsconfig.json` with the `.effeck` sources included. The plugin
uses the same compiler options and diagnostics as `effeck -p tsconfig.json`.
Import local sources with `./welcome.effeck`. Imports rewritten by `effeck` to
`./welcome.js` resolve back to `welcome.effeck` in Vite's module graph.

## Configure Vitest

```ts
// vitest.config.ts
import { defineConfig } from "vitest/config"
import effeck from "effeck-vite"

export default defineConfig({
  plugins: [effeck()],
  test: { include: ["src/**/*.test.effeck"] }
})
```

```ts
// src/answer.test.effeck
import { assert, it } from "@effect/vitest"
import { Effect } from "effect"

function answer() {
  return Effect.succeed(42)
}

it.effect("answers", () => {
  assert.strictEqual(answer(), 42)
})
```

Run `bunx vitest run` or `bunx vitest` for watch mode. No prior emission to
`dist/` is needed. `assert` and `expect` from `@effect/vitest` are trusted
exports of an Effect package, so a test calls them directly. A failed
assertion's stack points to its `.effeck` source line.

## Options

```ts
effeck({
  project: "tsconfig.test.json", // Relative to Vite's root; default: tsconfig.json.
  compiler: "/absolute/path/to/effeck" // Native binary or Node launcher.
})
```

By default, the plugin resolves `effeck` from the application's
root and runs its Node launcher. `EFFECK_BINARY` selects a native binary when no
`compiler` option is set.

## How it works

v0 runs one `effeck` subprocess for the whole project on the first module request
and after each source change. Requests share that compilation. Each plugin
instance owns an output directory under `node_modules/.effeck/vite-*` and
removes it when Vite closes. It forces JavaScript and source-map emission,
embeds source content, and disables declarations, incremental output, and
emission on errors. Source paths in compiler maps are resolved to the original
files before Vite composes the maps for its transformations.

The Vite root must contain the project's source files. The plugin overrides
`rootDir` to that root so emitted paths are predictable. Normal application
`dist/` output stays separate. Each change recompiles the project and invalidates
all loaded `.effeck` modules and their transitive importers. This conservative rule
covers exported contract and control changes without duplicating compiler logic.
Direct HTTP requests for `.effeck` files enter Vite's module transform pipeline.

The plugin never implements language syntax or lowering. Prelude, service,
program, and cross-file import support comes from the installed compiler.
A compiler error blocks JavaScript serving, includes its original diagnostic
text and source location in the overlay, and clears on the next successful edit.

## Acceptance checks

For contributors, from the Effeck repository root:

```sh
sh scripts/build-effeck.sh
cd packages/vite
bun install
bun run test
```

The harness runs a passing `.effeck` suite through Vitest, then verifies an expected
failure at `failure.test.effeck:6:28`. A real HTTP server check verifies JavaScript,
source maps, importer invalidation, terminal and overlay diagnostics, and recovery.
A production build checks `.effeck` entry support. Temporary projects and compiler
output are removed after each check.

The template and the users, http-api, and resilience examples also use this
plugin for their source tests. Their advanced language features require the
corresponding compiler lanes; the acceptance fixtures above use explicit Effect
imports and plain and Effect functions supported by the initial compiler.
