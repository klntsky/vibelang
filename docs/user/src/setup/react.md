# React

Add Effeck to a React app that builds with Vite. A component runs an Effeck program with a `ManagedRuntime` and renders three states: loading, the program's result, and an error the program recovers from. Install the packages with [Add Effeck to an existing project](existing-project.md) first. Follow this page instead of [Vite and Vitest](vite.md): it covers the same plugin, tsconfig.json, Vitest, and build steps.

## 1. Install the test packages

With Bun, run:

```sh
bun add -d happy-dom@20.14.6 @testing-library/react@16.3.3 @testing-library/dom@10.4.2
```

With npm, run:

```sh
npm install --save-dev --save-exact happy-dom@20.14.6 @testing-library/react@16.3.3 @testing-library/dom@10.4.2
```

## 2. Configure tsconfig.json

Find the configuration that includes `src`. In an app from Vite's `react-ts` template, `tsconfig.json` has `"files": []` and lists `tsconfig.app.json` and `tsconfig.node.json` as `references`; the one that includes `src` is `tsconfig.app.json`. Make two checks there:

1. `compilerOptions` sets `"rewriteRelativeImportExtensions": true`. Without it, `vite build` fails with TS5096, because the template sets `allowImportingTsExtensions` and the plugin emits JavaScript. A configuration without `allowImportingTsExtensions` fails with TS5097 instead.
2. `include` selects the `.effeck` files. The template's `"include": ["src"]` does.

## 3. Configure Vite and Vitest

Replace `vite.config.ts` with this file. It adds `effeck()` before `react()`, passes the configuration that includes `src` as `project`, and imports `defineConfig` from `vitest/config`, so the same file configures Vitest. Keep any other plugins the app has. If `tsconfig.json` itself includes `src`, call `effeck()` without the `project` option.

```ts
// TypeScript (.ts)
import react from "@vitejs/plugin-react"
import effeck from "effeck-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [effeck({ project: "tsconfig.app.json" }), react()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.effeck", "src/**/*.test.tsx"]
  }
})
```

## 4. Add an Effeck module

Create `src/users.effeck`. It declares a `Users` service and its layer, and `greet` recovers from `UserNotFound`, so `greet` cannot fail:

```effeck
// src/users.effeck
export schema User { id: string; name: string }

export error UserNotFound { id: string }

export service Users {
  find(id: string): User!UserNotFound
}

const directory: ReadonlyArray<User> = [
  { id: "42", name: "Ada Lovelace" },
  { id: "7", name: "Grace Hopper" }
]

export const UsersLive = Layer.succeed(Users, {
  find(id) {
    const user = directory.find((entry) => entry.id === id)
    if (user === undefined) throw new UserNotFound({ id })
    return user
  }
})

export function greet(id: string) {
  try {
    const user = Users.find(id)
    return `Welcome, ${user.name}`
  } catch (error) {
    return `No user ${error.id}`
  }
}
```

## 5. Run a program from a component

Create `src/runtime.ts`. One `ManagedRuntime` builds `UsersLive` once and runs every program in the app:

```ts
// TypeScript (.ts)
// src/runtime.ts
import { ManagedRuntime } from "effect"
import { UsersLive } from "./users.effeck"

export const runtime = ManagedRuntime.make(UsersLive)
```

Create `src/Greeting.tsx`. To TypeScript, `greet(id)` returns `Effect<string, never, Users>`; `runtime.runPromise` provides `Users` and returns a `Promise<string>`. The `AbortSignal` interrupts the program when the component unmounts or `id` changes:

```tsx
// src/Greeting.tsx
import { useEffect, useState } from "react"
import { runtime } from "./runtime.ts"
import { greet } from "./users.effeck"

export function Greeting({ id }: { readonly id: string }) {
  const [text, setText] = useState<string>()
  useEffect(() => {
    const controller = new AbortController()
    runtime.runPromise(greet(id), { signal: controller.signal }).then(setText, (cause) => {
      if (!controller.signal.aborted) throw cause
    })
    return () => controller.abort()
  }, [id])
  return <p>{text ?? "Loading…"}</p>
}
```

Render it from a component the app already has, such as `App` in `src/App.tsx`: import it with `import { Greeting } from "./Greeting.tsx"` and add `<Greeting id="42" />` and `<Greeting id="404" />`. The page shows `Loading…` twice, then `Welcome, Ada Lovelace` and `No user 404`.

## 6. Test the module and the component

Create `src/users.test.effeck`:

```effeck
// src/users.test.effeck
import { assert, it } from "@effect/vitest"
import { greet, UserNotFound, Users, UsersLive } from "./users.effeck"

it.effect("greets a known user") {
  const message = Effect.provide(() => greet("42"), UsersLive)
  assert.strictEqual(message, "Welcome, Ada Lovelace")
}

it.effect("fails a lookup with UserNotFound") {
  provide UsersLive {
    try {
      Users.find("404")
      assert.fail("expected UserNotFound")
    } catch (error) {
      assert.instanceOf(error, UserNotFound)
      assert.strictEqual(error.id, "404")
    }
  }
}

it.effect("recovers UserNotFound in greet") {
  const message = Effect.provide(() => greet("404"), UsersLive)
  assert.strictEqual(message, "No user 404")
}
```

Create `src/Greeting.test.tsx`:

```tsx
// src/Greeting.test.tsx
import { render, screen } from "@testing-library/react"
import { expect, it } from "vitest"
import { Greeting } from "./Greeting.tsx"

it("shows loading, then the user and the recovered error", async () => {
  render(<><Greeting id="42" /><Greeting id="404" /></>)
  expect(screen.getAllByText("Loading…")).toHaveLength(2)
  expect(await screen.findByText("Welcome, Ada Lovelace")).toBeDefined()
  expect(await screen.findByText("No user 404")).toBeDefined()
})
```

If `package.json` has no `test` script, add `"test": "vitest run"`. Run `npx vitest run`. The output includes:

```out
 Test Files  2 passed (2)
      Tests  4 passed (4)
```

## 7. Type-check and build

TypeScript cannot read `.effeck` files, so the template's `tsc -b` fails with TS2307: `Cannot find module './users.effeck'`. `effeck -b` replaces it. It builds the same project references and checks `.effeck`, `.ts`, and `.tsx` files as one program. In `package.json`, change the `build` script:

```json
{
  "scripts": {
    "build": "effeck -b && vite build"
  }
}
```

Run `npx effeck -b`. It prints nothing and exits with status 0. Then run `npm run build`, or `bun run build`. It exits with status 0, and its last line starts with `✓ built in`. Tell the user which files you added and changed.

## Next

Set up the editor: [VS Code and Cursor](vscode.md), [Neovim](neovim.md), [Zed](zed.md), or [other editors](other-editors.md). Then verify the setup with step 4 of [Set up Effeck](../setup.md).
