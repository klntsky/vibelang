# Svelte

Add Effeck to a Svelte 5 app that builds with Vite. A component runs an Effeck program with a `ManagedRuntime` and renders three states: loading, the program's result, and an error the program recovers from. Install the packages with [Add Effeck to an existing project](existing-project.md) first. Follow this page instead of [Vite and Vitest](vite.md): it covers the same plugin, tsconfig.json, Vitest, and build steps.

## 1. Install the test packages

With Bun, run:

```sh
bun add -d happy-dom@20.14.6 @testing-library/svelte@5.4.2
```

With npm, run:

```sh
npm install --save-dev --save-exact happy-dom@20.14.6 @testing-library/svelte@5.4.2
```

## 2. Configure tsconfig.json

Find the configuration that includes `src`. In an app from Vite's `svelte-ts` template, `tsconfig.json` has `"files": []` and lists `tsconfig.app.json` and `tsconfig.node.json` as `references`; the one that includes `src` is `tsconfig.app.json`. Make two changes there:

1. In `compilerOptions`, set `"rewriteRelativeImportExtensions": true`. Without it, `vite build` fails with TS5097 for each `.ts` file that imports an `.effeck` module. A configuration that sets `allowImportingTsExtensions` fails with TS5096 instead.
2. Add `"src/**/*.effeck"` to `include`. The template lists `.ts`, `.js`, and `.svelte` globs only. Without the `.effeck` glob, the compiler skips `.effeck` files that no other file imports, such as tests, and Vitest fails with `ENOENT`.

```json
{
  "include": ["src/**/*.ts", "src/**/*.js", "src/**/*.svelte", "src/**/*.effeck"]
}
```

## 3. Configure Vite and Vitest

Replace `vite.config.ts` with this file. It adds `effeck()` before `svelte()`, passes the configuration that includes `src` as `project`, and imports `defineConfig` from `vitest/config`, so the same file configures Vitest. `svelteTesting()` makes Vitest load Svelte's browser build and clean up after each test. Keep any other plugins the app has. If `tsconfig.json` itself includes `src`, call `effeck()` without the `project` option.

```ts
// TypeScript (.ts)
import { svelte } from "@sveltejs/vite-plugin-svelte"
import { svelteTesting } from "@testing-library/svelte/vite"
import effeck from "effeck-vite"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [effeck({ project: "tsconfig.app.json" }), svelte(), svelteTesting()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.effeck", "src/**/*.test.ts"]
  }
})
```

## 4. Add an Effeck module

Create `src/cart.effeck`. `quote` reserves a product and recovers from `OutOfStock`, and `quoteCart` runs two quotes concurrently with `await.all`:

```ts
// src/cart.effeck
export schema Product { sku: string; name: string; cents: number; stock: number }

export error OutOfStock { sku: string; available: number }

export service Inventory {
  reserve(sku: string, quantity: number): Product!OutOfStock
}

const catalog: ReadonlyArray<Product> = [
  { sku: "tea", name: "Green tea", cents: 400, stock: 5 },
  { sku: "coffee", name: "Coffee beans", cents: 1200, stock: 1 }
]

export const InventoryLive = Layer.succeed(Inventory, {
  reserve(sku, quantity) {
    const product = catalog.find((entry) => entry.sku === sku)
    const available = product?.stock ?? 0
    if (product === undefined || available < quantity) throw new OutOfStock({ sku, available })
    return product
  }
})

export function quote(sku: string, quantity: number) {
  try {
    const product = Inventory.reserve(sku, quantity)
    const total = (product.cents * quantity / 100).toFixed(2)
    return `${quantity} x ${product.name}: $${total}`
  } catch (error) {
    return `Only ${error.available} ${error.sku} left`
  }
}

export function quoteCart() {
  return await.all [quote("tea", 2), quote("coffee", 3)]
}
```

## 5. Run a program from a component

Create `src/runtime.ts`. One `ManagedRuntime` builds `InventoryLive` once and runs every program in the app:

```ts
// TypeScript (.ts)
// src/runtime.ts
import { ManagedRuntime } from "effect"
import { InventoryLive } from "./cart.effeck"

export const runtime = ManagedRuntime.make(InventoryLive)
```

Create `src/CartQuote.svelte`. Its `<script lang="ts">` block imports `./cart.effeck` by its source name. `runtime.runPromise(quoteCart())` provides `Inventory` and returns a `Promise`, which the `{#await}` block renders:

```svelte
<script lang="ts">
  import { runtime } from "./runtime.ts"
  import { quoteCart } from "./cart.effeck"

  const quotes = runtime.runPromise(quoteCart())
</script>

<main>
  <h1>Cart</h1>
  {#await quotes}
    <p>Loading…</p>
  {:then lines}
    {#each lines as line (line)}
      <p>{line}</p>
    {/each}
  {/await}
</main>
```

Render it from a component the app already has, such as `src/App.svelte`: add `import CartQuote from "./CartQuote.svelte"` to its script block and `<CartQuote />` to its markup. The page shows `Loading…`, then `2 x Green tea: $8.00` and `Only 1 coffee left`.

## 6. Test the module and the component

Create `src/cart.test.effeck`:

```ts
// src/cart.test.effeck
import { assert, it } from "@effect/vitest"
import { InventoryLive, Inventory, OutOfStock, quote, quoteCart } from "./cart.effeck"

it.effect("quotes a product in stock") {
  const line = Effect.provide(() => quote("tea", 2), InventoryLive)
  assert.strictEqual(line, "2 x Green tea: $8.00")
}

it.effect("fails a reservation with OutOfStock") {
  provide InventoryLive {
    try {
      Inventory.reserve("coffee", 3)
      assert.fail("expected OutOfStock")
    } catch (error) {
      assert.instanceOf(error, OutOfStock)
      assert.strictEqual(error.available, 1)
    }
  }
}

it.effect("recovers OutOfStock in quote") {
  const line = Effect.provide(() => quote("coffee", 3), InventoryLive)
  assert.strictEqual(line, "Only 1 coffee left")
}

it.effect("quotes the cart concurrently") {
  const lines = Effect.provide(() => quoteCart(), InventoryLive)
  assert.deepStrictEqual(lines, ["2 x Green tea: $8.00", "Only 1 coffee left"])
}
```

Create `src/CartQuote.test.ts`:

```ts
// TypeScript (.ts)
import { render, screen } from "@testing-library/svelte"
import { expect, it } from "vitest"
import CartQuote from "./CartQuote.svelte"

it("shows loading, then the quote and the recovered error", async () => {
  render(CartQuote)
  expect(screen.getByText("Loading…")).toBeDefined()
  expect(await screen.findByText("2 x Green tea: $8.00")).toBeDefined()
  expect(await screen.findByText("Only 1 coffee left")).toBeDefined()
})
```

If `package.json` has no `test` script, add `"test": "vitest run"`. Run `npx vitest run`. The output includes:

```out
 Test Files  2 passed (2)
      Tests  5 passed (5)
```

## 7. Type-check and build

No type checker covers a `.svelte` file that imports an `.effeck` module. svelte-check reads `.svelte` files with TypeScript 6, which cannot resolve `.effeck` modules, so the template's `check` script fails. svelte-check 4.7.6 reports TS2307 for each import of `./cart.effeck`, and then TS2345 for the `{#each}` block, whose `lines` it types as `unknown`. With `--output human`, it prints each file's absolute path; here they are shortened to `src/`:

```out
src/runtime.ts:4:31
Error: Cannot find module './cart.effeck' or its corresponding type declarations.

src/CartQuote.svelte:3:29
Error: Cannot find module './cart.effeck' or its corresponding type declarations. (ts)
```

The gap closes when Effeck's TypeScript 7 content mapper ships and these checkers can load `.effeck` files. Until then, type-check with `effeck -b` instead. It builds the same project references and checks `.effeck` and `.ts` files as one program; it does not check the script blocks of `.svelte` files. Keep those blocks thin: import a program, run it, and render its result, as `CartQuote.svelte` does. The `.effeck` tests and the component test cover the rest. In `package.json`, change the `check` script:

```json
{
  "scripts": {
    "check": "effeck -b"
  }
}
```

Run `npm run check`, or `bun run check`. It exits with status 0. Then run `npm run build`, or `bun run build`. It exits with status 0, and its last line starts with `✓ built in`. Tell the user which files you added and changed, and that `svelte-check` no longer runs in the `check` script.

## Next

Set up the editor: [VS Code and Cursor](vscode.md), [Neovim](neovim.md), [Zed](zed.md), or [other editors](other-editors.md). Then verify the setup with step 4 of [Set up Effeck](../setup.md).
