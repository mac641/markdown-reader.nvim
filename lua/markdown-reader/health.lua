-- :checkhealth markdown-reader
local M = {}

local health = vim.health

function M.check()
  health.start("markdown-reader")

  if vim.fn.has("nvim-0.12") == 1 then
    local v = vim.version()
    health.ok(("Neovim %d.%d.%d"):format(v.major, v.minor, v.patch))
  else
    health.error("Neovim 0.12 or newer is required")
  end

  if vim.fn.exists(":MarkdownReader") == 2 then
    health.ok("setup() was called")
  else
    health.warn("setup() was not called, so :MarkdownReader is missing", {
      'call require("markdown-reader").setup(), or set `opts = {}` in your lazy.nvim spec',
    })
  end

  if vim.uv.os_uname().sysname == "Linux" then
    if vim.env.WAYLAND_DISPLAY and vim.env.WAYLAND_DISPLAY ~= "" then
      health.ok("Wayland session: " .. vim.env.WAYLAND_DISPLAY)
    else
      health.error("no Wayland session (WAYLAND_DISPLAY is not set); X11 isn't supported", {
        "start Neovim from a Wayland desktop session",
        "under tmux or ssh, make sure WAYLAND_DISPLAY is passed on",
      })
    end
  end

  local bin, err = require("markdown-reader")._find_binary()
  if not bin then
    -- Without a path, there's always an error.
    health.error(err --[[@as string]])
    return
  end
  -- --headings opens no window but loads the viewer with all its libraries,
  -- so a missing GTK or WebKit shows up here.
  local ok, res = pcall(function()
    return vim.system({ bin, "--headings" }, { stdin = "# x\n", text = true }):wait(5000)
  end)
  if not ok then
    health.error(("could not run %s: %s"):format(bin, res))
  elseif res.code ~= 0 or res.stdout ~= "1 x\n" then
    health.error(("%s does not work (exit code %d):\n%s"):format(bin, res.code, vim.trim(res.stderr or "")), {
      "rebuild it with `make` in the plugin's directory; `make` lists the packages it needs",
    })
  else
    health.ok("viewer works: " .. bin)
  end
end

return M
