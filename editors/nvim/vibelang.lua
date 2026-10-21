-- VibeLang for Neovim 0.10 or later, with the built-in LSP client.
--
-- Load this file from the VibeLang repository, so it finds the tree-sitter
-- grammar in editors/tree-sitter-vibelang next to it:
--
--   dofile("/path/to/effectscript/editors/nvim/vibelang.lua").setup()
--
-- .vibe files then get the vibe filetype, highlighting, and
-- `vibelang --lsp --stdio` as their language server: the project's
-- node_modules/.bin/vibelang, else vibelang on PATH. Highlighting uses the
-- vibe parser once it is installed (:TSInstall vibe with nvim-treesitter),
-- else tree-sitter's TypeScript parser, else Vim's TypeScript syntax.
-- setup({ grammar = "/path/to/editors/tree-sitter-vibelang" }) names the
-- grammar when this file is copied elsewhere.
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

-- grammar_dir returns the tree-sitter-vibelang directory: opts.grammar, else
-- the one next to this file's directory, or nil when there is none.
local function grammar_dir(opts)
  local dir = opts.grammar
  if not dir then
    local source = debug.getinfo(1, "S").source
    if source:sub(1, 1) ~= "@" then
      return nil
    end
    dir = vim.fs.joinpath(vim.fs.dirname(source:sub(2)), "..", "tree-sitter-vibelang")
  end
  dir = vim.uv.fs_realpath(vim.fn.expand(dir))
  if dir and vim.uv.fs_stat(vim.fs.joinpath(dir, "src", "parser.c")) then
    return dir
  end
  return nil
end

-- highlight_language returns the parser that highlights .vibe files: vibe
-- when it is installed, else typescript.
local function highlight_language()
  if pcall(vim.treesitter.language.add, "vibe") then
    return "vibe"
  end
  return "typescript"
end

function M.setup(opts)
  opts = opts or {}
  vim.filetype.add({ extension = { vibe = "vibe" } })

  local grammar = grammar_dir(opts)
  if grammar then
    -- The grammar's queries/vibe/*.scm, which inherit TypeScript's.
    vim.opt.runtimepath:append(grammar)
    local ok, parsers = pcall(require, "nvim-treesitter.parsers")
    if ok and type(parsers.get_parser_configs) == "function" then
      -- :TSInstall vibe compiles the grammar's src/ into nvim-treesitter's parsers.
      parsers.get_parser_configs().vibe = {
        install_info = { url = grammar, files = { "src/parser.c", "src/scanner.c" } },
        filetype = "vibe",
      }
    end
  end
  vim.treesitter.language.register(highlight_language(), "vibe")

  local filetypes = { "vibe" }
  if opts.typescript then
    vim.list_extend(filetypes, typescript_filetypes)
  end
  vim.api.nvim_create_autocmd("FileType", {
    group = vim.api.nvim_create_augroup("vibelang", { clear = true }),
    pattern = filetypes,
    callback = function(args)
      if vim.bo[args.buf].filetype == "vibe" then
        -- A parser installed since setup takes over from TypeScript's.
        local lang = highlight_language()
        vim.treesitter.language.register(lang, "vibe")
        if not pcall(vim.treesitter.start, args.buf, lang) then
          -- No tree-sitter parser for VibeLang or TypeScript: use Vim's TypeScript syntax.
          vim.bo[args.buf].syntax = "typescript"
        end
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
