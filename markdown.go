package main

import (
	"io"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

var mdparser = parser.NewWithExtensions(
	parser.CommonExtensions |
		parser.AutoHeadingIDs |
		parser.NoEmptyLineBeforeBlock |
		parser.Footnotes,
)

var mdrenderer = html.NewRenderer(html.RendererOptions{
	Flags: html.CommonFlags | html.HrefTargetBlank,
})

func stripLinksFromMarkdown(md string) string {
	// Regular expression to match Markdown links and HTML links
	linkRegex := regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)|<a[^>]*>(.*?)</a>`)

	// Replace both Markdown and HTML links with just the link text
	strippedMD := linkRegex.ReplaceAllString(md, "$1$2")

	return strippedMD
}

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

func mdToHTML(md string, usingTelegramInstantView bool, skipLinks bool) string {
	md = strings.ReplaceAll(md, "\u00A0", " ")
	md = replaceNostrURLsWithTags(nostrEveryMatcher, md)

	// create markdown parser with extensions
	doc := mdparser.Parse([]byte(md))

	renderer := mdrenderer
	if usingTelegramInstantView {
		renderer = tgivmdrenderer
	}

	// create HTML renderer with extensions
	output := string(markdown.Render(doc, renderer))

	if skipLinks {
		output = stripLinksFromMarkdown(output)
	}

	// sanitize content
	output = sanitizeXSS(output)

	return output
}
