package main

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/fiatjaf/njump/i18n"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip05"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/pelletier/go-toml"
)

func renderEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PathValue("code")

	isEmbed := r.URL.Query().Get("embed") != ""

	if strings.HasPrefix(code, "nostr:") {
		// remove the "nostr:" prefix
		http.Redirect(w, r, "/"+code[6:], http.StatusFound)
		return
	}

	// decode the nip19 code we've received
	prefix, decoded, err := nip19.Decode(code)
	if err != nil {
		// if it's a 32-byte hex assume it's an event id
		if _, err := hex.DecodeString(code); err == nil && len(code) == 64 {
			redirectNevent, _ := nip19.EncodeEvent(code, []string{}, "")
			http.Redirect(w, r, "/"+redirectNevent, http.StatusFound)
			return
		}

		// it may be a NIP-05
		if nip05.IsValidIdentifier(code) {
			renderProfile(ctx, r, w, code)
			return
		}

		// otherwise error
		w.Header().Set("Cache-Control", "max-age=60")
		log.Warn().Err(err).Str("code", code).Msg("invalid code")
		w.WriteHeader(http.StatusNotFound)
		errorTemplate(ErrorPageParams{Errors: err.Error()}).Render(ctx, w)
		return
	}

	// render npub and nprofile using a separate function
	if prefix == "npub" || prefix == "nprofile" {
		// it's a profile
		renderProfile(ctx, r, w, code)
		return
	}

	// get data for this event
	data, err := grabData(ctx, code, true)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		log.Warn().Err(err).Str("code", code).Msg("event not found on render_event")
		w.WriteHeader(http.StatusNotFound)
		errorTemplate(ErrorPageParams{Errors: err.Error(), Clients: generateClientList(999999, code)}).Render(ctx, w)
		return
	}

	// if we originally got a note code or an nevent with no hints
	// augment the URL to point to an nevent with hints -- redirect
	if p, ok := decoded.(nostr.EventPointer); (ok && p.Author == "" && len(p.Relays) == 0) || prefix == "note" {
		url := "/" + data.nevent
		if r.URL.RawQuery != "" {
			url += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	}

	// from here onwards we know we're rendering an event
	//

	// banned or unallowed conditions
	if banned, reason := internal.isBannedEvent(data.event.ID); banned {
		w.Header().Set("Cache-Control", "max-age=60")
		log.Warn().Err(err).Str("code", code).Str("reason", reason).Msg("event banned")
		http.Error(w, "event banned", http.StatusNotFound)
		return
	}
	if banned, reason := internal.isBannedPubkey(data.event.PubKey); banned {
		w.Header().Set("Cache-Control", "max-age=60")
		log.Warn().Err(err).Str("code", code).Str("reason", reason).Msg("pubkey banned")
		http.Error(w, "pubkey banned", http.StatusNotFound)
		return
	}
	hasURL := urlRegex.MatchString(data.event.Content)
	if isMaliciousBridged(data.event.author) ||
		(hasURL && hasProhibitedWordOrTag(data.event.Event)) ||
		(hasURL && hasExplicitMedia(ctx, data.event.Event)) {
		log.Warn().Str("event", data.nevent).Msg("detect prohibited content")
		http.Error(w, "event is not allowed", http.StatusNotFound)
		return
	}

	// gather page style from user-agent
	style := getPreviewStyle(r)

	// gather host
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	useTextImage := false

	if data.event.Kind == 1 || data.event.Kind == 11 || data.event.Kind == 1111 || data.event.Kind == 30023 {
		if data.image == "" && data.video == "" && len(data.event.Content) > 133 {
			useTextImage = true
		}
		if style == StyleTwitter {
			useTextImage = true
		}
	} else if data.event.Kind == 20 {
		useTextImage = false
	}

	if tgiv := r.URL.Query().Get("tgiv"); tgiv == "true" || (style == StyleTelegram && tgiv != "false") {
		// do telegram instant preview (only works on telegram mobile apps, not desktop)
		if data.event.Kind == 30023 || // do it for longform articles
			((data.event.Kind == 1 || data.event.Kind == 11 || data.event.Kind == 1111) && len(data.event.Content) > 650) || // or very long notes/group messages
			(data.parentLink != "") || // or notes that are replies (so we can navigate to them from telegram)
			(strings.Contains(data.content, "nostr:")) || // or notes that quote other stuff (idem)
			// or shorter notes that should be using text-to-image stuff but are not because they have video or images
			((data.event.Kind == 1 || data.event.Kind == 11 || data.event.Kind == 1111) && len(data.event.Content)-len(data.image) > 133 && !useTextImage) ||
			(data.event.Kind == 20) {

			data.templateId = TelegramInstantView
			useTextImage = false
		}
	} else if style == StyleSlack || style == StyleDiscord {
		useTextImage = false
	}

	subscript := ""
	if data.event.Kind >= 30000 && data.event.Kind < 40000 {
		tValue := "~"
		for _, tag := range data.event.Tags {
			if tag[0] == "t" {
				tValue = tag[1]
				break
			}
		}
		subscript = fmt.Sprintf("%s: %s", kindNames[data.event.Kind], tValue)
	} else if kindName, ok := kindNames[data.event.Kind]; ok {
		subscript = kindName
	} else {
		subscript = fmt.Sprintf("kind:%d event", data.event.Kind)
	}
	if data.event.subject != "" {
		subscript += " (" + data.event.subject + ")"
	}

	subscript += " by " + data.event.author.ShortName()
	if data.event.isReply() {
		subscript += " (reply)"
	}

	seenOnRelays := ""
	if len(data.event.relays) > 0 {
		relays := make([]string, len(data.event.relays))
		for i, r := range data.event.relays {
			relays[i] = trimProtocolAndEndingSlash(r)
		}
		seenOnRelays = fmt.Sprintf("seen on %s", strings.Join(relays, ", "))
	}

	textImageURL := ""
	description := ""
	if useTextImage {
		textImageURL = fmt.Sprintf("https://%s/njump/image/%s?%s", host, code, r.URL.RawQuery)
		if data.event.subject != "" {
			if seenOnRelays != "" {
				description = fmt.Sprintf("%s -- %s", data.event.subject, seenOnRelays)
			} else {
				description = data.event.subject
			}
		} else {
			description = seenOnRelays
		}
	} else if data.event.summary != "" {
		description = data.event.summary
	} else {
		// if content is valid JSON, parse that and print as TOML for easier readability
		var parsedJson any
		if err := json.Unmarshal([]byte(data.event.Content), &parsedJson); err == nil {
			if t, err := toml.Marshal(parsedJson); err == nil && len(t) > 0 {
				description = string(t)
			}
		} else {
			// otherwise replace npub/nprofiles with names and trim length
			description = replaceUserReferencesWithNames(ctx, []string{data.event.Content}, "")[0]
			if len(description) > 240 {
				description = description[:240]
			}
		}
	}

	// titleizedContent
	titleizedContent := urlRegex.ReplaceAllString(
		strings.TrimSpace(
			strings.Replace(
				strings.Replace(
					replaceUserReferencesWithNames(ctx, []string{data.event.Content}, "")[0],
					"\r\n", " ", -1),
				"\n", " ", -1,
			),
		),
		"",
	)

	if titleizedContent == "" {
		titleizedContent = subscript
	}

	if len(titleizedContent) > 85 {
		words := strings.Fields(titleizedContent)
		titleizedContent = ""
		for _, word := range words {
			if len(titleizedContent)+len(word)+1 <= 85 { // +1 for space
				if titleizedContent != "" {
					titleizedContent += " "
				}
				titleizedContent += word
			} else {
				break
			}
		}
		titleizedContent = titleizedContent + " ..."
	}

	// content massaging
	for i, tag := range data.event.Tags {
		if len(tag) < 2 {
			continue
		}

		placeholderTag := "#[" + fmt.Sprintf("%d", i) + "]"
		nreplace := ""
		if tag[0] == "p" {
			nreplace, _ = nip19.EncodePublicKey(tag[1])
		} else if tag[0] == "e" {
			nreplace, _ = nip19.EncodeEvent(tag[1], []string{}, "")
		} else {
			continue
		}
		data.content = strings.ReplaceAll(data.content, placeholderTag, "nostr:"+nreplace)
	}
	if data.event.Kind == 30023 || data.event.Kind == 30024 {
		// Remove duplicate title inside the body
		data.content = strings.ReplaceAll(data.content, "# "+data.event.subject, "")
		data.content = mdToHTML(data.content, data.templateId == TelegramInstantView)
	} else if data.event.Kind == 30818 {
		data.content = asciidocToHTML(data.content)
	} else {
		// first we run basicFormatting, which turns URLs into their appropriate HTML tags
		data.content = basicFormatting(html.EscapeString(data.content), true, false, false)
		// then we render quotes as HTML, which will also apply basicFormatting to all the internal quotes
		data.content = renderQuotesAsHTML(ctx, data.content, data.templateId == TelegramInstantView)
		// we must do this because inside <blockquotes> we must treat <img>s differently when telegram_instant_view
	}

	w.Header().Set("Content-Type", "text/html")
	if data.templateId == TelegramInstantView {
		w.Header().Set("Cache-Control", "no-cache")
	} else if len(data.content) != 0 {
		w.Header().Set("Cache-Control", "max-age=604800")
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	// oembed discovery
	oembed := ""
	if data.templateId == Note {
		oembed = (&url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/services/oembed",
			RawQuery: (url.Values{
				"url": {fmt.Sprintf("https://%s/%s", host, code)},
			}).Encode(),
		}).String()
		w.Header().Add("Link", "<"+oembed+"&format=json>; rel=\"alternate\"; type=\"application/json+oembed\"")
		w.Header().Add("Link", "<"+oembed+"&format=xml>; rel=\"alternate\"; type=\"text/xml+oembed\"")
	}

	detailsData := DetailsParams{
		HideDetails:     true,
		CreatedAt:       data.createdAt,
		KindDescription: data.kindDescription,
		KindNIP:         data.kindNIP,
		EventJSON:       toJSONHTML(data.event.Event),
		Kind:            data.event.Kind,
		SeenOn:          data.event.relays,
		Metadata:        data.event.author,
	}

	opengraph := OpenGraphParams{
		BigImage:     textImageURL,
		Image:        data.image,
		Video:        data.video,
		VideoType:    data.videoType,
		ProxiedImage: "https://" + host + "/njump/proxy?src=" + data.image,

		Superscript: data.event.authorLong() + " on Nostr",
		Subscript:   subscript,
		Text:        strings.TrimSpace(description),
	}

	var component templ.Component
	baseEventPageParams := BaseEventPageParams{
		Event: data.event,
		Style: style,
		Alt:   data.alt,
	}

	switch data.templateId {
	case TelegramInstantView:
		component = telegramInstantViewTemplate(TelegramInstantViewParams{
			Video:       data.video,
			VideoType:   data.videoType,
			Image:       data.image,
			Summary:     template.HTML(data.event.summary),
			Content:     template.HTML(data.content),
			Description: description,
			Subject:     data.event.subject,
			Metadata:    data.event.author,
			AuthorLong:  data.event.authorLong(),
			CreatedAt:   data.createdAt,
			Parent:      data.event.getParent(),
		})

	case Note:
		if style == StyleTwitter {
			// twitter has started sprinkling this over our image, so let's make it invisible
			opengraph.SingleTitle = string(INVISIBLE_SPACE)
		}

		if opengraph.BigImage == "" && style != StyleTwitter && strings.HasSuffix(opengraph.Text, opengraph.Image) {
			// if a note is mostly about an image, we should prefer to display the image in a big card
			// this works, for example, in telegram, and it may work in other places --
			// but we can't do this on twitter because when twitter sees a big image it hides all the text and title
			// also twitter images only work if they're proxied and for now we're not proxying this
			opengraph.Text = opengraph.Text[0 : len(opengraph.Text)-len(opengraph.Image)]
			opengraph.BigImage = opengraph.Image
		}

		content := data.content
		for tag := range data.event.Tags.FindAll("emoji") {
			// custom emojis
			if len(tag) >= 3 && isValidShortcode(tag[1]) {
				u, err := url.Parse(tag[2])
				if err == nil {
					content = strings.ReplaceAll(content, ":"+tag[1]+":", `<img class="h-[29px] inline m-0" src="`+u.String()+`" alt=":`+tag[1]+`:"/>`)
				}
			}
		}

		params := NotePageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				Oembed:      oembed,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},
			Clients:          generateClientList(data.event.Kind, data.nevent),
			Details:          detailsData,
			Content:          template.HTML(content),
			TitleizedContent: titleizedContent,
		}

		component = noteTemplate(params, isEmbed)

	case LongForm:
		if data.cover != "" {
			opengraph.Image = data.cover
			opengraph.BigImage = data.cover
		} else if style == StyleTwitter {
			// twitter has started sprinkling this over our image, so let's make it invisible
			opengraph.SingleTitle = string(INVISIBLE_SPACE)
		}

		if text, err := markdownExtractor.PlainText(opengraph.Text); err == nil {
			opengraph.Text = *text
		}

		params := NotePageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				Oembed:      oembed,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},
			Clients:          generateClientList(data.event.Kind, data.naddr),
			Details:          detailsData,
			Content:          template.HTML(data.content),
			Cover:            data.cover,
			TitleizedContent: data.event.subject, // we store the "title" tag here too
		}

		component = noteTemplate(params, isEmbed)

	case FileMetadata:
		opengraph.Image = data.kind1063Metadata.DisplayImage()
		params := FileMetadataPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},

			Details: detailsData,
			Clients: generateClientList(data.event.Kind, data.nevent),

			FileMetadata: *data.kind1063Metadata,
			IsImage:      data.kind1063Metadata.IsImage(),
			IsVideo:      data.kind1063Metadata.IsVideo(),
		}
		params.Details.Extra = fileMetadataDetails(params)

		component = fileMetadataTemplate(params, isEmbed)

	case LiveEvent:
		opengraph.Image = data.kind30311Metadata.Image
		params := LiveEventPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},

			Details:   detailsData,
			LiveEvent: *data.kind30311Metadata,
			Clients: generateClientList(data.event.Kind, data.naddr,
				func(c ClientReference, s string) string {
					if c == nostrudel {
						s = strings.Replace(s, "/u/", "/streams/", 1)
					}
					return s
				},
			),
		}

		component = liveEventTemplate(params, isEmbed)

	case LiveEventMessage:
		params := LiveEventMessagePageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},

			Details:          detailsData,
			Content:          template.HTML(data.content),
			TitleizedContent: titleizedContent,
			Clients:          generateClientList(data.event.Kind, data.naddr),
		}

		component = liveEventMessageTemplate(params, isEmbed)

	case CalendarEvent:
		if data.kind31922Or31923Metadata.Image != "" {
			opengraph.Image = data.kind31922Or31923Metadata.Image
		}

		// Fallback for deprecated 'name' field
		if data.kind31922Or31923Metadata.Title == "" {
			for _, tag := range data.event.Tags {
				if tag[0] == "name" {
					data.kind31922Or31923Metadata.Title = tag[1]
					break
				}
			}
		}

		var startAtDate, startAtTime string
		var endAtDate, endAtTime string

		location, err := time.LoadLocation(data.kind31922Or31923Metadata.StartTzid)
		if err != nil {
			// Set default TimeZone to UTC
			location = time.UTC
		}

		startAtDate = data.kind31922Or31923Metadata.Start.In(location).Format("02 Jan 2006")
		endAtDate = data.kind31922Or31923Metadata.End.In(location).Format("02 Jan 2006")
		if data.kind31922Or31923Metadata.CalendarEventKind == 31923 {
			startAtTime = data.kind31922Or31923Metadata.Start.In(location).Format("15:04")
			endAtTime = data.kind31922Or31923Metadata.End.In(location).Format("15:04")
		}

		// Reset EndDate/Time if it is non initialized (beginning of the Unix epoch)
		if data.kind31922Or31923Metadata.End == (time.Time{}) {
			endAtDate = ""
			endAtTime = ""
		}

		params := CalendarPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},
			TimeZone:      getUTCOffset(location),
			StartAtDate:   startAtDate,
			StartAtTime:   startAtTime,
			EndAtDate:     endAtDate,
			EndAtTime:     endAtTime,
			CalendarEvent: *data.kind31922Or31923Metadata,
			Details:       detailsData,
			Content:       template.HTML(data.content),
			Clients:       generateClientList(data.event.Kind, data.naddr),
		}

		component = calendarEventTemplate(params, isEmbed)

	case WikiEvent:
		opengraph.Superscript = "wiki entry: " + data.Kind30818Metadata.Title
		if strings.ToLower(data.Kind30818Metadata.Title) == data.Kind30818Metadata.Handle {
			opengraph.Subscript = "by " + data.event.author.ShortName()
		} else {
			opengraph.Subscript = fmt.Sprintf("\"%s\" by %s", data.Kind30818Metadata.Handle, data.event.author.ShortName())
		}

		params := WikiPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},
			PublishedAt: data.Kind30818Metadata.PublishedAt.Format("02 Jan 2006"),
			WikiEvent:   data.Kind30818Metadata,
			Details:     detailsData,
			Content:     data.content,
			Clients: generateClientList(
				data.event.Kind,
				data.naddr,
				func(client ClientReference, url string) string {
					return strings.Replace(url, "{handle}", data.Kind30818Metadata.Handle, -1)
				},
				func(client ClientReference, url string) string {
					return strings.Replace(url, "{authorPubkey}", data.event.PubKey, -1)
				},
				func(client ClientReference, url string) string {
					return strings.Replace(url, "{npub}", data.event.author.Npub(), -1)
				},
			),
		}

		component = wikiEventTemplate(params, isEmbed)

	case Highlight:
		if data.Kind9802Metadata.Comment == "" {
			opengraph.Superscript = data.Kind9802Metadata.SourceURL
			opengraph.Subscript = "Highlight by " + data.event.author.ShortName()
			opengraph.Text = "> " + opengraph.Text
		} else {
			opengraph.Superscript = data.Kind9802Metadata.SourceURL
			opengraph.Subscript = "Annotation by " + data.event.author.ShortName()
			opengraph.Text = data.Kind9802Metadata.Comment + "\n> " + opengraph.Text
		}

		params := HighlightPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},
			Content:        template.HTML(data.content),
			HighlightEvent: data.Kind9802Metadata,
			Details:        detailsData,
			Clients:        generateClientList(data.event.Kind, data.nevent),
		}

		component = highlightTemplate(params, isEmbed)

	case Other:
		detailsData.HideDetails = false // always open this since we know nothing else about the event

		params := OtherPageParams{
			BaseEventPageParams: baseEventPageParams,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
				Lang:        i18n.LanguageFromContext(ctx),
			},

			Details:         detailsData,
			Kind:            data.event.Kind,
			KindDescription: data.kindDescription,
		}

		component = otherTemplate(params)

	default:
		log.Error().Int("templateId", int(data.templateId)).Msg("no way to render")
		http.Error(w, "tried to render an unsupported template at render_event.go", 500)
		return
	}

	if err := component.Render(ctx, w); err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
	}
	return
}
