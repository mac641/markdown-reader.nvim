package webview

/*
#include <stdlib.h>

#include <webkit/webkit.h>

static void load_html(void *view, const char *html, const char *base_uri) {
	webkit_web_view_load_html(WEBKIT_WEB_VIEW(view), html, base_uri);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// LoadHTML loads a complete page such that relative URLs resolve against
// baseURI, the file:// URI of a directory, and local files may be read.
// webview's own SetHtml can't do that: it loads with no base URI.
func (v *WebView) LoadHTML(html, baseURI string) error {
	view := v.nativeView()
	if view == nil {
		return errors.New("no web view")
	}
	h := C.CString(html)
	defer C.free(unsafe.Pointer(h))
	b := C.CString(baseURI)
	defer C.free(unsafe.Pointer(b))
	C.load_html(view, h, b)
	return nil
}

// Cleanup releases what LoadHTML kept around. Safe to call more than once.
func (v *WebView) Cleanup() {}
