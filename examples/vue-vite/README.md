# Vue and Vite

A Vue single-file component renders a todo board that an Effeck program
builds. `src/todos.effeck` declares a `Todo` schema, a `TodoNotFound` error,
and a `Todos` service. The `TodosLive` layer keeps the list in a `Map` that its
setup creates once, when the layer builds. `completeTodo` recovers
`TodoNotFound` with `try`/`catch`, and `loadBoard` declares the contract
`Board!!Todos`, so the compiler rejects an error that escapes it.
`src/App.vue` runs `loadBoard()` with a `ManagedRuntime` built from
`TodosLive` and renders `Loading…` until the program completes.

## Run the app

Run `sh scripts/setup.sh` from the repository root first. It builds the
compiler and packs the runtime tarball that `package.json` installs. Then,
from this directory:

```sh
bun install
bun run dev
```

Open the URL that Vite prints. The page shows `Loading…`, then:

```text
Todos
[x] Declare the schema
[x] Write the service
[ ] Render the board
Completed: Write the service
No todo #99
```

## Test the app

```sh
bun run test
```

The command stops at the first failure. It runs three steps:

1. `effeck -p tsconfig.json --noEmit` type-checks `src/todos.effeck` and every
   `.ts` file as one program. It does not check `App.vue`; see
   [Type checking `.vue` files](#type-checking-vue-files).
2. `vitest run` runs two files. `src/todos.test.effeck` checks
   `completeTodo`, the `TodoNotFound` failure, its recovery, and the board
   with `@effect/vitest`. `src/App.test.ts` mounts `App` in happy-dom with
   `@vue/test-utils`, checks the loading text, and waits for the board.
3. `vite build` writes the production bundle to `dist/`.

The output ends with:

```text
 Test Files  2 passed (2)
      Tests  5 passed (5)
...
✓ built in 205ms
```

## How the pieces connect

- `vite.config.ts` lists `effeck()` from `effeck-vite` before `vue()`. The
  plugin compiles the project with the `effeck` binary, and Vite serves each
  `.effeck` module as JavaScript. Vitest reads the same file.
- `App.vue` uses `<script setup lang="ts">` and imports `./todos.effeck` by
  its source name. `.ts` files import it the same way.
  `"rewriteRelativeImportExtensions": true` in `tsconfig.json` permits that
  import. Without it, the compiler reports TS5097.
- `src/env.d.ts` declares `*.vue` modules, so `effeck` can check `main.ts` and
  `App.test.ts`, which import `App.vue`.

## Type checking `.vue` files

No type checker covers a `.vue` file that imports an `.effeck` module.
`effeck` does not read `.vue` files. vue-tsc reads them with TypeScript 6,
which cannot resolve `.effeck` modules. vue-tsc 3.3.13 reports this for the
component and for `runtime.ts`:

```text
src/App.vue(4,39): error TS2307: Cannot find module './todos.effeck' or its corresponding type declarations.
src/runtime.ts(2,27): error TS2307: Cannot find module './todos.effeck' or its corresponding type declarations.
```

vue-tsc also reports TS2876 for each `./App.vue` import. With
`rewriteRelativeImportExtensions` set, TypeScript rejects an import of a
source file whose extension it cannot rewrite, and vue-tsc makes `App.vue` a
source file. `effeck` reports neither error: it sees `App.vue` only through
`src/env.d.ts`.

The project therefore has no vue-tsc step. `App.vue` keeps its script block
thin: it imports `loadBoard` and the `Board` type and runs one program.
`todos.effeck`, its tests, and the component test carry the checks.

## Coming from Effect

`completeTodo` in Effect TypeScript:

```ts
// TypeScript (.ts)
const completeTodo = Effect.fnUntraced(function* (id: number) {
  const todos = yield* Todos
  return yield* todos.complete(id).pipe(
    Effect.map((todo) => `Completed: ${todo.title}`),
    Effect.catchTag("TodoNotFound", (error) => Effect.succeed(`No todo #${error.id}`))
  )
})
```
