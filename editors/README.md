# Editor setup

Every editor runs the same language server: `esc --lsp --stdio`, the EffectScript compiler that a project installs from `@effectscript/compiler` (`node_modules/.bin/esc`). It serves `.es` and `.ts` files of the program that `tsconfig.json` describes: diagnostics with TypeScript codes and ES codes, hovers with inferred contracts, definitions, references, renames, and completions.

The server reports a file's diagnostics when the editor asks for them (LSP 3.17 pull diagnostics, `textDocument/diagnostic`), so the editor's LSP client must support pull diagnostics. VS Code, Cursor, Neovim 0.10 or later, and current Zed releases do.

| Editor | Setup | Tested |
| --- | --- | --- |
| VS Code, Cursor | The extension in [vscode/](vscode/README.md) | `bun run test` in `editors/vscode`, on VS Code 1.98.2 and 1.105.1 |
| Neovim 0.10 or later | [nvim/effectscript.lua](nvim/effectscript.lua), below | `editors/nvim/test.sh`, on Neovim 0.10.3 |
| Zed | Settings, below | No: Zed has no headless mode |

## Neovim

Copy `nvim/effectscript.lua` to `~/.config/nvim/lua/effectscript.lua` and add this line to `init.lua`:

```lua
require("effectscript").setup()
```

`.es` files then get the `effectscript` filetype, TypeScript highlighting (tree-sitter's TypeScript parser when it is installed, else Vim's TypeScript syntax), and `esc --lsp --stdio` as their language server. The server is the project's `node_modules/.bin/esc`, else `esc` on `PATH`; `setup({ cmd = "/path/to/esc" })` names another. Neovim's built-in LSP client then shows diagnostics, and its defaults give hover on `K`, go to definition on `<C-]>` (through 'tagfunc'), and completion on `<C-x><C-o>` (through 'omnifunc').

`setup({ typescript = true })` also attaches the server to `.ts` and `.js` files, so a `.ts` file that imports an `.es` module resolves it. Turn off your other TypeScript server (`ts_ls`, `vtsls`) for those projects first, or both report every `.ts` file.

`editors/nvim/test.sh` checks the configuration in a headless Neovim: it copies `typescript/effectscript/lsp-fixture` to a scratch project whose `node_modules/.bin/esc` is the compiler under test, then checks the filetype and highlighting, that the project's `esc` attaches, TS2322 and ES1003 diagnostics, and a hover with the inferred contract and a call's success value. It reads `NVIM`, `ESC`, `EFFECT_NODE_MODULES`, and `EFFECTSCRIPT_RUNTIME`.

## Zed

Zed defines languages through extensions, so these settings treat `.es` files as TypeScript and replace the binary of Zed's TypeScript server, `vtsls`, with `esc`. Put them in the project's `.zed/settings.json`, so other TypeScript projects keep `vtsls`:

```json
{
  "file_types": {
    "TypeScript": ["es"]
  },
  "lsp": {
    "vtsls": {
      "binary": {
        "path": "/absolute/path/to/project/node_modules/.bin/esc",
        "arguments": ["--lsp", "--stdio"]
      }
    }
  },
  "languages": {
    "TypeScript": {
      "language_servers": ["vtsls", "!typescript-language-server", "..."]
    }
  }
}
```

The path must be absolute. The `esc` that `scripts/setup.sh` links into `~/.local/bin` works too when written out in full, such as `/Users/you/.local/bin/esc`. Zed then sends `.es` files with the language id `typescript`, which the server accepts, and the server serves the project's `.ts` files too. These settings are untested: Zed cannot run headlessly, so nothing here checked them in Zed. An `initialize` request with `vtsls`-style initialization options, as Zed sends, succeeds against `esc`.

## Other LSP clients

Run `esc --lsp --stdio` for `*.es` files, with `tsconfig.json` as the root marker, and send either `effectscript` or `typescript` as the language id. Add `*.ts` files to let the server answer for them too, and turn off the editor's other TypeScript server for the project. On GitHub, add `*.es linguist-language=TypeScript` to `.gitattributes`.
