# Build with the vibelang CLI

`vibelang` replaces `tsc`. It checks `.vibe` and `.ts` files as one program, emits JavaScript, and accepts TypeScript's compiler flags. Install the packages with [Add VibeLang to an existing project](existing-project.md) first.

## 1. Configure tsconfig.json

If the project has no `tsconfig.json`, create this one. It selects `.vibe` files under `src/` and emits Node ESM to `dist/`:

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

If the project has a `tsconfig.json`, keep it, and check that `include` selects `.vibe` files. A directory entry such as `"src"` selects them. A glob such as `"src/**/*.ts"` does not; add `"src/**/*.vibe"` beside it.

## 2. Replace tsc in the scripts

In `package.json`, replace `tsc` with `vibelang` in every script. If the project has no build script, add these:

```json
{
  "scripts": {
    "build": "vibelang",
    "start": "vibelang && node --enable-source-maps dist/main.js",
    "test": "vitest run"
  }
}
```

`vibelang` with no arguments checks and emits the project, `vibelang --noEmit` only checks it, `vibelang --watch` rebuilds on every save, and `vibelang -b` builds project references.

## 3. Compile a first module

Create `src/hello.vibe`, in a directory that `include` selects:

```vibe
// src/hello.vibe
// run: Console.log(hello("VibeLang"))
export function hello(name: string) {
  return `Hello, ${name}!`
}
// Output: Hello, VibeLang!
```

Run `npx vibelang`. It prints nothing, exits with status 0, and writes `hello.js` to the `outDir`, `dist/` in the configuration above.

In `.vibe` files, import local modules by their source name: `import { hello } from "./hello.vibe"`. In `.ts` files, import `./hello.js`, which resolves to `hello.vibe`.

## 4. Add an entry point

The `start` script runs `dist/main.js`, so create `src/main.vibe`:

```vibe
// src/main.vibe
import { Console } from "effect"
import { hello } from "./hello.vibe"

export function main() {
  Console.log(hello("VibeLang"))
}
// Output: Hello, VibeLang!
```

Run `npm run start`, or `bun run start`. The output ends with:

```out
Hello, VibeLang!
```

The compiler runs the exported `main` when Node executes the file directly. Importing the module does not run it.

## 5. Test with Vitest

Create `vitest.config.ts`, which compiles `.vibe` tests through `vibelang-vite`:

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

Create `src/hello.test.vibe`:

```vibe
// src/hello.test.vibe
import { assert, it } from "@effect/vitest"
import { hello } from "./hello.vibe"

it.effect("says hello") {
  assert.strictEqual(hello("VibeLang"), "Hello, VibeLang!")
}
```

Run `npx vitest run`. The output includes:

```out
 Test Files  1 passed (1)
      Tests  1 passed (1)
```

Tell the user that you added `src/hello.vibe`, `src/main.vibe`, and `src/hello.test.vibe`. They can keep them as a starting point or delete them.

## Next

Set up the editor: [VS Code and Cursor](vscode.md), [Neovim](neovim.md), [Zed](zed.md), or [other editors](other-editors.md). Then verify the setup with step 4 of [Set up VibeLang](../setup.md).
