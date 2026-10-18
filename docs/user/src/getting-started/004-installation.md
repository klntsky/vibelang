# Installation

```sh
bun create vibelang@latest my-app
cd my-app
bun run start
```

VibeLang compiles `.vibe` sources to JavaScript with the native `vibelang`
compiler. Version 0.0.1 installs from npm.

> **For agents.** To set up VibeLang with a coding agent, give it this prompt:
>
> ```text
> Set up VibeLang in this project: read https://effeck.dev/setup.md and follow it.
> ```
>
> The agent detects the project's package manager, build, and editor, and
> follows the matching steps in [Set up VibeLang](../setup.md), which you can
> read too.

## Prerequisites

Install Node 26 or later and Bun 1.4 or later. The compiler is a native binary.
`vibelang` lists one package per platform as an optional dependency, and your
package manager installs only the one that matches your machine:

| Platform | Package | Tested for 0.0.1 |
| --- | --- | --- |
| macOS on Apple silicon | `vibelang-darwin-arm64` | Yes |
| macOS on Intel | `vibelang-darwin-x64` | No |
| Linux on x64 | `vibelang-linux-x64` | Yes |
| Linux on arm64 | `vibelang-linux-arm64` | No |
| Windows on x64 | `vibelang-win32-x64` | No |
| Windows on arm64 | `vibelang-win32-arm64` | No |

Keep optional dependencies enabled. If an install skips them, for example with
`npm install --omit=optional`, `vibelang` cannot find its binary and exits with an
error.

## Create a project

```sh
bun create vibelang@latest my-app
cd my-app
bun run start
bun run test
```

With npm, run `npm create vibelang@latest my-app`. Either command copies the
starter into `my-app`, pins the VibeLang packages to the version that created it,
and installs dependencies. Pass `--no-install` to skip installation, then run
`bun install` in the project before building.

`bun run start` builds the project and prints:

```out
Hello, VibeLang!
Recovered from EmptyName
```

See [Getting Started](../013-getting-started.md) for the starter's files and
[Running Effects](009-running-effects.md) for execution.

## Add VibeLang to an existing project

```sh
bun add vibelang-runtime@0.0.1 effect@4.0.5 @effect/platform-node@4.0.5
bun add -d vibelang@0.0.1 vibelang-vite@0.0.1 vitest@5.0.5 @effect/vitest@4.0.5
```

With npm, run `npm install --save-exact` for the first line and
`npm install --save-dev --save-exact` for the second.

| Package | Install as | Purpose |
| --- | --- | --- |
| `vibelang-runtime` | Dependency | The continuation kernel and helpers that compiled code imports |
| `effect`, `@effect/platform-node` | Dependency | Effect 4.0.5 and its Node platform |
| `vibelang` | Dev dependency | The compiler and language server, its native binary, and the VS Code extension |
| `vibelang-vite` | Dev dependency | Compiles `.vibe` modules for Vite and Vitest |
| `vitest`, `@effect/vitest` | Dev dependency | Vitest 5.0.5 and `@effect/vitest` 4.0.5 for tests |

`vibelang-runtime` requires `effect` 4.0.5 exactly and checks Effect's version
when it loads. Keep `effect`, `@effect/platform-node`, and `@effect/vitest` at
4.0.5, so the runtime and your program share one copy of `effect`. Keep
`vibelang`, `vibelang-runtime`, and `vibelang-vite` at the same version: they are
released together.

Add a `tsconfig.json` that selects `.vibe` sources and emits Node ESM to
`dist/`:

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

Add scripts to `package.json`, which must have `"type": "module"`:

```json
{
  "type": "module",
  "scripts": {
    "build": "vibelang",
    "start": "vibelang && node --enable-source-maps dist/main.js",
    "test": "vitest run"
  }
}
```

Test `.vibe` files through the Vite plugin:

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

The plugin compiles the project that `tsconfig.json` describes, so keep
`"rewriteRelativeImportExtensions": true` from the configuration above. Without
it, a `.ts` file that imports a `.vibe` module fails with TS5097. If your
`tsconfig.json` has `"files": []` and only lists `references`, as Vite's
templates do, pass the referenced configuration that includes your sources:
`vibelang({ project: "tsconfig.app.json" })`. [Vite and Vitest](../setup/vite.md)
covers the setup step by step.

To update, install the new version of all three VibeLang packages together:

```sh
bun add vibelang-runtime@latest
bun add -d vibelang@latest vibelang-vite@latest
```

VibeLang is v0, so a new version can change the language in breaking ways. Read
its release notes before updating.

## Configure your editor

From the project directory, after installing dependencies:

```sh
code --install-extension node_modules/vibelang/editor/vibelang.vsix
# Or, for Cursor:
cursor --install-extension node_modules/vibelang/editor/vibelang.vsix
```

Open the project directory. The extension starts the project's compiler as the
language server, `vibelang --lsp --stdio`, for diagnostics, hovers, and
completions. The extension is not on the VS Code Marketplace or Open VSX yet;
install it again from `node_modules` after you update `vibelang`.

Other editors run `vibelang --lsp --stdio` through their LSP client for `.vibe`
and `.ts` files. See [Devtools](012-devtools.md).

## Compiler commands

From a project, run:

```sh
bun run build
bun run dev
bun run test
bun x --no-install vibelang --noEmit
```

The starter's `tsconfig.json` selects `.vibe` files under `src/`, emits Node
ESM to `dist/`, enables strict checking and source maps, and prevents emit on
errors. Local source imports use `.vibe`; generated JavaScript imports use
`.js`. `vibelang` accepts TypeScript compiler flags such as `--noEmit`, `--watch`,
and `-b`.

## Coming from Effect

Install `vibelang` for `.vibe` compilation and `vibelang-runtime` alongside
`effect@4.0.5`. Write application logic in `.vibe` and keep ordinary
TypeScript integrations in `.ts`. The compiler checks both file types as one
program.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/installation)
