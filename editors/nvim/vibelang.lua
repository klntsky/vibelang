-- VibeLang for Neovim 0.10 or later, with the built-in LSP client.
--
-- Copy this file to ~/.config/nvim/lua/vibelang.lua and add
--
--   require("vibelang").setup()
--
-- to init.lua. .vibe files then get the vibe filetype, TypeScript
-- highlighting, and `vibelang --lsp --stdio` as their language server: the
-- project's node_modules/.bin/vibelang, else vibelang on PATH.
--
-- setup({ typescript = true }) also runs vibelang for .ts and .js files. Turn off
-- your other TypeScript server (ts_ls, vtsls) in those projects first, or
-- both report every .ts file.
local M = {}

local typescript_filetypes = { "typescript", "typescriptreact", "javascript", "javascriptreact" }

-- esc_command returns the vibelang that serves a project rooted at root.
local function esc_command(root)
  if root then
    local local_esc = vim.fs.joinpath(root, "node_modules", ".bin", "vibelang")
    if vim.fn.executable(local_esc) == 1 then
      return local_esc
    end
  end
  return "vibelang"
end

function M.setup(opts)
  opts = opts or {}
  vim.filetype.add({ extension = { vibe = "vibe" } })
  vim.treesitter.language.register("typescript", "vibe")

  local filetypes = { "vibe" }
  if opts.typescript then
    vim.list_extend(filetypes, typescript_filetypes)
  end
  vim.api.nvim_create_autocmd("FileType", {
    group = vim.api.nvim_create_augroup("vibelang", { clear = true }),
    pattern = filetypes,
    callback = function(args)
      if vim.bo[args.buf].filetype == "vibe" and not pcall(vim.treesitter.start, args.buf, "typescript") then
        -- No tree-sitter parser for TypeScript: use Vim's TypeScript syntax.
        vim.bo[args.buf].syntax = "typescript"
      end
      local root = vim.fs.root(args.buf, { "tsconfig.json", "package.json", ".git" })
      vim.lsp.start({
        name = "vibelang",
        cmd = { opts.cmd or esc_command(root), "--lsp", "--stdio" },
        root_dir = root,
      }, { bufnr = args.buf })
    end,
  })
end

return M
