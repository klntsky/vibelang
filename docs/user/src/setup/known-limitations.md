# Known limitations

VibeLang is in early alpha.

- Platforms: [macOS arm64/x64, Linux arm64/x64; Windows via WSL](../getting-started/004-installation.md#prerequisites). Native Windows binaries are not available yet.
- Effect: [`effect@4.0.5` exactly](existing-project.md); other versions are unsupported.
- Resources: [a JavaScript Disposable can leak when interruption is pending at the end of masked acquisition](../resource-management/030-scope.md#known-limitation-pending-interruption-and-javascript-disposables). Use protected `Effect.acquireRelease` to acquire a plain `{ value: disposable }` holder whose release disposes `holder.value`; returning the Disposable itself and disposing it in release closes it twice on ordinary exit.
- Stock TypeScript: `tsc`, `vue-tsc`, and `svelte-check` report TS2307 for `.vibe` imports; [build with `vibelang`](vibelang-cli.md#2-replace-tsc-in-the-scripts). `vibelang-ts-mapper`, a TypeScript 7.1 content mapper that lets stock TypeScript check `.vibe` files, is experimental and unpublished. It is tested with `typescript@7.1.0-beta`, which runs it only under `tsc --runExternalCode`.
- Editors: [VS Code and Cursor use the bundled VSIX](vscode.md). VS Code Marketplace and Open VSX publishing are not ready.
- Stability: [v0: breaking changes expected](release-notes.md). Read release notes before upgrading.
