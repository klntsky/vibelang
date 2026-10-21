# vibelang

> **Warning:** VibeLang is v0. The language will change in major, breaking ways. It is not suitable for production use.

The VibeLang compiler and language server. VibeLang is a readable syntax for
Effect that compiles to TypeScript: `vibelang` checks `.vibe` and `.ts` files
together and emits JavaScript that runs on Effect 4. Documentation:
[vibelang.sh](https://vibelang.sh).

## Start a project

```sh
bun create vibelang@latest my-app
# or
npm create vibelang@latest my-app
```

## Add VibeLang to an existing project

```sh
npm install effect@4.0.5 vibelang-runtime
npm install --save-dev vibelang
```

Generated JavaScript imports `effect` and `vibelang-runtime`, so both are runtime
dependencies. Point `tsconfig.json` at your `.vibe` sources:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "rootDir": "src",
    "outDir": "dist",
    "rewriteRelativeImportExtensions": true
  },
  "include": ["src/**/*.vibe"]
}
```

| Command | What it does |
| --- | --- |
| `npx vibelang` | Check and compile the project in `tsconfig.json`, like `tsc` |
| `npx vibelang --watch` | Recompile on every save |
| `npx vibelang --lsp --stdio` | Run the language server for an editor |
| `npx vibelang --version` | Print the VibeLang and TypeScript versions |

For tests with Vitest, add [`vibelang-vite`](https://www.npmjs.com/package/vibelang-vite).

## Run .vibe modules in Node

```sh
node --import vibelang/register src/main.ts
```

`vibelang/register` lets Node import `.vibe` modules from source, so a
TypeScript entry point, a script, or a test that spawns `node` needs no
`dist/`. The first `.vibe` import compiles the whole project in the nearest
`tsconfig.json` above the module, once per process; every `.vibe` module of that
project comes from that compile. Set `VIBELANG_PROJECT` to a tsconfig path to
choose another project. A module keeps its source URL, so a `.ts` file imported
by both a TypeScript and a `.vibe` module loads once. Diagnostics that stop the
compile are thrown at the import. Add `--enable-source-maps` for stack traces
with `.vibe` lines. It needs Node 22.15 or later, for `module.registerHooks`.

## Compile projects from a tool

`vibelang/project` is the whole-project compile that `vibelang-vite`,
`vibelang-unplugin`, and `vibelang/register` share.
`projects({ root, project, compiler })` serves each `.vibe` module from one
compile per tsconfig: a module inside `root` belongs to `project`; one outside
it, such as the source of a workspace package another package imports, belongs
to the nearest `tsconfig.json` above it. `load(file)` returns the module's
JavaScript, its source map, and its project's diagnostics; `invalidate()` makes
every project compile again on its next load.

## Editor support

This package ships the VS Code extension. In a project with `vibelang` installed:

```sh
code --install-extension node_modules/vibelang/editor/vibelang.vsix
cursor --install-extension node_modules/vibelang/editor/vibelang.vsix
```

The extension starts the project's own `vibelang` as the language server.

## Agent skill

The package ships an agent skill for writing VibeLang, `node_modules/vibelang/skills/vibelang/SKILL.md`: every form, rule and idiom of this version, in about 4,000 tokens. Point an agent's instructions at it (D292).

## Format .vibe files with dprint

The package ships a [dprint](https://dprint.dev) plugin for `.vibe` files. Add
it beside dprint's TypeScript plugin, and add `vibe` to `includes`:

```json
{
  "includes": ["**/*.{ts,vibe}"],
  "plugins": [
    "https://plugins.dprint.dev/typescript-0.93.4.wasm@cdbf74066391bdb11ddaef9ea95299636a97d637f5a9bf7f121d584875647c7a",
    "npm:vibelang/dprint-plugin.json"
  ]
}
```

`dprint fmt` and `dprint check` then format `.vibe` files with the
configuration's `typescript` settings, so they read like the project's `.ts`
files. See [Format with dprint](https://vibelang.sh/setup/formatting).

## Lint .vibe files with ESLint

`vibelang/eslint` is an ESLint parser for `.vibe` files: it gives ESLint the
tree typescript-eslint gives for TypeScript, so the same rules and plugins
apply. Install `eslint` and `typescript-eslint` 8 beside `vibelang`:

```js
// eslint.config.js
import tseslint from "typescript-eslint"
import vibelang from "vibelang/eslint"

export default tseslint.config({
  files: ["src/**/*.vibe"],
  extends: [tseslint.configs.recommended],
  languageOptions: { parser: vibelang }
})
```

Rules that need type information don't run on `.vibe` files;
`vibelang --noEmit` checks types. The
[Devtools page](https://vibelang.sh/getting-started/012-devtools) lists the node types of VibeLang's own forms.

## How the package finds the native compiler

`vibelang` is a small Node launcher. In 0.0.2, the native compiler comes from one of six
optional dependencies, and npm installs the one that matches your platform:
`vibelang-darwin-arm64`, `vibelang-darwin-x64`, `vibelang-linux-arm64`,
`vibelang-linux-x64`, `vibelang-win32-arm64`, or `vibelang-win32-x64`. The launcher
runs `VIBELANG_BINARY` when set, then `native/vibelang` beside the package in a
VibeLang checkout, then the platform package. If you installed with optional
dependencies omitted, reinstall without that option. The launcher forwards
arguments, standard streams, signals, and the exit status.

## License

Apache-2.0. The compiler is a modified version of the TypeScript 7 compiler
(typescript-go); see `LICENSE` and `NOTICE`.

## Develop the compiler

From the VibeLang repository root:

```sh
sh scripts/build-vibelang.sh      # writes packages/compiler/native/vibelang and dprint-plugin.json
packages/compiler/bin/vibelang --version
node --test packages/compiler/test/*.test.mjs
```

A checkout's launcher prefers `native/vibelang`, so projects that link to the
checkout use a rebuilt compiler without reinstalling.
