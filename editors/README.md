# Editor setup

Every editor runs the same language server: `effeck --lsp --stdio`, the Effeck compiler that a project installs from `effeck` (`node_modules/.bin/effeck`). It serves `.effeck` and `.ts` files of the program that `tsconfig.json` describes: diagnostics with TypeScript codes and ES codes, hovers with inferred contracts, definitions, references, renames, and completions.

The server reports a file's diagnostics when the editor asks for them (LSP 3.17 pull diagnostics, `textDocument/diagnostic`), so the editor's LSP client must support pull diagnostics. VS Code, Cursor, Neovim 0.10 or later, and current Zed releases do.

| Editor | Setup | Tested |
| --- | --- | --- |
| VS Code, Cursor | The extension in [vscode/](vscode/README.md) | `bun run test` in `editors/vscode`, on VS Code 1.98.2 and 1.105.1 |
| Neovim 0.10 or later | [nvim/effeck.lua](nvim/effeck.lua), below | `editors/nvim/test.sh`, on Neovim 0.10.3 |
| Zed | Settings, below | No: Zed has no headless mode |

## Neovim

Copy `nvim/effeck.lua` to `~/.config/nvim/lua/effeck.lua` and add this line to `init.lua`:

```lua
require("effeck").setup()
```

`.effeck` files then get the `effeck` filetype, TypeScript highlighting (tree-sitter's TypeScript parser when it is installed, else Vim's TypeScript syntax), and `effeck --lsp --stdio` as their language server. The server is the project's `node_modules/.bin/effeck`, else `effeck` on `PATH`; `setup({ cmd = "/path/to/effeck" })` names another. Neovim's built-in LSP client then shows diagnostics, and its defaults give hover on `K`, go to definition on `<C-]>` (through 'tagfunc'), and completion on `<C-x><C-o>` (through 'omnifunc').

`setup({ typescript = true })` also attaches the server to `.ts` and `.js` files, so a `.ts` file that imports an `.effeck` module resolves it. Turn off your other TypeScript server (`ts_ls`, `vtsls`) for those projects first, or both report every `.ts` file.

`editors/nvim/test.sh` checks the configuration in a headless Neovim: it copies `typescript/effeck/lsp-fixture` to a scratch project that has `effeck` installed, with its Node launcher at `node_modules/.bin/effeck` and the compiler under test as its native binary, then checks the filetype and highlighting, that the project's `effeck` attaches, TS2322 and EK1003 diagnostics, and a hover with the inferred contract and a call's success value. It reads `NVIM`, `EFFECK`, `EFFECT_NODE_MODULES`, and `EFFECK_RUNTIME`.

## Zed

Zed defines languages through extensions, so these settings treat `.effeck` files as TypeScript and replace the binary of Zed's TypeScript server, `vtsls`, with `effeck`. Put them in the project's `.zed/settings.json`, so other TypeScript projects keep `vtsls`:

```json
{
  "file_types": {
    "TypeScript": ["es"]
  },
  "lsp": {
    "vtsls": {
      "binary": {
        "path": "/absolute/path/to/project/node_modules/.bin/effeck",
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

The path must be absolute. The `effeck` that `scripts/setup.sh` links into `~/.local/bin` works too when written out in full, such as `/Users/you/.local/bin/effeck`. Zed then sends `.effeck` files with the language id `typescript`, which the server accepts, and the server serves the project's `.ts` files too. These settings are untested: Zed cannot run headlessly, so nothing here checked them in Zed. An `initialize` request with `vtsls`-style initialization options, as Zed sends, succeeds against `effeck`.

## Other LSP clients

Run `effeck --lsp --stdio` for `*.effeck` files, with `tsconfig.json` as the root marker, and send either `effeck` or `typescript` as the language id. Add `*.ts` files to let the server answer for them too, and turn off the editor's other TypeScript server for the project. On GitHub, add `*.effeck linguist-language=TypeScript` to `.gitattributes`.
