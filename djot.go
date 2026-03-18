package main

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/sivukhin/godjot/djot_parser"
	"github.com/sivukhin/godjot/html_writer"
)

func normalizeDTag(input string) string {
	input = strings.ToLower(input)
	input = strings.Join(strings.Fields(input), "-")

	var sb strings.Builder
	prevDash := false
	for _, r := range input {
		if r == '-' {
			if !prevDash {
				sb.WriteRune(r)
			}
			prevDash = true
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r > 127 {
			sb.WriteRune(r)
			prevDash = false
		}
	}
	result := strings.Trim(sb.String(), "-")
	return result
}

func processWikilinks(djotInput string) string {
	lines := strings.Split(djotInput, "\n")

	defPattern := regexp.MustCompile(`^\[([^\]]+)\]:\s*(.+)$`)
	definedRefs := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		match := defPattern.FindStringSubmatch(line)
		if match != nil {
			ref := strings.TrimSpace(match[1])
			definedRefs[ref] = true
		}
	}

	implicitPattern := regexp.MustCompile(`\[([^\]]+)\]\[\]`)
	explicitPattern := regexp.MustCompile(`\[([^\]]+)\]\[([^\]]+)\]`)

	djotInput = implicitPattern.ReplaceAllStringFunc(djotInput, func(match string) string {
		submatch := implicitPattern.FindStringSubmatch(match)
		display := strings.TrimSpace(submatch[1])

		if definedRefs[display] {
			return match
		}

		normalized := normalizeDTag(display)
		return `<span class="wikilink" title="wikilink to ` + html.EscapeString(normalized) + `">` + display + `</span>`
	})

	djotInput = explicitPattern.ReplaceAllStringFunc(djotInput, func(match string) string {
		submatch := explicitPattern.FindStringSubmatch(match)
		display := strings.TrimSpace(submatch[1])
		ref := strings.TrimSpace(submatch[2])

		if ref == "" {
			if definedRefs[display] {
				return match
			}
			normalized := normalizeDTag(display)
			return `<span class="wikilink" title="wikilink to ` + html.EscapeString(normalized) + `">` + display + `</span>`
		}

		if definedRefs[ref] {
			return match
		}

		normalized := normalizeDTag(ref)
		return `<span class="wikilink" title="wikilink to ` + html.EscapeString(normalized) + `">` + display + `</span>`
	})

	return djotInput
}

var nostrLinkMatcher = regexp.MustCompile(`href="nostr:((npub|note|nevent|nprofile|naddr)1[a-z0-9]+)"`)

func djotToHTML(djotInput string) string {
	djotInput = processWikilinks(djotInput)

	ast := djot_parser.BuildDjotAst([]byte(djotInput))
	result := djot_parser.NewConversionContext(
		"html",
		djot_parser.DefaultConversionRegistry,
		nil,
	).ConvertDjotToHtml(&html_writer.HtmlWriter{}, ast...)

	result = nostrLinkMatcher.ReplaceAllString(result, `href="/$1"`)

	return result
}
