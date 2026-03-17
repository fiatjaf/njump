package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sivukhin/godjot/djot_parser"
	"github.com/sivukhin/godjot/html_writer"
)

func parseWikilinks(djot string) string {
	pattern := regexp.MustCompile(`\[\[([^\|\]]+)(?:\|([^\]]+))?\]\]`)

	replacement := func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		target := strings.TrimSpace(submatches[1])
		display := strings.TrimSpace(submatches[2])

		if display == "" {
			display = target
		}

		targetFormatted := strings.ToLower(strings.ReplaceAll(target, " ", "-"))
		return fmt.Sprintf("[.bg-lavender.dark:prose:text-neutral-50.dark:text-neutral-50.dark:bg-garnet.px-1]#%s# [.wikilinks]#(link:https://wikistr.com/%s[Wikistr], link:https://wikifreedia.xyz/%s[Wikifreedia])#", display, targetFormatted, targetFormatted)
	}

	transformedText := pattern.ReplaceAllStringFunc(djot, replacement)
	return transformedText
}

func djotToHTML(djotInput string) string {
	djotInput = parseWikilinks(djotInput)

	ast := djot_parser.BuildDjotAst([]byte(djotInput))
	result := djot_parser.NewConversionContext(
		"html",
		djot_parser.DefaultConversionRegistry,
		nil,
	).ConvertDjotToHtml(&html_writer.HtmlWriter{}, ast...)

	return result
}
