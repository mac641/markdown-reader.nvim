local M = {}

---@class markdown-reader.Config
M.defaults = {
  -- Path to the mdreader viewer. nil uses <plugin>/bin/mdreader, built by `make`.
  binary = nil,
  -- Update the preview while typing; false updates on :write only.
  live = true,
  -- Delay after the last edit before the preview re-renders.
  debounce_ms = 150,
  -- Keep the preview on the part of the document the Neovim window shows,
  -- with the cursor line in view.
  follow_cursor = true,
  -- Start the viewer with --debug (enables the web inspector).
  debug = false,
}

---@type markdown-reader.Config
M.options = vim.deepcopy(M.defaults)

---@param opts? table
function M.set(opts)
  M.options = vim.tbl_deep_extend("force", vim.deepcopy(M.defaults), opts or {})
end

return M
