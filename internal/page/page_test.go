package page

import (
	"strings"
	"testing"
)

func TestFileURI(t *testing.T) {
	for _, c := range []struct {
		path string
		dir  bool
		want string
	}{
		{"/home/me/notes", true, "file:///home/me/notes/"},
		{"/", true, "file:///"},
		{"/a b/c#d?.md", false, "file:///a%20b/c%23d%3F.md"},
		{"/ü", false, "file:///%C3%BC"},
	} {
		if got := FileURI(c.path, c.dir); got != c.want {
			t.Errorf("FileURI(%q, %v) = %q, want %q", c.path, c.dir, got, c.want)
		}
	}
}

func TestHTML(t *testing.T) {
	got := HTML("a<b.md", "file:///x/", "<p>hi</p>\n")
	for _, want := range []string{
		`<base href="file:///x/">`,
		"<title>a&lt;b.md</title>",
		"<main id=\"content\" class=\"markdown-body\">\n<p>hi</p>\n</main>",
		"window.mdr = {",
		".markdown-body",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page is missing %q", want)
		}
	}
}
