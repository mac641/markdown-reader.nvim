// Package webview is a minimal Go binding of webview/webview, vendored in
// third_party/webview, covering what mdreader needs.
//
// The window must be created and driven from the main thread (macOS
// requires it), so the main package locks its goroutine to the main thread
// with runtime.LockOSThread in init. Only Dispatch and Terminate may be
// called from other goroutines.
package webview

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/webview -DWEBVIEW_STATIC
#cgo CXXFLAGS: -I${SRCDIR}/../../third_party/webview -DWEBVIEW_STATIC -std=c++11
#cgo linux LDFLAGS: -ldl
#cgo linux pkg-config: gtk4 webkitgtk-6.0
#cgo darwin LDFLAGS: -framework WebKit -framework Cocoa

#include <stdlib.h>
#include "glue.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"unsafe"
)

// WebView is a window showing a web page.
type WebView struct {
	w       C.webview_t
	tmpFile string // macOS: the page, written out to be loaded from disk
}

// New opens a window. debug enables the web inspector.
func New(debug bool) (*WebView, error) {
	var d C.int
	if debug {
		d = 1
	}
	w := C.webview_create(d, nil)
	if w == nil {
		return nil, errors.New("could not create a window (is a display available?)")
	}
	return &WebView{w: w}, nil
}

// SetTitle sets the window title.
func (v *WebView) SetTitle(title string) {
	s := C.CString(title)
	defer C.free(unsafe.Pointer(s))
	C.webview_set_title(v.w, s)
}

// SetSize sets the window's size in pixels.
func (v *WebView) SetSize(width, height int) {
	C.webview_set_size(v.w, C.int(width), C.int(height), C.WEBVIEW_HINT_NONE)
}

// Eval runs JavaScript in the page.
func (v *WebView) Eval(js string) {
	s := C.CString(js)
	defer C.free(unsafe.Pointer(s))
	C.webview_eval(v.w, s)
}

// Bind defines window.<name>(...args) in the page. Calling it runs f with
// the arguments as a JSON array; the JavaScript promise resolves to null.
func (v *WebView) Bind(name string, f func(args string)) {
	s := C.CString(name)
	defer C.free(unsafe.Pointer(s))
	// The handle keeps f alive for as long as the window exists.
	h := cgo.NewHandle(binding{v, f})
	C.mdr_bind(v.w, s, C.uintptr_t(h))
}

type binding struct {
	v *WebView
	f func(args string)
}

//export goBindCallback
func goBindCallback(id, req *C.char, h C.uintptr_t) {
	b := cgo.Handle(h).Value().(binding)
	b.f(C.GoString(req))
	null := C.CString("null")
	defer C.free(unsafe.Pointer(null))
	C.webview_return(b.v.w, id, 0, null)
}

// Dispatch runs f on the main thread. Safe to call from any goroutine.
func (v *WebView) Dispatch(f func()) {
	C.mdr_dispatch(v.w, C.uintptr_t(cgo.NewHandle(f)))
}

//export goDispatchCallback
func goDispatchCallback(h C.uintptr_t) {
	handle := cgo.Handle(h)
	f := handle.Value().(func())
	handle.Delete()
	f()
}

// Run shows the window and processes events until Terminate is called.
func (v *WebView) Run() { C.webview_run(v.w) }

// Terminate makes Run return. Safe to call from any goroutine.
func (v *WebView) Terminate() { C.webview_terminate(v.w) }

func (v *WebView) nativeView() unsafe.Pointer {
	return C.webview_get_native_handle(v.w, C.WEBVIEW_NATIVE_HANDLE_KIND_BROWSER_CONTROLLER)
}
