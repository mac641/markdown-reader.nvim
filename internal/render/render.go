// Package render turns Markdown into the HTML shown in the preview.
//
// It speaks GitHub's dialect (tables, task lists, strikethrough, autolinks,
// footnotes, alerts) and tags block elements with data-line, their 1-based
// source line, which the page uses to scroll to the editor's cursor.
// Headings get GitHub's anchor IDs.
package render

import (
	"bytes"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	alert "github.com/yuin/goldmark-alert"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	east "github.com/yuin/goldmark/v2/extension/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

var (
	mdParser = parser.New(
		parser.WithExtensions(extension.GFMParser, extension.FootnoteParser, alert.Parser),
		parser.WithASTTransformers(util.Prioritized[parser.ASTTransformer](blockTransformer{}, 1000)),
	)
	mdRenderer = html.New(
		// Raw HTML in the document is shown as such, like on GitHub.
		html.WithUnsafe(),
		html.WithExtensions(extension.GFMHTMLRenderer, extension.FootnoteHTMLRenderer, alert.HTMLRenderer,
			// Last, so that its renderers win.
			overrides{}),
	)
)

// HTML renders a Markdown document.
func HTML(src []byte) (string, error) {
	var out bytes.Buffer
	doc := mdParser.Parse(src)
	if err := mdRenderer.Render(&out, src, doc); err != nil {
		return "", err
	}
	return out.String(), nil
}

// A Heading is where a heading starts and its anchor ID.
type Heading struct {
	Line int // 1-based
	ID   string
}

// Headings lists the headings of a document in order.
func Headings(src []byte) []Heading {
	var hs []Heading
	_ = ast.Walk(mdParser.Parse(src), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering {
			line, _ := attr(h, "data-line", src)
			id, _ := attr(h, "id", src)
			l, _ := strconv.Atoi(line)
			hs = append(hs, Heading{l, id})
		}
		return ast.WalkContinue, nil
	})
	return hs
}

func attr(n ast.Node, name string, src []byte) (string, bool) {
	v, ok := n.Attribute(name)
	if !ok {
		return "", false
	}
	return v.Value(src), true
}

func setAttr(n ast.Node, name, value string) {
	n.SetAttribute(name, text.NewMultiLineValueFromString(value, text.IdentityDecoder))
}

// blockTransformer sets data-line on every block whose position is known,
// gives headings their IDs and marks task list items.
type blockTransformer struct{}

func (blockTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	lines := lineStarts(src)
	ids := map[string]int{} // IDs given out, for uniqueID
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(ast.BlockNode); !ok || !entering {
			return ast.WalkContinue, nil
		}
		switch n.Kind() {
		case ast.KindDocument, east.KindTableHeader, east.KindTableBody, east.KindTableCell:
			// Rows are enough to scroll by.
			return ast.WalkContinue, nil
		}
		if pos := n.Pos(); pos >= 0 {
			line := sort.Search(len(lines), func(i int) bool { return lines[i] > pos })
			setAttr(n, "data-line", strconv.Itoa(line))
		}
		if extension.IsTask(n) {
			setAttr(n, "class", "task-list-item")
		}
		if n.Kind() == ast.KindHeading {
			if id := uniqueID(ids, slug(plainText(n, src))); id != "" {
				setAttr(n, "id", id)
			}
		}
		return ast.WalkContinue, nil
	})
}

// lineStarts returns the byte offset at which each line begins.
func lineStarts(src []byte) []int {
	starts := []int{0}
	for i, c := range src {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// plainText is the text of n without markup, like the DOM's textContent.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.WriteString(c.Value.Value(src))
		case *ast.CodeSpan:
			b.WriteString(c.Value.Value(src))
			return ast.WalkSkipChildren, nil
		case *ast.AutoLink:
			b.WriteString(c.Label.Value(src))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// slug turns heading text into an anchor ID the way GitHub does: lower
// case, letters, digits, "-" and "_" kept, spaces turned into "-", all
// else dropped.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r), r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// uniqueID numbers repeated IDs like GitHub ("a", "a-1", "a-2"), skipping
// numbered IDs that headings already have.
func uniqueID(seen map[string]int, id string) string {
	unique := id
	for {
		if _, taken := seen[unique]; !taken {
			break
		}
		seen[id]++
		unique = id + "-" + strconv.Itoa(seen[id])
	}
	seen[unique] = 0
	return unique
}

// overrides replaces some of goldmark's node renderers.
type overrides struct{}

func (overrides) RendererOptions(*html.Config) []html.Option {
	return []html.Option{
		// goldmark's code blocks don't render attributes; ours put them on <pre>.
		html.WithNodeRenderer(ast.KindCodeBlock, html.NodeRendererFunc(renderCodeBlock)),
	}
}

// renderCodeBlock is goldmark's, plus attributes (i.e. data-line) on <pre>.
func renderCodeBlock(writer io.Writer, src []byte, node ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	w := writer.(util.BufWriter)
	if !entering {
		_, _ = w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	n := node.(*ast.CodeBlock)
	tw := html.ContextTextWriter(rc)
	_, _ = w.WriteString("<pre")
	html.RenderAttributes(w, src, n, nil, rc)
	_, _ = w.WriteString("><code")
	if lang, ok := n.Language(src); ok {
		_, _ = w.WriteString(` class="language-`)
		_, _ = tw.WriteString(lang)
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')
	_, _ = n.Value.WriteTo(tw, src)
	return ast.WalkContinue, nil
}
