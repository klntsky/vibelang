-- Headless check of effectscript.lua, run by editors/nvim/test.sh on a copy of
-- typescript/effectscript/lsp-fixture whose node_modules/.bin/esc is the
-- compiler under test:
--
--   nvim --headless --clean -l editors/nvim/test.lua <project> <effectscript.lua>
local project, config = arg[1], arg[2]
vim.cmd("filetype on")
dofile(config).setup()

local failures = 0
local function check(name, body)
  local ok, err = pcall(body)
  if ok then
    io.stdout:write("ok   " .. name .. "\n")
  else
    failures = failures + 1
    io.stdout:write("FAIL " .. name .. ": " .. tostring(err) .. "\n")
  end
end

local function edit(relative)
  vim.cmd.edit(vim.fs.joinpath(project, relative))
  return vim.api.nvim_get_current_buf()
end

local function wait_for(what, predicate)
  if not vim.wait(90000, predicate, 100) then
    error("timed out waiting for " .. what)
  end
end

local function client_of(buf)
  wait_for("esc to attach", function()
    return #vim.lsp.get_clients({ bufnr = buf, name = "esc" }) > 0
  end)
  return vim.lsp.get_clients({ bufnr = buf, name = "esc" })[1]
end

local function has_diagnostic(buf, code, source)
  for _, d in ipairs(vim.diagnostic.get(buf)) do
    if d.code == code and (source == nil or d.source == source) then
      return true
    end
  end
  return false
end

local function position_of(buf, needle)
  for row, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
    local col = line:find(needle, 1, true)
    if col then
      return { line = row - 1, character = col - 1 }
    end
  end
  error("no " .. needle)
end

check("filetype: .es files are effectscript, highlighted as TypeScript", function()
  local buf = edit("src/broken.es")
  assert(vim.bo[buf].filetype == "effectscript", "filetype " .. vim.bo[buf].filetype)
  assert(vim.bo[buf].syntax == "typescript" or vim.treesitter.highlighter.active[buf], "no highlighting")
end)

check("lsp: the project's node_modules/.bin/esc attaches", function()
  local client = client_of(vim.api.nvim_get_current_buf())
  local cmd = client.config.cmd[1]
  assert(cmd:sub(-#"node_modules/.bin/esc") == "node_modules/.bin/esc", "command " .. cmd)
end)

check("diagnostics: TS2322 on broken.es", function()
  local buf = vim.api.nvim_get_current_buf()
  wait_for("TS2322", function()
    return has_diagnostic(buf, 2322)
  end)
end)

check("diagnostics: ES1003 with source ES on wait.es", function()
  local buf = edit("src/wait.es")
  wait_for("ES1003", function()
    return has_diagnostic(buf, 1003, "ES")
  end)
end)

check("hover: a call that runs shows the contract and the success value", function()
  local buf = edit("src/main.es")
  client_of(buf)
  local params = { textDocument = vim.lsp.util.make_text_document_params(buf), position = position_of(buf, "lookup(1)") }
  local text = ""
  for _, response in pairs(vim.lsp.buf_request_sync(buf, "textDocument/hover", params, 30000) or {}) do
    local contents = response.result and response.result.contents
    if contents then
      text = text .. (contents.value or vim.inspect(contents))
    end
  end
  assert(text:find("Effect<string, NotFound, never>", 1, true), "hover " .. text)
  assert(text:find("Runs here and returns `string`", 1, true), "hover " .. text)
end)

for _, client in ipairs(vim.lsp.get_clients()) do
  client.stop()
end
vim.wait(5000, function()
  return #vim.lsp.get_clients() == 0
end)
os.exit(failures > 0 and 1 or 0)
