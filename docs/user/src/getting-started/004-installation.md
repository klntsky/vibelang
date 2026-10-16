# Installation

EffectScript targets Effect 4.0.5 and compiles to TypeScript for JavaScript runtimes. Install the library dependencies separately from the language compiler; a TypeScript runtime alone does not interpret `.es` files.

## Runtime prerequisites

```ts
// hello.es
export function hello() {
  Effect.log("EffectScript is ready")
  return "ready"
}
// Effect<string, never, never>
```

Use a runtime supported by `effect@4.0.5` and its platform packages. The language's runtime evidence was gathered on Node.js 26.10 and Bun 1.4.1. The compiler uses TypeScript 7.1's checker and checks `.ts` and `.es` files in one program.

`@effectscript/compiler` installs the native `esc` binary; `@effectscript/runtime` supplies the runtime helpers used by generated code. v0 requires Effect 4.0.5 exactly, and the runtime rejects another version when it loads.

## Add dependencies manually

```sh
mkdir hello-effectscript
cd hello-effectscript
npm init -y
npm install effect@4.0.5 @effectscript/runtime
npm install --save-dev @effectscript/compiler
npx esc --init
```

Set `"type": "module"` in `package.json` and `"strict": true` in `tsconfig.json`. Enable `declaration` and `sourceMap` to emit declarations and source maps. Keep the package lockfile so every developer uses the same versions. Other package managers install the same packages with their corresponding commands.

```sh
npx esc --noEmit
npx esc
npx esc --watch
```

These commands check, check and emit, and rebuild on changes, respectively. `esc` accepts every `tsc` flag, including `-b` for project builds; `esc --lsp --stdio` serves editors and `esc --api` serves build tools.

## Set up a JavaScript runtime

```ts
// main.es
import { NodeRuntime } from "@effect/platform-node"
import { hello } from "./hello.es"

NodeRuntime.runMain(() => hello())
```

For Node applications, add an Effect 4 compatible `@effect/platform-node` release. Bun and browser applications use the matching `@effect/platform-bun` and `@effect/platform-browser` integrations when they need platform services. Keep platform packages compatible with the pinned core library.

After the compiler emits the application, run the generated JavaScript entry with Node or Bun. Commands such as `node main.es` do not supply the language's compilation step. `.es` imports name source files, such as `./hello.es`; emission rewrites that specifier to `./hello.js`.

## Integrate Vite and React

```sh
npm create vite@latest effectscript-ui -- --template react-ts
cd effectscript-ui
npm install
npm install effect@4.0.5 @effectscript/runtime
npm install --save-dev @effectscript/compiler @effectscript/vite
```

Add the plugin to Vite's configuration:

```ts
// TypeScript (.ts)
// vite.config.ts
import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import effectscript from "@effectscript/vite"

export default defineConfig({
  plugins: [react(), effectscript()]
})
```

Keep React components in `.tsx`, and place application operations in `.es` modules. For example, export this operation:

```ts
// increment.es
export function increment(current: number) {
  return current + 1
}
```

Then call its emitted module from a React component:

```ts
// TypeScript (.ts)
// Counter.tsx
import { useState } from "react"
import { Effect } from "effect"
import { increment } from "./increment.js"

export function Counter() {
  const [count, setCount] = useState(0)
  return <button onClick={() => {
    const next = Effect.runSync(increment(count))
    setCount(next)
  }}>Count: {count}</button>
}
```

The compiler resolves TypeScript's `./increment.js` import to `increment.es`. `@effectscript/vite` compiles requested `.es` modules through one `esc --api` process, serves JavaScript with source maps, and reports compiler errors in Vite's overlay. Hot reload also invalidates importers when exported contracts, control classes, or plain JavaScript variants change.

Use `runSync` only for work known to remain synchronous. For operations that pause or need shared services, use a [managed runtime](../036-runtime.md) at the framework boundary and dispose it when its owner shuts down.

## Coming from Effect

Install `@effectscript/compiler` for `.es` compilation and `@effectscript/runtime` alongside `effect@4.0.5`. Use `esc` in place of `tsc` for a program that includes `.es` files, and add `@effectscript/vite` for Vite applications.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/installation)
