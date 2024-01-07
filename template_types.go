package main

import (
	"context"
	"html"
	"html/template"
	"regexp"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip10"
	"github.com/nbd-wtf/go-nostr/nip19"
	sdk "github.com/nbd-wtf/nostr-sdk"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

type Metadata struct {
	sdk.ProfileMetadata
}

func (m Metadata) Npub() string {
	npub, _ := nip19.EncodePublicKey(m.PubKey)
	return npub
}

func (m Metadata) NpubShort() string {
	npub := m.Npub()
	return npub[:8] + "…" + npub[len(npub)-4:]
}

type EnhancedEvent struct {
	event  *nostr.Event
	relays []string
}

func (ee EnhancedEvent) IsReply() bool {
	return nip10.GetImmediateReply(ee.event.Tags) != nil
}

func (ee EnhancedEvent) Reply() *nostr.Tag {
	return nip10.GetImmediateReply(ee.event.Tags)
}

func (ee EnhancedEvent) Preview() template.HTML {
	lines := strings.Split(html.EscapeString(ee.event.Content), "\n")
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
	content := ee.event.Content
	content = basicFormatting(html.EscapeString(content), true, false, false)
	content = renderQuotesAsHTML(context.Background(), content, false)
	if ee.IsReply() {
		nevent, _ := nip19.EncodeEvent(ee.Reply().Value(), ee.relays, ee.event.PubKey)
		neventShort := nevent[:8] + "…" + nevent[len(nevent)-4:]
		content = "In reply to <a href='/" + nevent + "'>" + neventShort + "</a><br/>_________________________<br/><br/>" + content
	}
	return content
}

func (ee EnhancedEvent) Thumb() string {
	imgRegex := regexp.MustCompile(`(https?://[^\s]+\.(?:png|jpe?g|gif|bmp|svg)(?:/[^\s]*)?)`)
	matches := imgRegex.FindAllStringSubmatch(ee.event.Content, -1)
	if len(matches) > 0 {
		// The first match group captures the image URL
		return matches[0][1]
	}
	return ""
}

func (ee EnhancedEvent) Npub() string {
	npub, _ := nip19.EncodePublicKey(ee.event.PubKey)
	return npub
}

func (ee EnhancedEvent) NpubShort() string {
	npub := ee.Npub()
	return npub[:8] + "…" + npub[len(npub)-4:]
}

func (ee EnhancedEvent) Nevent() string {
	nevent, _ := nip19.EncodeEvent(ee.event.ID, ee.relays, ee.event.PubKey)
	return nevent
}

func (ee EnhancedEvent) CreatedAtStr() string {
	return time.Unix(int64(ee.event.CreatedAt), 0).Format("2006-01-02 15:04:05")
}

func (ee EnhancedEvent) ModifiedAtStr() string {
	return time.Unix(int64(ee.event.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00")
}

type Kind1063Metadata struct {
	Magnet    string
	Dim       string
	Size      string
	Summary   string
	Image     string
	URL       string
	AES256GCM string
	M         string
	X         string
	I         string
	Blurhash  string
	Thumb     string
}

type Kind30311Metadata struct {
	Title    string
	Summary  string
	Image    string
	Status   string
	Host     sdk.ProfileMetadata
	HostNpub string
	Tags     []string
}

type Kind1311Metadata struct {
	// ...
}

func (fm Kind1063Metadata) IsVideo() bool { return strings.Split(fm.M, "/")[0] == "video" }
func (fm Kind1063Metadata) IsImage() bool { return strings.Split(fm.M, "/")[0] == "image" }
func (fm Kind1063Metadata) DisplayImage() string {
	if fm.Image != "" {
		return fm.Image
	} else if fm.IsImage() {
		return fm.URL
	} else {
		return ""
	}
}
