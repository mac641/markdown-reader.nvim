package webview

/*
#include <stdlib.h>
#include <objc/message.h>
#include <objc/runtime.h>

static id nsurl_from_path(const char *path) {
	id str = ((id (*)(id, SEL, const char *))objc_msgSend)(
		(id)objc_getClass("NSString"), sel_registerName("stringWithUTF8String:"), path);
	return ((id (*)(id, SEL, id))objc_msgSend)(
		(id)objc_getClass("NSURL"), sel_registerName("fileURLWithPath:"), str);
}

// [view loadFileURL:file allowingReadAccessToURL:root]
static void load_file(void *view, const char *file, const char *root) {
	id pool = ((id (*)(id, SEL))objc_msgSend)(
		(id)objc_getClass("NSAutoreleasePool"), sel_registerName("new"));
	((void (*)(id, SEL, id, id))objc_msgSend)(
		(id)view, sel_registerName("loadFileURL:allowingReadAccessToURL:"),
		nsurl_from_path(file), nsurl_from_path(root));
	((void (*)(id, SEL))objc_msgSend)(pool, sel_registerName("drain"));
}
*/
import "C"

import (
	"errors"
	"os"
	"unsafe"
)

// LoadHTML loads a complete page such that relative URLs resolve against
// baseURI, the file:// URI of a directory, and local files may be read.
//
// WKWebView's loadHTMLString:baseURL: doesn't reliably let the sandboxed
// web content process read local files, so the page is written to a
// temporary file and loaded with loadFileURL:allowingReadAccessToURL:,
// which does. baseURI is already in the page's <base>.
func (v *WebView) LoadHTML(html, baseURI string) error {
	view := v.nativeView()
	if view == nil {
		return errors.New("no web view")
	}
	f, err := os.CreateTemp("", "mdreader-*.html")
	if err != nil {
		return err
	}
	v.tmpFile = f.Name()
	_, err = f.WriteString(html)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		v.Cleanup()
		return err
	}

	file := C.CString(v.tmpFile)
	defer C.free(unsafe.Pointer(file))
	root := C.CString("/")
	defer C.free(unsafe.Pointer(root))
	C.load_file(view, file, root)
	return nil
}

// Cleanup removes the temporary page; once it has loaded it's no longer
// needed. Safe to call more than once.
func (v *WebView) Cleanup() {
	if v.tmpFile != "" {
		os.Remove(v.tmpFile)
		v.tmpFile = ""
	}
}
