// Package page builds the HTML document the viewer loads once; later
// content updates replace only the body, through page.js.
package page

import (
	_ "embed"
	"html"
	"net/url"
	"strings"
)

//go:embed assets/style.css
var styleCSS string

//go:embed assets/page.js
var pageJS string

// FileURI returns the file:// URI of an absolute path, with a trailing
// slash if dir is set so that relative URLs resolve inside it.
func FileURI(path string, dir bool) string {
	if dir && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

// HTML returns the complete page: body is rendered Markdown, and relative
// URLs in it resolve against baseURI.
func HTML(title, baseURI, body string) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString(`<base href="` + html.EscapeString(baseURI) + "\">\n")
	b.WriteString("<title>" + html.EscapeString(title) + "</title>\n")
	b.WriteString("<style>\n" + styleCSS + "</style>\n</head>\n<body>\n")
	b.WriteString("<main id=\"content\" class=\"markdown-body\">\n" + body + "</main>\n")
	b.WriteString("<script>\n" + pageJS + "</script>\n</body>\n</html>\n")
	return b.String()
}
