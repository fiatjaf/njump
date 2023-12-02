package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/exp/slices"
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
	1311:  "Live Chat Message",
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
	30311: "Live Event",
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
	1311:  "53",
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
	30311: "53",
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
		var relays []string
		if (len(*replyTag) > 2) && ((*replyTag)[2] != "") {
			relays = []string{(*replyTag)[2]}
		}
		parentNevent, _ = nip19.EncodeEvent((*replyTag)[1], relays, "")
	}
	return parentNevent
}

func attachRelaysToEvent(eventId string, relays ...string) []string {
	key := "rls:" + eventId
	existingRelays := make([]string, 0, 10)
	if exists := cache.GetJSON(key, &existingRelays); exists {
		relays = unique(append(existingRelays, relays...))
	}

	// cleanup
	filtered := make([]string, 0, len(relays))
	for _, relay := range relays {
		if !isntRealRelay(relay) {
			filtered = append(filtered, relay)
		}
	}

	cache.SetJSONWithTTL(key, filtered, time.Hour*24*7)
	return filtered
}

func getRelaysForEvent(eventId string) []string {
	key := "rls:" + eventId
	relays := make([]string, 0, 10)
	cache.GetJSON(key, &relays)
	return relays
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

func replaceURLsWithTags(input string, imageReplacementTemplate, videoReplacementTemplate string, skipLinks bool) string {
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
			if skipLinks {
				return match
			} else {
				return "<a href=\"" + match + "\">" + match + "</a>"
			}
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
			return fmt.Sprintf(`<a href="/%s" class="bg-lavender dark:prose:text-neutral-50 dark:text-neutral-50 dark:bg-garnet px-1"><span>%s</span> (<span class="italic">%s</span>)</a>`, nip19, name, first_chars+"…"+last_chars)
		} else {
			return fmt.Sprintf(`<a href="/%s" class="bg-lavender dark:prose:text-neutral-50 dark:text-neutral-50 dark:bg-garnet px-1">%s</a>`, nip19, first_chars+"…"+last_chars)
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
			`<blockquote class="border-l-05rem border-l-strongpink border-solid"><div class="-ml-4 bg-gradient-to-r from-gray-100 dark:from-zinc-800 to-transparent mr-0 mt-0 mb-4 pl-4 pr-2 py-2">quoting %s </div> %s </blockquote>`, match, event.Content)
		return basicFormatting(content, false, usingTelegramInstantView, false)
	})
}

func linkQuotes(input string) string {
	return nostrNoteNeventMatcher.ReplaceAllStringFunc(input, func(match string) string {
		nip19 := match[len("nostr:"):]
		first_chars := nip19[:8]
		last_chars := nip19[len(nip19)-4:]
		return fmt.Sprintf(`<a href="/%s">%s</a>`, nip19, first_chars+"…"+last_chars)
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

func basicFormatting(input string, skipNostrEventLinks bool, usingTelegramInstantView bool, skipLinks bool) string {
	nostrMatcher := nostrEveryMatcher
	if skipNostrEventLinks {
		nostrMatcher = nostrNpubNprofileMatcher
	}

	imageReplacementTemplate := ` <img src="%s"> `
	videoReplacementTemplate := `<video controls width="100%%" class="max-h-[90vh] bg-neutral-300 dark:bg-zinc-700"><source src="%s"></video>`
	if usingTelegramInstantView {
		// telegram instant view doesn't like when there is an image inside a blockquote (like <p><img></p>)
		// so we use this custom thing to stop all blockquotes before the images, print the images then
		// start a new blockquote afterwards -- we do the same with the markdown renderer for <p> tags on mdToHtml
		imageReplacementTemplate = "</blockquote>" + imageReplacementTemplate + "<blockquote>"
		videoReplacementTemplate = "</blockquote>" + videoReplacementTemplate + "<blockquote>"
	}

	lines := strings.Split(input, "\n")
	for i, line := range lines {
		line = replaceURLsWithTags(line, imageReplacementTemplate, videoReplacementTemplate, skipLinks)
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

func unique(strSlice []string) []string {
	slices.Sort(strSlice)
	j := 0
	for i := 1; i < len(strSlice); i++ {
		if strSlice[j] != strSlice[i] {
			j++
			strSlice[j] = strSlice[i]
		}
	}
	return strSlice[:j+1]
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

func humanDate(createdAt nostr.Timestamp) string {
	ts := createdAt.Time()
	now := time.Now()
	if ts.Before(now.AddDate(0, -9, 0)) {
		return ts.UTC().Format("02 Jan 2006")
	} else if ts.Before(now.AddDate(0, 0, -6)) {
		return ts.UTC().Format("Jan _2")
	} else {
		return ts.UTC().Format("Mon, Jan _2 15:04 UTC")
	}
}

func shouldUseRelayForNip19(relayUrl string) bool {
	for _, excluded := range excludedRelays {
		if strings.Contains(relayUrl, excluded) {
			return false
		}
	}
	urlp, err := url.Parse(relayUrl)
	if err != nil {
		return false
	}
	if urlp.Scheme != "wss" && urlp.Scheme != "ws" {
		return false
	}
	if urlp.Path != "" && urlp.Path != "/" {
		return false
	}
	return true
}

func getRandomRelay() string {
	if serial == 0 {
		serial = rand.Intn(len(everything))
	}
	serial = (serial + 1) % len(everything)
	return everything[serial]
}

func isntRealRelay(url string) bool {
	if len(url) < 6 {
		// this is just invalid
		return true
	}

	// hardcoded
	if url == "wss://relay.noswhere.com" {
		return true
	}

	// if there is a "/" after the initial "wss://" part that means this is probably a "virtual relay"
	// like wss://feeds.nostr.band/topic or wss://filter.nostr.wine/pubkey or wss://cache2.primal.net/v1
	// and should not be used in computing outbox model relay recommendations
	substr := []byte(url[6:])
	return bytes.IndexByte(substr, '/') != -1
}
