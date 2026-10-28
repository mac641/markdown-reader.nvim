# Builds bin/mdreader, the preview window used by markdown-reader.nvim.
#
# The viewer is Go, plus cgo for webview/webview (third_party/webview).
# macOS uses the system WebKit framework; Linux uses the GTK 4 port of
# WebKitGTK (webkitgtk-6.0), found via pkg-config.

GO ?= go
PKG_CONFIG ?= pkg-config
BIN := bin/mdreader

UNAME_S := $(shell uname -s)

ifneq ($(UNAME_S),Darwin)
  MISSING_DEPS := $(shell $(PKG_CONFIG) --exists gtk4 webkitgtk-6.0 2>/dev/null || echo 1)
endif

# NixOS links no headers or pkg-config files into the system, even for
# libraries in environment.systemPackages, so there the build runs inside
# shell.nix, which provides them and Go.
ifneq ($(and $(MISSING_DEPS),$(wildcard /etc/NIXOS)),)
  ifndef IN_NIX_SHELL
    VIA_NIX_SHELL := 1
  endif
endif

all: $(BIN)

# go build tracks its own dependencies and caches, so always ask it. The
# viewer doesn't use git metadata, so it isn't stamped in: git refuses to
# read a checkout owned by another user, which would fail the build.
ifdef VIA_NIX_SHELL
$(BIN):
	nix-shell --run '$(MAKE) $(BIN)'
else
$(BIN): check-deps
	$(GO) build -trimpath -buildvcs=false -ldflags '-s -w' -o $@ ./cmd/mdreader
endif

check-deps:
	@command -v $(GO) >/dev/null || { \
	  echo "error: Go not found; install it from your package manager or https://go.dev/dl/" >&2; exit 1; }
ifdef MISSING_DEPS
	@echo "error: GTK 4 or WebKitGTK 6.0 development files not found (pkg-config: gtk4, webkitgtk-6.0)." >&2
	@echo "  Ubuntu:   sudo apt install golang-go build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev" >&2
	@echo "  Fedora:   sudo dnf install golang gcc-c++ make 'pkgconfig(gtk4)' 'pkgconfig(webkitgtk-6.0)'" >&2
	@echo "  openSUSE: sudo zypper install go gcc-c++ make 'pkgconfig(gtk4)' 'pkgconfig(webkitgtk-6.0)'" >&2
	@echo "  NixOS:    nix-shell --run make   (make does this itself outside of a nix-shell)" >&2
	@exit 1
endif

# The Go tests cover everything but the window and need no WebKit, so CI can
# run them anywhere; the plugin tests run against tests/lua/fake-viewer.sh.
test: test-go test-lua

test-go:
	$(GO) test ./internal/ipc ./internal/page ./internal/render

test-lua:
	nvim --headless --clean -l tests/lua/test_plugin.lua

clean:
	rm -rf bin

.PHONY: all $(BIN) check-deps test test-go test-lua clean
