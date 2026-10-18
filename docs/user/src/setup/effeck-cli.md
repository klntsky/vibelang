# Build with the effeck CLI

`effeck` replaces `tsc`. It checks `.effeck` and `.ts` files as one program, emits JavaScript, and accepts TypeScript's compiler flags. Install the packages with [Add Effeck to an existing project](existing-project.md) first.

## 1. Configure tsconfig.json

If the project has no `tsconfig.json`, create this one. It selects `.effeck` files under `src/` and emits Node ESM to `dist/`:

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

If the project has a `tsconfig.json`, keep it, and check that `include` selects `.effeck` files. A directory entry such as `"src"` selects them. A glob such as `"src/**/*.ts"` does not; add `"src/**/*.effeck"` beside it.

## 2. Replace tsc in the scripts

In `package.json`, replace `tsc` with `effeck` in every script. If the project has no build script, add these:

```json
{
  "scripts": {
    "build": "effeck",
    "start": "effeck && node --enable-source-maps dist/main.js",
    "test": "vitest run"
  }
}
```

`effeck` with no arguments checks and emits the project, `effeck --noEmit` only checks it, `effeck --watch` rebuilds on every save, and `effeck -b` builds project references.

## 3. Compile a first module

Create `src/hello.effeck`, in a directory that `include` selects:

```ts
// src/hello.effeck
export function hello(name: string) {
  return `Hello, ${name}!`
}
```

Run `npx effeck`. It prints nothing, exits with status 0, and writes `hello.js` to the `outDir`, `dist/` in the configuration above.

In `.effeck` files, import local modules by their source name: `import { hello } from "./hello.effeck"`. In `.ts` files, import `./hello.js`, which resolves to `hello.effeck`.

## 4. Test with Vitest

Create `vitest.config.ts`, which compiles `.effeck` tests through `effeck-vite`:

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

Create `src/hello.test.effeck`:

```ts
// src/hello.test.effeck
import { assert, it } from "@effect/vitest"
import { hello } from "./hello.effeck"

it.effect("says hello") {
  assert.strictEqual(hello("Effeck"), "Hello, Effeck!")
}
```

Run `npx vitest run`. The output includes:

```out
 Test Files  1 passed (1)
      Tests  1 passed (1)
```

Tell the user that you added `src/hello.effeck` and `src/hello.test.effeck`. They can keep them as a starting point or delete them.

## Next

Set up the editor: [VS Code and Cursor](vscode.md), [Neovim](neovim.md), [Zed](zed.md), or [other editors](other-editors.md). Then verify the setup with step 4 of [Set up Effeck](../setup.md).
