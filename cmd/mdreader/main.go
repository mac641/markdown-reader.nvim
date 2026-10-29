// mdreader renders a Markdown file in a native web view window.
//
//	mdreader [--debug] [--stdin] FILE
//	mdreader --render-only FILE
//	mdreader --headings
//
// With --stdin the document isn't read from FILE but sent by the editor as
// messages on stdin (see internal/ipc); FILE then only names the window and
// is the base for relative URLs, and needn't exist. Clicked links are
// reported on stdout as "OPEN <url>" lines so the editor can decide how to
// open them.
//
// --headings reads Markdown on stdin and prints "<line> <id>" for each
// heading, its 1-based line and anchor ID, so the editor can follow links
// to headings.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/mac641/markdown-reader.nvim/internal/ipc"
	"github.com/mac641/markdown-reader.nvim/internal/page"
	"github.com/mac641/markdown-reader.nvim/internal/render"
	"github.com/mac641/markdown-reader.nvim/internal/webview"
)

func init() {
	// GTK and Cocoa must run on the main thread, which is the one running
	// main as long as this goroutine stays locked to it.
	runtime.LockOSThread()
}

// viewer is the window's state. Its methods run on the main thread only:
// other goroutines hand work over with wv.Dispatch.
type viewer struct {
	wv       *webview.WebView
	ready    bool     // page.js has loaded and defined window.mdr
	base     *string  // a base URI that arrived before ready
	pending  *content // the latest content that arrived before ready
	scroll   *[3]int  // the latest SCROLL lines (top, bottom, cursor), if any
	exitCode int
}

func main() {
	debug := flag.Bool("debug", false, "enable the web inspector")
	fromStdin := flag.Bool("stdin", false, "read the document from stdin messages")
	renderOnly := flag.Bool("render-only", false, "print the rendered HTML and exit")
	headings := flag.Bool("headings", false, "print the line and anchor ID of each heading of the Markdown on stdin")
	flag.Usage = func() {
		// Nothing to do if even the usage can't be written.
		_, _ = fmt.Fprintln(flag.CommandLine.Output(),
			"usage: mdreader [--debug] [--stdin] FILE\n       mdreader --render-only FILE\n       mdreader --headings")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *headings {
		if flag.NArg() != 0 || *debug || *fromStdin || *renderOnly {
			flag.Usage()
			os.Exit(2)
		}
		os.Exit(printHeadings())
	}
	if flag.NArg() != 1 || (*renderOnly && *fromStdin) {
		flag.Usage()
		os.Exit(2)
	}
	os.Exit(run(flag.Arg(0), *debug, *fromStdin, *renderOnly))
}

func printHeadings() int {
	md, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fail(err)
	}
	out := bufio.NewWriter(os.Stdout)
	for _, h := range render.Headings(md) {
		if _, err := fmt.Fprintf(out, "%d %s\n", h.Line, h.ID); err != nil {
			return fail(err)
		}
	}
	if err := out.Flush(); err != nil {
		return fail(err)
	}
	return 0
}

func run(path string, debug, fromStdin, renderOnly bool) int {
	var body string
	if !fromStdin {
		md, err := os.ReadFile(path)
		if err != nil {
			return fail(err)
		}
		if body, err = render.HTML(md); err != nil {
			return fail(err)
		}
	}
	if renderOnly {
		fmt.Print(body)
		return 0
	}

	title, baseURI, err := describe(path)
	if err != nil {
		return fail(err)
	}

	wv, err := webview.New(debug)
	if err != nil {
		return fail(err)
	}
	v := &viewer{wv: wv}
	wv.SetTitle(title)
	wv.SetSize(900, 1000)
	wv.Bind("mdrReady", func(string) { v.onReady() })
	wv.Bind("mdrOpen", v.onOpen)

	go v.waitSignal()
	if fromStdin {
		go v.readStdin()
	}
	if err := wv.LoadHTML(page.HTML(title, baseURI, body), baseURI); err != nil {
		wv.Cleanup()
		return fail(fmt.Errorf("could not load the page: %w", err))
	}
	wv.Run()

	// The window is deliberately not destroyed: readStdin may still be
	// blocked on stdin and dispatch to it at any moment. os.Exit ends the
	// process without running C++ destructors either.
	wv.Cleanup()
	return v.exitCode
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "mdreader:", err)
	return 1
}

// describe returns the window title for a document at path and the base
// URI for its relative URLs.
func describe(path string) (title, baseURI string, err error) {
	abs, err := absolutePath(path)
	if err != nil {
		return "", "", err
	}
	return filepath.Base(abs), page.FileURI(filepath.Dir(abs), true), nil
}

// absolutePath is like filepath.Abs, but resolves symlinks if path exists.
func absolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// readStdin turns editor messages into updates of the window.
func (v *viewer) readStdin() {
	in := bufio.NewReader(os.Stdin)
	for {
		msg, err := ipc.Read(in)
		if errors.Is(err, io.EOF) {
			v.wv.Dispatch(func() { v.quit(0) })
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "mdreader: protocol error:", err)
			v.wv.Dispatch(func() { v.quit(1) })
			return
		}

		switch msg.Kind {
		case ipc.Content:
			// Rendering happens here, off the main thread.
			html, err := render.HTML(msg.Text)
			if err != nil {
				fmt.Fprintln(os.Stderr, "mdreader: failed to render content:", err)
				continue
			}
			c := content{html, lineCount(msg.Text)}
			v.wv.Dispatch(func() { v.showContent(c) })
		case ipc.File:
			title, baseURI, err := describe(msg.Path)
			if err != nil {
				fmt.Fprintln(os.Stderr, "mdreader:", err)
				continue
			}
			v.wv.Dispatch(func() { v.showFile(title, baseURI) })
		case ipc.Scroll:
			lines := [3]int{msg.Top, msg.Bottom, msg.Cursor}
			v.wv.Dispatch(func() { v.showScroll(lines) })
		case ipc.Quit:
			v.wv.Dispatch(func() { v.quit(0) })
			return
		}
	}
}

// waitSignal turns SIGTERM, SIGINT and SIGHUP into a regular shutdown.
func (v *viewer) waitSignal() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	<-sigs
	v.wv.Dispatch(func() { v.quit(0) })
}

func (v *viewer) showFile(title, baseURI string) {
	v.wv.SetTitle(title)
	v.setBase(baseURI)
}

func (v *viewer) setBase(baseURI string) {
	if !v.ready {
		v.base = &baseURI
		return
	}
	js, _ := json.Marshal(baseURI)
	v.wv.Eval("mdr.setBase(" + string(js) + ");")
}

// content is a rendered document.
type content struct {
	html  string
	lines int // in the Markdown
}

// lineCount counts lines like an editor: a final newline ends the last
// line rather than starting another one.
func lineCount(md []byte) int {
	n := bytes.Count(md, []byte("\n"))
	if len(md) > 0 && md[len(md)-1] != '\n' {
		n++
	}
	return n
}

func (v *viewer) showContent(c content) {
	if !v.ready {
		v.pending = &c
		return
	}
	js, _ := json.Marshal(c.html) // a string always marshals
	v.wv.Eval(fmt.Sprintf("mdr.update(%s, %d);", js, c.lines))
}

func (v *viewer) showScroll(lines [3]int) {
	v.scroll = &lines
	if v.ready {
		v.evalScroll()
	}
}

func (v *viewer) evalScroll() {
	v.wv.Eval(fmt.Sprintf("mdr.follow(%d, %d, %d);", v.scroll[0], v.scroll[1], v.scroll[2]))
}

func (v *viewer) quit(code int) {
	v.exitCode = code
	v.wv.Terminate()
}

// onReady is window.mdrReady(), called by page.js once it has loaded. It
// replays what arrived before.
func (v *viewer) onReady() {
	v.ready = true
	v.wv.Cleanup()
	if v.base != nil {
		v.setBase(*v.base)
		v.base = nil
	}
	if v.pending != nil {
		v.showContent(*v.pending)
		v.pending = nil
	}
	if v.scroll != nil {
		v.evalScroll()
	}
}

// onOpen is window.mdrOpen(url), called for clicked links.
func (v *viewer) onOpen(args string) {
	var urls []string
	if err := json.Unmarshal([]byte(args), &urls); err != nil || len(urls) == 0 {
		return
	}
	// A line break would let the page forge protocol lines.
	if url := urls[0]; !strings.ContainsAny(url, "\r\n") {
		fmt.Printf("OPEN %s\n", url)
	}
}
