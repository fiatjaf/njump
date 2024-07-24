package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bytesparadise/libasciidoc"
	"github.com/bytesparadise/libasciidoc/pkg/configuration"
)

func parseWikilinks(asciidoc string) string {
	// Define the regex pattern to match the wiki links
	pattern := regexp.MustCompile(`\[\[([^\|\]]+)(?:\|([^\]]+))?\]\]`)

	// Define the replacement function
	replacement := func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		target := strings.TrimSpace(submatches[1])
		display := strings.TrimSpace(submatches[2])

		if display == "" {
			display = target
		}

		targetFormatted := strings.ToLower(strings.ReplaceAll(target, " ", "-"))
		return fmt.Sprintf("link:/wiki/%s[%s]", targetFormatted, display)
	}

	// Use regex ReplaceAllStringFunc with the replacement function
	transformedText := pattern.ReplaceAllStringFunc(asciidoc, replacement)
	return transformedText
}

func asciidocToHTML(asciidoc string) string {
	//Parsing wikilinks
	asciidoc = parseWikilinks(asciidoc)
	// Rendering
	input := strings.NewReader(asciidoc)
	var output = &strings.Builder{}
	config := configuration.NewConfiguration(
		[]configuration.Setting{
			configuration.WithFilename("test.adoc"),
			configuration.WithBackEnd("html5"),
		}...,
	)
	_, err := libasciidoc.Convert(input, output, config)
	if err != nil {
		log.Printf("Error converting AsciiDoc: %v", err)
	}
	return output.String()
}
