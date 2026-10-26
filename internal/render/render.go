// Package render turns Markdown into the HTML shown in the preview.
//
// It speaks GitHub's dialect (tables, task lists, strikethrough, autolinks,
// footnotes, alerts) and tags block elements with data-line, their 1-based
// source line, which the page uses to scroll to the editor's cursor.
package render

import (
	"bytes"
	"io"
	"sort"
	"strconv"

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
		parser.WithASTTransformers(util.Prioritized[parser.ASTTransformer](lineTransformer{}, 1000)),
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

// lineTransformer sets data-line on every block whose position is known
// and marks task list items.
type lineTransformer struct{}

func (lineTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	lines := lineStarts(reader.Source())
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
			n.SetAttribute("data-line", text.NewMultiLineValueFromString(strconv.Itoa(line), text.IdentityDecoder))
		}
		if extension.IsTask(n) {
			n.SetAttribute("class", text.NewMultiLineValueFromString("task-list-item", text.IdentityDecoder))
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
