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
  MISSING_DEPS := $(shell $(PKG_CONFIG) --exists gtk4 webkitgtk-6.0 || echo 1)
endif

all: $(BIN)

# go build tracks its own dependencies and caches, so always ask it.
$(BIN): check-deps
	$(GO) build -trimpath -ldflags '-s -w' -o $@ ./cmd/mdreader

check-deps:
	@command -v $(GO) >/dev/null || { \
	  echo "error: Go not found; install it from your package manager or https://go.dev/dl/" >&2; exit 1; }
ifdef MISSING_DEPS
	@echo "error: GTK 4 or WebKitGTK 6.0 development files not found (pkg-config: gtk4, webkitgtk-6.0)." >&2
	@echo "  Ubuntu:   sudo apt install golang-go build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev" >&2
	@echo "  Fedora:   sudo dnf install golang gcc-c++ make 'pkgconfig(gtk4)' 'pkgconfig(webkitgtk-6.0)'" >&2
	@echo "  openSUSE: sudo zypper install go gcc-c++ make 'pkgconfig(gtk4)' 'pkgconfig(webkitgtk-6.0)'" >&2
	@echo "  NixOS:    nix-shell -p go gnumake pkg-config gtk4 webkitgtk_6_0 --run make" >&2
	@exit 1
endif

# The Go tests cover everything but the window and need no WebKit, so CI can
# run them anywhere.
test:
	$(GO) test ./internal/ipc ./internal/page ./internal/render

clean:
	rm -rf bin

.PHONY: all $(BIN) check-deps test clean
