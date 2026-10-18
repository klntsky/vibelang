# Set up VibeLang

VibeLang is a language for writing Effect programs with TypeScript syntax. These steps set it up in a project. Each step gives a command and the output to expect. When the output differs, stop and show the user the command and its output. VibeLang is v0: the language will change in major, breaking ways, and it is not suitable for production use.

## 1. Check the tools

Run `node --version` and expect `v26.0.0` or later. If the project uses Bun, run `bun --version` and expect `1.4.0` or later.

Version 0.0.1 ships binaries for macOS and Linux: `darwin-arm64`, `darwin-x64`, `linux-arm64`, and `linux-x64`. `win32` binaries come in a later release.

Run `npm view vibelang@0.0.1 version` and expect `0.0.1`. An `E404` error means that version is unavailable from the configured registry: report the registry and error before continuing.

## 2. Detect the project

Read the project's root directory and record what you find:

| Fact | Signal | Result |
| --- | --- | --- |
| Project | No `package.json` | New project |
| | A `package.json` | Existing project |
| Package manager | `bun.lock` or `bun.lockb` | Bun |
| | `package-lock.json`, or no lockfile | npm |
| | `pnpm-lock.yaml` or `yarn.lock` | pnpm or Yarn: install the packages the npm commands name with `pnpm add` or `yarn add` |
| Build | `vite` or `vitest` in `package.json`, or a `vite.config.*` or `vitest.config.*` file | Vite |
| | `webpack`, `rollup`, `esbuild`, or `@rspack/core` in `package.json`, or `bun build` in a script | Another bundler: no published plugin in 0.0.1 |
| | None of these | The `vibelang` CLI |
| Framework | `react`, `vue`, or `svelte` in `package.json`, and the build is Vite | React, Vue, or Svelte. With `@sveltejs/kit` or `nuxt`, tell the user these pages do not cover it |
| Editor | A `.vscode/` or `.cursor/` directory | VS Code or Cursor |
| | A `.zed/` directory | Zed |
| | None of these | Ask the user |

## 3. Follow the matching pages

Read each page that matches and follow its steps, in this order: project, build or framework, editor.

| Case | Page |
| --- | --- |
| New project | [Start a new project](setup/new-project.md); it sets up the build and tests |
| Existing project | [Add VibeLang to an existing project](setup/existing-project.md) |
| Build with the `vibelang` CLI, as with `tsc` | [Build with the vibelang CLI](setup/vibelang-cli.md) |
| Vite or Vitest, without React, Vue, or Svelte | [Vite and Vitest](setup/vite.md) |
| React, Vue, or Svelte with Vite | [React](setup/react.md), [Vue](setup/vue.md), or [Svelte](setup/svelte.md), in place of the Vite page |
| VS Code or Cursor | [VS Code and Cursor](setup/vscode.md) |
| Neovim | [Neovim](setup/neovim.md) |
| Zed | [Zed](setup/zed.md) |
| Another editor with an LSP client | [Other editors](setup/other-editors.md) |

Version 0.0.1 has no published integration for webpack, Rollup, esbuild, Rspack, or `bun build`. The repository contains `vibelang-unplugin`, but it is not on npm. If the project builds with one of them, tell the user, and ask whether to build the `.vibe` sources with the `vibelang` CLI instead.

## 4. Verify the setup

From the project directory, run:

```sh
npx vibelang --version
npx vibelang --noEmit
```

The first command prints `VibeLang 0.0.1 (TypeScript 7.1.0-dev)`. The second prints nothing and exits with status 0. Then run the project's `build` and `test` scripts; each must exit with status 0. Report to the user the pages you followed and each command's output.
