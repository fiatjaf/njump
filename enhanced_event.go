package main

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip10"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip22"
	"fiatjaf.com/nostr/nip73"
	"fiatjaf.com/nostr/sdk"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

type EnhancedEvent struct {
	*nostr.Event
	relays  []string
	subject string
	summary string
	author  sdk.ProfileMetadata
}

func NewEnhancedEvent(
	ctx context.Context,
	event nostr.Event,
) EnhancedEvent {
	ee := EnhancedEvent{Event: &event}

	for _, tag := range event.Tags {
		if len(tag) < 2 {
			continue
		}

		if tag[0] == "subject" || tag[0] == "title" {
			ee.subject = tag[1]
		}
		if tag[0] == "summary" {
			ee.summary = tag[1]
		}
	}

	ee.author = getMetadata(ctx, event)

	return ee
}

func (ee EnhancedEvent) authorLong() string {
	if ee.author.Name != "" {
		return fmt.Sprintf("%s (%s)", ee.author.Name, ee.author.NpubShort())
	}
	return ee.author.Npub()
}

func (ee EnhancedEvent) getParent() nostr.Pointer {
	switch ee.Kind {
	case 1, 1063:
		return nip10.GetImmediateParent(ee.Tags)
	case 1111:
		return nip22.GetImmediateParent(ee.Tags)
	case 1311:
		if atag := ee.Tags.Find("a"); atag != nil {
			pointer, err := nostr.EntityPointerFromTag(atag)
			if err == nil {
				return pointer
			}
		}
	}

	return nil
}

func (ee EnhancedEvent) isReply() bool {
	return nip22.GetImmediateParent(ee.Tags) != nil ||
		nip10.GetImmediateParent(ee.Event.Tags) != nil
}

func (ee EnhancedEvent) Preview() template.HTML {
	lines := strings.Split(html.EscapeString(ee.Event.Content), "\n")
	var processedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		processedLine := shortenNostrURLs(line)
		processedLines = append(processedLines, processedLine)
	}

	return template.HTML(strings.Join(processedLines, "<br/>"))
}

func (ee EnhancedEvent) RssTitle() string {
	regex := regexp.MustCompile(`(?i)<br\s?/?>`)
	replacedString := regex.ReplaceAllString(string(ee.Preview()), " ")
	words := strings.Fields(replacedString)
	title := ""
	for i, word := range words {
		if len(title)+len(word)+1 <= 65 { // +1 for space
			if title != "" {
				title += " "
			}
			title += word
		} else {
			if i > 1 { // the first word len is > 65
				title = title + " ..."
			} else {
				title = ""
			}
			break
		}
	}

	content := ee.RssContent()
	distance := levenshtein.DistanceForStrings([]rune(title), []rune(content), levenshtein.DefaultOptions)
	similarityThreshold := 5
	if distance <= similarityThreshold {
		return ""
	} else {
		return title
	}
}

func (ee EnhancedEvent) RssContent() string {
	content := ee.Event.Content
	content = basicFormatting(html.EscapeString(content), true, false, false)
	content = renderQuotesAsHTML(context.Background(), content, false)
	if parent := ee.getParent(); parent != nil {
		if external, ok := parent.(nip73.ExternalPointer); ok {
			content = "In reply to <a target='_blank' href='" + external.Thing + "'>" + external.Thing + "</a><br/>_________________________<br/><br/>" + content
		} else {
			code := nip19.EncodePointer(parent)
			codeShort := code[:8] + "…" + code[len(code)-4:]
			content = "In reply to <a href='/" + code + "'>" + codeShort + "</a><br/>_________________________<br/><br/>" + content
		}
	}
	return content
}

func (ee EnhancedEvent) Thumb() string {
	imgRegex := regexp.MustCompile(`(https?://[^\s]+\.(?:png|jpe?g|gif|bmp|svg)(?:/[^\s]*)?)`)
	matches := imgRegex.FindAllStringSubmatch(ee.Event.Content, -1)
	if len(matches) > 0 {
		// The first match group captures the image URL
		return matches[0][1]
	}
	return ""
}

func (ee EnhancedEvent) Npub() string {
	return nip19.EncodeNpub(ee.Event.PubKey)
}

func (ee EnhancedEvent) NpubShort() string {
	npub := ee.Npub()
	return npub[:8] + "…" + npub[len(npub)-4:]
}

func (ee EnhancedEvent) Nevent() string {
	return nip19.EncodeNevent(ee.Event.ID, ee.relays, ee.Event.PubKey)
}

func (ee EnhancedEvent) CreatedAtStr() string {
	return time.Unix(int64(ee.Event.CreatedAt), 0).Format("2006-01-02 15:04:05 MST")
}

func (ee EnhancedEvent) ModifiedAtStr() string {
	return time.Unix(int64(ee.Event.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00")
}
