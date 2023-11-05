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
	userAgent := r.Header.Get("User-Agent")
	searchEngineRegex := regexp.MustCompile(`Googlebot|Bingbot|Yahoo|Baidu|Yandex|DuckDuckGo|Sogou|Exabot`)
	fmt.Println(r.URL.Path, "#/", userAgent)

	// force note1 to become nevent1
	if strings.HasPrefix(code, "note1") {
		_, redirectHex, err := nip19.Decode(code)
		if err != nil {
			w.Header().Set("Cache-Control", "max-age=60")
			http.Error(w, "error fetching event: "+err.Error(), 404)
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
		http.Error(w, "error fetching event: "+err.Error(), 404)
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

	if style == StyleTelegram || r.URL.Query().Get("tgiv") == "true" {
		// do telegram instant preview (only works on telegram mobile apps, not desktop)
		if data.event.Kind == 30023 || // do it for longform articles
			(data.event.Kind == 1 && len(data.event.Content) > 650) || // or very long notes
			// or shorter notes that should be using text-to-image stuff but are not because they have video or images
			(data.event.Kind == 1 && len(data.event.Content) > 133 && !useTextImage) {
			data.templateId = TelegramInstantView
			useTextImage = false
		}
	} else if style == StyleSlack || style == StyleDiscord {
		useTextImage = false
	}

	title := ""
	twitterTitle := title
	if data.event.Kind == 0 && data.metadata.Name != "" {
		title = data.metadata.Name
	} else {
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
			if searchEngineRegex.MatchString(userAgent) {
				urlRegex := regexp.MustCompile(`(https?)://[^\s/$.?#].[^\s]*`)
				title = urlRegex.ReplaceAllString(data.event.Content, "")
			} else {
				title = kindName
			}
		} else {
			title = fmt.Sprintf("kind:%d event", data.event.Kind)
		}
		if subject != "" {
			if searchEngineRegex.MatchString(userAgent) {
				title = subject
			} else {
				title += " (" + subject + ")"
			}
		}
		date := data.event.CreatedAt.Time().UTC().Format("2006-01-02 15:04")
		if len(title) > 65 {
			words := strings.Fields(title)
			title = ""
			for _, word := range words {
				if len(title)+len(word)+1 <= 65 { // +1 for space
					if title != "" {
						title += " "
					}
					title += word
				} else {
					break
				}
			}
			title = title + " ..."
			title += " at " + date
		}
		twitterTitle += " by " + data.authorShort + " at " + date
	}

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
			description := replaceUserReferencesWithNames(r.Context(), []string{data.event.Content})[0]
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
	if titleizedContent == "" {
		titleizedContent = title
	} else if len(titleizedContent) <= 65 {
		titleizedContent = "\"" + titleizedContent + "\""
	} else {
		titleizedContent = "\"" + titleizedContent[:64] + "…\""
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
		err = NoteTemplate.Render(w, &NotePage{
			OpenGraphPartial: OpenGraphPartial{
				IsTwitter:        style == StyleTwitter,
				Proxy:            "https://" + host + "/njump/proxy?src=",
				Title:            title,
				TwitterTitle:     twitterTitle,
				TitleizedContent: titleizedContent,
				TextImageURL:     textImageURL,
				Image:            data.image,
				Video:            data.video,
				VideoType:        data.videoType,
				Description:      description,
				AuthorLong:       data.authorLong,
			},
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
		err = FileMetadataTemplate.Render(w, &FileMetadataPage{
			OpenGraphPartial: OpenGraphPartial{
				IsTwitter:        style == StyleTwitter,
				Proxy:            "https://" + host + "/njump/proxy?src=",
				TitleizedContent: titleizedContent,
				TwitterTitle:     twitterTitle,
				Title:            title,
				TextImageURL:     textImageURL,
				Video:            data.video,
				VideoType:        data.videoType,
				Image:            data.kind1063Metadata.DisplayImage(),
				Description:      description,
				AuthorLong:       data.authorLong,
			},
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
			Naddr:           data.naddr,
			Kind:            data.event.Kind,
			KindDescription: data.kindDescription,
		})
	}

	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
