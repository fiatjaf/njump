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
)

type Event struct {
	Nevent       string
	Content      string
	CreatedAt    string
	ModifiedAt   string
	ParentNevent string
}

func render(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, ":~", r.Header.Get("user-agent"))
	w.Header().Set("Content-Type", "text/html")

	typ := ""
	code := r.URL.Path[1:]
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
		code = code[:len(code)-4]
		typ = "profile_sitemap"
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

	hostname := r.Header.Get("X-Forwarded-Host")
	style := getPreviewStyle(r)

	// code can be a nevent, nprofile, npub or nip05 identifier, in which case we try to fetch the associated event
	event, err := getEvent(r.Context(), code)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		http.Error(w, "error fetching event: "+err.Error(), 404)
		return
	}

	npub, _ := nip19.EncodePublicKey(event.PubKey)
	nevent, _ := nip19.EncodeEvent(event.ID, []string{}, event.PubKey)
	naddr := ""
	createdAt := time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02 15:04:05")
	modifiedAt := time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00")
	content := ""

	author := event
	var renderableLastNotes []*Event
	parentNevent := ""
	authorRelays := []string{}

	if event.Kind == 0 {
		key := ""
		events_num := 100
		if typ == "profile_sitemap" {
			key = "lns:" + event.PubKey
			events_num = 50000
		} else {
			typ = "profile"
			key = "ln:" + event.PubKey
		}

		rawAuthorRelays := []string{}
		ctx, cancel := context.WithTimeout(r.Context(), time.Second*4)
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
			ctx, cancel := context.WithTimeout(r.Context(), time.Second*4)
			lastNotes = getLastNotes(ctx, code, events_num)
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
			w.Header().Set("Cache-Control", "max-age=60")
			http.Error(w, "error fetching event: "+err.Error(), 404)
			return
		}
	} else {
		if event.Kind == 1 || event.Kind == 7 || event.Kind == 30023 || event.Kind == 30024 {
			typ = "note"
			content = event.Content
			parentNevent = getParentNevent(event)
		} else if event.Kind == 6 {
			typ = "note"
			if reposted := event.Tags.GetFirst([]string{"e", ""}); reposted != nil {
				original_nevent, _ := nip19.EncodeEvent((*reposted)[1], []string{}, "")
				content = "Repost of nostr:" + original_nevent
			}
		} else if event.Kind >= 30000 && event.Kind < 40000 {
			typ = "address"
			if d := event.Tags.GetFirst([]string{"d", ""}); d != nil {
				naddr, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), []string{})
			}
		} else {
			typ = "other"
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
		author, _ = getEvent(ctx, npub)
		cancel()
	}

	kindDescription := kindNames[event.Kind]
	if kindDescription == "" {
		kindDescription = fmt.Sprintf("Kind %d", event.Kind)
	}
	kindNIP := kindNIPS[event.Kind]

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

	var subject string
	var summary string
	for _, tag := range event.Tags {
		if tag[0] == "subject" || tag[0] == "title" {
			subject = tag[1]
		}
		if tag[0] == "summary" {
			summary = tag[1]
		}
	}

	useTextImage := (event.Kind == 1 || event.Kind == 30023) &&
		image == "" && video == "" && len(event.Content) > 120

	if style == "telegram" {
		if event.Kind == 30023 ||
			(event.Kind == 1 && len(event.Content) > 650) {
			typ = "telegram_instant_view"
			useTextImage = false
		}
	}
	if style == "slack" || style == "discord" {
		useTextImage = false
	}

	title := ""
	twitterTitle := title
	if event.Kind == 0 && metadata.Name != "" {
		title = metadata.Name
	} else {
		if event.Kind >= 30000 && event.Kind < 40000 {
			tValue := "~"
			for _, tag := range event.Tags {
				if tag[0] == "t" {
					tValue = tag[1]
					break
				}
			}
			title = fmt.Sprintf("%s: %s", kindNames[event.Kind], tValue)
		} else if kindName, ok := kindNames[event.Kind]; ok {
			title = kindName
		} else {
			title = fmt.Sprintf("kind:%d event", event.Kind)
		}
		if subject != "" {
			title += " (" + subject + ")"
		}
		twitterTitle += " by " + authorShort
		date := event.CreatedAt.Time().UTC().Format("2006-01-02 15:04")
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
		textImageURL = fmt.Sprintf("https://%s/njump/image/%s", hostname, code)
		if subject != "" {
			description = fmt.Sprintf("%s -- %s", subject, seenOnRelays)
		} else {
			description = seenOnRelays
		}
	} else if summary != "" {
		description = summary
	} else {
		description = prettyJsonOrRaw(event.Content)
		if len(description) > 240 {
			description = description[:240]
		}
	}

	// content massaging
	for index, value := range event.Tags {
		placeholderTag := "#[" + fmt.Sprintf("%d", index) + "]"
		nreplace := ""
		if value[0] == "p" {
			nreplace, _ = nip19.EncodePublicKey(value[1])
		} else if value[0] == "e" {
			nreplace, _ = nip19.EncodeEvent(value[1], []string{}, "")
		} else {
			continue
		}
		content = strings.ReplaceAll(content, placeholderTag, "nostr:"+nreplace)
	}
	if event.Kind == 30023 || event.Kind == 30024 {
		content = mdToHTML(content)
	} else {
		content = renderInlineMentions(basicFormatting(html.EscapeString(content)))
	}

	params := map[string]any{
		"createdAt":        createdAt,
		"modifiedAt":       modifiedAt,
		"clients":          generateClientList(code, event),
		"type":             typ,
		"title":            title,
		"twitterTitle":     twitterTitle,
		"npub":             npub,
		"npubShort":        npubShort,
		"nevent":           nevent,
		"naddr":            naddr,
		"metadata":         metadata,
		"authorLong":       authorLong,
		"subject":          subject,
		"description":      description,
		"event":            event,
		"content":          content,
		"titleizedContent": titleize(content),
		"textImageURL":     textImageURL,
		"videoType":        videoType,
		"image":            image,
		"video":            video,
		"proxy":            "https://" + hostname + "/njump/proxy?src=",
		"kindDescription":  kindDescription,
		"kindNIP":          kindNIP,
		"lastNotes":        renderableLastNotes,
		"parentNevent":     parentNevent,
		"authorRelays":     authorRelays,
		"CanonicalHost":    s.CanonicalHost,
	}

	// if a mapping is not found fallback to raw
	currentTemplate, ok := templateMapping[typ]
	if !ok {
		currentTemplate = "other.html"
	}

	if strings.Contains(typ, "profile") && len(renderableLastNotes) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else if !strings.Contains(typ, "profile") && len(content) != 0 {
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
