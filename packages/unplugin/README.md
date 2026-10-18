# effeck-unplugin

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

Compile `.effeck` modules with the native Effeck compiler in webpack, Rspack,
Rollup, Rolldown, esbuild, and Bun. The compiler owns type and language
diagnostics. The plugin serves its JavaScript and source maps, reports its
diagnostics as build errors with file, line, and column, and recompiles in
watch mode. For Vite and Vitest, use [effeck-vite](../vite/README.md).

## Install

```sh
npm install --save-dev effeck effeck-unplugin
```

The plugin runs the `effeck` compiler package, an optional peer dependency:
install it, or pass a `compiler` path. It needs Node 22 or later.

## Configure

Keep your normal `tsconfig.json` with the `.effeck` sources included. The plugin
uses the same compiler options and diagnostics as `effeck -p tsconfig.json`.

```js
// webpack.config.js
import effeck from "effeck-unplugin/webpack"
export default { plugins: [effeck()] }

// rspack.config.js
import effeck from "effeck-unplugin/rspack"
export default { plugins: [effeck()] }

// rollup.config.js and rolldown.config.js
import effeck from "effeck-unplugin/rollup" // or effeck-unplugin/rolldown
export default { input: "src/main.ts", plugins: [effeck()] }

// esbuild
import effeck from "effeck-unplugin/esbuild"
await esbuild.build({ entryPoints: ["src/main.ts"], bundle: true, plugins: [effeck()] })

// Bun
import effeck from "effeck-unplugin/bun"
await Bun.build({ entrypoints: ["src/main.ts"], outdir: "out", plugins: [effeck()] })
```

Import local sources as `./welcome.effeck` and `./config.ts`. The compiler
rewrites those imports to `./welcome.js` and `./config.js`; the plugin resolves
them back to `welcome.effeck` and `config.ts`. The plugin compiles only
`.effeck` modules. Your `.ts` files go through your bundler's usual TypeScript
support.

## Options

```js
effeck({
  project: "tsconfig.app.json", // Relative to root; default: tsconfig.json.
  root: "/path/to/app", // Default: the webpack or Rspack context, esbuild's absWorkingDir, or the working directory.
  compiler: "/absolute/path/to/effeck" // Native binary or Node launcher.
})
```

Without a `compiler` option, the plugin runs `EFFECK_BINARY`, else the launcher
of the `effeck` package resolved from the root. Set `EFFECK_UNPLUGIN_DEBUG=1` to
trace each build start, compile, and module load on stderr.

## How it works

Effeck's emit is type-directed: a module's JavaScript depends on the contracts of
the modules it imports. So v0 compiles the whole project with one `effeck`
process per build, on the first `.effeck` module load, and every watch rebuild
compiles it again. The compiler writes to a directory under the system
temporary directory; the plugin reads the JavaScript and source maps into
memory and deletes it. The plugin forces JavaScript and source-map emission,
embeds source content, overrides `rootDir` to the root, and disables
declarations, incremental output, and emission on errors. A source saved
during a compile starts another compile before any module is served.

Each `.effeck` module declares every project source and the tsconfig as watch
files, so an edit to any of them rebuilds the `.effeck` modules. A failed
compile is reported once, as a build error whose message lists each diagnostic
as `file:line:column - error TS2322: text`.

## Limitations

- **Bun:** `Bun.build` ignores plugin source maps, so a Bun bundle's map points
  into the compiled JavaScript of each `.effeck` module, not its source. Bun has
  no watch API for plugins.
- **webpack and Rspack:** the error carries its position in the message only;
  unplugin does not pass a location to webpack. webpack before 5.112.0 can miss
  a save made as a watch build finishes
  ([webpack#22646](https://github.com/webpack/webpack/pull/22646)).
- **esbuild:** `.effeck` modules load in the `effeck` namespace.
- **All:** a build compiles the whole project, so its time grows with the
  project, not with the change. New source files are watched after the next
  build that includes them.
- **Vite:** `effeck-unplugin/vite` exists for parity tests. It lacks
  effeck-vite's dev-server middleware and hot-update invalidation.

## Acceptance checks

For contributors, from the Effeck repository root:

```sh
sh packages/unplugin/test/run.sh
```

The script builds the compiler and `effeck-runtime` from the checkout, then
bundles `test/fixture` with webpack, Rspack, Rollup, Rolldown, esbuild, Bun, and
Vite. For each bundler it runs the bundle and compares its output with
`test/expected.txt`, checks that the source map maps a token back to its
`.effeck` line and column, and checks that a type error fails the build with its
`.effeck` position. In watch mode it edits the `.effeck` source, a `.ts` file the
`.effeck` module imports only for types, and the tsconfig, and waits for each
rebuild.
