package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
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
			http.Error(w, "error fetching event: "+err.Error(), 404)
			return
		}
		redirectNevent, _ := nip19.EncodeEvent(redirectHex.(string), []string{}, "")
		http.Redirect(w, r, "/"+redirectNevent, http.StatusFound)
	}

	host := r.Header.Get("X-Forwarded-Host")
	style := getPreviewStyle(r)

	data, err := grabData(r.Context(), code, false)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		http.Error(w, "error fetching event: "+err.Error(), 404)
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

	if style == "telegram" || r.URL.Query().Get("tgiv") == "true" {
		// do telegram instant preview (only works on telegram mobile apps, not desktop)
		if data.event.Kind == 30023 || // do it for longform articles
			(data.event.Kind == 1 && len(data.event.Content) > 650) || // or very long notes
			// or shorter notes that should be using text-to-image stuff but are not because they have video or images
			(data.event.Kind == 1 && len(data.event.Content) > 133 && !useTextImage) {
			data.templateId = TelegramInstantView
			useTextImage = false
		}
	} else if style == "slack" || style == "discord" {
		useTextImage = false
	}

	title := ""
	titleizedContent := ""
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
			title = kindName
		} else {
			title = fmt.Sprintf("kind:%d event", data.event.Kind)
		}
		if subject != "" {
			title += " (" + subject + ")"
		}
		twitterTitle += " by " + data.authorShort
		date := data.event.CreatedAt.Time().UTC().Format("2006-01-02 15:04")
		title += " at " + date
		twitterTitle += " at " + date
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
			res := replaceUserReferencesWithNames(r.Context(), []string{data.event.Content})
			description = res[0]
			if len(description) > 240 {
				description = description[:240]
			}
			titleizedContent = strings.TrimSpace(
				strings.Replace(
					strings.Replace(description, "\r\n", " ", -1),
					"\n", " ", -1,
				),
			)
			if len(titleizedContent) <= 65 {
				titleizedContent = "\"" + titleizedContent + "\""
			} else {
				titleizedContent = "\"" + titleizedContent[:64] + "…\""
			}
		}
	}
	if titleizedContent == "" {
		titleizedContent = title
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

	// pretty JSON
	eventJSON, _ := json.MarshalIndent(data.event, "", "  ")

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

	// migrating to templ
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
			HeadCommonPartial: HeadCommonPartial{IsProfile: false},
			DetailsPartial: DetailsPartial{
				HideDetails:     true,
				CreatedAt:       data.createdAt,
				KindDescription: data.kindDescription,
				KindNIP:         data.kindNIP,
				EventJSON:       string(eventJSON),
				Kind:            data.event.Kind,
			},
			ClientsPartial: ClientsPartial{
				Clients: generateClientList(code, data.event),
			},

			AuthorLong:       data.authorLong,
			Content:          template.HTML(data.content),
			CreatedAt:        data.createdAt,
			Description:      description,
			Image:            data.image,
			Metadata:         data.metadata,
			Nevent:           data.nevent,
			Npub:             data.npub,
			NpubShort:        data.npubShort,
			Oembed:           oembed,
			ParentLink:       data.parentLink,
			Proxy:            "https://" + host + "/njump/proxy?src=",
			SeenOn:           data.relays,
			Style:            style,
			Subject:          subject,
			TextImageURL:     textImageURL,
			Title:            title,
			TitleizedContent: titleizedContent,
			TwitterTitle:     twitterTitle,
			Video:            data.video,
			VideoType:        data.videoType,
		})
	case Other:
		err = OtherTemplate.Render(w, &OtherPage{
			HeadCommonPartial: HeadCommonPartial{IsProfile: false},
			DetailsPartial: DetailsPartial{
				HideDetails:     false,
				CreatedAt:       data.createdAt,
				KindDescription: data.kindDescription,
				KindNIP:         data.kindNIP,
				EventJSON:       string(eventJSON),
				Kind:            data.event.Kind,
			},

			IsParameterizedReplaceable: data.event.Kind >= 30000 && data.event.Kind < 40000,
			Naddr:                      data.naddr,
			Npub:                       data.npub,
			Kind:                       data.event.Kind,
			KindDescription:            data.kindDescription,
		})
	}

	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
