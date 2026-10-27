// Package page builds the HTML document the viewer loads once; later
// content updates replace only the body, through page.js.
package page

import (
	_ "embed"
	"html"
	"net/url"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

//go:embed assets/style.css
var styleCSS string

//go:embed assets/page.js
var pageJS string

// codeCSS colors the classes the render package puts on highlighted code,
// in GitHub's light and dark styles. Each style gets its own media block,
// since the two don't style the same classes: light's dark punctuation
// would otherwise show on the dark background.
var codeCSS = "@media (prefers-color-scheme: light) {\n" + tokenCSS("github") + "}\n" +
	"@media (prefers-color-scheme: dark) {\n" + tokenCSS("github-dark") + "}\n"

// tokenCSS returns the rules for chroma's token classes in a style, scoped
// to <pre>. Left out are the style's background (style.css has its own),
// line numbers (not shown) and lexer errors, which GitHub doesn't mark
// either; half-typed code would light up red.
func tokenCSS(style string) string {
	var all strings.Builder
	_ = chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&all, styles.Get(style))
	var b strings.Builder
	for _, rule := range strings.Split(all.String(), "\n") {
		// Rules look like "/* Keyword */ .chroma .k { color: #cf222e }".
		_, sel, ok := strings.Cut(rule, "*/ .chroma .")
		if ok && !strings.HasPrefix(rule, "/* Line") && !strings.HasPrefix(rule, "/* Error") {
			b.WriteString("pre ." + sel + "\n")
		}
	}
	return b.String()
}

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
	b.WriteString("<style>\n" + styleCSS + codeCSS + "</style>\n</head>\n<body>\n")
	b.WriteString("<main id=\"content\" class=\"markdown-body\">\n" + body + "</main>\n")
	b.WriteString("<script>\n" + pageJS + "</script>\n</body>\n</html>\n")
	return b.String()
}
