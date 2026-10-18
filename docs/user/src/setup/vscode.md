# VS Code and Cursor

The `effeck` package ships the editor extension `effeck.effeck` as `node_modules/effeck/editor/effeck.vsix`. The extension runs the project's compiler as the language server, `effeck --lsp --stdio`, for diagnostics, hovers, definitions, references, renames, and completions. It works in VS Code 1.91 or later and in Cursor. It is not on the VS Code Marketplace or Open VSX yet.

## 1. Install the extension

From the project directory, after installing dependencies, run the command for the user's editor:

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
cursor --install-extension node_modules/effeck/editor/effeck.vsix
```

## 2. Check the extension

```sh
code --list-extensions | grep effeck
```

For Cursor, run `cursor --list-extensions | grep effeck`. The output is:

```out
effeck.effeck
```

## 3. Tell the user what to do in the editor

Tell the user to open the project directory and to trust the workspace. The extension does not run in an untrusted workspace, because it runs the workspace's `node_modules/.bin/effeck`. When it cannot find `effeck`, the status item next to the language mode reads "effeck not found".

If a `.ts` file imports an `.effeck` module, tell the user to run **Effeck: Serve TypeScript Files in This Workspace** from the command palette. VS Code's built-in TypeScript cannot resolve `.effeck` files. In Cursor, the command turns the built-in TypeScript features off for the workspace; in VS Code, it shows the extension to disable, "TypeScript and JavaScript Language Features".

After an update of `effeck`, install the extension again from `node_modules`.
