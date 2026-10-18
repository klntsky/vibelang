# Add Effeck to an existing project

Install Effeck's packages in a project that has a `package.json`, and check that the compiler runs. Then set up the build.

## 1. Check package.json

`package.json` must have `"type": "module"`. If it does not, ask the user before you add it: it changes how Node loads the project's `.js` files.

If `package.json` depends on a version of `effect` other than 4.0.5, stop and ask the user. The Effeck runtime requires `effect` 4.0.5 exactly.

## 2. Install the packages

With Bun, run:

```sh
bun add effeck-runtime@0.0.1 effect@4.0.5 @effect/platform-node@4.0.5
bun add -d effeck@0.0.1 effeck-vite@0.0.1 vitest@5.0.5 @effect/vitest@4.0.5
```

With npm, run:

```sh
npm install --save-exact effeck-runtime@0.0.1 effect@4.0.5 @effect/platform-node@4.0.5
npm install --save-dev --save-exact effeck@0.0.1 effeck-vite@0.0.1 vitest@5.0.5 @effect/vitest@4.0.5
```

| Package | Install as | Purpose |
| --- | --- | --- |
| `effeck-runtime` | Dependency | The continuation kernel and helpers that compiled code imports |
| `effect`, `@effect/platform-node` | Dependency | Effect 4.0.5 and its Node platform |
| `effeck` | Dev dependency | The compiler and language server, its native binary, and the VS Code extension |
| `effeck-vite` | Dev dependency | Compiles `.effeck` modules for Vite and Vitest |
| `vitest`, `@effect/vitest` | Dev dependency | Vitest 5.0.5 and `@effect/vitest` 4.0.5 for tests |

Keep these versions. `effeck-runtime` checks Effect's version when it loads, so keep `effect`, `@effect/platform-node`, and `@effect/vitest` at 4.0.5. `effeck`, `effeck-runtime`, and `effeck-vite` are released together; keep them at one version.

Keep optional dependencies enabled. `effeck` gets its native compiler from one optional dependency per platform, such as `effeck-darwin-arm64`. If an install skips them, for example with `npm install --omit=optional`, `effeck` cannot find its binary and exits with an error.

## 3. Check the compiler

```sh
npx effeck --version
```

The output is:

```out
Effeck 0.0.1 (TypeScript 7.1.0-dev)
```

## Next

Set up the build. For React, Vue, or Svelte with Vite, read [React](react.md), [Vue](vue.md), or [Svelte](svelte.md). For other Vite or Vitest projects, read [Vite and Vitest](vite.md). Without a bundler, read [Build with the effeck CLI](effeck-cli.md). For another bundler, step 3 of [Set up Effeck](../setup.md) says what to do.
