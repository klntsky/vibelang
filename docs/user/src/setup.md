# Set up VibeLang

VibeLang is a language for Effect programs in TypeScript syntax. These steps set it up in a project; each gives a command and its expected output. When the output differs, stop and show the user the command and its output. VibeLang is v0: the language will change in major, breaking ways, and it is not suitable for production use.

## 1. Check the tools

Run `node --version` and expect `v26.0.0` or later. With Bun, run `bun --version` and expect `1.4.0` or later.

Version 0.0.5 publishes native packages for macOS and Linux on arm64 and x64. Native Windows binaries are not available yet. Use WSL and follow the Linux instructions. The end-to-end npm and Bun rehearsal runs on Linux x64.

Run `npm view vibelang@0.0.5 version` and expect `0.0.5`. `E404` means the configured registry lacks that version: report the registry and error before continuing.

## 2. Detect the project

Read the project's root directory and record:

| Fact | Signal | Result |
| --- | --- | --- |
| Project | No `package.json` | New project |
| | A `package.json` | Existing project |
| Package manager | `bun.lock` or `bun.lockb` | Bun |
| | `package-lock.json`, or no lockfile | npm |
| | `pnpm-lock.yaml` or `yarn.lock` | pnpm or Yarn: install the npm commands' packages with `pnpm add` or `yarn add` |
| Build | `vite` or `vitest` in `package.json`, or a `vite.config.*` or `vitest.config.*` file | Vite |
| | `webpack`, `rollup`, `esbuild`, or `@rspack/core` in `package.json`, or `bun build` in a script | Another bundler: `vibelang-unplugin` |
| | None of these | The `vibelang` CLI |
| Framework | `react`, `vue`, or `svelte` in `package.json`, with a Vite build | React, Vue, or Svelte. With `@sveltejs/kit` or `nuxt`, tell the user these pages don't cover it |
| Editor | A `.vscode/` or `.cursor/` directory | VS Code or Cursor |
| | A `.zed/` directory | Zed |
| | None of these | Ask the user |

## 3. Follow the matching pages

Follow each matching page, in this order: project, build or framework, editor.

| Case | Page |
| --- | --- |
| New project | [Start a new project](setup/new-project.md); it sets up the build and tests |
| Existing project | [Add VibeLang to an existing project](setup/existing-project.md) |
| Build with the `vibelang` CLI, as with `tsc` | [Build with the vibelang CLI](setup/vibelang-cli.md) |
| Vite or Vitest, without React, Vue, or Svelte | [Vite and Vitest](setup/vite.md) |
| React, Vue, or Svelte with Vite | [React](setup/react.md), [Vue](setup/vue.md), or [Svelte](setup/svelte.md), instead of the Vite page |
| Format with dprint, or a `dprint.json` in the project | [Format with dprint](setup/formatting.md) |
| VS Code or Cursor | [VS Code and Cursor](setup/vscode.md) |
| Neovim | [Neovim](setup/neovim.md) |
| Zed | [Zed](setup/zed.md) |
| Another editor with an LSP client | [Other editors](setup/other-editors.md) |

For webpack, Rspack, Rollup, Rolldown, esbuild, and `bun build`, version 0.0.5 includes `vibelang-unplugin`: install `vibelang@0.0.5` and `vibelang-unplugin@0.0.5` as dev dependencies and follow the [plugin configuration](https://www.npmjs.com/package/vibelang-unplugin#configure) for the bundler. Vite and Vitest use `vibelang-vite`.

## 4. Verify the setup

From the project directory, run:

```sh
npx vibelang --version
npx vibelang --noEmit
```

The first prints `VibeLang 0.0.5 (TypeScript 7.1.0-dev)`; the second prints nothing and exits with status 0. Then run the project's `build` and `test` scripts; each must exit with status 0. Report the pages you followed and each command's output to the user.
