/* C trampolines between webview's callbacks and the Go functions exported
 * in webview.go. Go values cross as cgo.Handle numbers in the void *arg
 * that webview passes back to the callback. */
#ifndef MDR_GLUE_H
#define MDR_GLUE_H

#include <stdint.h>

#include "webview.h"

webview_error_t mdr_bind(webview_t w, const char *name, uintptr_t handle);
webview_error_t mdr_dispatch(webview_t w, uintptr_t handle);

#endif
