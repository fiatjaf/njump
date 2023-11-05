package main

import (
	"context"
	"encoding/json"
	"fmt"
	html "html"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
	"mvdan.cc/xurls/v2"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip10"
	"github.com/nbd-wtf/go-nostr/nip19"
	sdk "github.com/nbd-wtf/nostr-sdk"
)

const XML_HEADER = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"

var (
	urlSuffixMatcher         = regexp.MustCompile(`[\w-_.]+\.[\w-_.]+(\/[\/\w]*)?$`)
	nostrEveryMatcher        = regexp.MustCompile(`nostr:((npub|note|nevent|nprofile|naddr)1[a-z0-9]+)\b`)
	nostrNoteNeventMatcher   = regexp.MustCompile(`nostr:((note|nevent)1[a-z0-9]+)\b`)
	nostrNpubNprofileMatcher = regexp.MustCompile(`nostr:((npub|nprofile)1[a-z0-9]+)\b`)

	urlMatcher = func() *regexp.Regexp {
		// hack to only allow these schemes while still using this library
		xurls.Schemes = []string{"https"}
		xurls.SchemesNoAuthority = []string{"blob"}
		xurls.SchemesUnofficial = []string{"http"}
		return xurls.Strict()
	}()
	imageExtensionMatcher = regexp.MustCompile(`.*\.(png|jpg|jpeg|gif|webp)(\?.*)?$`)
	videoExtensionMatcher = regexp.MustCompile(`.*\.(mp4|ogg|webm|mov)(\?.*)?$`)
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

var kindNIPs = map[int]string{
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

type ClientReference struct {
	ID   string
	Name string
	URL  template.URL
}

func generateClientList(style Style, code string, event *nostr.Event) []ClientReference {
	if event.Kind == 1 || event.Kind == 6 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "snort", Name: "Snort", URL: template.URL("https://Snort.social/e/" + code)},
			{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/n/" + code)},
			{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/thread/" + event.ID)},
			{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
			{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/thread/" + event.ID)},
			{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code)},
			{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code)},
			{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code)},
		}
	} else if event.Kind == 0 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "nosta", Name: "Nosta", URL: template.URL("https://nosta.me/" + code)},
			{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code)},
			{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/@" + code)},
			{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
			{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/profile/" + event.PubKey)},
			{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/u/" + code)},
			{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code)},
			{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code)},
		}
	} else if event.Kind == 30023 || event.Kind == 30024 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "yakihonne", Name: "YakiHonne", URL: template.URL("https://yakihonne.com/article/" + code)},
			{ID: "habla", Name: "Habla", URL: template.URL("https://habla.news/a/" + code)},
			{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code)},
			{ID: "blogstack", Name: "Blogstack", URL: template.URL("https://blogstack.io/" + code)},
		}
	} else if event.Kind == 1063 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code)},
			{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
		}
	}
	return nil
}

func generateRelayBrowserClientList(host string) []ClientReference {
	return []ClientReference{
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/relays/" + host)},
	}
}

type Style string

const (
	StyleTelegram   Style = "telegram"
	StyleTwitter          = "twitter"
	StyleIos              = "ios"
	StyleAndroid          = "android"
	StyleMattermost       = "mattermost"
	StyleSlack            = "slack"
	StyleDiscord          = "discord"
	StyleWhatsapp         = "whatsapp"
	StyleIframely         = "iframely"
	StyleNormal           = "normal"
	StyleUnknown          = "unknown"
)

func getPreviewStyle(r *http.Request) Style {
	if style := r.URL.Query().Get("style"); style != "" {
		// debug mode
		return Style(style)
	}

	ua := strings.ToLower(r.Header.Get("User-Agent"))
	accept := r.Header.Get("Accept")

	switch {
	case strings.Contains(ua, "telegrambot"):
		return StyleTelegram
	case strings.Contains(ua, "twitterbot"):
		return StyleTwitter
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ipod"):
		return StyleIos
	case strings.Contains(ua, "android"):
		return StyleAndroid
	case strings.Contains(ua, "mattermost"):
		return StyleMattermost
	case strings.Contains(ua, "slack"):
		return StyleSlack
	case strings.Contains(ua, "discord"):
		return StyleDiscord
	case strings.Contains(ua, "whatsapp"):
		return StyleWhatsapp
	case strings.Contains(ua, "iframely"):
		return StyleIframely
	case strings.Contains(accept, "text/html"):
		return StyleNormal
	default:
		return StyleUnknown
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

func attachRelaysToEvent(event *nostr.Event, relays ...string) {
	key := "rls:" + event.ID
	existingRelays := make([]string, 0, 10)
	if exists := cache.GetJSON(key, &existingRelays); exists {
		relays = unique(append(existingRelays, relays...))
	}
	cache.SetJSONWithTTL(key, relays, time.Hour*24*7)
}

func scheduleEventExpiration(eventId string, ts time.Duration) {
	key := "ttl:" + eventId
	nextExpiration := time.Now().Add(ts).Unix()
	var currentExpiration int64
	if exists := cache.GetJSON(key, &currentExpiration); exists {
		if nextExpiration < currentExpiration {
			return
		}
	}
	cache.SetJSON(key, nextExpiration)
}

// Rendering functions
// ### ### ### ### ### ### ### ### ### ### ###

func replaceURLsWithTags(input string, imageReplacementTemplate, videoReplacementTemplate string) string {
	return urlMatcher.ReplaceAllStringFunc(input, func(match string) string {
		switch {
		case imageExtensionMatcher.MatchString(match):
			// Match and replace image URLs with a custom replacement
			// Usually is html <img> => ` <img src="%s" alt=""> `
			// or markdown !()[...] tags for further processing => `![](%s)`
			return fmt.Sprintf(imageReplacementTemplate, match)
		case videoExtensionMatcher.MatchString(match):
			// Match and replace video URLs with a custom replacement
			// Usually is html <video> => ` <video controls width="100%%"><source src="%s"></video> `
			// or markdown !()[...] tags for further processing => `![](%s)`
			return fmt.Sprintf(videoReplacementTemplate, match)
		default:
			return "<a href=\"" + match + "\">" + match + "</a>"
		}
	})
}

func replaceNostrURLsWithTags(matcher *regexp.Regexp, input string) string {
	// match and replace npup1, nprofile1, note1, nevent1, etc
	return matcher.ReplaceAllStringFunc(input, func(match string) string {
		nip19 := match[len("nostr:"):]
		first_chars := nip19[:8]
		last_chars := nip19[len(nip19)-4:]
		if strings.HasPrefix(nip19, "npub1") || strings.HasPrefix(nip19, "nprofile1") {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
			defer cancel()
			name := getNameFromNip19(ctx, nip19)
			return fmt.Sprintf(`<a href="/%s" class="bg-lavender dark:bg-garnet px-1"><span class="font-bold">%s</span> (<span class="italic">%s</span>)</a>`, nip19, name, first_chars+"…"+last_chars)
		} else {
			return fmt.Sprintf(`<a href="/%s" class="bg-lavender dark:bg-garnet px-1">%s</a>`, nip19, first_chars+"…"+last_chars)
		}
	})
}

func shortenNostrURLs(input string) string {
	// match and replace npup1, nprofile1, note1, nevent1, etc
	return nostrEveryMatcher.ReplaceAllStringFunc(input, func(match string) string {
		nip19 := match[len("nostr:"):]
		first_chars := nip19[:8]
		last_chars := nip19[len(nip19)-4:]
		if strings.HasPrefix(nip19, "npub1") || strings.HasPrefix(nip19, "nprofile1") {
			return "@" + first_chars + "…" + last_chars
		} else {
			return "#" + first_chars + "…" + last_chars
		}
	})
}

func getNameFromNip19(ctx context.Context, nip19 string) string {
	author, _, err := getEvent(ctx, nip19, nil)
	if err != nil {
		return nip19
	}
	metadata, err := sdk.ParseMetadata(author)
	if err != nil {
		return nip19
	}
	if metadata.Name == "" {
		return nip19
	}
	return metadata.Name
}

// replaces an npub/nprofile with the name of the author, if possible
func replaceUserReferencesWithNames(ctx context.Context, input []string) []string {
	// Match and replace npup1 or nprofile1
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	for i, line := range input {
		input[i] = nostrNpubNprofileMatcher.ReplaceAllStringFunc(line, func(match string) string {
			submatch := nostrNpubNprofileMatcher.FindStringSubmatch(match)
			nip19 := submatch[1]
			return getNameFromNip19(ctx, nip19)
		})
	}
	return input
}

// replace nevent and note with their text, HTML-formatted
func renderQuotesAsHTML(ctx context.Context, input string, usingTelegramInstantView bool) string {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	return nostrNoteNeventMatcher.ReplaceAllStringFunc(input, func(match string) string {
		submatch := nostrNoteNeventMatcher.FindStringSubmatch(match)
		nip19 := submatch[1]

		event, _, err := getEvent(ctx, nip19, nil)
		if err != nil {
			log.Warn().Str("nip19", nip19).Msg("failed to get nip19")
			return nip19
		}

		content := fmt.Sprintf(
			`<blockquote class="pl-4 pr-0 pt-0 pb-2 border-l-05rem border-l-strongpink border-solid"><div class="-ml-4 bg-gray-100 dark:bg-zinc-800 mr-0 mt-0 mb-4 pl-4 pr-2 py-2">quoting %s </div> %s </blockquote>`, match, event.Content)
		return basicFormatting(content, false, usingTelegramInstantView)
	})
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

func basicFormatting(input string, skipNostrEventLinks bool, usingTelegramInstantView bool) string {
	nostrMatcher := nostrEveryMatcher
	if skipNostrEventLinks {
		nostrMatcher = nostrNpubNprofileMatcher
	}

	imageReplacementTemplate := ` <img src="%s"> `
	if usingTelegramInstantView {
		// telegram instant view doesn't like when there is an image inside a blockquote (like <p><img></p>)
		// so we use this custom thing to stop all blockquotes before the images, print the images then
		// start a new blockquote afterwards -- we do the same with the markdown renderer for <p> tags on mdToHtml
		imageReplacementTemplate = "</blockquote>" + imageReplacementTemplate + "<blockquote>"
	}

	lines := strings.Split(input, "\n")
	for i, line := range lines {
		line = replaceURLsWithTags(line,
			imageReplacementTemplate,
			`<video controls width="100%%" class="max-h-[90vh] bg-neutral-300 dark:bg-zinc-700"><source src="%s"></video>`,
		)

		line = replaceNostrURLsWithTags(nostrMatcher, line)
		lines[i] = line
	}
	return strings.Join(lines, "<br/>")
}

func previewNotesFormatting(input string) string {
	lines := strings.Split(input, "\n")
	var processedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		processedLine := shortenNostrURLs(line)
		processedLines = append(processedLines, processedLine)
	}

	return strings.Join(processedLines, "<br/>")
}

func mdToHTML(md string, usingTelegramInstantView bool) string {
	md = strings.ReplaceAll(md, "\u00A0", " ")
	md = replaceNostrURLsWithTags(nostrEveryMatcher, md)

	// create markdown parser with extensions
	p := parser.NewWithExtensions(
		parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock | parser.Footnotes)
	doc := p.Parse([]byte(md))

	var customNodeHook mdhtml.RenderNodeFunc = nil
	if usingTelegramInstantView {
		// telegram instant view really doesn't like when there is an image inside a paragraph (like <p><img></p>)
		// so we use this custom thing to stop all paragraphs before the images, print the images then start a new
		// paragraph afterwards.
		customNodeHook = func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			if img, ok := node.(*ast.Image); ok {
				if entering {
					src := img.Destination
					w.Write([]byte(`</p><img src="`))
					mdhtml.EscLink(w, src)
					w.Write([]byte(`" alt="`))
				} else {
					if img.Title != nil {
						w.Write([]byte(`" title="`))
						mdhtml.EscapeHTML(w, img.Title)
					}
					w.Write([]byte(`" /><p>`))
				}
				return ast.GoToNext, true
			}
			return ast.GoToNext, false
		}
	}

	// create HTML renderer with extensions
	opts := mdhtml.RendererOptions{
		Flags:          mdhtml.CommonFlags | mdhtml.HrefTargetBlank,
		RenderNodeHook: customNodeHook,
	}
	renderer := mdhtml.NewRenderer(opts)
	output := string(markdown.Render(doc, renderer))

	// sanitize content
	output = sanitizeXSS(output)

	return output
}

func unique(strSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range strSlice {
		if _, ok := keys[entry]; !ok {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func trimProtocol(relay string) string {
	relay = strings.TrimPrefix(relay, "wss://")
	relay = strings.TrimPrefix(relay, "ws://")
	relay = strings.TrimPrefix(relay, "wss:/") // Some browsers replace upfront '//' with '/'
	relay = strings.TrimPrefix(relay, "ws:/")  // Some browsers replace upfront '//' with '/'
	return relay
}

func normalizeWebsiteURL(u string) string {
	if strings.HasPrefix(u, "http") {
		return u
	}
	return "https://" + u
}

func eventToHTML(evt *nostr.Event) template.HTML {
	tagsHTML := "["
	for t, tag := range evt.Tags {
		tagsHTML += "\n    ["
		for i, item := range tag {
			cls := `"text-zinc-500 dark:text-zinc-50"`
			if i == 0 {
				cls = `"text-amber-500 dark:text-amber-200"`
			}
			itemJSON, _ := json.Marshal(item)
			tagsHTML += "\n      <span class=" + cls + ">" + html.EscapeString(string(itemJSON))
			if i < len(tag)-1 {
				tagsHTML += ","
			} else {
				tagsHTML += "\n    "
			}
		}
		tagsHTML += "]"
		if t < len(evt.Tags)-1 {
			tagsHTML += ","
		} else {
			tagsHTML += "\n  "
		}
	}
	tagsHTML += "]"

	contentJSON, _ := json.Marshal(evt.Content)

	keyCls := "text-purple-700 dark:text-purple-300"

	return template.HTML(fmt.Sprintf(
		`{
  <span class="`+keyCls+`">"id":</span> <span class="text-zinc-500 dark:text-zinc-50">"%s"</span>,
  <span class="`+keyCls+`">"pubkey":</span> <span class="text-zinc-500 dark:text-zinc-50">"%s"</span>,
  <span class="`+keyCls+`">"created_at":</span> <span class="text-green-600">%d</span>,
  <span class="`+keyCls+`">"kind":</span> <span class="text-amber-500 dark:text-amber-200">%d</span>,
  <span class="`+keyCls+`">"tags":</span> %s,
  <span class="`+keyCls+`">"content":</span> <span class="text-zinc-500 dark:text-zinc-50">%s</span>,
  <span class="`+keyCls+`">"sig":</span> <span class="text-zinc-500 dark:text-zinc-50 content">"%s"</span>
}`, evt.ID, evt.PubKey, evt.CreatedAt, evt.Kind, tagsHTML, html.EscapeString(string(contentJSON)), evt.Sig),
	)
}

func limitAt[V any](list []V, n int) []V {
	if len(list) < n {
		return list
	}
	return list[0:n]
}
