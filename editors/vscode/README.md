# EffectScript for VS Code

The extension `effectscript.effectscript` registers the EffectScript language for `.es` files and runs the EffectScript compiler, `esc --lsp --stdio`, as their language server. It works in VS Code 1.91 or later and in Cursor.

| Feature | What you see |
| --- | --- |
| Diagnostics | TypeScript's errors at `.es` spans, and EffectScript's own, such as `ES(1003)` on `async` |
| Hover | A function's inferred contract, `function lookup(id: number): Effect<string, NotFound, never>`, and at a call that runs, "Runs here and returns `string`." |
| Navigation | Go to definition, find references, and rename across `.es` and `.ts` files |
| Completions | TypeScript's completions, with EffectScript types: inside an `.es` function, a call that runs has its success type |
| Highlighting | A TextMate grammar derived from VS Code's TypeScript grammar, with root scope `source.effectscript` |

## Install

`sh scripts/setup.sh` at the repository root builds the extension and prints the install commands. To build it alone:

```sh
cd editors/vscode
bun install
bun run package                                   # writes effectscript.vsix
code --install-extension effectscript.vsix        # VS Code
cursor --install-extension effectscript.vsix      # Cursor
```

Then open a project that has `@effectscript/compiler` installed, such as one made with `bun create effectscript`.

## How it finds esc

The extension runs the first of:

1. The `effectscript.server.path` setting. A relative path is relative to the first workspace folder.
2. The `node_modules/.bin/esc` nearest to the open `.es` file, then nearest to each workspace folder. `bun install` puts it there from `@effectscript/compiler`.
3. `esc` on `PATH`. `scripts/setup.sh` links `~/.local/bin/esc` when that directory is on `PATH`.

If none exists, the extension says so and the status item next to the language mode reads "esc not found". After rebuilding the compiler, run **EffectScript: Restart Language Server**.

## TypeScript files

The language server checks the `.ts` files of the program together with the `.es` files, so `.es` code sees TypeScript types. Which server answers for an open `.ts` file depends on `effectscript.server.typeScriptFiles`:

| Value | `.ts` and `.js` files are served by |
| --- | --- |
| `auto` (default) | EffectScript when the built-in TypeScript features are off, else VS Code's built-in TypeScript |
| `on` | EffectScript, and also VS Code's built-in TypeScript unless it is off, so results appear twice |
| `off` | VS Code's built-in TypeScript |

The built-in features count as off when the built-in extension "TypeScript and JavaScript Language Features" is disabled, or, in Cursor, when `typescript.experimental.useTsgo` is `true`. Turn them off when a `.ts` file imports an `.es` module: VS Code's TypeScript cannot resolve `.es` files. **EffectScript: Serve TypeScript Files in This Workspace** does it in Cursor and shows the extension to disable in VS Code.

The extension never forwards `typescript.validate.enable` to the server, so turning off the built-in TypeScript diagnostics does not turn off EffectScript's.

## Settings and commands

| Setting | Default | Meaning |
| --- | --- | --- |
| `effectscript.server.path` | `""` | The `esc` to run |
| `effectscript.server.typeScriptFiles` | `"auto"` | Whether the server also serves `.ts` and `.js` files |
| `effectscript.trace.server` | `"off"` | `"messages"` or `"verbose"` logs the LSP traffic to the EffectScript output |

| Command | Does |
| --- | --- |
| EffectScript: Restart Language Server | Stops the server and starts it again, finding `esc` anew |
| EffectScript: Show Language Server Output | Opens the EffectScript output, which has the server's log |
| EffectScript: Serve TypeScript Files in This Workspace | Turns the built-in TypeScript features off for the workspace, as above |

The extension does not run in untrusted workspaces, because it executes the workspace's `node_modules/.bin/esc`. Highlighting still works there.

## Develop

```sh
bun run build            # type-check with tsc, bundle with esbuild to dist/extension.js
bun run package          # build, then write effectscript.vsix
bun run test             # install effectscript.vsix into a fresh VS Code and check it
bun run derive-grammar   # regenerate syntaxes/ from the installed VS Code's TypeScript grammar
```

`bun run test` needs `esc` at `../../typescript/built/local/esc` (or `ESC`), `effect` in `../../experiments/node_modules` (or `EFFECT_NODE_MODULES`), and the built runtime in `../../packages/runtime` (or `EFFECTSCRIPT_RUNTIME`). It downloads VS Code 1.98.2 to `.vscode-test/` (`VSCODE_VERSION`, `VSCODE_TEST_CACHE`), or uses the VS Code at `VSCODE_EXECUTABLE`. It copies `typescript/effectscript/lsp-fixture` to a scratch project, installs the `.vsix` into a fresh profile, and checks the language, the grammar's scopes, TypeScript and ES diagnostics, hovers, a definition, and completions of prelude names and service members; a second run disables the built-in TypeScript extension and checks that EffectScript reports `.ts` diagnostics. On Linux, run it under `xvfb-run -a`. On macOS it opens a VS Code window for about a minute.

## License

The grammar and the language configuration are derived from MIT-licensed work by Microsoft; `ThirdPartyNotices.txt` carries their notices.
