# VS Code and Cursor

The `vibelang` package ships the editor extension `vibelang.vibelang` as `node_modules/vibelang/editor/vibelang.vsix`. The extension runs the project's compiler as the language server, `vibelang --lsp --stdio`, for diagnostics, hovers, definitions, references, renames, and completions. It works in VS Code 1.91 or later and in Cursor. It is not on the VS Code Marketplace or Open VSX yet.

## 1. Install the extension

From the project directory, after installing dependencies, run the command for the user's editor:

```sh
code --install-extension node_modules/vibelang/editor/vibelang.vsix
cursor --install-extension node_modules/vibelang/editor/vibelang.vsix
```

## 2. Check the extension

```sh
code --list-extensions | grep vibelang
```

For Cursor, run `cursor --list-extensions | grep vibelang`. The output is:

```out
vibelang.vibelang
```

## 3. Tell the user what to do in the editor

Tell the user to open the project directory and to trust the workspace. The extension does not run in an untrusted workspace, because it runs the workspace's `node_modules/.bin/vibelang`. When it cannot find `vibelang`, the status item next to the language mode reads "vibelang not found".

If a `.ts` file imports a `.vibe` module, tell the user to run **VibeLang: Serve TypeScript Files in This Workspace** from the command palette. VS Code's built-in TypeScript cannot resolve `.vibe` files. In Cursor, the command turns the built-in TypeScript features off for the workspace; in VS Code, it shows the extension to disable, "TypeScript and JavaScript Language Features".

After an update of `vibelang`, install the extension again from `node_modules`.
