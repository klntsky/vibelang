# Installation

Effeck compiles `.effeck` sources to JavaScript with the native `effeck` compiler.
This private preview installs from your local repository. The `@effeck`
packages are not published to npm.

## Prerequisites

Install Go, Bun, and Node and make them available on `PATH`. The tested versions
are Go 1.26, Bun 1.4.1, and Node 26.10. Go downloads the compiler repository's
Go 1.27 toolchain automatically on its first build. Setup requires network access
for that download and package installation.

The starter pins `effect` and `@effect/platform-node` to **4.0.5**. Its tests pin
`@effect/vitest` to **4.0.5** and `vitest` to **5.0.5**. Keep those versions together.

## Install from the repository

Run this command from the Effeck checkout:

```sh
sh scripts/setup.sh
```

Setup builds `packages/compiler/native/effeck`, installs and builds the runtime,
packs the runtime into `packages/runtime/effeck-runtime.tgz`, and copies
the starter into `~/.bun-create/effeck`. It writes absolute `file:`
dependencies pointing to this checkout's compiler and Vite packages and to the
runtime tarball. A project installs its own copy of the runtime, so the runtime
and your program load the project's one copy of `effect`.
Keep the checkout at that path. Rerun setup after moving or updating it, then
run `bun update @effeck/runtime effeck @effeck/vite`
in existing projects. If you move the checkout, update those projects' `file:`
paths in `package.json` before reinstalling.

Setup creates an `effeck` launcher in `~/.local/bin` when that directory is already
on `PATH`. Otherwise, use the project scripts below, which find the project's
compiler automatically. Setup never edits your shell configuration or replaces
an unrelated executable or template. It does replace a link to another
checkout's launcher or to a missing file, which an earlier setup left when its
checkout moved. `EFFECK_BINARY` overrides the native binary
when you need to test another build.

If `packages/runtime` or `editors/vscode` has not landed yet, setup prints the
missing path. Rerun setup after those packages land. The compiler still builds;
installing and running the starter requires the runtime.

## Create a project

```sh
bun create effeck my-app --no-git
cd my-app
bun run build
bun run start
bun run test
```

Bun installs dependencies while scaffolding. If installation is interrupted, run
`bun install` in the project before building. `--no-git` leaves repository setup
under your control. See [Getting Started](../013-getting-started.md) for the
starter files and [Running Effects](009-running-effects.md) for execution.

These commands use the real native compiler. They never fall back to the
TypeScript-API prototype. Run `sh scripts/e2e.sh` in the Effeck repository
to check setup, scaffold, install, build, execution, and tests in a temporary
project. It reports the first failing step and returns a nonzero status.

## Configure your editor

When `editors/vscode` exists, setup builds its `.vsix` and prints the exact install
commands for VS Code and Cursor:

```sh
code --install-extension /path/to/effeck/editors/vscode/effeck.vsix
# Or use cursor --install-extension with the same path.
```

Open the scaffolded project directory. The extension uses the project's
`effeck` to start `effeck --lsp --stdio`. Other editors can run that
same command through their LSP client for `.effeck` and `.ts` files.

Setup builds the extension but does not install it for you.

## Compiler commands

From a scaffolded project, run:

```sh
bun run build
bun run dev
bun run test
bun x --no-install effeck --noEmit
```

The template's `tsconfig.json` selects `.effeck` files under `src/`, emits Node ESM
to `dist/`, enables strict checking and source maps, and prevents emit on errors.
Local source imports use `.effeck`; generated JavaScript imports use `.js`.
`effeck` accepts TypeScript compiler flags such as `--noEmit`, `--watch`, and `-b`.

## Coming from Effect

Use the local `effeck` for `.effeck` compilation and
`@effeck/runtime` alongside `effect@4.0.5`. Write application logic in `.effeck`
and keep ordinary TypeScript integrations in `.ts`. The compiler checks both
file types as one program.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/installation)
