-- Plugin tests against a fake viewer. Run with `make test-lua`, i.e.
--   nvim --headless --clean -l tests/lua/test_plugin.lua
local root = vim.fn.getcwd()
vim.opt.rtp:prepend(root)

local mr = require("markdown-reader")
local log = vim.fn.tempname()
vim.env.MDR_FAKE_LOG = log
mr.setup({ binary = root .. "/tests/lua/fake-viewer.sh", debounce_ms = 20 })

local failures = 0
local current = "?"

local function check(cond, msg)
  if not cond then
    failures = failures + 1
    io.stderr:write(("FAIL [%s] %s\n"):format(current, msg))
  end
end

local function read_log()
  local f = io.open(log, "rb")
  if not f then
    return ""
  end
  local s = f:read("*a")
  f:close()
  return s
end

local function wait_log(pattern)
  local found = vim.wait(2000, function()
    return read_log():find(pattern) ~= nil
  end, 10)
  check(found, ("log never matched %q; log:\n%s"):format(pattern, read_log()))
end

local function pid_alive(pid)
  return vim.uv.kill(pid, 0) == 0
end

local function test(name, fn)
  current = name
  os.remove(log)
  vim.cmd("silent! %bwipeout!")
  fn()
  mr.close_all()
end

local scratch_count = 0

--- A new buffer with a unique name in tests/fixtures.
local function scratch(lines)
  scratch_count = scratch_count + 1
  vim.cmd.enew()
  local buf = vim.api.nvim_get_current_buf()
  vim.api.nvim_buf_set_name(buf, ("%s/tests/fixtures/doc%d.md"):format(root, scratch_count))
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  return buf
end

-- Capture notifications instead of printing them.
local notes = {}
vim.notify = function(msg, level)
  table.insert(notes, { msg = msg, level = level })
end

test("open sends arguments, content and cursor position", function()
  local buf = scratch({ "# Title", "", "body" })
  vim.api.nvim_win_set_cursor(0, { 3, 0 })
  mr.open()
  check(mr.is_open(buf), "not open")
  wait_log("SCROLL 3 %d+\n")
  local s = read_log()
  local args = "ARGS --stdin " .. vim.api.nvim_buf_get_name(buf) .. "\n"
  check(s:find(args, 1, true) ~= nil, "args: " .. s)
  check(s:find("CONTENT 14\n# Title\n\nbody\n", 1, true) ~= nil, "content: " .. s)
end)

test("edits are debounced into one update", function()
  local buf = scratch({ "a" })
  mr.open()
  wait_log("CONTENT 2\na\n")
  for i = 1, 5 do
    vim.api.nvim_buf_set_lines(buf, 0, -1, false, { "edit " .. i })
    vim.cmd.doautocmd("TextChanged")
  end
  wait_log("CONTENT 7\nedit 5\n")
  local _, updates = read_log():gsub("CONTENT", "")
  check(updates == 2, ("expected 2 CONTENT messages, got %d"):format(updates))
end)

test("cursor moves send SCROLL only when the position changes", function()
  scratch({ "1", "2", "3", "4" })
  mr.open()
  wait_log("SCROLL 1 ")
  vim.api.nvim_win_set_cursor(0, { 4, 0 })
  vim.cmd.doautocmd("CursorMoved")
  vim.cmd.doautocmd("CursorMoved")
  wait_log("SCROLL 4 ")
  local _, n = read_log():gsub("SCROLL 4 ", "")
  check(n == 1, ("expected one SCROLL 4, got %d"):format(n))
end)

test("live = false updates on write only", function()
  mr.setup({ binary = root .. "/tests/lua/fake-viewer.sh", debounce_ms = 20, live = false })
  local buf = scratch({ "a" })
  mr.open()
  wait_log("CONTENT 2\na\n")
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, { "b" })
  vim.cmd.doautocmd("TextChanged")
  vim.wait(100)
  check(not read_log():find("CONTENT 2\nb\n", 1, true), "updated before write")
  vim.cmd.doautocmd("BufWritePost")
  wait_log("CONTENT 2\nb\n")
  mr.setup({ binary = root .. "/tests/lua/fake-viewer.sh", debounce_ms = 20 })
end)

test("close sends QUIT and closes stdin", function()
  local buf = scratch({ "x" })
  mr.open()
  wait_log("CONTENT")
  mr.close()
  check(not mr.is_open(buf), "still open")
  wait_log("QUIT\nEOF\n")
end)

test("toggle and wiping the buffer close the viewer", function()
  local buf = scratch({ "x" })
  mr.toggle()
  check(mr.is_open(buf), "toggle did not open")
  mr.toggle()
  check(not mr.is_open(buf), "toggle did not close")
  mr.open()
  wait_log("CONTENT")
  vim.cmd("bwipeout! " .. buf)
  check(not mr.is_open(buf), "open after wipeout")
  wait_log("QUIT\nEOF\n")
end)

test("a viewer that ignores QUIT and SIGTERM gets killed", function()
  vim.env.MDR_FAKE_STUBBORN = "1"
  scratch({ "x" })
  mr.open()
  wait_log("CONTENT")
  local pid = mr._state(0).viewer.proc.pid
  mr.close()
  wait_log("EOF\n")
  check(pid_alive(pid), "exited before escalation was needed")
  local gone = vim.wait(3000, function()
    return not pid_alive(pid)
  end, 20)
  check(gone, "still running after SIGKILL deadline")
  vim.env.MDR_FAKE_STUBBORN = nil
end)

test("a viewer that exits on its own is detached", function()
  local buf = scratch({ "x" })
  mr.open()
  wait_log("CONTENT")
  -- Like closing the window, which makes a real viewer exit.
  vim.uv.kill(mr._state(buf).viewer.proc.pid, "sigterm")
  check(vim.wait(2000, function()
    return not mr.is_open(buf)
  end, 10), "still attached")
end)

test("a crashing viewer is reported", function()
  vim.env.MDR_FAKE_FAIL = "1"
  local buf = scratch({ "x" })
  notes = {}
  mr.open()
  check(vim.wait(2000, function()
    return not mr.is_open(buf)
  end, 10), "still attached")
  vim.wait(50)
  local msg = notes[1] and notes[1].msg or ""
  check(msg:find("exited with code 3") and msg:find("fake viewer failed"), "notification: " .. msg)
  vim.env.MDR_FAKE_FAIL = nil
end)

test("OPEN lines open URLs and Markdown files", function()
  local opened
  local ui_open = vim.ui.open
  vim.ui.open = function(url)
    opened = url
  end

  vim.env.MDR_FAKE_OPEN = "https://neovim.io/"
  scratch({ "x" })
  mr.open()
  check(vim.wait(2000, function()
    return opened ~= nil
  end, 10), "vim.ui.open not called")
  check(opened == "https://neovim.io/", "opened " .. tostring(opened))
  mr.close_all()

  vim.env.MDR_FAKE_OPEN = "file://" .. root .. "/tests/fixtures/other%20file.md#section"
  scratch({ "x" })
  mr.open()
  local target = root .. "/tests/fixtures/other file.md"
  check(vim.wait(2000, function()
    return vim.api.nvim_buf_get_name(0) == target
  end, 10), "did not :edit " .. target .. ", at " .. vim.api.nvim_buf_get_name(0))

  vim.env.MDR_FAKE_OPEN = nil
  vim.ui.open = ui_open
end)

test("missing binary is reported", function()
  mr.setup({ binary = "/nonexistent/mdreader" })
  notes = {}
  local buf = scratch({ "x" })
  mr.open()
  check(not mr.is_open(buf), "opened")
  check(notes[1] and notes[1].msg:find("not executable"), "no error notification")
  mr.setup({ binary = root .. "/tests/lua/fake-viewer.sh", debounce_ms = 20 })
end)

test(":MarkdownReader command", function()
  mr.setup({ binary = root .. "/tests/lua/fake-viewer.sh", debounce_ms = 20 })
  local buf = scratch({ "x" })
  vim.cmd("MarkdownReader")
  check(mr.is_open(buf), "toggle did not open")
  vim.cmd("MarkdownReader close")
  check(not mr.is_open(buf), "close did not close")
  vim.cmd("MarkdownReader open")
  check(mr.is_open(buf), "open did not open")
  check(vim.deep_equal(vim.fn.getcompletion("MarkdownReader o", "cmdline"), { "open" }), "completion")
end)

test("VimLeavePre closes all viewers", function()
  local a, b = scratch({ "a" }), scratch({ "b" })
  mr.open(a)
  mr.open(b)
  vim.api.nvim_exec_autocmds("VimLeavePre", {})
  check(not mr.is_open(a) and not mr.is_open(b), "viewers still open")
end)

-- Let the last viewers exit before Neovim does.
vim.wait(200)
if failures > 0 then
  io.stderr:write(("%d check(s) failed\n"):format(failures))
  os.exit(1)
end
print("all plugin tests passed")
