# Installation

```sh
bun create effeck my-app
cd my-app
bun run start
```

Effeck compiles `.effeck` sources to JavaScript with the native `effeck`
compiler. Version 0.0.1 installs from npm.

> **For agents.** To set up Effeck with a coding agent, give it this prompt:
>
> ```text
> Set up Effeck in this project: read https://effeck.dev/setup.md and follow it.
> ```
>
> The agent detects the project's package manager, build, and editor, and
> follows the matching steps in [Set up Effeck](../setup.md), which you can
> read too.

## Prerequisites

Install Node 26 or later and Bun 1.4 or later. The compiler is a native binary.
`effeck` lists one package per platform as an optional dependency, and your
package manager installs only the one that matches your machine:

| Platform | Package | Tested for 0.0.1 |
| --- | --- | --- |
| macOS on Apple silicon | `effeck-darwin-arm64` | Yes |
| macOS on Intel | `effeck-darwin-x64` | No |
| Linux on x64 | `effeck-linux-x64` | Yes |
| Linux on arm64 | `effeck-linux-arm64` | No |
| Windows on x64 | `effeck-win32-x64` | No |
| Windows on arm64 | `effeck-win32-arm64` | No |

Keep optional dependencies enabled. If an install skips them, for example with
`npm install --omit=optional`, `effeck` cannot find its binary and exits with an
error.

## Create a project

```sh
bun create effeck my-app
cd my-app
bun run start
bun run test
```

With npm, run `npm create effeck@latest my-app`. Either command copies the
starter into `my-app`, pins the Effeck packages to the version that created it,
and installs dependencies. Pass `--no-install` to skip installation, then run
`bun install` in the project before building.

`bun run start` builds the project and prints:

```out
Hello, Effeck!
Recovered from EmptyName
```

See [Getting Started](../013-getting-started.md) for the starter's files and
[Running Effects](009-running-effects.md) for execution.

## Add Effeck to an existing project

```sh
bun add effeck-runtime@0.0.1 effect@4.0.5 @effect/platform-node@4.0.5
bun add -d effeck@0.0.1 effeck-vite@0.0.1 vitest@5.0.5 @effect/vitest@4.0.5
```

With npm, run `npm install --save-exact` for the first line and
`npm install --save-dev --save-exact` for the second.

| Package | Install as | Purpose |
| --- | --- | --- |
| `effeck-runtime` | Dependency | The continuation kernel and helpers that compiled code imports |
| `effect`, `@effect/platform-node` | Dependency | Effect 4.0.5 and its Node platform |
| `effeck` | Dev dependency | The compiler and language server, its native binary, and the VS Code extension |
| `effeck-vite` | Dev dependency | Compiles `.effeck` modules for Vite and Vitest |
| `vitest`, `@effect/vitest` | Dev dependency | Vitest 5.0.5 and `@effect/vitest` 4.0.5 for tests |

`effeck-runtime` requires `effect` 4.0.5 exactly and checks Effect's version
when it loads. Keep `effect`, `@effect/platform-node`, and `@effect/vitest` at
4.0.5, so the runtime and your program share one copy of `effect`. Keep
`effeck`, `effeck-runtime`, and `effeck-vite` at the same version: they are
released together.

Add a `tsconfig.json` that selects `.effeck` sources and emits Node ESM to
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
  "include": ["src/**/*.effeck"]
}
```

Add scripts to `package.json`, which must have `"type": "module"`:

```json
{
  "type": "module",
  "scripts": {
    "build": "effeck",
    "start": "effeck && node --enable-source-maps dist/main.js",
    "test": "vitest run"
  }
}
```

Test `.effeck` files through the Vite plugin:

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

The plugin compiles the project that `tsconfig.json` describes, so keep
`"rewriteRelativeImportExtensions": true` from the configuration above. Without
it, a `.ts` file that imports an `.effeck` module fails with TS5097. If your
`tsconfig.json` has `"files": []` and only lists `references`, as Vite's
templates do, pass the referenced configuration that includes your sources:
`effeck({ project: "tsconfig.app.json" })`. [Vite and Vitest](../setup/vite.md)
covers the setup step by step.

To update, install the new version of all three Effeck packages together:

```sh
bun add effeck-runtime@latest
bun add -d effeck@latest effeck-vite@latest
```

Effeck is v0, so a new version can change the language in breaking ways. Read
its release notes before updating.

## Configure your editor

From the project directory, after installing dependencies:

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
# Or, for Cursor:
cursor --install-extension node_modules/effeck/editor/effeck.vsix
```

Open the project directory. The extension starts the project's compiler as the
language server, `effeck --lsp --stdio`, for diagnostics, hovers, and
completions. The extension is not on the VS Code Marketplace or Open VSX yet;
install it again from `node_modules` after you update `effeck`.

Other editors run `effeck --lsp --stdio` through their LSP client for `.effeck`
and `.ts` files. See [Devtools](012-devtools.md).

## Compiler commands

From a project, run:

```sh
bun run build
bun run dev
bun run test
bun x --no-install effeck --noEmit
```

The starter's `tsconfig.json` selects `.effeck` files under `src/`, emits Node
ESM to `dist/`, enables strict checking and source maps, and prevents emit on
errors. Local source imports use `.effeck`; generated JavaScript imports use
`.js`. `effeck` accepts TypeScript compiler flags such as `--noEmit`, `--watch`,
and `-b`.

## Coming from Effect

Install `effeck` for `.effeck` compilation and `effeck-runtime` alongside
`effect@4.0.5`. Write application logic in `.effeck` and keep ordinary
TypeScript integrations in `.ts`. The compiler checks both file types as one
program.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/installation)
