package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/pelletier/go-toml"
)

type Event struct {
	Npub         string
	NpubShort    string
	Nevent       string
	Content      string
	CreatedAt    string
	ModifiedAt   string
	ParentNevent string
}

type Data struct {
	typ                 string
	event               *nostr.Event
	npub                string
	npubShort           string
	nevent              string
	naddr               string
	createdAt           string
	modifiedAt          string
	parentNevent        string
	metadata            nostr.ProfileMetadata
	authorRelays        []string
	authorLong          string
	authorShort         string
	renderableLastNotes []*Event
	kindDescription     string
	kindNIP             string
	video               string
	videoType           string
	image               string
	content             string
}

func grabData(ctx context.Context, code string, isProfileSitemap bool) (*Data, error) {
	// code can be a nevent, nprofile, npub or nip05 identifier, in which case we try to fetch the associated event
	event, err := getEvent(ctx, code)
	if err != nil {
		return nil, err
	}

	npub, _ := nip19.EncodePublicKey(event.PubKey)
	nevent, _ := nip19.EncodeEvent(event.ID, []string{}, event.PubKey)
	naddr := ""
	createdAt := time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02 15:04:05")
	modifiedAt := time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00")

	author := event
	var renderableLastNotes []*Event
	parentNevent := ""
	authorRelays := []string{}
	var content string
	var typ string

	switch event.Kind {
	case 0:
		key := ""
		eventsToFetch := 100
		if isProfileSitemap {
			typ = "profile_sitemap"
			key = "lns:" + event.PubKey
			eventsToFetch = 50000
		} else {
			typ = "profile"
			key = "ln:" + event.PubKey
		}

		rawAuthorRelays := []string{}
		ctx, cancel := context.WithTimeout(ctx, time.Second*4)
		rawAuthorRelays = relaysForPubkey(ctx, event.PubKey)
		cancel()
		for _, relay := range rawAuthorRelays {
			for _, excluded := range excludedRelays {
				if strings.Contains(relay, excluded) {
					continue
				}
			}
			if strings.Contains(relay, "/npub1") {
				continue // Skip relays with personalyzed query like filter.nostr.wine
			}
			authorRelays = append(authorRelays, trimProtocol(relay))
		}

		var lastNotes []*nostr.Event

		if ok := cache.GetJSON(key, &lastNotes); !ok {
			ctx, cancel := context.WithTimeout(ctx, time.Second*4)
			lastNotes = getLastNotes(ctx, code, eventsToFetch)
			cancel()
			if len(lastNotes) > 0 {
				cache.SetJSONWithTTL(key, lastNotes, time.Hour*24)
			}
		}

		renderableLastNotes = make([]*Event, len(lastNotes))
		for i, n := range lastNotes {
			nevent, _ := nip19.EncodeEvent(n.ID, []string{}, n.PubKey)
			renderableLastNotes[i] = &Event{
				Nevent:       nevent,
				Content:      n.Content,
				CreatedAt:    time.Unix(int64(n.CreatedAt), 0).Format("2006-01-02 15:04:05"),
				ModifiedAt:   time.Unix(int64(n.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00"),
				ParentNevent: getParentNevent(n),
			}
		}
		if err != nil {
			return nil, err
		}
	case 1, 7, 30023, 30024:
		typ = "note"
		content = event.Content
		parentNevent = getParentNevent(event)
	case 6:
		typ = "note"
		if reposted := event.Tags.GetFirst([]string{"e", ""}); reposted != nil {
			original_nevent, _ := nip19.EncodeEvent((*reposted)[1], []string{}, "")
			content = "Repost of nostr:" + original_nevent
		}
	default:
		if event.Kind >= 30000 && event.Kind < 40000 {
			typ = "address"
			if d := event.Tags.GetFirst([]string{"d", ""}); d != nil {
				naddr, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), []string{})
			}
		} else {
			typ = "other"
		}
	}

	if event.Kind != 0 {
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		author, _ = getEvent(ctx, npub)
		cancel()
	}

	kindDescription := kindNames[event.Kind]
	if kindDescription == "" {
		kindDescription = fmt.Sprintf("Kind %d", event.Kind)
	}
	kindNIP := kindNIPs[event.Kind]

	imageMatch := regexp.MustCompile(`https:\/\/[^ ]*\.(gif|jpe?g|png|webp)`).FindStringSubmatch(event.Content)
	var image string
	if len(imageMatch) > 0 {
		image = imageMatch[0]
	}

	videoMatch := regexp.MustCompile(`https:\/\/[^ ]*\.(mp4|webm)`).FindStringSubmatch(event.Content)
	var video string
	if len(videoMatch) > 0 {
		video = videoMatch[0]
	}

	var videoType string
	if video != "" {
		if strings.HasSuffix(video, "mp4") {
			videoType = "mp4"
		} else {
			videoType = "webm"
		}
	}

	npubShort := npub[:8] + "…" + npub[len(npub)-4:]
	authorLong := npub
	authorShort := npubShort

	var metadata nostr.ProfileMetadata
	if author != nil {
		if err := json.Unmarshal([]byte(author.Content), &metadata); err == nil {
			authorLong = fmt.Sprintf("%s (%s)", metadata.Name, npub)
			authorShort = fmt.Sprintf("%s (%s)", metadata.Name, npubShort)
		}
	}

	return &Data{
		typ:                 typ,
		event:               event,
		npub:                npub,
		npubShort:           npubShort,
		nevent:              nevent,
		naddr:               naddr,
		authorRelays:        authorRelays,
		createdAt:           createdAt,
		modifiedAt:          modifiedAt,
		parentNevent:        parentNevent,
		metadata:            metadata,
		authorLong:          authorLong,
		authorShort:         authorShort,
		renderableLastNotes: renderableLastNotes,
		kindNIP:             kindNIP,
		kindDescription:     kindDescription,
		video:               video,
		videoType:           videoType,
		image:               image,
		content:             content,
	}, nil
}

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
	} else if strings.HasPrefix(code, "npub") {
		code = code[:len(code)-4]
		if strings.HasSuffix(code, ".xml") {
			isProfileSitemap = true
		}
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

	// pretty JSON
	eventJSON, _ := json.MarshalIndent(data.event, "", "  ")

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
		"CanonicalHost":    s.CanonicalHost,
	}

	// if a mapping is not found fallback to raw
	currentTemplate, ok := templateMapping[data.typ]
	if !ok {
		currentTemplate = "other.html"
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

	if err := tmpl.ExecuteTemplate(w, currentTemplate, params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}

	return
}
