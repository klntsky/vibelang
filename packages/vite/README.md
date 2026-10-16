# @effectscript/vite

Compile `.es` modules with the native EffectScript compiler in Vite and Vitest.
The compiler owns type and language diagnostics. The plugin serves its JavaScript
and source maps, reports errors in the terminal and Vite overlay, and invalidates
source modules and their importers after edits.

## Install

For this unpublished local build, first run `sh scripts/setup.sh` from the
EffectScript checkout. In your application, install the local packages, using
absolute paths for projects outside the checkout:

```sh
bun add effect@4.0.5 /absolute/path/to/effectscript/packages/runtime
bun add -d /absolute/path/to/effectscript/packages/compiler \
  /absolute/path/to/effectscript/packages/vite vite
# For tests:
bun add -d vitest@5.0.5 @effect/vitest@4.0.5
```

Keep the checkout available. After updating it, rerun setup, then run
`bun update @effectscript/vite` in each project. No package is published to npm. The plugin supports Vite 7 and 8 on Node 22+;
the acceptance tests use Vite 8 and Vitest 5.

## Configure Vite

```ts
// vite.config.ts
import { defineConfig } from "vite"
import effectscript from "@effectscript/vite"

export default defineConfig({ plugins: [effectscript()] })
```

Keep your normal `tsconfig.json` with the `.es` sources included. The plugin
uses the same compiler options and diagnostics as `esc -p tsconfig.json`.
Import local sources with `./welcome.es`. Imports rewritten by `esc` to
`./welcome.js` resolve back to `welcome.es` in Vite's module graph.

## Configure Vitest

```ts
// vitest.config.ts
import { defineConfig } from "vitest/config"
import effectscript from "@effectscript/vite"

export default defineConfig({
  plugins: [effectscript()],
  test: { include: ["src/**/*.test.es"] }
})
```

```ts
// src/answer.test.es
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
assertion's stack points to its `.es` source line.

## Options

```ts
effectscript({
  project: "tsconfig.test.json", // Relative to Vite's root; default: tsconfig.json.
  compiler: "/absolute/path/to/esc" // Native binary or Node launcher.
})
```

By default, the plugin resolves `@effectscript/compiler` from the application's
root and runs its Node launcher. `ESC_BINARY` selects a native binary when no
`compiler` option is set. The compiler package is an optional peer because local
Bun installs otherwise try to fetch the unpublished package from the registry;
install it explicitly or provide a compiler path.

## How it works

v0 runs one `esc` subprocess for the whole project on the first module request
and after each source change. Requests share that compilation. Each plugin
instance owns an output directory under `node_modules/.effectscript/vite-*` and
removes it when Vite closes. It forces JavaScript and source-map emission,
embeds source content, and disables declarations, incremental output, and
emission on errors. Source paths in compiler maps are resolved to the original
files before Vite composes the maps for its transformations.

The Vite root must contain the project's source files. The plugin overrides
`rootDir` to that root so emitted paths are predictable. Normal application
`dist/` output stays separate. Each change recompiles the project and invalidates
all loaded `.es` modules and their transitive importers. This conservative rule
covers exported contract and control changes without duplicating compiler logic.
Direct HTTP requests for `.es` files enter Vite's module transform pipeline.

The plugin never implements language syntax or lowering. Prelude, service,
program, and cross-file import support comes from the installed compiler.
A compiler error blocks JavaScript serving, includes its original diagnostic
text and source location in the overlay, and clears on the next successful edit.

## Acceptance checks

From the repository root:

```sh
sh scripts/build-esc.sh
cd packages/vite
bun install
bun run test
```

The harness runs a passing `.es` suite through Vitest, then verifies an expected
failure at `failure.test.es:6:28`. A real HTTP server check verifies JavaScript,
source maps, importer invalidation, terminal and overlay diagnostics, and recovery.
A production build checks `.es` entry support. Temporary projects and compiler
output are removed after each check.

The template and the users, http-api, and resilience examples also use this
plugin for their source tests. Their advanced language features require the
corresponding compiler lanes; the acceptance fixtures above use explicit Effect
imports and plain and Effect functions supported by the initial compiler.
