package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

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
	} else if strings.HasPrefix(code, "npub") || strings.HasPrefix(code, "nprofile") {
		// it's a profile
		renderProfile(w, r, code)
		return
	}

	fmt.Println(r.URL.Path, "#/", r.Header.Get("user-agent"))

	// force note1 to become nevent1
	if strings.HasPrefix(code, "note1") {
		_, redirectHex, err := nip19.Decode(code)
		if err != nil {
			w.Header().Set("Cache-Control", "max-age=60")
			http.Error(w, "error decoding note1 code: "+err.Error(), 404)
			return
		}
		redirectNevent, _ := nip19.EncodeEvent(redirectHex.(string), []string{}, "")
		http.Redirect(w, r, "/"+redirectNevent, http.StatusFound)
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	style := getPreviewStyle(r)

	data, err := grabData(r.Context(), code, false)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		http.Error(w, "failed to fetch event related data: "+err.Error(), 404)
		return
	}

	if data.event.Kind == 0 {
		// it's a NIP-05 profile
		renderProfile(w, r, data.npub)
		return
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

	useTextImage := (data.event.Kind == 1 || data.event.Kind == 30023) &&
		data.image == "" && data.video == "" && len(data.event.Content) > 133

	if tgiv := r.URL.Query().Get("tgiv"); tgiv == "true" || (style == StyleTelegram && tgiv != "false") {
		// do telegram instant preview (only works on telegram mobile apps, not desktop)
		if data.event.Kind == 30023 || // do it for longform articles
			(data.event.Kind == 1 && len(data.event.Content) > 650) || // or very long notes
			// or shorter notes that should be using text-to-image stuff but are not because they have video or images
			(data.event.Kind == 1 && len(data.event.Content)-len(data.image) > 133 && !useTextImage) {
			data.templateId = TelegramInstantView
			useTextImage = false
		}
	} else if style == StyleSlack || style == StyleDiscord {
		useTextImage = false
	}

	title := ""
	if data.event.Kind >= 30000 && data.event.Kind < 40000 {
		tValue := "~"
		for _, tag := range data.event.Tags {
			if tag[0] == "t" {
				tValue = tag[1]
				break
			}
		}
		title = fmt.Sprintf("%s: %s", kindNames[data.event.Kind], tValue)
	} else if kindName, ok := kindNames[data.event.Kind]; ok {
		title = kindName
	} else {
		title = fmt.Sprintf("kind:%d event", data.event.Kind)
	}
	if subject != "" {
		title += " (" + subject + ")"
	}
	title += " by " + data.authorShort

	seenOnRelays := ""
	if len(data.relays) > 0 {
		seenOnRelays = fmt.Sprintf("seen on %s", strings.Join(data.relays, ", "))
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
			description = replaceUserReferencesWithNames(r.Context(), []string{data.event.Content})[0]
			if len(description) > 240 {
				description = description[:240]
			}
		}
	}

	// titleizedContent
	titleizedContent := strings.TrimSpace(
		strings.Replace(
			strings.Replace(
				replaceUserReferencesWithNames(r.Context(), []string{data.event.Content})[0],
				"\r\n", " ", -1),
			"\n", " ", -1,
		),
	)

	// Remove image/video urls
	urlRegex := regexp.MustCompile(`(https?)://[^\s/$.?#]+\.(?i:jpg|jpeg|png|gif|bmp|mp4|mov|avi|mkv|webm|ogg)`)
	titleizedContent = urlRegex.ReplaceAllString(titleizedContent, "")

	if titleizedContent == "" {
		titleizedContent = title
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
		data.content = mdToHTML(data.content, data.templateId == TelegramInstantView)
	} else {
		// first we run basicFormatting, which turns URLs into their appropriate HTML tags
		data.content = basicFormatting(html.EscapeString(data.content), true, false)
		// then we render quotes as HTML, which will also apply basicFormatting to all the internal quotes
		data.content = renderQuotesAsHTML(r.Context(), data.content, data.templateId == TelegramInstantView)
		// we must do this because inside <blockquotes> we must treat <img>s different when telegram_instant_view
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

	detailsData := DetailsPartial{
		HideDetails:     true,
		CreatedAt:       data.createdAt,
		KindDescription: data.kindDescription,
		KindNIP:         data.kindNIP,
		EventJSON:       eventToHTML(data.event),
		Kind:            data.event.Kind,
		SeenOn:          data.relays,
		Npub:            data.npub,
		Nprofile:        data.nprofile,

		// kind-specific stuff
		FileMetadata: data.kind1063Metadata,
		LiveEvent:    data.kind30311Metadata,
	}

	opengraph := OpenGraphPartial{
		BigImage:     textImageURL,
		Image:        data.image,
		Video:        data.video,
		VideoType:    data.videoType,
		ProxiedImage: "https://" + host + "/njump/proxy?src=" + data.image,

		Superscript: data.authorLong,
		Subscript:   title,
		Text:        strings.TrimSpace(description),
	}

	switch data.templateId {
	case TelegramInstantView:
		err = TelegramInstantViewTemplate.Render(w, &TelegramInstantViewPage{
			Video:       data.video,
			VideoType:   data.videoType,
			Image:       data.image,
			Summary:     template.HTML(summary),
			Content:     template.HTML(data.content),
			Description: description,
			Subject:     subject,
			Metadata:    data.metadata,
			AuthorLong:  data.authorLong,
			CreatedAt:   data.createdAt,
		})
	case Note:
		if style == StyleTwitter {
			// twitter only uses one title, so we ensure it is this
			// we can't set this for other platforms as some will reuse stuff from twitter-specific tags
			opengraph.SingleTitle = "by " + data.authorShort + " at " + humanDate(data.event.CreatedAt)
		}

		if opengraph.BigImage == "" && style != StyleTwitter && strings.HasSuffix(opengraph.Text, opengraph.Image) {
			// if a note is mostly about an image, we should prefer to display the image in a big card
			// this works, for example, in telegram, and it may work in other places --
			// but we can't do this on twitter because when twitter sees a big image it hides all the text and title
			// also twitter images only work if they're proxied and for now we're not proxying this
			opengraph.Text = opengraph.Text[0 : len(opengraph.Text)-len(opengraph.Image)]
			opengraph.BigImage = opengraph.Image
		}

		if data.naddr != "" {
			code = data.naddr
		}

		err = NoteTemplate.Render(w, &NotePage{
			OpenGraphPartial: opengraph,
			HeadCommonPartial: HeadCommonPartial{
				IsProfile:          false,
				Oembed:             oembed,
				TailwindDebugStuff: tailwindDebugStuff,
				NaddrNaked:         data.naddrNaked,
				NeventNaked:        data.neventNaked,
			},
			DetailsPartial: detailsData,
			ClientsPartial: ClientsPartial{
				Clients: generateClientList(style, code, data.event),
			},

			Content:          template.HTML(data.content),
			CreatedAt:        data.createdAt,
			Metadata:         data.metadata,
			Npub:             data.npub,
			NpubShort:        data.npubShort,
			ParentLink:       data.parentLink,
			Subject:          subject,
			TitleizedContent: titleizedContent,
		})
	case FileMetadata:
		opengraph.Image = data.kind1063Metadata.DisplayImage()

		err = FileMetadataTemplate.Render(w, &FileMetadataPage{
			OpenGraphPartial: opengraph,
			HeadCommonPartial: HeadCommonPartial{
				IsProfile:          false,
				TailwindDebugStuff: tailwindDebugStuff,
				NaddrNaked:         data.naddrNaked,
				NeventNaked:        data.neventNaked,
			},
			DetailsPartial: detailsData,
			ClientsPartial: ClientsPartial{
				Clients: generateClientList(style, code, data.event),
			},

			CreatedAt:        data.createdAt,
			Metadata:         data.metadata,
			Npub:             data.npub,
			NpubShort:        data.npubShort,
			Style:            style,
			Subject:          subject,
			TitleizedContent: titleizedContent,
			Alt:              data.alt,

			FileMetadata: *data.kind1063Metadata,
			IsImage:      data.kind1063Metadata.IsImage(),
			IsVideo:      data.kind1063Metadata.IsVideo(),
		})
	case LiveEvent:
		opengraph.Image = data.kind30311Metadata.Image

		err = LiveEventTemplate.Render(w, &LiveEventPage{
			OpenGraphPartial: opengraph,
			HeadCommonPartial: HeadCommonPartial{
				IsProfile:          false,
				TailwindDebugStuff: tailwindDebugStuff,
				NaddrNaked:         data.naddrNaked,
				NeventNaked:        data.neventNaked,
			},
			DetailsPartial: detailsData,
			ClientsPartial: ClientsPartial{
				Clients: generateClientList(style, data.naddr, data.event),
			},

			CreatedAt:        data.createdAt,
			Metadata:         data.metadata,
			Npub:             data.npub,
			NpubShort:        data.npubShort,
			Style:            style,
			Subject:          subject,
			TitleizedContent: titleizedContent,
			Alt:              data.alt,

			LiveEvent: *data.kind30311Metadata,
		})
	case LiveEventMessage:
		// opengraph.Image = data.kind1311Metadata.Image

		err = LiveEventMessageTemplate.Render(w, &LiveEventMessagePage{
			OpenGraphPartial: opengraph,
			HeadCommonPartial: HeadCommonPartial{
				IsProfile:          false,
				TailwindDebugStuff: tailwindDebugStuff,
				NaddrNaked:         data.naddrNaked,
				NeventNaked:        data.neventNaked,
			},
			DetailsPartial: detailsData,
			ClientsPartial: ClientsPartial{
				Clients: generateClientList(style, data.naddr, data.event),
			},

			Content:          template.HTML(data.content),
			CreatedAt:        data.createdAt,
			Metadata:         data.metadata,
			Npub:             data.npub,
			NpubShort:        data.npubShort,
			ParentLink:       data.parentLink,
			Style:            style,
			Subject:          subject,
			TitleizedContent: titleizedContent,
			Alt:              data.alt,

			LiveEventMessage: *data.kind1311Metadata,
		})
	case Other:
		detailsData.HideDetails = false // always open this since we know nothing else about the event

		err = OtherTemplate.Render(w, &OtherPage{
			HeadCommonPartial: HeadCommonPartial{
				IsProfile:          false,
				TailwindDebugStuff: tailwindDebugStuff,
				NaddrNaked:         data.naddrNaked,
				NeventNaked:        data.neventNaked,
			},
			DetailsPartial:  detailsData,
			Alt:             data.alt,
			Kind:            data.event.Kind,
			KindDescription: data.kindDescription,
		})
	default:
		log.Error().Int("templateId", int(data.templateId)).Msg("no way to render")
		http.Error(w, "tried to render an unsupported template at render_event.go", 500)
	}

	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
