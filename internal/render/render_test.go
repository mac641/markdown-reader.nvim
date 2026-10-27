package render

import (
	"strings"
	"testing"
)

// expect checks that rendering md produces HTML containing each want.
func expect(t *testing.T, md string, want ...string) {
	t.Helper()
	html, err := HTML([]byte(md))
	if err != nil {
		t.Fatalf("rendering %q: %v", md, err)
	}
	for _, w := range want {
		if !strings.Contains(html, w) {
			t.Errorf("rendering %q\n  missing: %s\n  got: %s", md, w, html)
		}
	}
}

func TestDataLines(t *testing.T) {
	expect(t, "# Title\n\ntext\n", `<h1 data-line="1" id="title">Title</h1>`, `<p data-line="3">text</p>`)
	expect(t, "\n\n> quoted\n", `<blockquote data-line="3">`)
	expect(t, "- a\n- b\n", `<ul data-line="1">`, `<li data-line="1">a</li>`, `<li data-line="2">b</li>`)
	expect(t, "- [ ] todo\n", `<li data-line="1" class="task-list-item">`)
	expect(t, "x\n\n---\n", `<hr data-line="3">`)
	expect(t, "x\r\ny\r\n\r\nz\r\n", `<p data-line="4">z</p>`)
	expect(t, "x\n\n```c\ncode\n```\n", `<pre data-line="3"><code class="language-c">`)
	expect(t, "x\n\n    indented\n", `<pre data-line="3"><code>indented`)
	expect(t, "| a |\n|---|\n| 1 |\n", `<table data-line="1">`)
}

func TestGitHubDialect(t *testing.T) {
	expect(t, "| a | b |\n|:--|--:|\n| 1 | 2 |\n", `<tr data-line="3">`, `<td style="text-align:right">2</td>`)
	expect(t, "~~gone~~\n", "<del>gone</del>")
	expect(t, "- [x] done\n", `<li data-line="1" class="task-list-item"><input checked="" disabled="" type="checkbox"> done`)
	expect(t, "see https://example.org\n", `<a href="https://example.org">`)
	expect(t, "a[^n]\n\n[^n]: foot\n", `<a href="#fn:1" class="footnote-ref"`, `<li id="fn:1"`)
	expect(t, "<details>\n<summary>s</summary>\n</details>\n", "<details>\n<summary>s</summary>")
}

func TestAlerts(t *testing.T) {
	// The paragraph's position includes the marker line.
	expect(t, "> [!NOTE]\n> hi\n",
		`<div class="markdown-alert markdown-alert-note">`,
		"Note\n</p>",
		`<p data-line="1">hi</p>`)
	expect(t, "> [!warning]\n> *careful*\n> more\n",
		`markdown-alert-warning`, "<em>careful</em>\nmore</p>")
	expect(t, "> [!NOTE] Custom title\n> hi\n", "Custom title\n</p>")
	expect(t, "> [!TIP]\n>\n> - a\n", "Tip\n</p>\n<ul")
	expect(t, "> [!BOGUS]\n> hi\n", "<blockquote")
}

func TestEscaping(t *testing.T) {
	expect(t, "a < b & \"c\"\n", "a &lt; b &amp; &quot;c&quot;")
	expect(t, "`<x>`\n", "<code>&lt;x&gt;</code>")
	expect(t, "&copy;\n", "©")
	expect(t, "[l](<a b>)\n", `<a href="a%20b">`)
	expect(t, "[l](/x?a=1&b=2 \"t\\\"\")\n", `href="/x?a=1&amp;b=2" title="t&quot;"`)
	expect(t, "![alt *em*](o.png \"t\")\n", `<img src="o.png" alt="alt em" title="t">`)
	expect(t, "```js\n<x>\n```\n", `<code class="language-js">&lt;x&gt;`)
}

func TestHeadingIDs(t *testing.T) {
	// The cases of github-slugger, which GitHub uses.
	expect(t, "# Über *the* `code` [link](http://x) & <b>stuff</b>!\n", `id="über-the-code-link--stuff"`)
	expect(t, "# a\n# a\n# a-1\n# a\n", `id="a"`, `id="a-1"`, `id="a-1-1"`, `id="a-2"`)
	expect(t, "Setext\n======\n", `<h1 data-line="1" id="setext">`)
	expect(t, "# snake_case and 3.14\n", `id="snake_case-and-314"`)
	expect(t, "# https://x.org/a\n", `id="httpsxorga"`)

	html, _ := HTML([]byte("# !!!\n"))
	if strings.Contains(html, "id=") {
		t.Errorf("heading without letters got an ID: %s", html)
	}
}

func TestHeadings(t *testing.T) {
	got := Headings([]byte("# One\n\ntext\n\nTwo\n---\n\n```\n# not a heading\n```\n## One\n"))
	want := []Heading{{1, "one"}, {5, "two"}, {11, "one-1"}}
	if len(got) != len(want) {
		t.Fatalf("Headings = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Headings[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestEdgeCases(t *testing.T) {
	expect(t, "")
	expect(t, "a\x00b\n", "a�b")
}
