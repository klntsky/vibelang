# Effeck for VS Code

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

The extension `effeck.effeck` registers the Effeck language for `.effeck` files and runs the Effeck compiler, `effeck --lsp --stdio`, as their language server. It works in VS Code 1.91 or later and in Cursor.

| Feature | What you see |
| --- | --- |
| Diagnostics | TypeScript's errors at `.effeck` spans, and Effeck's own, such as `EK(1003)` on `async` |
| Hover | A function's inferred contract, `function lookup(id: number): Effect<string, NotFound, never>`, and at a call that runs, "Runs here and returns `string`." |
| Navigation | Go to definition, find references, and rename across `.effeck` and `.ts` files |
| Completions | TypeScript's completions, with Effeck types: inside an `.effeck` function, a call that runs has its success type |
| Highlighting | A TextMate grammar derived from VS Code's TypeScript grammar, with root scope `source.effeck` |

## Install

`sh scripts/setup.sh` at the repository root builds the extension and prints the install commands. To build it alone:

```sh
cd editors/vscode
bun install
bun run package                                   # writes effeck.vsix
code --install-extension effeck.vsix        # VS Code
cursor --install-extension effeck.vsix      # Cursor
```

Then open a project that has `effeck` installed, such as one made with `bun create effeck`.

## How it finds effeck

The extension runs the first of:

1. The `effeck.server.path` setting. A relative path is relative to the first workspace folder.
2. The `node_modules/.bin/effeck` nearest to the open `.effeck` file, then nearest to each workspace folder. `bun install` puts it there from `effeck`.
3. `effeck` on `PATH`. `scripts/setup.sh` links `~/.local/bin/effeck` when that directory is on `PATH`.

If none exists, the extension says so and the status item next to the language mode reads "effeck not found". After rebuilding the compiler, run **Effeck: Restart Language Server**.

## TypeScript files

The language server checks the `.ts` files of the program together with the `.effeck` files, so `.effeck` code sees TypeScript types. Which server answers for an open `.ts` file depends on `effeck.server.typeScriptFiles`:

| Value | `.ts` and `.js` files are served by |
| --- | --- |
| `auto` (default) | Effeck when the built-in TypeScript features are off, else VS Code's built-in TypeScript |
| `on` | Effeck, and also VS Code's built-in TypeScript unless it is off, so results appear twice |
| `off` | VS Code's built-in TypeScript |

The built-in features count as off when the built-in extension "TypeScript and JavaScript Language Features" is disabled, or, in Cursor, when `typescript.experimental.useTsgo` is `true`. Turn them off when a `.ts` file imports an `.effeck` module: VS Code's TypeScript cannot resolve `.effeck` files. **Effeck: Serve TypeScript Files in This Workspace** does it in Cursor and shows the extension to disable in VS Code.

The extension never forwards `typescript.validate.enable` to the server, so turning off the built-in TypeScript diagnostics does not turn off Effeck's.

## Settings and commands

| Setting | Default | Meaning |
| --- | --- | --- |
| `effeck.server.path` | `""` | The `effeck` to run |
| `effeck.server.typeScriptFiles` | `"auto"` | Whether the server also serves `.ts` and `.js` files |
| `effeck.trace.server` | `"off"` | `"messages"` or `"verbose"` logs the LSP traffic to the Effeck output |

| Command | Does |
| --- | --- |
| Effeck: Restart Language Server | Stops the server and starts it again, finding `effeck` anew |
| Effeck: Show Language Server Output | Opens the Effeck output, which has the server's log |
| Effeck: Serve TypeScript Files in This Workspace | Turns the built-in TypeScript features off for the workspace, as above |

The extension does not run in untrusted workspaces, because it executes the workspace's `node_modules/.bin/effeck`. Highlighting still works there.

## Develop

```sh
bun run build            # type-check with tsc, bundle with esbuild to dist/extension.js
bun run package          # build, then write effeck.vsix
bun run test             # install effeck.vsix into a fresh VS Code and check it
bun run derive-grammar   # regenerate syntaxes/ from the installed VS Code's TypeScript grammar
```

`bun run test` needs `effeck` at `../../typescript/built/local/effeck` (or `EFFECK`), `effect` in `../../experiments/node_modules` (or `EFFECT_NODE_MODULES`), and the built runtime in `../../packages/runtime` (or `EFFECK_RUNTIME`). It downloads VS Code 1.98.2 to `.vscode-test/` (`VSCODE_VERSION`, `VSCODE_TEST_CACHE`), or uses the VS Code at `VSCODE_EXECUTABLE`. It copies `typescript/effeck/lsp-fixture` to a scratch project, installs `effeck` there as `bun install` does, with the compiler under test as its native binary, installs the `.vsix` into a fresh profile, and checks the language, the grammar's scopes, TypeScript and EK diagnostics, hovers, a definition, and completions of prelude names and service members; a second run disables the built-in TypeScript extension and checks that Effeck reports `.ts` diagnostics. On Linux, run it under `xvfb-run -a`. On macOS it opens a VS Code window for about a minute.

## License

The grammar and the language configuration are derived from MIT-licensed work by Microsoft; `ThirdPartyNotices.txt` carries their notices.
