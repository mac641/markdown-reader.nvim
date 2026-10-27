package webview

/*
#include <stdlib.h>

#include <glib.h>
*/
import "C"

import "unsafe"

func init() {
	// GTK takes the Wayland app ID from the program name, which a Go
	// program never sets; without it, the window is "GTK Application"
	// and window manager rules can't match it.
	name := C.CString("mdreader")
	defer C.free(unsafe.Pointer(name))
	C.g_set_prgname(name)
}
