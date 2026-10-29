# markdown-reader.nvim

A Markdown preview for Neovim that renders in a native window instead of a browser tab. The preview updates while you
type and scrolls along with your cursor.

- GitHub-flavoured Markdown: tables, task lists, strikethrough, autolinks, footnotes and alerts (`> [!NOTE]`)
- Syntax highlighting of fenced code blocks, in GitHub's colors
- Live updates while typing (debounced), or on `:write` only
- Scroll sync: the preview shows the part of the document the Neovim window shows, glides along as you move, and keeps
  the cursor line in view
- Links: relative images and links resolve against the file's directory, `#heading` links scroll the preview, links to
  other Markdown files open in Neovim at the linked heading and the preview follows, all others open in your default
  application
- One preview window per buffer, closed together with the buffer or Neovim
- Light and dark theme, following the system setting

The preview is a small Go program, `mdreader`, that shows the rendered page in the system's web engine (WebKitGTK on
Linux, WKWebView on macOS). The Lua plugin starts and stops it and talks to it over a pipe.

## Requirements

- Neovim 0.12 or newer
- To build the viewer: Go 1.25 or newer, a C++ compiler, `make`, and on Linux `pkg-config` with the GTK 4 and WebKitGTK
  6.0 development files
- On Linux, a Wayland session; X11 isn't supported

## Getting started

### Install the build dependencies

**Ubuntu / Debian**

```sh
sudo apt install golang-go build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev
```

**Fedora**

```sh
sudo dnf install golang gcc-c++ make 'pkgconfig(gtk4)' 'pkgconfig(webkitgtk-6.0)'
```

**openSUSE Tumbleweed**

```sh
sudo zypper install go gcc-c++ make 'pkgconfig(gtk4)' 'pkgconfig(webkitgtk-6.0)'
```

**NixOS**

Nothing to install besides `make`: NixOS doesn't expose libraries' headers to builds, so on NixOS `make` runs itself
inside the repository's `shell.nix`, which provides Go, GTK 4 and WebKitGTK.

**macOS**

```sh
xcode-select --install   # C/C++ compiler, make
brew install go
```

WebKit ships with macOS.

### Install the plugin

With [lazy.nvim](https://github.com/folke/lazy.nvim):

```lua
{
  "mac641/markdown-reader.nvim",
  build = "make",
  cmd = "MarkdownReader",
  keys = {
    {
      "<leader>mp",
      function() require("markdown-reader").toggle() end,
      ft = "markdown",
      desc = "Toggle Markdown preview",
    },
    {
      "<leader>ms",
      function() require("markdown-reader").close() end,
      ft = "markdown",
      desc = "Close Markdown preview",
    },
  },
  opts = {},
}
```

`build = "make"` compiles the viewer into the plugin's `bin/mdreader`. `opts` (even empty) makes lazy.nvim call
`setup()`, which defines `:MarkdownReader`. The plugin sets no mappings of its own; the `keys` above are a suggestion
and load the plugin on first use.

With any other plugin manager, add the repository, run `make` in its directory after installing or updating it, and call
`setup()` from your config:

```lua
require("markdown-reader").setup()
vim.api.nvim_create_autocmd("FileType", {
  pattern = "markdown",
  callback = function(ev)
    vim.keymap.set("n", "<leader>mp", "<cmd>MarkdownReader<cr>", { buffer = ev.buf })
  end,
})
```

### 3. Check it

Run `:checkhealth markdown-reader`. It checks that `setup()` was called, that Neovim runs in a Wayland session on Linux,
and that the viewer is built and starts, which catches missing GTK or WebKit libraries. Then open a Markdown file and
run `:MarkdownReader`.

## Usage

|                                  |                                          |
| -------------------------------- | ---------------------------------------- |
| `:MarkdownReader`                | toggle the preview of the current buffer |
| `:MarkdownReader open` / `close` | open or close it                         |

From Lua: `require("markdown-reader").open()`, `.close()`, `.toggle()` and `.is_open()`, each taking an optional buffer
number.

`:help markdown-reader` has all of this README's user documentation inside Neovim.

## Configuration

`setup()` defines the command and takes these options (the defaults):

```lua
require("markdown-reader").setup({
  -- Path to the viewer. nil uses <plugin>/bin/mdreader, built by `make`.
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
})
```

## Window placement

The preview is a normal window, and your desktop decides where it goes: Wayland doesn't let programs place windows. On
Linux its app ID is `mdreader`, so window manager rules can match it.

**sway** opens it next to the focused window, i.e. beside Neovim. To keep the focus in Neovim:

```
no_focus [app_id="mdreader"]
```

**niri** opens it as a new column next to Neovim. To keep the focus in Neovim and give it half the screen:

```kdl
window-rule {
    match app-id="^mdreader$"
    open-focused false
    default-column-width { proportion 0.5; }
}
```

**COSMIC** tiles it next to Neovim when tiling is on for the workspace (Super+Y).

**GNOME** and **macOS** open it as a floating window. On GNOME, Super+→ tiles it to the right half of the screen.

## Development

### Setup

Install the build dependencies above, plus Neovim for the plugin tests. With Nix, `nix-shell` gives you all of them and
the linters (golangci-lint, shellcheck, deno).

```sh
git clone https://github.com/mac641/markdown-reader.nvim
cd markdown-reader.nvim
make          # builds bin/mdreader
make test     # Go tests, then the plugin tests in headless Neovim
```

To use your checkout in Neovim, point lazy.nvim at it:

```lua
{ dir = "~/path/to/markdown-reader.nvim", build = "make", cmd = "MarkdownReader", opts = {} }
```

or, without a plugin manager:

```vim
:set rtp+=~/path/to/markdown-reader.nvim | lua require("markdown-reader").setup()
```

Run `make` again after changing Go code; Lua changes take effect after restarting Neovim.

### Running the viewer by hand

```sh
bin/mdreader tests/fixtures/sample.md                 # show a file
bin/mdreader --render-only tests/fixtures/sample.md   # print the HTML body
bin/mdreader --headings < tests/fixtures/sample.md    # line and anchor ID of each heading
bin/mdreader --debug tests/fixtures/sample.md         # with the web inspector
```

`--debug` enables WebKit's inspector: right-click the page and choose _Inspect Element_ to see the DOM, the console and
page.js.

To speak the protocol yourself, as the plugin does:

```sh
{ printf 'CONTENT 8\n# Hello\n'; sleep 3; printf 'SCROLL 1 1 1\n'; sleep 3; printf 'QUIT\n'; } |
  bin/mdreader --stdin /tmp/hello.md
```

### Tests

|                 |                                                                                                                       |
| --------------- | --------------------------------------------------------------------------------------------------------------------- |
| `make test-go`  | unit tests of the protocol parser, page builder and renderer (`internal/*/..._test.go`); no WebKit or display needed  |
| `make test-lua` | the plugin, against `tests/lua/fake-viewer.sh`, a shell script that logs what it receives instead of opening a window |

Neither needs a display. CI (`.github/workflows/ci.yml`) runs both on Linux and macOS, plus golangci-lint, gofmt,
shellcheck and `deno fmt --check`. It also builds the viewer with each install command above (Ubuntu, Fedora and
openSUSE containers, and `shell.nix`). The window itself is checked by hand, e.g. with `tests/fixtures/sample.md`, which
uses every supported feature.

### External Dependencies

- [webview](https://github.com/webview/webview) (MIT), a C++ library without Go module, lives in `third_party/webview`
  and is pinned in `third_party/VERSIONS` (tag, commit and tarball hash). To update it:

  ```sh
  scripts/update-vendor.sh webview <tag>
  ```

### Commits

Use [Conventional Commits](https://www.conventionalcommits.org/).

## Architecture

### Overview

```
┌──────────── Neovim ────────────┐              ┌──────────── mdreader ─────────────┐
│ lua/markdown-reader/           │   stdin      │ readStdin goroutine               │
│   init.lua  autocmds, state ───┼─────────────▶│   ipc.Read → render.HTML ─┐       │
│   viewer.lua  process, protocol│  CONTENT,    │                           │       │
│   config.lua                   │  SCROLL,     │ waitSignal goroutine ─────┤       │
│   setup() → :MarkdownReader    │  QUIT        │                    Dispatch       │
│                                │              │                           ▼       │
│                                │   stdout     │ main thread: GTK/Cocoa loop       │
│                       :edit ◀──┼──────────────┼── onOpen ◀── page.js ◀── Eval     │
│                  vim.ui.open   │  OPEN <url>  │               (WebKit page)       │
└────────────────────────────────┘              └───────────────────────────────────┘
```

Neovim owns the document: the viewer never reads the file itself (in `--stdin` mode), so unsaved changes show up
immediately. The viewer is a separate process, so a crash in WebKit can't take Neovim down, and the Lua side needs
nothing but `vim.system`.

### Repository layout

```
lua/markdown-reader/
  init.lua                   setup() and :MarkdownReader; one viewer per buffer; autocmds for
                             edits, cursor, buffer unload and VimLeavePre
  viewer.lua                 starts the process, speaks the protocol, closes it
  config.lua                 defaults
  health.lua                 :checkhealth markdown-reader
cmd/mdreader/main.go         the viewer: flags, window, goroutines
internal/ipc/                parses the stdin protocol
internal/render/             Markdown → HTML with goldmark: data-line attributes, heading IDs, highlighting
internal/page/               the HTML page around it; assets/ holds style.css and page.js
internal/webview/            cgo binding of webview; per-OS page loaders
third_party/webview/         vendored webview (C++)
tests/                       fixtures, plugin tests and the fake viewer
doc/markdown-reader.txt      :help markdown-reader
```

### Protocol

The plugin starts `mdreader --stdin <path>`. The path names the window and is the base for relative URLs; it needn't
exist. Messages on stdin:

| Message                            | Meaning                                                                                               |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `CONTENT <n>\n` + n bytes          | replace the document with these n bytes of Markdown                                                   |
| `SCROLL <top> <bottom> <cursor>\n` | the editor shows source lines `<top>` to `<bottom>`, with the cursor on line `<cursor>` (all 1-based) |
| `FILE <n>\n` + n bytes             | the document is now the file at this path: new window title and base for URLs                         |
| `QUIT\n`                           | close the window                                                                                      |

End of input also closes the window, so the viewer never outlives Neovim. Anything malformed makes it exit with status 1
and a message on stderr, which the plugin shows. On stdout, the viewer prints `OPEN <url>\n` for each clicked link.

CONTENT carries a byte count rather than a terminator, so documents may contain anything, including lines that look like
messages.

### The viewer

**Threads.** GTK and Cocoa must be driven from the process's main thread. `main.go` locks the main goroutine to it
(`runtime.LockOSThread` in `init`) and runs the event loop there. Background goroutines — one reading stdin, one waiting
for SIGTERM/SIGINT/SIGHUP — never touch the window; they hand closures to `WebView.Dispatch`, which webview runs on the
main thread. All viewer state is therefore only accessed from one thread and needs no locks.

**Rendering.** Markdown is rendered on the stdin goroutine, so a large document doesn't block the window.
`internal/render` uses goldmark v2 with its GFM, footnote and alert extensions. An AST transformer adds `data-line="N"`
to each block, computed from the byte offset goldmark records for it, and the HTML renderer writes these out. The same
transformer gives headings the anchor IDs GitHub gives them (the text in lower case, punctuation dropped, spaces as `-`,
numbered when repeated), so `#heading` links work as they do on GitHub. Fenced code in a language
[chroma](https://github.com/alecthomas/chroma) knows is highlighted with chroma's CSS classes; `internal/page` adds the
colors of chroma's `github` and `github-dark` styles to the page, without their backgrounds, so code blocks keep the
page's look.

**The page** is loaded once, with a `<base href>` pointing at the Markdown file's directory, so relative images and
links resolve as on disk. Later updates replace only the body through `mdr.update(html)` in `page.js`, keeping scroll
position and avoiding a reload flash. Content or scroll positions that arrive before `page.js` has loaded are kept and
replayed when it calls `window.mdrReady()`.

Loading the page needs a base URL and local file access, which webview's own `SetHtml` doesn't offer, so
`internal/webview` has a loader per OS: `load_linux.go` calls `webkit_web_view_load_html`; `load_darwin.go` writes the
page to a temporary file and calls WKWebView's `loadFileURL:allowingReadAccessToURL:`, deleting the file once loaded.

**cgo.** `internal/webview` is the only package with C. webview's C API takes C function pointers with a `void *`
argument; Go closures can't be passed to C, so the binding passes a `cgo.Handle` (a number standing for a Go value) as
that argument, and small C trampolines in `glue.c` call back into exported Go functions that look the closure up again.

**Exit.** The viewer leaves with `os.Exit` without destroying the window: the stdin goroutine may still be blocked in a
read and could dispatch to a destroyed window otherwise.

### Scroll sync

On every cursor move or scroll in a previewed buffer, the plugin sends `SCROLL <top> <bottom> <cursor>`: the first and
last line the Neovim window shows and the cursor's line (only when they changed, and again after each re-render, since
the layout may have shifted). Syncing what the window shows, rather than where the cursor sits in it, means the preview
only moves when Neovim's text does, and always in the same direction.

`page.js` collects the page positions of all elements with `data-line` and finds where a source line is rendered by
binary search between the two blocks around it, interpolating in between. It then widens that position to the row of
text there. The lines Neovim shows rarely fill exactly one preview window, so the page lines up their start with its top
edge while Neovim shows the start of the document, their end with its bottom edge while it shows the end, and shifts
gradually in between, by how many lines are hidden above and below. The page's padding counts as part of the first and
last line, so the edges of the document never make the page jump. The cursor's row is kept at least two rows away from
the window's edges, and no row of text is cut in half at the top edge. The page glides to its new position like a
critically damped spring: it speeds up from rest and slows down into place, and a new position takes over at the current
speed, so holding a key gives one even motion. Jumps longer than half the window, like `gg` or `G`, skip to half a
window before the target and glide the rest. Scrolling the preview by hand stops the glide and leaves the page there
until Neovim's view changes again; the preview never moves Neovim's cursor.

Positions are cached until the content, window size or a loaded image changes the layout, which also re-applies the last
position.

### Links

`page.js` intercepts every click on a link. `#anchor` links scroll within the page; everything else is resolved against
`<base>` and sent to the viewer through the `window.mdrOpen` binding, which prints `OPEN <url>`. The viewer itself never
navigates away.

The plugin hands everything but `file://` URLs of `.md`/`.markdown` files to `vim.ui.open`. Those it opens with `:edit`
in the window showing the previewed buffer. If the URL has a `#fragment`, it asks `mdreader --headings` for the lines
and IDs of the file's headings, so that Neovim and the preview agree on IDs, and puts the cursor on the matching one.
Then the preview moves to the new buffer: the plugin sends `FILE` with its path, which changes the window title and the
page's `<base>`, and its content. If that buffer already has a preview of its own, the clicked one closes instead.

### Shutdown

`close()` sends `QUIT` and closes stdin. If the viewer is still running after 500 ms it gets SIGTERM, and after another
second SIGKILL. If the viewer exits on its own (its window was closed), the plugin forgets it; if it exits with an
error, the plugin shows its stderr.
