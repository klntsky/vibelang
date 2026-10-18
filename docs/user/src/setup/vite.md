# Vite and Vitest

`effeck-vite` compiles `.effeck` modules for Vite and Vitest with the project's `effeck` compiler. It supports Vite 7 and 8. It serves JavaScript and source maps, and reports compiler errors in the terminal and in Vite's overlay. Install the packages with [Add Effeck to an existing project](existing-project.md) first.

## 1. Add the plugin to Vite

Add `effeck()` to the `plugins` array in `vite.config.ts`, beside the plugins the project already has:

```ts
// TypeScript (.ts)
// vite.config.ts
import { defineConfig } from "vite"
import effeck from "effeck-vite"

export default defineConfig({ plugins: [effeck()] })
```

## 2. Configure tsconfig.json

The plugin compiles the project that `tsconfig.json` describes, with its options and diagnostics. Check three things:

1. `compilerOptions` sets `"rewriteRelativeImportExtensions": true`. Without it, a `.ts` file that imports `./greet.effeck` fails with TS5097. `"allowImportingTsExtensions": true` alone fails with TS5096, because the plugin emits JavaScript.
2. `include` selects the `.effeck` files, and they are under Vite's root.
3. If `tsconfig.json` has `"files": []` and only lists `references`, as Vite's templates do, pass the referenced configuration that includes `src` to the plugin, for example `effeck({ project: "tsconfig.app.json" })`.

## 3. Add a first module

Create `src/greet.effeck`:

```ts
// src/greet.effeck
export function greet(name: string) {
  return `Hello, ${name}!`
}
```

Import it from `.ts` or `.effeck` modules by its source name, `./greet.effeck`. TypeScript code sees `greet` as a function that returns an Effect. For example, a `.ts` module can run it with one of Effect's runners:

```ts
// TypeScript (.ts)
// src/use-greet.ts
import { Effect } from "effect"
import { greet } from "./greet.effeck"

export const message = Effect.runSync(greet("Vite"))
```

## 4. Test with Vitest

If the project has no Vitest configuration, create `vitest.config.ts`. If it has one, add the plugin and the `include` pattern to it instead.

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

Create `src/greet.test.effeck`:

```ts
// src/greet.test.effeck
import { assert, it } from "@effect/vitest"
import { greet } from "./greet.effeck"

it.effect("greets") {
  assert.strictEqual(greet("Vite"), "Hello, Vite!")
}
```

Run `npx vitest run`. Tests need no prior build. The output includes:

```out
 Test Files  1 passed (1)
      Tests  1 passed (1)
```

## 5. Build

Run `npx vite build`. It exits with status 0, and its last line starts with `✓ built in`. Tell the user which files you added.

## Next

Set up the editor: [VS Code and Cursor](vscode.md), [Neovim](neovim.md), [Zed](zed.md), or [other editors](other-editors.md). Then verify the setup with step 4 of [Set up Effeck](../setup.md).
