package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/pelletier/go-toml"
)

func render(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, "#/", r.Header.Get("user-agent"))
	w.Header().Set("Content-Type", "text/html")

	code := r.URL.Path[1:]
	isProfileSitemap := false

	if strings.HasPrefix(code, "e/") {
		code, _ = nip19.EncodeEvent(code[2:], []string{}, "")
	} else if strings.HasPrefix(code, "p/") {
		if urlSuffixMatcher.MatchString(code) {
			// it's a nip05
			code = code[2:]
		} else {
			// it's a hex pubkey
			code, _ = nip19.EncodePublicKey(code[2:])
		}
	} else if strings.HasPrefix(code, "r/") {
		hostname := code[2:]
		if strings.HasPrefix(hostname, "wss:/") || strings.HasPrefix(hostname, "ws:/") {
			hostname = trimProtocol(hostname)
			http.Redirect(w, r, "/r/"+hostname, http.StatusFound)
		} else {
			renderRelayPage(w, r)
		}
		return
	} else if strings.HasPrefix(code, "nostr:") {
		http.Redirect(w, r, "/"+code[6:], http.StatusFound)
	} else if strings.HasPrefix(code, "npub") && strings.HasSuffix(code, ".xml") {
		isProfileSitemap = true
		code = code[:len(code)-4]
	}

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

	if code == "" {
		renderHomepage(w, r)
		return
	}

	host := r.Header.Get("X-Forwarded-Host")
	style := getPreviewStyle(r)

	data, err := grabData(r.Context(), code, isProfileSitemap)
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
			data.typ = "telegram_instant_view"
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
	//  event.seenOn && event.seenOn.length > 0
	//    ? `seen on [ ${event.seenOn.join(' ')} ]`
	//    : ''

	textImageURL := ""
	description := ""
	if useTextImage {
		textImageURL = fmt.Sprintf("https://%s/njump/image/%s", host, code)
		if subject != "" {
			description = fmt.Sprintf("%s -- %s", subject, seenOnRelays)
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
			titleizedContent = strings.Replace(
				strings.Replace(description, "\r\n", " ", -1),
				"\n", " ", -1,
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
		data.content = mdToHTML(data.content, data.typ == "telegram_instant_view")
	} else {
		// first we run basicFormatting, which turns URLs into their appropriate HTML tags
		data.content = basicFormatting(html.EscapeString(data.content), true, false)
		// then we render quotes as HTML, which will also apply basicFormatting to all the internal quotes
		data.content = renderQuotesAsHTML(r.Context(), data.content, data.typ == "telegram_instant_view")
		// we must do this because inside <blockquotes> we must treat <img>s different when telegram_instant_view
	}

	if data.typ == "telegram_instant_view" {
		w.Header().Set("Cache-Control", "no-cache")
	} else if strings.Contains(data.typ, "profile") && len(data.renderableLastNotes) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else if !strings.Contains(data.typ, "profile") && len(data.content) != 0 {
		w.Header().Set("Cache-Control", "max-age=604800")
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	// pretty JSON
	eventJSON, _ := json.MarshalIndent(data.event, "", "  ")

	// oembed discovery
	oembed := ""
	if data.typ == "note" {
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

	// template
	params := map[string]any{
		"createdAt":        data.createdAt,
		"modifiedAt":       data.modifiedAt,
		"clients":          generateClientList(code, data.event),
		"type":             data.typ,
		"title":            title,
		"titleizedContent": titleizedContent,
		"twitterTitle":     twitterTitle,
		"npub":             data.npub,
		"npubShort":        data.npubShort,
		"nevent":           data.nevent,
		"naddr":            data.naddr,
		"metadata":         data.metadata,
		"authorLong":       data.authorLong,
		"subject":          subject,
		"description":      description,
		"summary":          summary,
		"event":            data.event,
		"eventJSON":        string(eventJSON),
		"content":          data.content,
		"textImageURL":     textImageURL,
		"videoType":        data.videoType,
		"image":            data.image,
		"video":            data.video,
		"proxy":            "https://" + host + "/njump/proxy?src=",
		"kindDescription":  data.kindDescription,
		"kindNIP":          data.kindNIP,
		"lastNotes":        data.renderableLastNotes,
		"parentNevent":     data.parentNevent,
		"authorRelays":     data.authorRelays,
		"oembed":           oembed,
		"s":                s,
	}

	// if a mapping is not found fallback to raw
	currentTemplate, ok := templateMapping[data.typ]
	if !ok {
		currentTemplate = "other.html"
	}

	if err := tmpl.ExecuteTemplate(w, currentTemplate, params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}

	return
}
