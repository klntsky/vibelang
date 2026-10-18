# Other editors

Any editor with an LSP client can run VibeLang's language server, `vibelang --lsp --stdio`, the compiler that the project installs as `node_modules/.bin/vibelang`. It serves the `.vibe` and `.ts` files of the program that `tsconfig.json` describes. It reports diagnostics when the editor asks for them, through LSP 3.17 pull diagnostics (`textDocument/diagnostic`), so the client must support pull diagnostics.

## 1. Check the server

From the project directory, run:

```sh
npx vibelang --version
```

The output is `VibeLang 0.0.1 (TypeScript 7.1.0-dev)`.

## 2. Configure the LSP client

| Setting | Value |
| --- | --- |
| Command | `node_modules/.bin/vibelang --lsp --stdio`, or an absolute path to it if the client needs one |
| Files | `*.vibe`; add `*.ts` to let the server answer for TypeScript files too |
| Root marker | `tsconfig.json` |
| Language id | `vibe` or `typescript` |

If the server answers for `.ts` files, turn off the editor's other TypeScript server for the project, or both report every `.ts` file. JetBrains IDEs can run the server through the LSP4IJ plugin.

## 3. Highlight .vibe files on GitHub

Add this line to the project's `.gitattributes`:

```text
*.vibe linguist-language=TypeScript
```
