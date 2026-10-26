#include "glue.h"

#include "_cgo_export.h"

static void bind_callback(const char *id, const char *req, void *arg) {
    goBindCallback((char *)id, (char *)req, (uintptr_t)arg);
}

static void dispatch_callback(webview_t w, void *arg) {
    (void)w;
    goDispatchCallback((uintptr_t)arg);
}

webview_error_t mdr_bind(webview_t w, const char *name, uintptr_t handle) {
    return webview_bind(w, name, bind_callback, (void *)handle);
}

webview_error_t mdr_dispatch(webview_t w, uintptr_t handle) {
    return webview_dispatch(w, dispatch_callback, (void *)handle);
}
