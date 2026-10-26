local config = require("markdown-reader.config")
local Viewer = require("markdown-reader.viewer")

local M = {}

---@class markdown-reader.State
---@field viewer markdown-reader.Viewer
---@field augroup integer
---@field timer uv.uv_timer_t debounces content updates
---@field last_scroll? string last SCROLL arguments sent

---@type table<integer, markdown-reader.State>
local states = {}

local function notify(msg, level)
  vim.notify("markdown-reader: " .. msg, level or vim.log.levels.INFO)
end

local function plugin_root()
  local file = debug.getinfo(1, "S").source:sub(2)
  return vim.fn.fnamemodify(file, ":p:h:h:h")
end

---@return string? path
---@return string? err
local function find_binary()
  local bin = config.options.binary or (plugin_root() .. "/bin/mdreader")
  if vim.fn.executable(bin) == 1 then
    return bin
  end
  if config.options.binary then
    return nil, ("viewer not executable: %s"):format(bin)
  end
  return nil, ("viewer not built; run `make` in %s"):format(plugin_root())
end

---@param buf? integer
---@return integer
local function resolve(buf)
  return (buf == nil or buf == 0) and vim.api.nvim_get_current_buf() or buf
end

local function push_content(buf)
  local state = states[buf]
  local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
  state.viewer:content(table.concat(lines, "\n") .. "\n")
end

--- Sends the cursor position of the current window, if it shows buf.
---@param force? boolean send even if unchanged
local function push_scroll(buf, force)
  local state = states[buf]
  local win = vim.api.nvim_get_current_win()
  if not config.options.follow_cursor or vim.api.nvim_win_get_buf(win) ~= buf then
    return
  end
  local line = vim.api.nvim_win_get_cursor(win)[1]
  local row = vim.api.nvim_win_call(win, vim.fn.winline)
  local height = vim.api.nvim_win_get_height(win)
  local percent = math.floor((row - 1) / math.max(height - 1, 1) * 100 + 0.5)
  percent = math.max(0, math.min(100, percent))

  local key = line .. " " .. percent
  if force or key ~= state.last_scroll then
    state.last_scroll = key
    state.viewer:scroll(line, percent)
  end
end

--- Re-renders after debounce_ms without further edits.
local function schedule_content(buf)
  local state = states[buf]
  state.timer:stop()
  state.timer:start(
    config.options.debounce_ms,
    0,
    vim.schedule_wrap(function()
      if states[buf] == state then
        push_content(buf)
        -- The layout changed, so the cursor's place in it may have too.
        push_scroll(buf, true)
      end
    end)
  )
end

---@param line string
local function on_viewer_line(line)
  local url = line:match("^OPEN (.+)$")
  if not url then
    return
  end
  local path = url:match("^file://") and vim.uri_to_fname((url:gsub("#.*$", "")))
  if path and (path:match("%.md$") or path:match("%.markdown$")) then
    vim.cmd.edit(vim.fn.fnameescape(path))
  else
    vim.ui.open(url)
  end
end

--- Forgets buf's viewer without stopping it.
local function detach(buf)
  local state = states[buf]
  if not state then
    return
  end
  states[buf] = nil
  pcall(vim.api.nvim_del_augroup_by_id, state.augroup)
  state.timer:stop()
  state.timer:close()
end

local function attach_autocmds(buf, state)
  local group = state.augroup
  local events = config.options.live and { "TextChanged", "TextChangedI", "TextChangedP" }
    or { "BufWritePost" }
  vim.api.nvim_create_autocmd(events, {
    group = group,
    buffer = buf,
    callback = function()
      schedule_content(buf)
    end,
  })
  vim.api.nvim_create_autocmd({ "CursorMoved", "CursorMovedI", "WinScrolled", "BufWinEnter" }, {
    group = group,
    buffer = buf,
    callback = function()
      push_scroll(buf)
    end,
  })
  vim.api.nvim_create_autocmd("BufUnload", {
    group = group,
    buffer = buf,
    callback = function()
      M.close(buf)
    end,
  })
end

--- Opens a preview of buf (default: current buffer).
---@param buf? integer
function M.open(buf)
  buf = resolve(buf)
  if states[buf] then
    return
  end
  local bin, err = find_binary()
  if not bin then
    notify(err, vim.log.levels.ERROR)
    return
  end

  -- The viewer only uses the name for its title and to resolve relative
  -- links and images, so an unnamed buffer gets one in the cwd.
  local name = vim.api.nvim_buf_get_name(buf)
  if name == "" then
    name = vim.fn.getcwd() .. "/untitled.md"
  end
  local cmd = { bin, "--stdin" }
  if config.options.debug then
    table.insert(cmd, "--debug")
  end
  table.insert(cmd, name)

  local state = {}
  local viewer, start_err = Viewer.start(cmd, {
    on_line = on_viewer_line,
    on_exit = function(code, signal, stderr)
      if states[buf] == state then
        detach(buf)
      end
      if not state.viewer.closing and (code ~= 0 or signal ~= 0) then
        local how = signal ~= 0 and ("signal %d"):format(signal) or ("code %d"):format(code)
        notify(("viewer exited with %s\n%s"):format(how, vim.trim(stderr)), vim.log.levels.ERROR)
      end
    end,
  })
  if not viewer then
    notify(("could not start %s: %s"):format(bin, start_err), vim.log.levels.ERROR)
    return
  end
  state.viewer = viewer
  state.augroup = vim.api.nvim_create_augroup("markdown-reader." .. buf, { clear = true })
  state.timer = assert(vim.uv.new_timer())
  states[buf] = state

  attach_autocmds(buf, state)
  -- The viewers would also quit on stdin EOF once Neovim is gone; this just
  -- makes it immediate.
  vim.api.nvim_create_autocmd("VimLeavePre", {
    group = vim.api.nvim_create_augroup("markdown-reader", { clear = true }),
    callback = M.close_all,
  })
  push_content(buf)
  push_scroll(buf, true)
end

--- Closes buf's preview (default: current buffer).
---@param buf? integer
function M.close(buf)
  buf = resolve(buf)
  local state = states[buf]
  if state then
    detach(buf)
    state.viewer:close()
  end
end

---@param buf? integer
function M.toggle(buf)
  buf = resolve(buf)
  if states[buf] then
    M.close(buf)
  else
    M.open(buf)
  end
end

---@param buf? integer
---@return boolean
function M.is_open(buf)
  return states[resolve(buf)] ~= nil
end

--- For tests.
---@param buf? integer
---@return markdown-reader.State?
function M._state(buf)
  return states[resolve(buf)]
end

function M.close_all()
  for buf in pairs(states) do
    M.close(buf)
  end
end

local actions = { "toggle", "open", "close" }

--- Applies opts and defines :MarkdownReader.
---@param opts? markdown-reader.Config
function M.setup(opts)
  config.set(opts)
  vim.api.nvim_create_user_command("MarkdownReader", function(o)
    local action = o.fargs[1] or "toggle"
    if not vim.tbl_contains(actions, action) then
      notify("unknown action " .. action, vim.log.levels.ERROR)
      return
    end
    M[action]()
  end, {
    nargs = "?",
    desc = "Markdown preview: toggle (default), open or close",
    complete = function(lead)
      return vim.tbl_filter(function(a)
        return a:find(lead, 1, true) == 1
      end, actions)
    end,
  })
end

return M
