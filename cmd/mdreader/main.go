// mdreader renders a Markdown file in a native web view window.
//
//	mdreader [--debug] FILE
//	mdreader --render-only FILE
//
// Clicked links are reported on stdout as "OPEN <url>" lines so the editor
// can decide how to open them.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mac641/markdown-reader.nvim/internal/page"
	"github.com/mac641/markdown-reader.nvim/internal/render"
	"github.com/mac641/markdown-reader.nvim/internal/webview"
)

func init() {
	// GTK and Cocoa must run on the main thread, which is the one running
	// main as long as this goroutine stays locked to it.
	runtime.LockOSThread()
}

func main() {
	debug := flag.Bool("debug", false, "enable the web inspector")
	renderOnly := flag.Bool("render-only", false, "print the rendered HTML and exit")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(),
			"usage: mdreader [--debug] FILE\n       mdreader --render-only FILE")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	os.Exit(run(flag.Arg(0), *debug, *renderOnly))
}

func run(path string, debug, renderOnly bool) int {
	md, err := os.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	body, err := render.HTML(md)
	if err != nil {
		return fail(err)
	}
	if renderOnly {
		fmt.Print(body)
		return 0
	}

	abs, err := absolutePath(path)
	if err != nil {
		return fail(err)
	}
	title := filepath.Base(abs)
	baseURI := page.FileURI(filepath.Dir(abs), true)

	wv, err := webview.New(debug)
	if err != nil {
		return fail(err)
	}
	wv.SetTitle(title)
	wv.SetSize(900, 1000)
	wv.Bind("mdrOpen", onOpen)

	if err := wv.LoadHTML(page.HTML(title, baseURI, body), baseURI); err != nil {
		wv.Cleanup()
		return fail(fmt.Errorf("could not load the page: %w", err))
	}
	wv.Run()

	// os.Exit ends the process without running C++ destructors, so the
	// window is not destroyed.
	wv.Cleanup()
	return 0
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "mdreader:", err)
	return 1
}

// absolutePath is like filepath.Abs, but resolves symlinks if path exists.
func absolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real, nil
	}
	return abs, nil
}

// onOpen is window.mdrOpen(url), called for clicked links.
func onOpen(args string) {
	var urls []string
	if err := json.Unmarshal([]byte(args), &urls); err != nil || len(urls) == 0 {
		return
	}
	// A line break would let the page forge output lines.
	if url := urls[0]; !strings.ContainsAny(url, "\r\n") {
		fmt.Printf("OPEN %s\n", url)
	}
}
