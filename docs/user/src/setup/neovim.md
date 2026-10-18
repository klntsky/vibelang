# Neovim

Neovim 0.10 or later runs `effeck --lsp --stdio` with its built-in LSP client through one Lua module. `.effeck` files get the `effeck` filetype, TypeScript highlighting, and the project's compiler as their language server: `node_modules/.bin/effeck`, else `effeck` on `PATH`.

## 1. Check Neovim

Run `nvim --version`. The first line is `NVIM v0.10.0` or later.

## 2. Add the module

Save this file as `~/.config/nvim/lua/effeck.lua`:

```lua
-- Effeck for Neovim 0.10 or later, with the built-in LSP client.
--
-- Copy this file to ~/.config/nvim/lua/effeck.lua and add
--
--   require("effeck").setup()
--
-- to init.lua. .effeck files then get the effeck filetype, TypeScript
-- highlighting, and `effeck --lsp --stdio` as their language server: the
-- project's node_modules/.bin/effeck, else effeck on PATH.
--
-- setup({ typescript = true }) also runs effeck for .ts and .js files. Turn off
-- your other TypeScript server (ts_ls, vtsls) in those projects first, or
-- both report every .ts file.
local M = {}

local typescript_filetypes = { "typescript", "typescriptreact", "javascript", "javascriptreact" }

-- esc_command returns the effeck that serves a project rooted at root.
local function esc_command(root)
  if root then
    local local_esc = vim.fs.joinpath(root, "node_modules", ".bin", "effeck")
    if vim.fn.executable(local_esc) == 1 then
      return local_esc
    end
  end
  return "effeck"
end

function M.setup(opts)
  opts = opts or {}
  vim.filetype.add({ extension = { effeck = "effeck" } })
  vim.treesitter.language.register("typescript", "effeck")

  local filetypes = { "effeck" }
  if opts.typescript then
    vim.list_extend(filetypes, typescript_filetypes)
  end
  vim.api.nvim_create_autocmd("FileType", {
    group = vim.api.nvim_create_augroup("effeck", { clear = true }),
    pattern = filetypes,
    callback = function(args)
      if vim.bo[args.buf].filetype == "effeck" and not pcall(vim.treesitter.start, args.buf, "typescript") then
        -- No tree-sitter parser for TypeScript: use Vim's TypeScript syntax.
        vim.bo[args.buf].syntax = "typescript"
      end
      local root = vim.fs.root(args.buf, { "tsconfig.json", "package.json", ".git" })
      vim.lsp.start({
        name = "effeck",
        cmd = { opts.cmd or esc_command(root), "--lsp", "--stdio" },
        root_dir = root,
      }, { bufnr = args.buf })
    end,
  })
end

return M
```

Add this line to `~/.config/nvim/init.lua`, unless it is there already:

```lua
require("effeck").setup()
```

`setup({ typescript = true })` also attaches the server to `.ts` and `.js` files, so a `.ts` file that imports an `.effeck` module resolves it. Turn off the other TypeScript server, such as `ts_ls` or `vtsls`, for those projects first, or both report every `.ts` file. `setup({ cmd = "/path/to/effeck" })` names another compiler.

## 3. Check the filetype and the server

```sh
nvim --headless "+lua io.write(vim.filetype.match({ filename = 'main.effeck' }))" +qa
```

The output is `effeck`. Then, from the project directory, open one of its `.effeck` files, here `src/main.effeck`:

```sh
nvim --headless src/main.effeck "+lua vim.wait(5000, function() return #vim.lsp.get_clients({ name = 'effeck' }) > 0 end)" "+lua io.write(#vim.lsp.get_clients({ name = 'effeck' }) .. ' effeck client')" +qa
```

The output is `1 effeck client`. An error that the language server failed to spawn means Neovim found no `effeck`: install the project's dependencies.
