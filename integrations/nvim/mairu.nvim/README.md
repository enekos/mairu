# mairu.nvim

Neovim plugin for the Mairu AI Coding Agent. Integrates deep context, memory search, and agentic interactions directly into your editor (optimized for LazyVim).

## Features

- **Contextual Search (`<leader>ms`)**: Hit a keybinding on a symbol, and Mairu finds relevant memories and context nodes in a floating popup.
- **Command Palette (`<leader>mc`)**: Telescope-based command palette to store memories, perform natural language vibe queries, and manage context.
- **Ambient Context Sidebar (`<leader>mb`)**: Right-side panel that automatically updates to show relevant project context/memories based on the file you're currently editing.
- **Chat Interaction (`<leader>ma`)**: Split window layout for continuous chat with Mairu via the `vibe-query` API.

## Requirements

- Neovim >= 0.9.0
- `mairu` built and accessible in your `PATH` (or specified in config)
- `plenary.nvim` (Async HTTP)
- `nui.nvim` (UI Components)
- `telescope.nvim` (Command Palette)

## Installation (Lazy.nvim)

```lua
{
  "enekosarasola/mairu",
  dir = "~/mairu/integrations/nvim/mairu.nvim", -- Update with actual local path or github URL when published
  dependencies = {
    "nvim-lua/plenary.nvim",
    "MunifTanjim/nui.nvim",
    "nvim-telescope/telescope.nvim",
  },
  config = function()
    require("mairu").setup({
      server = {
        auto_start = true, -- Automatically start headless mairu context-server
        bin_path = "mairu", -- Make sure this is in your PATH or provide absolute path
        port = 8788,
      },
      ambient = {
        enabled = true,
        debounce_ms = 800,
        width = 40,
      }
    })
    
    -- Load default keymaps
    require("mairu").set_default_keymaps()
  end,
}
```

## Keybindings

If `set_default_keymaps()` is called:

| Key | Mode | Action |
|-----|------|--------|
| `<leader>ms` | Normal | Search Context (Word under cursor) |
| `<leader>ms` | Visual | Search Context (Selected text) |
| `<leader>mc` | Normal | Open Mairu Command Palette (Telescope) |
| `<leader>mb` | Normal | Toggle Ambient Context Sidebar |
| `<leader>ma` | Normal | Open Mairu Chat |

## How it works

The plugin operates by starting a background headless `mairu context-server` job when Neovim opens. It uses `plenary.curl` to communicate asynchronously over local HTTP APIs (`/api/search`, `/api/vibe/query`, etc.), ensuring Neovim's UI never blocks.
