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
	"regexp"
	"strings"

	"github.com/a-h/templ"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/pelletier/go-toml"
)

func renderEvent(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Path[1:] // hopefully a nip19 code

	// it's the homepage
	if code == "" {
		renderHomepage(w, r)
		return
	}

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
		if strings.Contains(code, ".") {
			renderProfile(w, r, code)
			return
		}

		// otherwise error
		w.Header().Set("Cache-Control", "max-age=60")
		w.WriteHeader(http.StatusNotFound)
		errorTemplate(ErrorPageParams{Errors: err.Error()}).Render(r.Context(), w)
		return
	}

	// Check if the embed parameter is set to "yes"
	embedParam := r.URL.Query().Get("embed")
	if embedParam == "yes" {
		renderEmbedded(w, r, code)
		return
	}

	// render npub and nprofile using a separate function
	if prefix == "npub" || prefix == "nprofile" {
		// it's a profile
		renderProfile(w, r, code)
		return
	}

	// get data for this event
	data, err := grabData(r.Context(), code, false)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		w.WriteHeader(http.StatusNotFound)
		errorTemplate(ErrorPageParams{Errors: err.Error()}).Render(r.Context(), w)
		return
	}

	// if the result is a kind:0 render this as a profile
	if data.event.Kind == 0 {
		renderProfile(w, r, data.metadata.Npub())
		return
	}

	// if we originally got a note code or an nevent with no hints
	// augment the URL to point to an nevent with hints -- redirect
	if p, ok := decoded.(nostr.EventPointer); (ok && p.Author == "" && len(p.Relays) == 0) || prefix == "note" {
		http.Redirect(w, r, "/"+data.nevent, http.StatusFound)
		return
	}

	// from here onwards we know we're rendering an event
	fmt.Println(r.URL.Path, "#/", r.Header.Get("user-agent"))

	// gather page style from user-agent
	style := getPreviewStyle(r)

	// gather host
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	var subject string
	var summary string
	for _, tag := range data.event.Tags {
		if tag[0] == "subject" || tag[0] == "title" {
			subject = tag[1]
		}
		if tag[0] == "summary" {
			summary = tag[1]
		}
	}

	useTextImage := false

	if data.event.Kind == 1 || data.event.Kind == 30023 {
		if data.image == "" && data.video == "" && len(data.event.Content) > 133 {
			useTextImage = true
		}
		if style == StyleTwitter {
			useTextImage = true
		}
	}

	if tgiv := r.URL.Query().Get("tgiv"); tgiv == "true" || (style == StyleTelegram && tgiv != "false") {
		// do telegram instant preview (only works on telegram mobile apps, not desktop)
		if data.event.Kind == 30023 || // do it for longform articles
			(data.event.Kind == 1 && len(data.event.Content) > 650) || // or very long notes
			(data.parentLink != "") || // or notes that are replies (so we can navigate to them from telegram)
			(strings.Contains(data.content, "nostr:")) || // or notes that quote other stuff (idem)
			// or shorter notes that should be using text-to-image stuff but are not because they have video or images
			(data.event.Kind == 1 && len(data.event.Content)-len(data.image) > 133 && !useTextImage) {
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
	if subject != "" {
		subscript += " (" + subject + ")"
	}
	subscript += " by " + data.metadata.ShortName()
	if data.event.isReply() {
		subscript += " (reply)"
	}

	seenOnRelays := ""
	if len(data.event.relays) > 0 {
		seenOnRelays = fmt.Sprintf("seen on %s", strings.Join(data.event.relays, ", "))
	}

	textImageURL := ""
	description := ""
	if useTextImage {
		textImageURL = fmt.Sprintf("https://%s/njump/image/%s?%s", host, code, r.URL.RawQuery)
		if subject != "" {
			if seenOnRelays != "" {
				description = fmt.Sprintf("%s -- %s", subject, seenOnRelays)
			} else {
				description = subject
			}
		} else {
			description = seenOnRelays
		}
	} else if summary != "" {
		description = summary
	} else {
		// if content is valid JSON, parse that and print as TOML for easier readability
		var parsedJson any
		if err := json.Unmarshal([]byte(data.event.Content), &parsedJson); err == nil {
			if t, err := toml.Marshal(parsedJson); err == nil && len(t) > 0 {
				description = string(t)
			}
		} else {
			// otherwise replace npub/nprofiles with names and trim length
			description = replaceUserReferencesWithNames(r.Context(), []string{data.event.Content}, "", "")[0]
			if len(description) > 240 {
				description = description[:240]
			}
		}
	}

	// titleizedContent
	titleizedContent := strings.TrimSpace(
		strings.Replace(
			strings.Replace(
				replaceUserReferencesWithNames(r.Context(), []string{data.event.Content}, "", "")[0],
				"\r\n", " ", -1),
			"\n", " ", -1,
		),
	)

	// Remove image/video urls
	urlRegex := regexp.MustCompile(`(https?)://[^\s/$.?#]+\.(?i:jpg|jpeg|png|gif|bmp|mp4|mov|avi|mkv|webm|ogg)`)
	titleizedContent = urlRegex.ReplaceAllString(titleizedContent, "")

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
	for index, value := range data.event.Tags {
		placeholderTag := "#[" + fmt.Sprintf("%d", index) + "]"
		nreplace := ""
		if value[0] == "p" {
			nreplace, _ = nip19.EncodePublicKey(value[1])
		} else if value[0] == "e" {
			nreplace, _ = nip19.EncodeEvent(value[1], []string{}, "")
		} else {
			continue
		}
		data.content = strings.ReplaceAll(data.content, placeholderTag, "nostr:"+nreplace)
	}
	if data.event.Kind == 30023 || data.event.Kind == 30024 {
		data.content = mdToHTML(data.content, data.templateId == TelegramInstantView, false)
	} else {
		// first we run basicFormatting, which turns URLs into their appropriate HTML tags
		data.content = basicFormatting(html.EscapeString(data.content), true, false, false)
		// then we render quotes as HTML, which will also apply basicFormatting to all the internal quotes
		data.content = renderQuotesAsHTML(r.Context(), data.content, data.templateId == TelegramInstantView)
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
		EventJSON:       data.event.ToJSONHTML(),
		Kind:            data.event.Kind,
		SeenOn:          data.event.relays,
		Metadata:        data.metadata,
	}

	opengraph := OpenGraphParams{
		BigImage:     textImageURL,
		Image:        data.image,
		Video:        data.video,
		VideoType:    data.videoType,
		ProxiedImage: "https://" + host + "/njump/proxy?src=" + data.image,

		Superscript: data.authorLong,
		Subscript:   subscript,
		Text:        strings.TrimSpace(description),
	}

	var component templ.Component
	baseEventPageParams := BaseEventPageParams{
		Event:    data.event,
		Metadata: data.metadata,
		Style:    style,
		Alt:      data.alt,
	}

	switch data.templateId {
	case TelegramInstantView:
		component = telegramInstantViewTemplate(TelegramInstantViewParams{
			Video:        data.video,
			VideoType:    data.videoType,
			Image:        data.image,
			Summary:      template.HTML(summary),
			Content:      template.HTML(data.content),
			Description:  description,
			Subject:      subject,
			Metadata:     data.metadata,
			AuthorLong:   data.authorLong,
			CreatedAt:    data.createdAt,
			ParentNevent: data.event.getParentNevent(),
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

		enhancedCode := data.nevent
		if data.naddr != "" {
			enhancedCode = data.naddr
		}

		content := data.content
		for _, tag := range data.event.Tags.GetAll([]string{"emoji"}) {
			if len(tag) >= 3 {
				content = strings.ReplaceAll(content, ":"+tag[1]+":", `<img class="emoji" src="`+tag[2]+`"/>`)
			}
		}
		component = noteTemplate(NotePageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				Oembed:      oembed,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
			},

			Clients:          generateClientList(data.event.Kind, enhancedCode),
			Details:          detailsData,
			Content:          template.HTML(content),
			Subject:          subject,
			TitleizedContent: titleizedContent,
		})
	case FileMetadata:
		opengraph.Image = data.kind1063Metadata.DisplayImage()
		params := FileMetadataPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
			},

			Details: detailsData,
			Clients: generateClientList(data.event.Kind, data.nevent),

			FileMetadata: *data.kind1063Metadata,
			IsImage:      data.kind1063Metadata.IsImage(),
			IsVideo:      data.kind1063Metadata.IsVideo(),
		}
		params.Details.Extra = fileMetadataDetails(params)

		component = fileMetadataTemplate(params)
	case LiveEvent:
		opengraph.Image = data.kind30311Metadata.Image
		component = liveEventTemplate(LiveEventPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
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
		})
	case LiveEventMessage:
		component = liveEventMessageTemplate(LiveEventMessagePageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
			},

			Details:          detailsData,
			Content:          template.HTML(data.content),
			TitleizedContent: titleizedContent,
			Clients:          generateClientList(data.event.Kind, data.naddr),
		})
	case CalendarEvent:
		if data.kind31922Or31923Metadata.Image != "" {
			opengraph.Image = data.kind31922Or31923Metadata.Image
		}
		component = calendarEventTemplate(CalendarPageParams{
			BaseEventPageParams: baseEventPageParams,
			OpenGraphParams:     opengraph,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
			},

			Details: detailsData,
			Content: template.HTML(data.content),
			Clients: generateClientList(data.event.Kind, data.naddr),
		})
	case Other:
		detailsData.HideDetails = false // always open this since we know nothing else about the event

		component = otherTemplate(OtherPageParams{
			BaseEventPageParams: baseEventPageParams,
			HeadParams: HeadParams{
				IsProfile:   false,
				NaddrNaked:  data.naddrNaked,
				NeventNaked: data.neventNaked,
			},

			Details:         detailsData,
			Kind:            data.event.Kind,
			KindDescription: data.kindDescription,
		})
	default:
		log.Error().Int("templateId", int(data.templateId)).Msg("no way to render")
		http.Error(w, "tried to render an unsupported template at render_event.go", 500)
		return
	}

	if err := component.Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
