local api = require("mairu.api")
local config = require("mairu.config")

local M = {}

local ns_id = vim.api.nvim_create_namespace("mairu_autocomplete")
local current_completion = nil
local extmark_id = nil
local is_fetching = false

-- Get lines around cursor
local function get_context()
  local r, c = unpack(vim.api.nvim_win_get_cursor(0))
  local row = r - 1
  local col = c
  
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)
  
  local prefix_lines = {}
  for i = 1, row do
    table.insert(prefix_lines, lines[i])
  end
  local current_line_prefix = string.sub(lines[row + 1] or "", 1, col)
  table.insert(prefix_lines, current_line_prefix)
  
  local suffix_lines = {}
  local current_line_suffix = string.sub(lines[row + 1] or "", col + 1)
  table.insert(suffix_lines, current_line_suffix)
  for i = row + 2, #lines do
    table.insert(suffix_lines, lines[i])
  end
  
  return {
    prefix = table.concat(prefix_lines, "\n"),
    suffix = table.concat(suffix_lines, "\n"),
    filename = vim.fn.expand("%:p")
  }
end

-- Clear ghost text
function M.clear()
  if extmark_id then
    vim.api.nvim_buf_del_extmark(0, ns_id, extmark_id)
    extmark_id = nil
  end
  current_completion = nil
end

-- Accept ghost text
function M.accept()
  if not current_completion then return false end
  
  local r, c = unpack(vim.api.nvim_win_get_cursor(0))
  local row = r - 1
  local col = c
  
  local completion_text = current_completion:gsub("\r", "")
  local lines = vim.split(completion_text, "\n", { plain = true })
  
  local line = vim.api.nvim_buf_get_lines(0, row, row + 1, false)[1] or ""
  local line_len = string.len(line)
  if col > line_len then col = line_len end
  
  vim.api.nvim_buf_set_text(0, row, col, row, col, lines)
  
  if #lines == 1 then
    vim.api.nvim_win_set_cursor(0, { row + 1, col + string.len(lines[1]) })
  else
    vim.api.nvim_win_set_cursor(0, { row + #lines, string.len(lines[#lines]) })
  end
  
  M.clear()
  return true
end

-- Show ghost text
local function show_ghost_text(text)
  M.clear()
  if not text or text == "" then return end
  
  local r, c = unpack(vim.api.nvim_win_get_cursor(0))
  local row = r - 1
  local col = c
  
  local completion_text = text:gsub("\r", "")
  local lines = vim.split(completion_text, "\n", { plain = true })
  
  local virt_text = { { lines[1], "Comment" } }
  local virt_lines = {}
  
  for i = 2, #lines do
    table.insert(virt_lines, { { lines[i], "Comment" } })
  end
  
  local opts = {
    virt_text = virt_text,
    virt_text_pos = "inline",
    hl_mode = "combine",
  }
  
  if #virt_lines > 0 then
    opts.virt_lines = virt_lines
  end
  
  extmark_id = vim.api.nvim_buf_set_extmark(0, ns_id, row, col, opts)
  current_completion = text
end

function M.trigger()
  if is_fetching then return end
  
  local ctx = get_context()
  
  is_fetching = true
  api.autocomplete({
    prefix = ctx.prefix,
    suffix = ctx.suffix,
    filename = ctx.filename
  }, function(data, err)
    is_fetching = false
    
    if err then
      -- optionally notify, but usually silent for autocomplete
      return
    end
    
    if data and data.completion and data.completion ~= "" then
      show_ghost_text(data.completion)
    end
  end)
end

function M.setup()
  -- Clear completion when moving cursor or leaving insert mode
  vim.api.nvim_create_autocmd({ "CursorMovedI", "InsertLeave" }, {
    callback = function()
      M.clear()
    end,
  })
end

return M
