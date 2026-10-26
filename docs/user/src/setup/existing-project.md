# Add VibeLang to an existing project

Install VibeLang's packages in a project with a `package.json` and check that the compiler runs; then set up the build.

## 1. Check package.json

`package.json` must have `"type": "module"`. If not, ask the user before adding it: it changes how Node loads the project's `.js` files.

The VibeLang runtime requires `effect` 4.0.5 exactly. The runtime depends on Effect internals and checks the exact version at load. Effect 4.0.6 changed how a cleanup failure combines with a body failure in a way VibeLang's failure rules do not accept yet, so projects on 4.0.6 must use 4.0.5 for now. If `package.json` depends on another `effect` version, stop and ask the user.

## 2. Install the packages

Before installing, add this transitive pin to `package.json` (npm and Bun both honor `overrides`):

```json
{
  "overrides": {
    "@effect/platform-node-shared": "4.0.5"
  }
}
```

This keeps the platform package from selecting a newer patch with a different Effect peer requirement.

With Bun, run:

```sh
bun add vibelang-runtime@0.0.9 effect@4.0.5 @effect/platform-node@4.0.5
bun add -d vibelang@0.0.9 vibelang-vite@0.0.9 vitest@5.0.5 @effect/vitest@4.0.5
```

With npm, run:

```sh
npm install --save-exact vibelang-runtime@0.0.9 effect@4.0.5 @effect/platform-node@4.0.5
npm install --save-dev --save-exact vibelang@0.0.9 vibelang-vite@0.0.9 vitest@5.0.5 @effect/vitest@4.0.5
```

| Package | Install as | Purpose |
| --- | --- | --- |
| `vibelang-runtime` | Dependency | The continuation kernel and helpers that compiled code imports |
| `effect`, `@effect/platform-node` | Dependency | Effect 4.0.5 and its Node platform |
| `vibelang` | Dev dependency | The compiler and language server, its native binary, and the VS Code extension |
| `vibelang-vite` | Dev dependency | Compiles `.vibe` modules for Vite and Vitest |
| `vitest`, `@effect/vitest` | Dev dependency | Vitest 5.0.5 and `@effect/vitest` 4.0.5 for tests |

Keep `effect`, `@effect/platform-node`, and `@effect/vitest` at 4.0.5. `vibelang`, `vibelang-runtime`, and `vibelang-vite` release together; keep them at one version.

Keep optional dependencies enabled: `vibelang` gets its native compiler from one optional dependency per platform, such as `vibelang-darwin-arm64`. If an install skips them, as `npm install --omit=optional` does, `vibelang` cannot find its binary and exits with an error.

VibeLang uses TypeScript 7, which rejects removed options such as `moduleResolution: "node"` / `"node10"` and `baseUrl` (TS5108 / TS5102); update an older tsconfig using the [TypeScript 7 migration notes](https://github.com/microsoft/typescript-go/blob/main/CHANGES.md).

## 3. Check the compiler

```sh
npx vibelang --version
```

The output is:

```out
VibeLang 0.0.9 (TypeScript 7.1.0-dev)
```

## Next

Set up the build: [React](react.md), [Vue](vue.md), or [Svelte](svelte.md) with Vite; [Vite and Vitest](vite.md) for other Vite or Vitest projects; [Build with the vibelang CLI](vibelang-cli.md) without a bundler. For another bundler, see step 3 of [Set up VibeLang](../setup.md).
