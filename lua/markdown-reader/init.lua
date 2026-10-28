local config = require("markdown-reader.config")
local Viewer = require("markdown-reader.viewer")

local M = {}

---@class markdown-reader.State
---@field buf integer the buffer shown; changes when a link is followed
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
  if buf == nil or buf == 0 then
    return vim.api.nvim_get_current_buf()
  end
  return buf
end

local function push_content(buf)
  local state = states[buf]
  local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
  state.viewer:content(table.concat(lines, "\n") .. "\n")
end

--- Sends what the current window shows of buf, if it shows buf: its first
--- line and the cursor's line.
---@param force? boolean send even if unchanged
local function push_scroll(buf, force)
  local state = states[buf]
  local win = vim.api.nvim_get_current_win()
  if not config.options.follow_cursor or vim.api.nvim_win_get_buf(win) ~= buf then
    return
  end
  local top = vim.fn.line("w0", win)
  local cursor = vim.api.nvim_win_get_cursor(win)[1]

  local key = top .. " " .. cursor
  if force or key ~= state.last_scroll then
    state.last_scroll = key
    state.viewer:scroll(top, cursor)
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

---@param buf integer
---@return string
local function document_name(buf)
  -- The viewer only uses the name for its title and to resolve relative
  -- links and images, so an unnamed buffer gets one in the cwd.
  local name = vim.api.nvim_buf_get_name(buf)
  return name ~= "" and name or vim.fn.getcwd() .. "/untitled.md"
end

local function attach_autocmds(buf, state)
  state.augroup = vim.api.nvim_create_augroup("markdown-reader." .. buf, { clear = true })
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

--- Moves a viewer from its buffer to buf, which then shows in it.
---@param state markdown-reader.State
---@param buf integer
local function retarget(state, buf)
  pcall(vim.api.nvim_del_augroup_by_id, state.augroup)
  states[state.buf] = nil
  state.buf = buf
  state.last_scroll = nil
  states[buf] = state
  attach_autocmds(buf, state)
  state.viewer:file(document_name(buf))
  push_content(buf)
  push_scroll(buf, true)
end

--- The 1-based line of the heading with anchor ID id in buf, using the
--- viewer's own IDs so both sides agree.
---@param buf integer
---@param id string
---@return integer?
local function find_heading(buf, id)
  local bin = find_binary()
  if not bin then
    return nil
  end
  local text = table.concat(vim.api.nvim_buf_get_lines(buf, 0, -1, false), "\n") .. "\n"
  local ok, res = pcall(function()
    return vim.system({ bin, "--headings" }, { stdin = text, text = true }):wait(2000)
  end)
  if not ok or res.code ~= 0 then
    return nil
  end
  for line, heading in res.stdout:gmatch("(%d+) ([^\n]*)") do
    if heading == id then
      return tonumber(line)
    end
  end
end

--- Follows a link clicked in state's viewer: Markdown files open in the
--- window showing the previewed buffer, at the linked heading if there is
--- one, and the viewer moves along; everything else goes to vim.ui.open.
---@param state markdown-reader.State
---@param url string
local function follow_link(state, url)
  local file, fragment = url:match("^(file://[^#]*)#?(.*)$")
  local path = file and vim.uri_to_fname(file)
  if not (path and (path:match("%.md$") or path:match("%.markdown$"))) then
    vim.ui.open(url)
    return
  end

  local win = vim.fn.bufwinid(state.buf)
  if win == -1 then
    win = vim.api.nvim_get_current_win()
  end
  -- :edit of the file already shown would fail if it has unsaved changes.
  if vim.api.nvim_buf_get_name(vim.api.nvim_win_get_buf(win)) ~= vim.fn.fnamemodify(path, ":p") then
    local ok, err = pcall(vim.api.nvim_win_call, win, function()
      vim.cmd.edit(vim.fn.fnameescape(path))
    end)
    if not ok then
      notify(tostring(err), vim.log.levels.ERROR)
      return
    end
  end
  local buf = vim.api.nvim_win_get_buf(win)

  if fragment ~= "" then
    local id = vim.uri_decode(fragment)
    local line = find_heading(buf, id)
    if line then
      vim.api.nvim_win_set_cursor(win, { line, 0 })
      vim.api.nvim_win_call(win, function()
        vim.cmd("normal! zt")
      end)
    else
      notify(("no heading #%s in %s"):format(id, vim.fn.fnamemodify(path, ":~:.")), vim.log.levels.WARN)
    end
  end

  if buf == state.buf then
    push_scroll(buf, true)
  elseif states[buf] then
    -- buf has a preview of its own already.
    M.close(state.buf)
  else
    retarget(state, buf)
  end
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

  local cmd = { bin, "--stdin" }
  if config.options.debug then
    table.insert(cmd, "--debug")
  end
  table.insert(cmd, document_name(buf))

  local state = { buf = buf }
  local viewer, start_err = Viewer.start(cmd, {
    on_line = function(line)
      local url = line:match("^OPEN (.+)$")
      if url and states[state.buf] == state then
        follow_link(state, url)
      end
    end,
    on_exit = function(code, signal, stderr)
      if states[state.buf] == state then
        detach(state.buf)
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

--- For :checkhealth.
M._find_binary = find_binary

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
