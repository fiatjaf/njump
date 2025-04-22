package main

import (
	stdhtml "html"
	"io"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
)

var mdrenderer = html.NewRenderer(html.RendererOptions{
	Flags: html.HrefTargetBlank | html.SkipHTML,
	RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
		switch v := node.(type) {
		case *ast.HTMLSpan:
			w.Write([]byte(stdhtml.EscapeString(string(v.Literal))))
			return ast.GoToNext, true
		case *ast.HTMLBlock:
			w.Write([]byte(stdhtml.EscapeString(string(v.Literal))))
			return ast.GoToNext, true
		}

		return ast.GoToNext, false
	},
})

var tgivmdrenderer = html.NewRenderer(html.RendererOptions{
	Flags: html.CommonFlags | html.HrefTargetBlank,
	RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
		// telegram instant view really doesn't like when there is an image inside a paragraph (like <p><img></p>)
		// so we use this custom thing to stop all paragraphs before the images, print the images then start a new
		// paragraph afterwards.
		if img, ok := node.(*ast.Image); ok {
			if entering {
				src := img.Destination
				w.Write([]byte(`</p><img src="`))
				html.EscLink(w, src)
				w.Write([]byte(`" alt="`))
			} else {
				if img.Title != nil {
					w.Write([]byte(`" title="`))
					html.EscapeHTML(w, img.Title)
				}
				w.Write([]byte(`" /><p>`))
			}
			return ast.GoToNext, true
		}
		return ast.GoToNext, false
	},
})

func mdToHTML(md string, usingTelegramInstantView bool) string {
	md = strings.ReplaceAll(md, "\u00A0", " ")

	// create markdown parser with extensions
	// this parser is stateful so it must be reinitialized every time
	doc := parser.NewWithExtensions(
		parser.AutoHeadingIDs |
			parser.NoIntraEmphasis |
			parser.FencedCode |
			parser.Autolink |
			parser.Footnotes |
			parser.SpaceHeadings |
			parser.Tables,
	).Parse([]byte(md))

	renderer := mdrenderer
	if usingTelegramInstantView {
		renderer = tgivmdrenderer
	}

	// create HTML renderer with extensions
	output := string(markdown.Render(doc, renderer))

	// sanitize content
	output = sanitizeXSS(output)

	// nostr urls
	output = replaceNostrURLsWithHTMLTags(nostrEveryMatcher, output)

	return output
}

func sanitizeXSS(html string) string {
	p := bluemonday.UGCPolicy()
	p.RequireNoFollowOnLinks(false)
	p.AllowElements("video", "source")
	p.AllowAttrs("controls", "width").OnElements("video")
	p.AllowAttrs("src", "width").OnElements("source")
	return p.Sanitize(html)
}
