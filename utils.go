package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip10"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/pelletier/go-toml"
)

var kindNames = map[int]string{
	0:     "Metadata",
	1:     "Short Text Note",
	2:     "Recommend Relay",
	3:     "Contacts",
	4:     "Encrypted Direct Messages",
	5:     "Event Deletion",
	6:     "Reposts",
	7:     "Reaction",
	8:     "Badge Award",
	40:    "Channel Creation",
	41:    "Channel Metadata",
	42:    "Channel Message",
	43:    "Channel Hide Message",
	44:    "Channel Mute User",
	1063:  "File Metadata",
	1984:  "Reporting",
	9734:  "Zap Request",
	9735:  "Zap",
	10000: "Mute List",
	10001: "Pin List",
	10002: "Relay List Metadata",
	13194: "Wallet Info",
	22242: "Client Authentication",
	23194: "Wallet Request",
	23195: "Wallet Response",
	24133: "Nostr Connect",
	30000: "Categorized People List",
	30001: "Categorized Bookmark List",
	30008: "Profile Badges",
	30009: "Badge Definition",
	30017: "Create or update a stall",
	30018: "Create or update a product",
	30023: "Long-form Content",
	30078: "Application-specific Data",
}

var kindNIPS = map[int]string{
	0:     "01",
	1:     "01",
	2:     "01",
	3:     "02",
	4:     "04",
	5:     "09",
	6:     "18",
	7:     "25",
	8:     "58",
	40:    "28",
	41:    "28",
	42:    "28",
	43:    "28",
	44:    "28",
	1063:  "94",
	1984:  "56",
	9734:  "57",
	9735:  "57",
	10000: "51",
	10001: "51",
	10002: "65",
	13194: "47",
	22242: "42",
	23194: "47",
	23195: "47",
	24133: "46",
	30000: "51",
	30001: "51",
	30008: "58",
	30009: "58",
	30017: "15",
	30018: "15",
	30023: "23",
	30078: "78",
}

var urlMatcher = regexp.MustCompile(`^(wss?:\/\/)?[\w-_.]+\.[\w-_.]+(\/[\/\w]*)?$`)

type ClientReference struct {
	Name string
	URL  string
}

func generateClientList(code string, event *nostr.Event) []ClientReference {
	if event.Kind == 1 || event.Kind == 6 {
		return []ClientReference{
			{Name: "native client", URL: "nostr:" + code},
			{Name: "Snort", URL: "https://Snort.social/e/" + code},
			{Name: "Coracle", URL: "https://coracle.social/" + code},
			{Name: "Satellite", URL: "https://satellite.earth/thread/" + event.ID},
			{Name: "Agora", URL: "https://agorasocial.app/" + code},
			{Name: "Iris", URL: "https://iris.to/" + code},
			{Name: "Yosup", URL: "https://yosup.app/thread/" + event.ID},
			{Name: "Primal", URL: "https://primal.net/thread/" + event.ID},
			{Name: "Nostr.band", URL: "https://nostr.band/" + code},
			{Name: "Highlighter", URL: "https://highlighter.com/a/" + code},
		}
	} else if event.Kind == 0 {
		return []ClientReference{
			{Name: "Your native client", URL: "nostr:" + code},
			{Name: "Nosta", URL: "https://nosta.me/" + code},
			{Name: "Snort", URL: "https://snort.social/p/" + code},
			{Name: "Coracle", URL: "https://coracle.social/" + code},
			{Name: "Satellite", URL: "https://satellite.earth/@" + code},
			{Name: "Agora", URL: "https://agorasocial.app/" + code},
			{Name: "Iris", URL: "https://iris.to/" + code},
			{Name: "Yosup", URL: "https://yosup.app/profile/" + event.PubKey},
			{Name: "Primal", URL: "https://primal.net/profile/" + event.PubKey},
			{Name: "Nostr.band", URL: "https://nostr.band/" + code},
			{Name: "Highlighter", URL: "https://highlighter.com/p/" + event.PubKey},
		}
	} else if event.Kind == 30023 {
		return []ClientReference{
			{Name: "Your native client", URL: "nostr:" + code},
			{Name: "YakiHonne", URL: "https://yakihonne.com/article/" + code},
			{Name: "Habla", URL: "https://habla.news/a/" + code},
			{Name: "Highlighter", URL: "https://highlighter.com/a/" + code},
			{Name: "Blogstack", URL: "https://blogstack.io/" + code},
		}
	}

	return nil
}

func mergeMaps[K comparable, V any](m1 map[K]V, m2 map[K]V) map[K]V {
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

func prettyJsonOrRaw(j string) string {
	var parsedContent any
	if err := json.Unmarshal([]byte(j), &parsedContent); err == nil {
		if t, err := toml.Marshal(parsedContent); err == nil && len(t) > 0 {
			return string(t)
		}
	}
	return j
}

func getPreviewStyle(r *http.Request) string {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	accept := r.Header.Get("Accept")

	switch {
	case strings.Contains(ua, "telegrambot"):
		return "telegram"
	case strings.Contains(ua, "twitterbot"):
		return "twitter"
	case strings.Contains(ua, "mattermost"):
		return "mattermost"
	case strings.Contains(ua, "slack"):
		return "slack"
	case strings.Contains(ua, "discord"):
		return "discord"
	case strings.Contains(ua, "whatsapp"):
		return "whatsapp"
	case strings.Contains(accept, "text/html"):
		return ""
	default:
		return "unknown"
	}
}

func getParentNevent(event *nostr.Event) string {
	parentNevent := ""
	replyTag := nip10.GetImmediateReply(event.Tags)
	if replyTag != nil {
		relay := ""
		if len(*replyTag) > 2 {
			relay = (*replyTag)[2]
		} else {
			relay = ""
		}
		parentNevent, _ = nip19.EncodeEvent((*replyTag)[1], []string{relay}, "")
	}
	return parentNevent
}

// Rendering functions
// ### ### ### ### ### ### ### ### ### ### ###

func replateImageURLsWithTags(input string, replacement string) string {
	// Match and replace image URLs with a custom replacement
	// Usually is html <img> => ` <img src="%s" alt=""> `
	// or markdown !()[...] tags for further processing => `![](%s)`
	var regex *regexp.Regexp
	imgsPattern := `\S*(\()?(https?://\S+(\.jpg|\.jpeg|\.png|\.webp|\.gif))\S*`
	regex = regexp.MustCompile(imgsPattern)
	input = regex.ReplaceAllStringFunc(input, func(match string) string {
		submatch := regex.FindStringSubmatch(match)
		if len(submatch) < 2 ||
			strings.Contains(submatch[0], "](") { // Markdown ![](...) image
			return match
		}
		capturedGroup := submatch[2]
		replacement := fmt.Sprintf(replacement, capturedGroup)
		return replacement
	})
	return input
}

func replateVideoURLsWithTags(input string, replacement string) string {
	// Match and replace video URLs with a custom replacement
	// Usually is html <video> => ` <video controls width="100%%"><source src="%s"></video> `
	// or markdown !()[...] tags for further processing => `![](%s)`
	var regex *regexp.Regexp
	videoPattern := `\S*(https?://\S+(\.mp4|\.ogg|\.webm|.mov))\S*`
	regex = regexp.MustCompile(videoPattern)
	input = regex.ReplaceAllStringFunc(input, func(match string) string {
		submatch := regex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		capturedGroup := submatch[1]
		replacement := fmt.Sprintf(replacement, capturedGroup)
		return replacement
	})
	return input
}

func replaceNostrURLsWithTags(input string) string {
	// Match and replace npup1, nprofile1, note1, nevent1, etc
	nostrRegexPattern := `\S*(nostr:)?((npub|note|nevent|nprofile|naddr)1[a-z0-9]+)\b`
	nostrRegex := regexp.MustCompile(nostrRegexPattern)
	input = nostrRegex.ReplaceAllStringFunc(input, func(match string) string {
		submatch := nostrRegex.FindStringSubmatch(match)
		if len(submatch) < 2 || strings.Contains(submatch[0], "/") {
			return match
		}
		capturedGroup := submatch[2]
		first6 := capturedGroup[:6]
		last6 := capturedGroup[len(capturedGroup)-6:]
		replacement := fmt.Sprintf(`<a href="/%s" class="nostr">%s</a>`, capturedGroup, first6+"…"+last6)
		return replacement
	})
	return input
}

func replaceURLsWithTags(line string) string {
	var rline string

	rline = replateImageURLsWithTags(line, ` <img src="%s" alt=""> `)
	if rline != line {
		return rline
	}

	rline = replateVideoURLsWithTags(line, `<video controls width="100%%"><source src="%s"></video>`)
	if rline != line {
		return rline
	}

	line = replaceNostrURLsWithTags(line)

	// Match and replace other URLs with <a> tags
	hrefRegexPattern := `\S*(https?://\S+)\S*`
	hrefRegex := regexp.MustCompile(hrefRegexPattern)
	line = hrefRegex.ReplaceAllString(line, `<a href="$1">$1</a>`)

	return line
}

func sanitizeXSS(html string) string {
	p := bluemonday.UGCPolicy()
	p.AllowStyling()
	p.RequireNoFollowOnLinks(false)
	p.AllowElements("video", "source", "iframe")
	p.AllowAttrs("controls", "width").OnElements("video")
	p.AllowAttrs("src", "width").OnElements("source")
	p.AllowAttrs("src", "frameborder").OnElements("iframe")
	return p.Sanitize(html)
}

func basicFormatting(input string) string {
	lines := strings.Split(input, "\n")

	var processedLines []string
	for _, line := range lines {
		processedLine := replaceURLsWithTags(line)
		processedLines = append(processedLines, processedLine)
	}

	return strings.Join(processedLines, "<br/>")
}

func mdToHTML(md string) string {
	md = strings.ReplaceAll(md, "\u00A0", " ")
	md = replateImageURLsWithTags(md, `![](%s)`)
	md = replateVideoURLsWithTags(md, `<video controls width="100%%"><source src="%s"></video>`)
	md = replaceNostrURLsWithTags(md)

	// create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock | parser.Footnotes
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(md))

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	output := string(markdown.Render(doc, renderer))

	// Sanitize content
	output = sanitizeXSS(output)

	return output
}
