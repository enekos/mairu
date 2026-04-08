local M = {}

M.options = {
  -- Server settings
  server = {
    auto_start = true,
    bin_path = "mairu", -- Assumes mairu is in PATH, or provide absolute path
    port = 8788,
    project = vim.fn.fnamemodify(vim.fn.getcwd(), ":t"), -- Current directory name
  },
  
  -- Ambient context settings
  ambient = {
    enabled = true,
    debounce_ms = 800,
    width = 40, -- Width of the sidebar
  },
  
  -- API settings
  api = {
    timeout = 10000, -- 10 seconds
  },
}

function M.setup(opts)
  M.options = vim.tbl_deep_extend("force", M.options, opts or {})
  -- Update project dynamically based on actual cwd if not overridden
  if not opts or not opts.server or not opts.server.project then
    M.options.server.project = vim.fn.fnamemodify(vim.fn.getcwd(), ":t")
  end
end

return M
