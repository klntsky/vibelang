# effeck

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

The Effeck compiler and language server. Effeck is a readable syntax for
Effect that compiles to TypeScript: `effeck` checks `.effeck` and `.ts` files
together and emits JavaScript that runs on Effect 4. Documentation:
[effeck.dev](https://effeck.dev).

## Start a project

```sh
bun create effeck my-app
# or
npm create effeck@latest my-app
```

## Add Effeck to an existing project

```sh
npm install effect@4.0.5 effeck-runtime
npm install --save-dev effeck
```

Generated JavaScript imports `effect` and `effeck-runtime`, so both are runtime
dependencies. Point `tsconfig.json` at your `.effeck` sources:

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
  "include": ["src/**/*.effeck"]
}
```

| Command | What it does |
| --- | --- |
| `npx effeck` | Check and compile the project in `tsconfig.json`, like `tsc` |
| `npx effeck --watch` | Recompile on every save |
| `npx effeck --lsp --stdio` | Run the language server for an editor |
| `npx effeck --version` | Print the Effeck and TypeScript versions |

For tests with Vitest, add [`effeck-vite`](https://www.npmjs.com/package/effeck-vite).

## Editor support

This package ships the VS Code extension. In a project with `effeck` installed:

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
cursor --install-extension node_modules/effeck/editor/effeck.vsix
```

The extension starts the project's own `effeck` as the language server.

## How the package finds the native compiler

`effeck` is a small Node launcher. The native compiler comes from one of six
optional dependencies, and npm installs the one that matches your platform:
`effeck-darwin-arm64`, `effeck-darwin-x64`, `effeck-linux-arm64`,
`effeck-linux-x64`, `effeck-win32-arm64`, or `effeck-win32-x64`. The launcher
runs `EFFECK_BINARY` when it is set, then `native/effeck` beside the package in
an Effeck checkout, then the platform package. If you installed with optional
dependencies omitted, reinstall without that option. The launcher forwards
arguments, standard streams, signals, and the exit status.

## License

Apache-2.0. The compiler is a modified version of the TypeScript 7 compiler
(typescript-go); see `LICENSE` and `NOTICE`.

## Develop the compiler

From the Effeck repository root:

```sh
sh scripts/build-effeck.sh      # writes packages/compiler/native/effeck
packages/compiler/bin/effeck --version
node --test packages/compiler/test/
```

A checkout's launcher prefers `native/effeck`, so projects that link to the
checkout use a rebuilt compiler without reinstalling.
