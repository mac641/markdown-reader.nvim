package webview

/*
#include <gtk/gtk.h>
*/
import "C"

import "os"

func init() {
	// GTK 4.16 and later draw with Vulkan by default, and setting it up
	// queries every GPU. On laptops with a second, powered-down GPU that
	// wakes it, which delays the window by about two seconds. GTK's
	// OpenGL renderer uses the GPU that drives the display, and if it
	// fails, GTK falls back to its usual choices. An explicit GSK_RENDERER
	// wins.
	if _, set := os.LookupEnv("GSK_RENDERER"); set {
		return
	}
	switch minor := C.gtk_get_minor_version(); {
	case minor >= 18:
		_ = os.Setenv("GSK_RENDERER", "gl")
	case minor >= 16:
		// Until 4.18, "gl" was the old OpenGL renderer.
		_ = os.Setenv("GSK_RENDERER", "ngl")
	}
}
