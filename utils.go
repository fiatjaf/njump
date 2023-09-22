package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
	"mvdan.cc/xurls/v2"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip10"
	"github.com/nbd-wtf/go-nostr/nip19"
)

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
			{Name: "Agora", URL: "https://agorasocial.app/" + event.ID},
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
			{Name: "Agora", URL: "https://agorasocial.app/people/" + event.PubKey},
			{Name: "Iris", URL: "https://iris.to/" + code},
			{Name: "Yosup", URL: "https://yosup.app/profile/" + event.PubKey},
			{Name: "Primal", URL: "https://primal.net/profile/" + event.PubKey},
			{Name: "Nostr.band", URL: "https://nostr.band/" + code},
			{Name: "Highlighter", URL: "https://highlighter.com/p/" + event.PubKey},
		}
	} else if event.Kind == 30023 || event.Kind == 30024 {
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

func replaceNostrURLs(matcher *regexp.Regexp, input string, style string) string {
	// Match and replace npup1, nprofile1, note1, nevent1, etc
	input = matcher.ReplaceAllStringFunc(input, func(match string) string {
		nip19 := match[len("nostr:"):]
		first_chars := nip19[:8]
		last_chars := nip19[len(nip19)-4:]
		replacement := ""
		if strings.HasPrefix(nip19, "npub1") || strings.HasPrefix(nip19, "nprofile1") {
			if style == "tags" {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
				defer cancel()
				name := getNameFromNip19(ctx, nip19)
				replacement = fmt.Sprintf(`<a href="/%s" class="nostr" ><strong>%s</strong> (<i>%s</i>)</a>`, nip19, name, first_chars+"…"+last_chars)
			} else if style == "short" {
				replacement = "@" + first_chars + "…" + last_chars
			} else {
				replacement = nip19
			}
			return replacement
		} else {
			if style == "tags" {
				replacement = fmt.Sprintf(`<a href="/%s" class="nostr">%s</a>`, nip19, first_chars+"…"+last_chars)
			} else if style == "short" {
				replacement = "#" + first_chars + "…" + last_chars
			} else {
				replacement = nip19
			}
			return replacement
		}
	})
	return input
}

func replaceNostrURLsWithTags(matcher *regexp.Regexp, input string) string {
	return replaceNostrURLs(matcher, input, "tags")
}

func shortenNostrURLs(input string) string {
	return replaceNostrURLs(nostrEveryMatcher, input, "short")
}

func getNameFromNip19(ctx context.Context, nip19 string) string {
	author, err := getEvent(ctx, nip19)
	if err != nil {
		return nip19
	}
	metadata, err := nostr.ParseMetadata(*author)
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

		event, err := getEvent(ctx, nip19)
		if err != nil {
			log.Warn().Str("nip19", nip19).Msg("failed to get nip19")
			return nip19
		}

		content := fmt.Sprintf(
			`<blockquote class="mention"><div>quoting %s </div> %s </blockquote>`, match, event.Content)
		return basicFormatting(content, false, usingTelegramInstantView)
	})
}

// replace nevent and note with their text, as an extra line prefixed by >
// this returns a slice of lines
func renderQuotesAsArrowPrefixedText(ctx context.Context, input string) []string {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	blocks := make([]string, 0, 8)
	matches := nostrNoteNeventMatcher.FindAllStringSubmatchIndex(input, -1)

	if len(matches) == 0 {
		// no matches, just return text as it is
		blocks = append(blocks, input)
		return blocks
	}

	// one or more matches, return multiple lines
	blocks = append(blocks, input[0:matches[0][0]])
	i := -1 // matches iteration counter
	b := 0  // current block index
	for _, match := range matches {
		i++

		matchText := input[match[0]:match[1]]
		submatch := nostrNoteNeventMatcher.FindStringSubmatch(matchText)
		nip19 := submatch[2]

		event, err := getEvent(ctx, nip19)
		if err != nil {
			// error case concat this to previous block
			blocks[b] += matchText
			continue
		}

		// add a new block with the quoted text
		blocks = append(blocks, "> "+event.Content)

		// increase block count
		b++
	}
	// add remaining text after the last match
	remainingText := input[matches[i][1]:]
	if strings.TrimSpace(remainingText) != "" {
		blocks = append(blocks, remainingText)
	}

	return blocks
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
			`<video controls width="100%%"><source src="%s"></video>`,
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

	var customNodeHook html.RenderNodeFunc = nil
	if usingTelegramInstantView {
		// telegram instant view really doesn't like when there is an image inside a paragraph (like <p><img></p>)
		// so we use this custom thing to stop all paragraphs before the images, print the images then start a new
		// paragraph afterwards.
		customNodeHook = func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
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

func titleize(s string) string {
	s = strings.Replace(s, "\r\n", " ", -1)
	s = strings.Replace(s, "\n", " ", -1)
	if len(s) <= 65 {
		return "\"" + s + "\""
	}
	return "\"" + s[:64] + "…\""
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

func loadNpubsArchive(ctx context.Context) {
	log.Debug().Msg("refreshing the npubs archive")

	contactsArchive := make([]string, 0, 500)

	for _, pubkey := range trustedPubKeys {
		ctx, cancel := context.WithTimeout(ctx, time.Second*4)
		pubkeyContacts := contactsForPubkey(ctx, pubkey)
		contactsArchive = append(contactsArchive, pubkeyContacts...)
		cancel()
	}

	contactsArchive = unique(contactsArchive)
	for _, contact := range contactsArchive {
		log.Debug().Msgf("adding contact %s", contact)
		cache.SetWithTTL("pa:"+contact, nil, time.Hour*24*90)
	}
}

func loadRelaysArchive(ctx context.Context) {
	log.Debug().Msg("refreshing the relays archive")

	relaysArchive := make([]string, 0, 500)

	for _, pubkey := range trustedPubKeys {
		ctx, cancel := context.WithTimeout(ctx, time.Second*4)
		pubkeyContacts := relaysForPubkey(ctx, pubkey)
		relaysArchive = append(relaysArchive, pubkeyContacts...)
		cancel()
	}

	relaysArchive = unique(relaysArchive)
	for _, relay := range relaysArchive {
		for _, excluded := range excludedRelays {
			if strings.Contains(relay, excluded) {
				log.Debug().Msgf("skipping relay %s", relay)
				continue
			}
		}
		if strings.Contains(relay, "/npub1") {
			continue // skip relays with personalyzed query like filter.nostr.wine
		}
		log.Debug().Msgf("adding relay %s", relay)
		cache.SetWithTTL("ra:"+relay, nil, time.Hour*24*7)
	}
}
