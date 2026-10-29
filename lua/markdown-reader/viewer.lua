--- One running mdreader process and the protocol spoken with it (see
--- internal/ipc).
---@class markdown-reader.Viewer
---@field proc vim.SystemObj
---@field closing boolean
---@field exited boolean
local Viewer = {}
Viewer.__index = Viewer

-- How long to wait for a viewer to quit before sending SIGTERM, and how
-- long after that before SIGKILL.
local QUIT_TIMEOUT_MS = 500
local TERM_TIMEOUT_MS = 1000

---@class markdown-reader.ViewerHandlers
---@field on_line fun(line: string) a line the viewer printed to stdout
---@field on_exit fun(code: integer, signal: integer, stderr: string)

--- Starts a viewer. Handlers run on the main loop.
---@param cmd string[]
---@param handlers markdown-reader.ViewerHandlers
---@return markdown-reader.Viewer? viewer
---@return string? err
function Viewer.start(cmd, handlers)
  local self = setmetatable({ closing = false, exited = false }, Viewer)
  local partial = ""
  local stderr = {}

  local ok, proc = pcall(vim.system, cmd, {
    stdin = true,
    stdout = function(_, data)
      if not data then
        return
      end
      partial = partial .. data
      for line in partial:gmatch("([^\n]*)\n") do
        vim.schedule(function()
          handlers.on_line(line)
        end)
      end
      partial = partial:match("[^\n]*$")
    end,
    stderr = function(_, data)
      if data then
        table.insert(stderr, data)
      end
    end,
  }, function(res)
    vim.schedule(function()
      self.exited = true
      handlers.on_exit(res.code, res.signal, table.concat(stderr))
    end)
  end)
  if not ok then
    return nil, tostring(proc)
  end
  self.proc = proc
  return self
end

---@param data string
function Viewer:send(data)
  if self.exited or self.closing then
    return
  end
  -- The pipe may already be gone if the viewer died; on_exit reports that.
  pcall(self.proc.write, self.proc, data)
end

---@param text string Markdown
function Viewer:content(text)
  self:send(("CONTENT %d\n"):format(#text) .. text)
end

--- Tells the viewer the document is now the file at path.
---@param path string
function Viewer:file(path)
  self:send(("FILE %d\n%s\n"):format(#path, path))
end

--- Tells the viewer what the editor shows: source lines top to bottom, with
--- the cursor on line cursor (all 1-based).
---@param top integer
---@param bottom integer
---@param cursor integer
function Viewer:scroll(top, bottom, cursor)
  self:send(("SCROLL %d %d %d\n"):format(top, bottom, cursor))
end

--- Asks the viewer to quit and makes sure it does: QUIT plus closing stdin,
--- then SIGTERM, then SIGKILL.
function Viewer:close()
  if self.exited or self.closing then
    return
  end
  self:send("QUIT\n")
  self.closing = true
  pcall(self.proc.write, self.proc, nil)

  vim.defer_fn(function()
    if self.exited then
      return
    end
    self.proc:kill("sigterm")
    vim.defer_fn(function()
      if not self.exited then
        self.proc:kill("sigkill")
      end
    end, TERM_TIMEOUT_MS)
  end, QUIT_TIMEOUT_MS)
end

return Viewer
