# Set up Effeck

Effeck is a language for building reliable TypeScript in the AI era, built on Effect. These steps set it up in a project. Each step gives a command and the output to expect. When the output differs, stop and show the user the command and its output. Effeck is v0: the language will change in major, breaking ways, and it is not suitable for production use.

## 1. Check the tools

Run `node --version` and expect `v26.0.0` or later. If the project uses Bun, run `bun --version` and expect `1.4.0` or later.

Run `npm view effeck@0.0.1 version` and expect `0.0.1`. An `E404` error means the packages are not on npm yet: stop and tell the user.

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
| | `webpack`, `rollup`, `esbuild`, or `@rspack/core` in `package.json`, or `bun build` in a script | Another bundler: not supported yet |
| | None of these | The `effeck` CLI |
| Editor | A `.vscode/` or `.cursor/` directory | VS Code or Cursor |
| | A `.zed/` directory | Zed |
| | None of these | Ask the user |

## 3. Follow the matching pages

Read each page that matches and follow its steps, in this order: project, build, editor.

| Case | Page |
| --- | --- |
| New project | [Start a new project](setup/new-project.md); it sets up the build and tests |
| Existing project | [Add Effeck to an existing project](setup/existing-project.md) |
| Build with the `effeck` CLI, as with `tsc` | [Build with the effeck CLI](setup/effeck-cli.md) |
| Vite or Vitest | [Vite and Vitest](setup/vite.md) |
| VS Code or Cursor | [VS Code and Cursor](setup/vscode.md) |
| Neovim | [Neovim](setup/neovim.md) |
| Zed | [Zed](setup/zed.md) |
| Another editor with an LSP client | [Other editors](setup/other-editors.md) |

Effeck does not support webpack, Rollup, esbuild, Rspack, or `bun build` yet. If the project builds with one of them, tell the user, and ask whether to build the `.effeck` sources with the `effeck` CLI instead.

## 4. Verify the setup

From the project directory, run:

```sh
npx effeck --version
npx effeck --noEmit
```

The first command prints `Effeck 0.0.1 (TypeScript 7.1.0-dev)`. The second prints nothing and exits with status 0. Then run the project's `build` and `test` scripts; each must exit with status 0. Report to the user the pages you followed and each command's output.
